//go:build !windows
// +build !windows

package pty

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

type UnixPTY struct {
	ptyFile *os.File
	cmd     *exec.Cmd
	mu      sync.Mutex
	closed  bool
}

func Start(command string, dir string, cols, rows uint16) (PTY, error) {
	if cols == 0 {
		cols = 120
	}
	if rows == 0 {
		rows = 30
	}

	command = strings.TrimSpace(command)
	if command == "" {
		command = "/bin/bash"
		if _, err := os.Stat("/bin/bash"); os.IsNotExist(err) {
			command = "/bin/sh"
		}
	}

	var cmd *exec.Cmd
	if strings.Contains(command, " ") {
		cmd = exec.Command("/bin/sh", "-c", command)
	} else {
		cmd = exec.Command(command)
	}

	if dir != "" {
		cmd.Dir = dir
	}

	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	winSize := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}

	ptyFile, err := pty.StartWithSize(cmd, winSize)
	if err != nil {
		return nil, fmt.Errorf("failed to start unix pty: %w", err)
	}

	return &UnixPTY{
		ptyFile: ptyFile,
		cmd:     cmd,
	}, nil
}

func (p *UnixPTY) Read(b []byte) (int, error) {
	return p.ptyFile.Read(b)
}

func (p *UnixPTY) Write(b []byte) (int, error) {
	return p.ptyFile.Write(b)
}

func (p *UnixPTY) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	_ = p.ptyFile.Close()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGHUP)
		_ = p.cmd.Process.Kill()
	}
	return nil
}

func (p *UnixPTY) Resize(cols, rows uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	winSize := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}
	return pty.Setsize(p.ptyFile, winSize)
}

func (p *UnixPTY) ProcessID() int {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

func (p *UnixPTY) Wait() (int, error) {
	if p.cmd == nil {
		return 0, nil
	}
	err := p.cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}
