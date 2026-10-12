package pty

import (
	"runtime"
	"testing"
)

func TestPTYSpawn(t *testing.T) {
	cmd := "cmd.exe /c echo PTY_OK"
	if runtime.GOOS != "windows" {
		cmd = "echo PTY_OK"
	}
	p, err := Start(cmd, "", 80, 25)
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer p.Close()

	buf := make([]byte, 1024)
	n, err := p.Read(buf)
	if err != nil && n == 0 {
		t.Fatalf("Failed to read from PTY: %v", err)
	}
	output := string(buf[:n])
	if n == 0 {
		t.Fatalf("Expected output from PTY, got 0 bytes")
	}
	t.Logf("Read %d bytes: %q", n, output)
}
