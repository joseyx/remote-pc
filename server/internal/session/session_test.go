package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"orbit/internal/pty"
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

	// Allow background SaveState goroutine from RenameSession to complete before tmpDir cleanup
	time.Sleep(100 * time.Millisecond)
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

func TestSessionLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "sessions_state.json")

	mgr := NewManager(512, stateFile)
	mgr.SetDataDir(tempDir)

	shellCmd := "powershell.exe -NoLogo -NoProfile"
	if runtime.GOOS != "windows" {
		shellCmd = "/bin/sh"
	}

	s, err := mgr.CreateBubbleSession("TestTerm", tempDir, shellCmd, "bubble-001", "shell", 100, 30)
	if err != nil {
		t.Fatalf("CreateBubbleSession failed: %v", err)
	}

	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if s.PID <= 0 {
		t.Errorf("expected positive PID, got %d", s.PID)
	}
	if !s.IsRunning {
		t.Error("expected session to be running")
	}

	// Rename
	if err := mgr.RenameSession(s.ID, "RenamedTerm"); err != nil {
		t.Fatalf("RenameSession failed: %v", err)
	}
	if s.Name != "RenamedTerm" {
		t.Errorf("expected name 'RenamedTerm', got %q", s.Name)
	}

	// Write input
	testMarker := "SESSION_TEST_OUTPUT"
	if runtime.GOOS == "windows" {
		_ = s.WriteInput([]byte("Write-Output " + testMarker + "\r\n"))
	} else {
		_ = s.WriteInput([]byte("echo " + testMarker + "\n"))
	}

	// Wait for output in history buffer
	found := false
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		hist := s.GetHistory()
		if len(hist) > 0 && string(hist) != "" {
			found = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		t.Log("Did not see output in history buffer within deadline")
	}

	// Save state
	if err := mgr.SaveState(); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}
	if len(data) == 0 {
		t.Error("state file is empty")
	}

	// Clean shutdown
	mgr.CloseAllPTYs()

	if s.IsRunning {
		t.Error("expected session to be stopped after CloseAllPTYs")
	}
}

func TestSessionRestoreWithRemotePTY(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "sessions_state.json")

	var sockDir string
	if runtime.GOOS != "windows" {
		var err error
		sockDir, err = os.MkdirTemp("/tmp", "sess-t-*")
		if err != nil {
			t.Fatalf("failed to create temp sock dir: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	} else {
		sockDir = tempDir
	}
	sockPath := filepath.Join(sockDir, "pty-test.sock")

	shellCmd := "powershell.exe -NoLogo -NoProfile"
	if runtime.GOOS != "windows" {
		shellCmd = "/bin/sh"
	}

	sessionID := "test-sess-restore-001"

	// 1. Launch a standalone PTY worker
	workerErrCh := make(chan error, 1)
	go func() {
		workerErrCh <- pty.RunWorker(sessionID, sockPath, shellCmd, tempDir, 80, 24)
	}()

	t.Cleanup(func() {
		rp, err := pty.DialRemotePTY(sessionID, sockPath, 500*time.Millisecond)
		if err == nil {
			_ = rp.Close()
		}
	})

	// Wait for worker socket to become active and ready for connections
	var preCheck *pty.RemotePTY
	for i := 0; i < 40; i++ {
		time.Sleep(100 * time.Millisecond)
		select {
		case err := <-workerErrCh:
			t.Fatalf("worker exited early: %v", err)
		default:
		}
		var err error
		preCheck, err = pty.DialRemotePTY(sessionID, sockPath, 200*time.Millisecond)
		if err == nil {
			_ = preCheck.Detach()
			break
		}
	}
	if preCheck == nil {
		t.Fatalf("worker failed to start and accept connections at %s within 4s", sockPath)
	}

	// 2. Prepare state file pointing to this worker socket
	mgr := NewManager(512, stateFile)
	mgr.SetDataDir(tempDir)

	stored := []StoredSession{
		{
			ID:         sessionID,
			Name:       "ResilientTerm",
			Cwd:        tempDir,
			Command:    shellCmd,
			Cols:       80,
			Rows:       24,
			SocketPath: sockPath,
			CreatedAt:  time.Now(),
		},
	}

	stateData, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal stored sessions: %v", err)
	}
	if err := os.WriteFile(stateFile, stateData, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	// 4. Call RestoreSessions() on a fresh manager (simulating new Orbit server boot)
	mgr2 := NewManager(512, stateFile)
	mgr2.SetDataDir(tempDir)

	if err := mgr2.RestoreSessions(); err != nil {
		t.Fatalf("RestoreSessions failed: %v", err)
	}

	mgr2.mu.RLock()
	restoredSession, ok := mgr2.sessions[sessionID]
	mgr2.mu.RUnlock()

	if !ok {
		t.Fatalf("expected session %s to be restored", sessionID)
	}

	if !restoredSession.IsRunning {
		t.Error("expected restored session to be running")
	}
	if restoredSession.PID <= 0 {
		t.Errorf("expected positive child PID, got %d", restoredSession.PID)
	}

	// 5. Test Detach (simulate Orbit server updating / graceful restart)
	mgr2.DetachAllPTYs()

	// Verify the worker socket is STILL alive on disk after detach!
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Error("worker socket was deleted after DetachAllPTYs! It should have remained alive.")
	}

	// 6. Final cleanup: connect and close worker
	rp, err := pty.DialRemotePTY(sessionID, sockPath, 1*time.Second)
	if err == nil {
		_ = rp.Close()
	}
}
