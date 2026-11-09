package pty

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// RemotePTY implements the PTY interface by proxying commands and I/O over a local UNIX domain socket
type RemotePTY struct {
	sessionID  string
	socketPath string
	conn       net.Conn
	workerPID  int
	childPID   int
	cols       uint16
	rows       uint16

	readBuf   *bytes.Buffer
	readCh    chan []byte
	exitCh    chan int
	exitCode  int
	hasExited bool

	mu       sync.Mutex
	closed   bool
	detached bool
}

// DialRemotePTY connects to a running PTY worker daemon
func DialRemotePTY(sessionID, socketPath string, timeout time.Duration) (*RemotePTY, error) {
	conn, err := net.DialTimeout("unix", socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to dial pty worker socket at %s: %w", socketPath, err)
	}

	// 1. Read Hello frame from worker
	frameType, payload, err := ReadFrame(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to read hello handshake from worker: %w", err)
	}

	if frameType != FrameControl {
		_ = conn.Close()
		return nil, fmt.Errorf("expected control frame from worker, got 0x%02x", frameType)
	}

	hello, err := ParseControlFrame(payload)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to parse hello message: %w", err)
	}

	rp := &RemotePTY{
		sessionID:  sessionID,
		socketPath: socketPath,
		conn:       conn,
		workerPID:  hello.WorkerPID,
		childPID:   hello.ChildPID,
		cols:       hello.Cols,
		rows:       hello.Rows,
		readBuf:    bytes.NewBuffer(nil),
		readCh:     make(chan []byte, 128),
		exitCh:     make(chan int, 1),
	}

	if hello.Type == "exit" {
		rp.hasExited = true
		rp.exitCode = hello.ExitCode
		rp.exitCh <- hello.ExitCode
	}

	// 2. Start background reader loop
	go rp.readLoop()

	return rp, nil
}

func (rp *RemotePTY) readLoop() {
	defer func() {
		close(rp.readCh)
	}()

	for {
		frameType, payload, err := ReadFrame(rp.conn)
		if err != nil {
			return
		}

		switch frameType {
		case FrameData:
			if len(payload) > 0 {
				rp.readCh <- payload
			}
		case FrameControl:
			msg, err := ParseControlFrame(payload)
			if err != nil {
				continue
			}
			if msg.Type == "exit" {
				rp.mu.Lock()
				rp.hasExited = true
				rp.exitCode = msg.ExitCode
				rp.mu.Unlock()

				select {
				case rp.exitCh <- msg.ExitCode:
				default:
				}
				return
			}
		}
	}
}

// Read reads output emitted by the child process
func (rp *RemotePTY) Read(p []byte) (int, error) {
	rp.mu.Lock()
	if rp.readBuf.Len() > 0 {
		n, err := rp.readBuf.Read(p)
		rp.mu.Unlock()
		return n, err
	}
	rp.mu.Unlock()

	data, ok := <-rp.readCh
	if !ok {
		return 0, io.EOF
	}

	// Fast path: if data fits directly into caller's slice, return without buffer write overhead
	if len(data) <= len(p) {
		copy(p, data)
		return len(data), nil
	}

	rp.mu.Lock()
	rp.readBuf.Write(data)
	n, err := rp.readBuf.Read(p)
	rp.mu.Unlock()
	return n, err
}

// Write sends user keystrokes / input to the child process
func (rp *RemotePTY) Write(p []byte) (int, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.closed {
		return 0, errors.New("remote pty is closed")
	}

	if err := WriteDataFrame(rp.conn, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Resize changes the terminal dimensions
func (rp *RemotePTY) Resize(cols, rows uint16) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.closed {
		return errors.New("remote pty is closed")
	}

	rp.cols = cols
	rp.rows = rows
	return WriteControlFrame(rp.conn, &ControlMsg{
		Type: "resize",
		Cols: cols,
		Rows: rows,
	})
}

// ProcessID returns the PID of the underlying shell / child process (e.g. powershell or claude)
func (rp *RemotePTY) ProcessID() int {
	return rp.childPID
}

// WorkerPID returns the PID of the detached PTY host worker
func (rp *RemotePTY) WorkerPID() int {
	return rp.workerPID
}

// SocketPath returns the UNIX domain socket path
func (rp *RemotePTY) SocketPath() string {
	return rp.socketPath
}

// Wait blocks until the child process terminates and returns its exit code
func (rp *RemotePTY) Wait() (int, error) {
	rp.mu.Lock()
	if rp.hasExited {
		code := rp.exitCode
		rp.mu.Unlock()
		return code, nil
	}
	rp.mu.Unlock()

	code, ok := <-rp.exitCh
	if !ok {
		rp.mu.Lock()
		defer rp.mu.Unlock()
		return rp.exitCode, nil
	}
	return code, nil
}

// Close explicitly terminates the remote worker and its child process
func (rp *RemotePTY) Close() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.closed {
		return nil
	}
	rp.closed = true

	// Tell worker to cleanly terminate ConPTY and child
	_ = WriteControlFrame(rp.conn, &ControlMsg{Type: "close"})

	// Give worker time to handle close frame and terminate child gracefully before abruptly tearing down connection
	_ = rp.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 64)
	for {
		_, err := rp.conn.Read(buf)
		if err != nil {
			break
		}
	}
	return rp.conn.Close()
}

// Detach disconnects the client connection WITHOUT killing the worker or child process
func (rp *RemotePTY) Detach() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.closed || rp.detached {
		return nil
	}
	rp.detached = true
	return rp.conn.Close()
}
