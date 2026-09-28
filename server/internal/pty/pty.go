package pty

import (
	"io"
)

type PTY interface {
	io.Reader
	io.Writer
	io.Closer
	Resize(cols, rows uint16) error
	ProcessID() int
	Wait() (int, error)
}
