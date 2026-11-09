package pty

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// WorkerRingBuffer holds recent output history in memory
type WorkerRingBuffer struct {
	buf     *bytes.Buffer
	maxSize int
	mu      sync.Mutex
}

func NewWorkerRingBuffer(maxSize int) *WorkerRingBuffer {
	return &WorkerRingBuffer{
		buf:     bytes.NewBuffer(make([]byte, 0, maxSize)),
		maxSize: maxSize,
	}
}

func (rb *WorkerRingBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.buf.Len()+len(p) > rb.maxSize {
		excess := (rb.buf.Len() + len(p)) - rb.maxSize
		b := rb.buf.Bytes()
		if excess >= len(b) {
			rb.buf.Reset()
		} else {
			idx := bytes.IndexByte(b[excess:], '\n')
			var remaining []byte
			if idx != -1 && (excess+idx+1) < len(b) {
				remaining = b[excess+idx+1:]
			} else {
				remaining = b[excess:]
			}
			rb.buf.Reset()
			rb.buf.Write(remaining)
		}
	}
	return rb.buf.Write(p)
}

func (rb *WorkerRingBuffer) Bytes() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	b := rb.buf.Bytes()
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// Worker represents the standalone PTY daemon managing ConPTY / child process
type Worker struct {
	sessionID  string
	socketPath string
	cmdStr     string
	cwd        string
	cols       uint16
	rows       uint16

	pty       PTY
	ring      *WorkerRingBuffer
	listener  net.Listener
	activeConn net.Conn
	connMu    sync.Mutex

	childExitCode int
	childExited   bool
	exitCh        chan struct{}
	closeOnce     sync.Once
}

// RunWorker starts the detached PTY worker and blocks until the child process exits or close command is received
func RunWorker(sessionID, socketPath, cmdStr, cwd string, cols, rows uint16) error {
	if cols == 0 {
		cols = 120
	}
	if rows == 0 {
		rows = 30
	}

	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Clean up stale socket if present
	_ = os.Remove(socketPath)

	// 1. Start the underlying PTY and child process
	p, err := Start(cmdStr, cwd, cols, rows)
	if err != nil {
		return fmt.Errorf("failed to start underlying pty: %w", err)
	}

	// 2. Start UNIX domain socket listener
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		_ = p.Close()
		return fmt.Errorf("failed to bind unix socket at %s: %w", socketPath, err)
	}

	w := &Worker{
		sessionID:  sessionID,
		socketPath: socketPath,
		cmdStr:     cmdStr,
		cwd:        cwd,
		cols:       cols,
		rows:       rows,
		pty:        p,
		ring:       NewWorkerRingBuffer(2 * 1024 * 1024), // 2 MB backlog buffer
		listener:   l,
		exitCh:     make(chan struct{}),
	}

	// Ensure cleanup on shutdown
	defer func() {
		w.closeOnce.Do(func() {
			_ = l.Close()
			_ = os.Remove(socketPath)
			_ = p.Close()
		})
	}()

	log.Printf("[PTY Worker %s] Started (PID %d, child PID %d) on %s", sessionID, os.Getpid(), p.ProcessID(), socketPath)

	// Goroutine: Monitor child process exit
	go func() {
		code, _ := p.Wait()
		w.connMu.Lock()
		w.childExited = true
		w.childExitCode = code
		conn := w.activeConn
		w.connMu.Unlock()

		if conn != nil {
			_ = WriteControlFrame(conn, &ControlMsg{
				Type:      "exit",
				SessionID: sessionID,
				ExitCode:  code,
			})
		}
		log.Printf("[PTY Worker %s] Child process exited with code %d", sessionID, code)
		close(w.exitCh)
		_ = l.Close()
	}()

	// Goroutine: Read from PTY stdout and broadcast to active connection + ring buffer
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				w.ring.Write(data)

				w.connMu.Lock()
				conn := w.activeConn
				w.connMu.Unlock()

				if conn != nil {
					if writeErr := WriteDataFrame(conn, data); writeErr != nil {
						log.Printf("[PTY Worker %s] Error writing to client connection: %v (client disconnected)", sessionID, writeErr)
						w.connMu.Lock()
						if w.activeConn == conn {
							_ = conn.Close()
							w.activeConn = nil
						}
						w.connMu.Unlock()
					}
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.Printf("[PTY Worker %s] PTY read finished: %v", sessionID, err)
				}
				break
			}
		}
	}()

	// Main accept loop: accept connections from Orbit server
	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-w.exitCh:
				// Normal exit due to child process termination
				return nil
			default:
				return fmt.Errorf("accept error on %s: %w", socketPath, err)
			}
		}

		w.handleNewClient(conn)
	}
}

func (w *Worker) handleNewClient(conn net.Conn) {
	w.connMu.Lock()
	// If an older connection is still hanging, close it and replace
	if w.activeConn != nil {
		_ = w.activeConn.Close()
	}
	w.activeConn = conn
	childExited := w.childExited
	childExitCode := w.childExitCode
	cols := w.cols
	rows := w.rows
	w.connMu.Unlock()

	log.Printf("[PTY Worker %s] Client connected from Orbit server", w.sessionID)

	// 1. Send Hello frame
	hello := &ControlMsg{
		Type:      "hello",
		SessionID: w.sessionID,
		WorkerPID: os.Getpid(),
		ChildPID:  w.pty.ProcessID(),
		Cols:      cols,
		Rows:      rows,
	}
	if childExited {
		hello.Type = "exit"
		hello.ExitCode = childExitCode
	}
	if err := WriteControlFrame(conn, hello); err != nil {
		log.Printf("[PTY Worker %s] Failed to send hello: %v", w.sessionID, err)
		_ = conn.Close()
		return
	}

	// 2. Replay recent ring buffer history if child hasn't exited
	backlog := w.ring.Bytes()
	if len(backlog) > 0 {
		_ = WriteDataFrame(conn, backlog)
	}

	if childExited {
		_ = conn.Close()
		return
	}

	// 3. Handle incoming frames from Orbit server in a dedicated goroutine
	go func(c net.Conn) {
		defer func() {
			w.connMu.Lock()
			if w.activeConn == c {
				w.activeConn = nil
			}
			w.connMu.Unlock()
			_ = c.Close()
			log.Printf("[PTY Worker %s] Client disconnected (PTY and child remain active)", w.sessionID)
		}()

		for {
			frameType, payload, err := ReadFrame(c)
			if err != nil {
				// Server closed connection or restarted
				return
			}

			switch frameType {
			case FrameData:
				// Terminal stdin from user
				if len(payload) > 0 {
					if _, err := w.pty.Write(payload); err != nil {
						log.Printf("[PTY Worker %s] Failed to write stdin to PTY: %v", w.sessionID, err)
						return
					}
				}

			case FrameControl:
				msg, err := ParseControlFrame(payload)
				if err != nil {
					log.Printf("[PTY Worker %s] Malformed control frame: %v", w.sessionID, err)
					continue
				}

				switch msg.Type {
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						w.connMu.Lock()
						w.cols = msg.Cols
						w.rows = msg.Rows
						w.connMu.Unlock()
						_ = w.pty.Resize(msg.Cols, msg.Rows)
					}
				case "ping":
					_ = WriteControlFrame(c, &ControlMsg{Type: "pong", SessionID: w.sessionID})
				case "close":
					log.Printf("[PTY Worker %s] Received explicit close command. Shutting down worker and child.", w.sessionID)
					w.closeOnce.Do(func() {
						_ = w.listener.Close()
						_ = os.Remove(w.socketPath)
						_ = w.pty.Close()
					})
					return
				}
			}
		}
	}(conn)
}
