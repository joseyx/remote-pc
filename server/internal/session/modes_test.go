package session

import (
	"bytes"
	"strings"
	"testing"
)

// filler writes about n bytes of plain output lines.
func filler(rb *ringBuffer, n int) {
	line := []byte(strings.Repeat("x", 99) + "\n")
	for written := 0; written < n; written += len(line) {
		_, _ = rb.Write(line)
	}
}

func TestModeTracker_SplitAndCombinedSequences(t *testing.T) {
	var mt modeTracker
	for _, chunk := range []string{
		"hello \x1b[?20", "04h world", // split mid-sequence
		"\x1b", "[?1049;1000h", // split after ESC, two modes in one sequence
		"\x1b[?1000l",        // turned off again
		"\x1b[?2026h",        // synchronized output: not tracked
		"\x1b[4h",            // ANSI insert mode (no '?'): not a DEC mode
		"\x1b[?25l", "\x1b=", // hidden cursor, application keypad
	} {
		mt.feed([]byte(chunk))
	}

	got := mt.snapshot()
	want := map[int]bool{2004: true, 1049: true, 1000: false, 25: false, modeKeypad: true}
	if len(got) != len(want) {
		t.Fatalf("tracked modes = %v, want %v", got, want)
	}
	for m, on := range want {
		if got[m] != on {
			t.Fatalf("mode %d = %v, want %v (all: %v)", m, got[m], on, got)
		}
	}
}

func TestModeTracker_FullResetClearsModes(t *testing.T) {
	var mt modeTracker
	mt.feed([]byte("\x1b[?2004h\x1b[?1049h"))
	mt.feed([]byte("\x1bc"))
	if got := mt.snapshot(); len(got) != 0 {
		t.Fatalf("RIS should reset every mode, got %v", got)
	}
}

func TestRingBuffer_TailRestoresModesSetBeforeTheCut(t *testing.T) {
	rb := newRingBuffer(2 * 1024 * 1024)
	// What an agent CLI typically sends at startup, then a lot of output.
	_, _ = rb.Write([]byte("\x1b[?2004h\x1b[?25l\x1b[?1000h\x1b[?1000l"))
	filler(rb, 1500*1024)
	_, _ = rb.Write([]byte("LAST-LINE\n"))

	tail := rb.Tail(subscribeReplayMaxBytes)
	want := "\x1b[0m\x1b[?25l\x1b[?2004h"
	if !bytes.HasPrefix(tail, []byte(want)) {
		t.Fatalf("tail should start with %q, got %q", want, tail[:min(len(tail), 40)])
	}
	if bytes.Contains(tail, []byte("\x1b[?1000h")) {
		t.Fatalf("a mode turned off again must not be restored")
	}
	if !bytes.HasSuffix(tail, []byte("LAST-LINE\n")) {
		t.Fatalf("tail must end with the most recent output")
	}
	if len(tail) > subscribeReplayMaxBytes+64 {
		t.Fatalf("prefix should stay small, tail is %d bytes", len(tail))
	}
}

func TestRingBuffer_TailUsesTheModeExpectedByItsOwnToggles(t *testing.T) {
	// The app entered the alternate screen long ago and leaves it inside the tail:
	// the replay must start on the alternate screen so the tail's own "leave" lands correctly.
	rb := newRingBuffer(2 * 1024 * 1024)
	_, _ = rb.Write([]byte("\x1b[?1049h"))
	filler(rb, 1024*1024)
	_, _ = rb.Write([]byte("\x1b[?1049lback on the main screen\n"))

	tail := rb.Tail(64 * 1024)
	if !bytes.HasPrefix(tail, []byte("\x1b[0m\x1b[?1049h")) {
		t.Fatalf("replay should re-enter the alternate screen first, got %q", tail[:min(len(tail), 40)])
	}

	// A mode the tail itself enables needs no prefix.
	rb2 := newRingBuffer(2 * 1024 * 1024)
	filler(rb2, 1024*1024)
	_, _ = rb2.Write([]byte("\x1b[?2004hprompt\n"))
	tail2 := rb2.Tail(64 * 1024)
	if n := bytes.Count(tail2, []byte("\x1b[?2004h")); n != 1 {
		t.Fatalf("expected the tail's own bracketed-paste enable only, found %d", n)
	}
}

func TestRingBuffer_TailUntouchedWhenHistoryIsComplete(t *testing.T) {
	rb := newRingBuffer(1024 * 1024)
	data := []byte("\x1b[?2004h\x1b[?25lhello\n")
	_, _ = rb.Write(data)
	if tail := rb.Tail(subscribeReplayMaxBytes); !bytes.Equal(tail, data) {
		t.Fatalf("complete history must be replayed as is, got %q", tail)
	}
}

func TestRingBuffer_ModesSurviveTrimAndStateRoundTrip(t *testing.T) {
	// The ring itself wrapped (pre-existing loss even without the replay cap).
	rb := newRingBuffer(64 * 1024)
	_, _ = rb.Write([]byte("\x1b[?2004h"))
	filler(rb, 256*1024)

	saved := rb.Tail(64 * 1024) // what SaveState persists
	if !bytes.Contains(saved, []byte("\x1b[?2004h")) {
		t.Fatalf("persisted tail must carry the active modes")
	}

	// RestoreSessions writes the persisted tail into a new ring buffer.
	restored := newRingBuffer(2 * 1024 * 1024)
	_, _ = restored.Write(saved)
	if !restored.modes.snapshot()[2004] {
		t.Fatalf("restored buffer should know bracketed paste is on")
	}
	if n := bytes.Count(restored.Tail(subscribeReplayMaxBytes), []byte("\x1b[?2004h")); n != 1 {
		t.Fatalf("replay after restore should enable bracketed paste exactly once, found %d", n)
	}
}
