//go:build !windows

package pty

import (
	"fmt"
	"os/exec"
	"syscall"
)

// SpawnWorker launches a detached pty-host worker on Unix
func SpawnWorker(exePath, sessionID, socketPath, cmdStr, cwd string, cols, rows uint16) error {
	args := []string{
		"pty-host",
		"--session-id", sessionID,
		"--socket", socketPath,
		"--dir", cwd,
		"--cols", fmt.Sprint(cols),
		"--rows", fmt.Sprint(rows),
		"--cmd", cmdStr,
	}

	cmd := exec.Command(exePath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	return cmd.Start()
}
