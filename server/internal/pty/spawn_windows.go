//go:build windows

package pty

import (
	"fmt"
	"os/exec"
	"syscall"
)

// SpawnWorker launches a detached pty-host worker on Windows
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
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}

	return cmd.Start()
}
