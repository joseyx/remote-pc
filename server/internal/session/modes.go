package session

import (
	"bytes"
	"strconv"
)

// Terminal mode tracking.
//
// The history replayed to a newly attached client is only the tail of the output, so the
// sequences an application sent at startup to set up the terminal (bracketed paste, hidden
// cursor, alternate screen, mouse reporting...) are usually no longer in it. A fresh xterm.js
// fed only that tail would end up in the wrong modes: without bracketed paste, for example, a
// multi-line paste reaches the application as plain keystrokes and every newline submits.
// The ring buffer therefore tracks those modes across all the output it ever saw, and the
// replay is prefixed with the sequences that restore them.

// trackedModes lists the DEC private modes (CSI ? Pm h / CSI ? Pm l) restored on replay, in
// the order they are emitted. The alternate-screen modes go first so the rest of the replay
// is drawn on the right buffer. Synchronized output (2026) is deliberately left out: it is
// transient, and replaying it "on" would freeze rendering until the application ends it.
var trackedModes = []int{
	1049, 1047, 47, // alternate screen buffer
	1,                                        // application cursor keys (DECCKM)
	25,                                       // cursor visible (DECTCEM)
	1000, 1002, 1003, 1005, 1006, 1015, 1016, // mouse reporting and encodings
	1004, // focus in/out reporting
	2004, // bracketed paste
}

// modeKeypad is a pseudo mode id for the application keypad, toggled by ESC = / ESC >.
const modeKeypad = -1

// maxCSIParamLen bounds the parameter bytes collected for a single CSI sequence.
const maxCSIParamLen = 64

var trackedModeSet = func() map[int]bool {
	set := map[int]bool{modeKeypad: true}
	for _, m := range trackedModes {
		set[m] = true
	}
	return set
}()

// modeDefault reports the state a mode has in a freshly reset terminal.
func modeDefault(mode int) bool {
	return mode == 25 // only the cursor starts enabled
}

// modeSink receives the mode changes found by escParser.
type modeSink interface {
	setMode(mode int, on bool)
	resetModes()
}

const (
	escGround = iota
	escEscape
	escCSI
)

// escParser is a minimal, streaming VT parser that only recognizes DEC private mode set/reset
// (CSI ? Pm h / l), the application keypad (ESC = / ESC >) and full reset (ESC c). Its state
// survives between feed calls, so sequences split across PTY reads are still recognized.
type escParser struct {
	state    int
	params   []byte
	overflow bool
}

// feed scans p and reports every mode change it completes to sink.
func (ep *escParser) feed(p []byte, sink modeSink) {
	for i := 0; i < len(p); i++ {
		b := p[i]
		switch ep.state {
		case escGround:
			j := bytes.IndexByte(p[i:], 0x1b)
			if j < 0 {
				return
			}
			i += j
			ep.state = escEscape
		case escEscape:
			switch b {
			case '[':
				ep.state = escCSI
				ep.params = ep.params[:0]
				ep.overflow = false
			case 'c':
				ep.state = escGround
				sink.resetModes()
			case '=':
				ep.state = escGround
				sink.setMode(modeKeypad, true)
			case '>':
				ep.state = escGround
				sink.setMode(modeKeypad, false)
			case 0x1b:
				// ESC ESC: the second one starts a new sequence.
			default:
				ep.state = escGround
			}
		case escCSI:
			switch {
			case b >= 0x40 && b <= 0x7e: // final byte
				ep.state = escGround
				if !ep.overflow && (b == 'h' || b == 'l') {
					ep.dispatchDECSET(b == 'h', sink)
				}
			case b >= 0x20 && b <= 0x3f: // parameter or intermediate byte
				if len(ep.params) < maxCSIParamLen {
					ep.params = append(ep.params, b)
				} else {
					ep.overflow = true
				}
			case b == 0x1b:
				ep.state = escEscape
			case b == 0x18 || b == 0x1a: // CAN / SUB abort the sequence
				ep.state = escGround
			case b < 0x20:
				// Other C0 controls are executed without aborting the sequence.
			default:
				ep.state = escGround
			}
		}
	}
}

// dispatchDECSET reports the modes of a completed "CSI ? Pm h|l" sequence. Anything else that
// ends in h or l (ANSI modes without "?", sequences with intermediates) is ignored.
func (ep *escParser) dispatchDECSET(on bool, sink modeSink) {
	if len(ep.params) < 2 || ep.params[0] != '?' {
		return
	}
	for _, c := range ep.params[1:] {
		if (c < '0' || c > '9') && c != ';' {
			return
		}
	}
	for _, field := range bytes.Split(ep.params[1:], []byte{';'}) {
		if len(field) == 0 || len(field) > 5 {
			continue
		}
		if mode, err := strconv.Atoi(string(field)); err == nil {
			sink.setMode(mode, on)
		}
	}
}

// modeTracker keeps the current value of every tracked mode the output has changed.
// It is not safe for concurrent use; ringBuffer guards it with its own mutex.
type modeTracker struct {
	parser escParser
	state  map[int]bool
}

// feed updates the tracked modes with a chunk of terminal output.
func (mt *modeTracker) feed(p []byte) {
	mt.parser.feed(p, mt)
}

func (mt *modeTracker) setMode(mode int, on bool) {
	if !trackedModeSet[mode] {
		return
	}
	if mt.state == nil {
		mt.state = make(map[int]bool)
	}
	mt.state[mode] = on
}

func (mt *modeTracker) resetModes() {
	mt.state = nil
}

// snapshot returns a copy of the current mode values.
func (mt *modeTracker) snapshot() map[int]bool {
	out := make(map[int]bool, len(mt.state))
	for m, on := range mt.state {
		out[m] = on
	}
	return out
}

// modesBefore records, for each tracked mode, the value it must have had right before a chunk
// of output: the opposite of the first change the chunk makes to it, or the default when the
// chunk resets the terminal before touching it.
type modesBefore struct {
	before map[int]bool
}

func (mb *modesBefore) setMode(mode int, on bool) {
	if !trackedModeSet[mode] {
		return
	}
	if _, seen := mb.before[mode]; !seen {
		mb.before[mode] = !on
	}
}

func (mb *modesBefore) resetModes() {
	for m := range trackedModeSet {
		if _, seen := mb.before[m]; !seen {
			mb.before[m] = modeDefault(m)
		}
	}
}

// modeReplayPrefix returns the sequences that put a freshly reset terminal into the modes that
// were active right before tail, given the session's current modes. Replaying the prefix and
// then tail leaves the terminal in the current modes: a mode tail never touches is restored to
// its current value, and one tail changes is set to the value tail itself expects to find.
func modeReplayPrefix(current map[int]bool, tail []byte) []byte {
	scan := modesBefore{before: make(map[int]bool)}
	var parser escParser
	parser.feed(tail, &scan)

	var out []byte
	for _, m := range trackedModes {
		on, known := scan.before[m]
		if !known {
			on, known = current[m]
		}
		if !known || on == modeDefault(m) {
			continue
		}
		out = append(out, "\x1b[?"...)
		out = strconv.AppendInt(out, int64(m), 10)
		if on {
			out = append(out, 'h')
		} else {
			out = append(out, 'l')
		}
	}

	keypad, known := scan.before[modeKeypad]
	if !known {
		keypad = current[modeKeypad]
	}
	if keypad {
		out = append(out, "\x1b="...)
	}
	return out
}
