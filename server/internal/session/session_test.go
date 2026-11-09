package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"orbit/internal/pty"
)

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
