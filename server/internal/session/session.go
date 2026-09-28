package session

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"remotepc-server/internal/pty"
)

type SessionInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cwd       string    `json:"cwd"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	IsRunning bool      `json:"isRunning"`
	ExitCode  int       `json:"exitCode"`
	CreatedAt time.Time `json:"createdAt"`
}

type Session struct {
	ID        string
	Name      string
	Cwd       string
	Command   string
	PID       int
	IsRunning bool
	ExitCode  int
	CreatedAt time.Time

	pty pty.PTY

	mu        sync.RWMutex
	buffer    *ringBuffer
	listeners map[chan []byte]struct{}
}

// ringBuffer stores recent terminal history up to maxSize
type ringBuffer struct {
	buf     *bytes.Buffer
	maxSize int
	mu      sync.Mutex
}

func newRingBuffer(maxSize int) *ringBuffer {
	return &ringBuffer{
		buf:     bytes.NewBuffer(make([]byte, 0, maxSize)),
		maxSize: maxSize,
	}
}

func (rb *ringBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// If buffer exceeds max size, trim old bytes
	if rb.buf.Len()+len(p) > rb.maxSize {
		excess := (rb.buf.Len() + len(p)) - rb.maxSize
		b := rb.buf.Bytes()
		if excess >= len(b) {
			rb.buf.Reset()
		} else {
			remaining := b[excess:]
			rb.buf.Reset()
			rb.buf.Write(remaining)
		}
	}
	return rb.buf.Write(p)
}

func (rb *ringBuffer) Bytes() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	res := make([]byte, rb.buf.Len())
	copy(res, rb.buf.Bytes())
	return res
}

type ClientConnection struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"sessionId"`
	SessionName string    `json:"sessionName"`
	RemoteAddr  string    `json:"remoteAddr"`
	UserAgent   string    `json:"userAgent"`
	ConnectedAt time.Time `json:"connectedAt"`
}

type Manager struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	connections  map[string]*ClientConnection
	maxHistoryKB int
}

func NewManager(maxHistoryKB int) *Manager {
	return &Manager{
		sessions:     make(map[string]*Session),
		connections:  make(map[string]*ClientConnection),
		maxHistoryKB: maxHistoryKB,
	}
}

func (m *Manager) CreateSession(name, cwd, command string, cols, rows uint16) (*Session, error) {
	if command == "" {
		command = "powershell.exe"
	}
	if name == "" {
		name = "Terminal"
	}

	p, err := pty.Start(command, cwd, cols, rows)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	s := &Session{
		ID:        uuid.New().String(),
		Name:      name,
		Cwd:       cwd,
		Command:   command,
		PID:       p.ProcessID(),
		IsRunning: true,
		CreatedAt: time.Now(),
		pty:       p,
		buffer:    newRingBuffer(m.maxHistoryKB * 1024),
		listeners: make(map[chan []byte]struct{}),
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()

	// Start asynchronous output pump
	go s.readLoop()
	// Start asynchronous process waiter
	go s.waitLoop()

	return s, nil
}

func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			// Append to history buffer
			s.buffer.Write(data)

			// Broadcast to all active listeners (WebSocket connections)
			s.mu.RLock()
			for ch := range s.listeners {
				select {
				case ch <- data:
				default:
					// Avoid blocking if client channel is full
				}
			}
			s.mu.RUnlock()
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("[Session %s] PTY stream reached EOF", s.ID)
			}
			break
		}
	}
}

func (s *Session) waitLoop() {
	code, err := s.pty.Wait()
	s.mu.Lock()
	s.IsRunning = false
	s.ExitCode = code
	s.mu.Unlock()

	if err != nil {
		log.Printf("[Session %s] process exited with error: %v", s.ID, err)
	} else {
		log.Printf("[Session %s] process exited with code %d", s.ID, code)
	}
}

func (s *Session) WriteInput(data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.IsRunning {
		return errors.New("session is not running")
	}
	_, err := s.pty.Write(data)
	return err
}

func (s *Session) Resize(cols, rows uint16) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.IsRunning {
		return errors.New("session is not running")
	}
	return s.pty.Resize(cols, rows)
}

func (s *Session) Subscribe() (chan []byte, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan []byte, 100)
	s.listeners[ch] = struct{}{}
	history := s.buffer.Bytes()
	return ch, history
}

func (s *Session) Unsubscribe(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.listeners, ch)
	close(ch)
}

func (s *Session) Close() error {
	s.mu.Lock()
	s.IsRunning = false
	s.mu.Unlock()

	return s.pty.Close()
}

func (s *Session) ToInfo() *SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &SessionInfo{
		ID:        s.ID,
		Name:      s.Name,
		Cwd:       s.Cwd,
		Command:   s.Command,
		PID:       s.PID,
		IsRunning: s.IsRunning,
		ExitCode:  s.ExitCode,
		CreatedAt: s.CreatedAt,
	}
}

func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) ListSessions() []*SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		res = append(res, s.ToInfo())
	}
	return res
}

func (m *Manager) DeleteSession(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("session not found")
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	return s.Close()
}

func (m *Manager) RegisterConnection(conn *ClientConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections[conn.ID] = conn
}

func (m *Manager) UnregisterConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connections, id)
}

func (m *Manager) ListConnections() []*ClientConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*ClientConnection, 0, len(m.connections))
	for _, c := range m.connections {
		res = append(res, c)
	}
	return res
}

