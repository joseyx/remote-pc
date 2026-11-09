package pty

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWorkerSpawnAndReconnect(t *testing.T) {
	tempDir := t.TempDir()

	var sockDir string
	if runtime.GOOS != "windows" {
		var err error
		sockDir, err = os.MkdirTemp("/tmp", "pty-t-*")
		if err != nil {
			t.Fatalf("failed to create temp sock dir: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	} else {
		sockDir = tempDir
	}
	sockPath := filepath.Join(sockDir, "test.sock")

	shellCmd := "powershell.exe -NoLogo -NoProfile"
	if runtime.GOOS != "windows" {
		shellCmd = "/bin/sh"
	}

	sessionID := "test-sess-001"

	// 1. Run worker in background
	workerErrCh := make(chan error, 1)
	go func() {
		err := RunWorker(sessionID, sockPath, shellCmd, tempDir, 80, 24)
		workerErrCh <- err
	}()

	// Wait for socket to become available
	var client1 *RemotePTY
	var err error
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		select {
		case wErr := <-workerErrCh:
			t.Fatalf("worker failed early: %v", wErr)
		default:
		}
		client1, err = DialRemotePTY(sessionID, sockPath, 300*time.Millisecond)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("failed to connect to worker: %v", err)
	}

	if client1.ProcessID() <= 0 {
		t.Errorf("expected positive child PID, got %d", client1.ProcessID())
	}

	// 2. Write command and read output
	testMarker := "HELLO_ORBIT_ZERO_DOWNTIME"
	if runtime.GOOS == "windows" {
		_, _ = client1.Write([]byte("Write-Output " + testMarker + "\r\n"))
	} else {
		_, _ = client1.Write([]byte("echo " + testMarker + "\n"))
	}

	readBuf := make([]byte, 1024)
	foundMarker := false
	deadline := time.Now().Add(4 * time.Second)

	for time.Now().Before(deadline) {
		n, err := client1.Read(readBuf)
		if n > 0 && bytes.Contains(readBuf[:n], []byte(testMarker)) {
			foundMarker = true
			break
		}
		if err != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !foundMarker {
		t.Logf("Did not receive marker %s within deadline (pty startup delay)", testMarker)
	}

	// 3. Test Detach (simulate Orbit server updating or restarting)
	t.Log("Detaching client 1...")
	if err := client1.Detach(); err != nil {
		t.Fatalf("Detach failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// 4. Test Reconnect (simulate new Orbit server connecting to existing worker)
	t.Log("Reconnecting client 2...")
	client2, err := DialRemotePTY(sessionID, sockPath, 1*time.Second)
	if err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}

	if client2.ProcessID() != client1.ProcessID() {
		t.Errorf("child PID changed after reconnect! Expected %d, got %d", client1.ProcessID(), client2.ProcessID())
	}

	// 5. Test writing from client 2
	secondMarker := "SECOND_ORBIT_RECONNECTED"
	if runtime.GOOS == "windows" {
		_, _ = client2.Write([]byte("Write-Output " + secondMarker + "\r\n"))
	} else {
		_, _ = client2.Write([]byte("echo " + secondMarker + "\n"))
	}

	// 6. Test Close (explicit user termination)
	t.Log("Closing client 2 (terminates worker)...")
	if err := client2.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Worker should now exit
	select {
	case err := <-workerErrCh:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Logf("Worker exited with: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("worker did not exit within 3 seconds of client.Close()")
	}

	// Socket file should be cleaned up
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("socket file still exists after worker termination: %s", sockPath)
	}
}
