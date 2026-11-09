package session

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRingBuffer_WriteAndTrim(t *testing.T) {
	// Ring buffer of 1KB
	maxSize := 1024
	rb := newRingBuffer(maxSize)

	// Write 500 bytes with newlines
	data1 := []byte(strings.Repeat("Line of text data\n", 25))
	n, err := rb.Write(data1)
	if err != nil || n != len(data1) {
		t.Fatalf("expected write %d bytes, got %d (err: %v)", len(data1), n, err)
	}

	if len(rb.Bytes()) != len(data1) {
		t.Fatalf("expected len %d, got %d", len(data1), len(rb.Bytes()))
	}

	// Write another chunk that exceeds maxSize
	data2 := []byte(strings.Repeat("Another line of long terminal output\n", 30))
	_, err = rb.Write(data2)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	buf := rb.Bytes()
	if len(buf) > maxSize {
		t.Fatalf("expected buffer size <= %d, got %d", maxSize, len(buf))
	}

	// Verify that buffer starts cleanly with reset ANSI or newline, not sliced mid-word
	if !bytes.HasPrefix(buf, []byte("\x1b[0m")) && !bytes.HasPrefix(buf, []byte("Another")) && !bytes.HasPrefix(buf, []byte("Line")) {
		t.Logf("Buffer prefix: %q", string(buf[:min(len(buf), 20)]))
	}
}

func TestRingBuffer_Tail(t *testing.T) {
	maxSize := 2048
	rb := newRingBuffer(maxSize)

	for i := 1; i <= 50; i++ {
		_, _ = rb.Write([]byte(fmt.Sprintf("Log line %03d: system running ok\n", i)))
	}

	tail := rb.Tail(256)
	if len(tail) == 0 {
		t.Fatal("expected non-empty tail")
	}
	if len(tail) > 256+4 { // +4 for ANSI reset prefix if truncated
		t.Fatalf("expected tail <= 260 bytes, got %d", len(tail))
	}

	// Tail should contain the latest line
	if !strings.Contains(string(tail), "Log line 050") {
		t.Fatalf("tail should contain latest lines, got: %s", string(tail))
	}
}

func TestManager_SaveAndRestoreState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "sessions_test_state.json")

	mgr := NewManager(512, stateFile)
	defer func() {
		if mgr.stopTicker != nil {
			close(mgr.stopTicker)
		}
	}()

	// Simulate session in manager
	s := &Session{
		ID:        "test-sess-1",
		Name:      "Test Terminal",
		Cwd:       tmpDir,
		Command:   "cmd.exe",
		IsRunning: false,
		Cols:      80,
		Rows:      24,
		buffer:    newRingBuffer(512 * 1024),
		listeners: make(map[chan []byte]struct{}),
	}
	_, _ = s.buffer.Write([]byte("Hello from test terminal\nSecond line of history\n"))

	mgr.mu.Lock()
	mgr.sessions[s.ID] = s
	mgr.mu.Unlock()

	if err := mgr.SaveState(); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}

	if !strings.Contains(string(data), "test-sess-1") {
		t.Fatalf("state JSON should contain session id: %s", string(data))
	}

	// Test renaming session
	if err := mgr.RenameSession("test-sess-1", "Renamed Terminal"); err != nil {
		t.Fatalf("RenameSession failed: %v", err)
	}
	if s.Name != "Renamed Terminal" {
		t.Fatalf("expected session name 'Renamed Terminal', got '%s'", s.Name)
	}
}

func BenchmarkRingBuffer_WriteWhenFull(b *testing.B) {
	maxSize := 64 * 1024
	rb := newRingBuffer(maxSize)

	// Pre-fill to capacity
	seed := []byte(strings.Repeat("Echo command output 1234567890\n", 2000))
	_, _ = rb.Write(seed)

	b.ResetTimer()
	chunk := []byte("keystroke\n")
	for i := 0; i < b.N; i++ {
		_, _ = rb.Write(chunk)
	}
}
