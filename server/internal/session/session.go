package session

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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
	BubbleID  string    `json:"bubbleId,omitempty"`
	Role      string    `json:"role,omitempty"` // "claude", "server", "shell"
}

type StoredSession struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cwd       string    `json:"cwd"`
	Command   string    `json:"command"`
	BubbleID  string    `json:"bubbleId,omitempty"`
	Role      string    `json:"role,omitempty"`
	Cols      uint16    `json:"cols"`
	Rows      uint16    `json:"rows"`
	BufferB64 string    `json:"bufferB64,omitempty"`
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
	BubbleID  string
	Role      string
	Cols      uint16
	Rows      uint16

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

	// If buffer exceeds max size, trim old bytes cleanly at newline boundary
	if rb.buf.Len()+len(p) > rb.maxSize {
		excess := (rb.buf.Len() + len(p)) - rb.maxSize
		b := rb.buf.Bytes()
		if excess >= len(b) {
			rb.buf.Reset()
		} else {
			// Find first newline at or after excess so we never slice mid-character or mid-escape sequence
			idx := bytes.IndexByte(b[excess:], '\n')
			var remaining []byte
			if idx != -1 && (excess+idx+1) < len(b) {
				remaining = b[excess+idx+1:]
			} else {
				remaining = b[excess:]
			}
			rb.buf.Reset()
			// Reset ANSI attributes so discarded text formatting never bleeds into remaining history
			rb.buf.WriteString("\x1b[0m")
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
	statePath    string
	stopTicker   chan struct{}
}

func NewManager(maxHistoryKB int, statePath ...string) *Manager {
	sp := "sessions_state.json"
	if len(statePath) > 0 && statePath[0] != "" {
		sp = statePath[0]
	}

	m := &Manager{
		sessions:     make(map[string]*Session),
		connections:  make(map[string]*ClientConnection),
		maxHistoryKB: maxHistoryKB,
		statePath:    sp,
		stopTicker:   make(chan struct{}),
	}

	// Periodic buffer flush every 15 seconds
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = m.SaveState()
			case <-m.stopTicker:
				return
			}
		}
	}()

	return m
}

func (m *Manager) CreateSession(name, cwd, command string, cols, rows uint16) (*Session, error) {
	return m.CreateBubbleSession(name, cwd, command, "", "shell", cols, rows)
}

func (m *Manager) CreateBubbleSession(name, cwd, command, bubbleID, role string, cols, rows uint16) (*Session, error) {
	if command == "" {
		command = "powershell.exe"
	}
	if name == "" {
		name = "Terminal"
	}
	if role == "" {
		role = "shell"
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
		BubbleID:  bubbleID,
		Role:      role,
		Cols:      cols,
		Rows:      rows,
		pty:       p,
		buffer:    newRingBuffer(m.maxHistoryKB * 1024),
		listeners: make(map[chan []byte]struct{}),
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	_ = m.saveStateLocked()
	m.mu.Unlock()

	// Start asynchronous output pump
	go s.readLoop()
	// Start asynchronous process waiter
	go s.waitLoop()

	return s, nil
}

func (m *Manager) RenameSession(id, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return errors.New("session not found")
	}
	s.mu.Lock()
	s.Name = newName
	s.mu.Unlock()
	_ = m.saveStateLocked()
	return nil
}

func (m *Manager) DeleteBubbleSessions(bubbleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.BubbleID == bubbleID {
			delete(m.sessions, id)
			_ = s.Close()
		}
	}
	_ = m.saveStateLocked()
}

func (m *Manager) CloseAllPTYs() {
	if m.stopTicker != nil {
		select {
		case m.stopTicker <- struct{}{}:
		default:
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		_ = s.Close()
	}
}

func (m *Manager) SaveState() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveStateLocked()
}

func (m *Manager) saveStateLocked() error {
	if m.statePath == "" {
		return nil
	}

	list := make([]StoredSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.RLock()
		buf := s.buffer.Bytes()
		// Limit saved history buffer to 512KB per session to keep JSON agile
		if len(buf) > 512*1024 {
			buf = buf[len(buf)-512*1024:]
		}
		st := StoredSession{
			ID:        s.ID,
			Name:      s.Name,
			Cwd:       s.Cwd,
			Command:   s.Command,
			BubbleID:  s.BubbleID,
			Role:      s.Role,
			Cols:      s.Cols,
			Rows:      s.Rows,
			BufferB64: base64.StdEncoding.EncodeToString(buf),
			CreatedAt: s.CreatedAt,
		}
		s.mu.RUnlock()
		list = append(list, st)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := m.statePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, m.statePath)
}

func (m *Manager) RestoreSessions() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.statePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var stored []StoredSession
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("failed to parse stored sessions: %w", err)
	}

	restoredCount := 0
	for _, st := range stored {
		if st.Cwd != "" {
			if _, statErr := os.Stat(st.Cwd); os.IsNotExist(statErr) {
				log.Printf("[Session Restore] Path '%s' no longer exists, skipping session %s (%s)", st.Cwd, st.ID, st.Name)
				continue
			}
		}

		cols := st.Cols
		if cols == 0 {
			cols = 100
		}
		rows := st.Rows
		if rows == 0 {
			rows = 30
		}
		cmd := st.Command
		if cmd == "" {
			cmd = "powershell.exe"
		}

		p, err := pty.Start(cmd, st.Cwd, cols, rows)
		if err != nil {
			log.Printf("[Session Restore] Could not restart PTY for session %s: %v", st.ID, err)
			continue
		}

		ring := newRingBuffer(m.maxHistoryKB * 1024)
		if len(st.BufferB64) > 0 {
			bufBytes, _ := base64.StdEncoding.DecodeString(st.BufferB64)
			if len(bufBytes) > 0 {
				ring.Write(bufBytes)
				ring.Write([]byte("\r\n\x1b[36m--- [Sesión restaurada tras reiniciar Remote PC] ---\x1b[0m\r\n"))
			}
		}

		s := &Session{
			ID:        st.ID,
			Name:      st.Name,
			Cwd:       st.Cwd,
			Command:   cmd,
			PID:       p.ProcessID(),
			IsRunning: true,
			CreatedAt: st.CreatedAt,
			BubbleID:  st.BubbleID,
			Role:      st.Role,
			Cols:      cols,
			Rows:      rows,
			pty:       p,
			buffer:    ring,
			listeners: make(map[chan []byte]struct{}),
		}

		m.sessions[s.ID] = s
		go s.readLoop()
		go s.waitLoop()
		restoredCount++
		log.Printf("[Session Restore] Restored session %s (%s) [PID %d] in %s", s.ID, s.Name, s.PID, s.Cwd)
	}

	if restoredCount > 0 {
		log.Printf("[Session Restore] Successfully restored %d active terminal sessions from previous run", restoredCount)
	}
	return nil
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
	s.mu.Lock()
	s.Cols = cols
	s.Rows = rows
	s.mu.Unlock()

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
		BubbleID:  s.BubbleID,
		Role:      s.Role,
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
	_ = m.saveStateLocked()
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
