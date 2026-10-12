//go:build !windows

package mcp

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

func startBackgroundDaemon(p string) error {
	cmd := exec.Command(p, "-no-open")
	cmd.Dir = filepath.Dir(p)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	return cmd.Start()
}
