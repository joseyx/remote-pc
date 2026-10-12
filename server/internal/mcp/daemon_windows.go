//go:build windows

package mcp

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

func startBackgroundDaemon(p string) error {
	cmd := exec.Command("cmd", "/c", "start", "", "/b", p, "-no-open")
	cmd.Dir = filepath.Dir(p)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd.Start()
}
