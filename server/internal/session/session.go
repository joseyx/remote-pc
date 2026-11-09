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
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"orbit/internal/config"
	"orbit/internal/pty"
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
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Cwd        string    `json:"cwd"`
	Command    string    `json:"command"`
	BubbleID   string    `json:"bubbleId,omitempty"`
	Role       string    `json:"role,omitempty"`
	Cols       uint16    `json:"cols"`
	Rows       uint16    `json:"rows"`
	BufferB64  string    `json:"bufferB64,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	SocketPath string    `json:"socketPath,omitempty"`
	WorkerPID  int       `json:"workerPid,omitempty"`
}

type Session struct {
	ID         string
	Name       string
	Cwd        string
	Command    string
	PID        int
	IsRunning  bool
	ExitCode   int
	CreatedAt  time.Time
	BubbleID   string
	Role       string
	Cols       uint16
	Rows       uint16
	SocketPath string
	WorkerPID  int

	pty pty.PTY

	mu        sync.RWMutex
	buffer    *ringBuffer
	listeners map[chan []byte]struct{}
	onWrite   func()
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
	dataDir      string
	stopTicker   chan struct{}
	dirty        atomic.Bool
}

func NewManager(maxHistoryKB int, statePath ...string) *Manager {
	sp := "sessions_state.json"
	if len(statePath) > 0 && statePath[0] != "" {
		sp = statePath[0]
	}

	dataDir := filepath.Dir(sp)
	if dataDir == "." || dataDir == "" {
		dataDir = config.GetDataDir()
	}

	m := &Manager{
		sessions:     make(map[string]*Session),
		connections:  make(map[string]*ClientConnection),
		maxHistoryKB: maxHistoryKB,
		statePath:    sp,
		dataDir:      dataDir,
		stopTicker:   make(chan struct{}),
	}

	// Periodic buffer flush every 15 seconds (only when dirty to eliminate disk thrashing)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if m.dirty.Swap(false) {
					_ = m.SaveState()
				}
			case <-m.stopTicker:
				return
			}
		}
	}()

	return m
}

func (m *Manager) SetDataDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataDir = dir
}

func (m *Manager) runDir() string {
	d := m.dataDir
	if d == "" {
		d = config.GetDataDir()
	}
	rd := filepath.Join(d, "run")
	// If path length would exceed UNIX domain socket limits (104 on macOS, 108 on Linux/Windows),
	// fallback to standard temporary directory which is short (e.g. /tmp/orbit-run or os.TempDir())
	if len(rd)+30 >= 100 {
		if runtime.GOOS != "windows" {
			rd = "/tmp/orbit-run"
		} else {
			rd = filepath.Join(os.TempDir(), "orbit-run")
		}
	}
	_ = os.MkdirAll(rd, 0755)
	return rd
}

func (m *Manager) startPTY(sessionID, command, cwd string, cols, rows uint16) (pty.PTY, string, int, error) {
	if cols == 0 {
		cols = 120
	}
	if rows == 0 {
		rows = 30
	}

	shortID := sessionID
	if len(shortID) > 16 {
		shortID = shortID[:16]
	}
	sockPath := filepath.Join(m.runDir(), fmt.Sprintf("pty-%s.sock", shortID))

	// Try spawning detached PTY worker
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}

	canSpawn := false
	if fi, err := os.Stat(exePath); err == nil && !fi.IsDir() {
		base := strings.ToLower(filepath.Base(exePath))
		if !strings.HasSuffix(base, ".test.exe") && !strings.HasSuffix(base, ".test") {
			canSpawn = true
		}
	}

	if canSpawn {
		if spawnErr := pty.SpawnWorker(exePath, sessionID, sockPath, command, cwd, cols, rows); spawnErr == nil {
			// Retry connecting to worker socket (up to 2.5 seconds)
			for i := 0; i < 25; i++ {
				time.Sleep(100 * time.Millisecond)
				if rp, dialErr := pty.DialRemotePTY(sessionID, sockPath, 400*time.Millisecond); dialErr == nil {
					return rp, sockPath, rp.WorkerPID(), nil
				}
			}
			log.Printf("[Session %s] Timeout connecting to PTY worker socket %s, falling back to in-process PTY", sessionID, sockPath)
			_ = os.Remove(sockPath)
		} else {
			log.Printf("[Session %s] Failed to spawn PTY worker: %v, falling back to in-process PTY", sessionID, spawnErr)
		}
	}

	// Fallback to in-process PTY
	p, err := pty.Start(command, cwd, cols, rows)
	if err != nil {
		return nil, "", 0, err
	}
	return p, "", 0, nil
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

	sessionID := uuid.New().String()
	p, sockPath, workerPID, err := m.startPTY(sessionID, command, cwd, cols, rows)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	s := &Session{
		ID:         sessionID,
		Name:       name,
		Cwd:        cwd,
		Command:    command,
		PID:        p.ProcessID(),
		IsRunning:  true,
		CreatedAt:  time.Now(),
		BubbleID:   bubbleID,
		Role:       role,
		Cols:       cols,
		Rows:       rows,
		SocketPath: sockPath,
		WorkerPID:  workerPID,
		pty:        p,
		buffer:     newRingBuffer(m.maxHistoryKB * 1024),
		listeners:  make(map[chan []byte]struct{}),
		onWrite:    func() { m.dirty.Store(true) },
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.dirty.Store(true)
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
	m.dirty.Store(true)
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
	m.dirty.Store(true)
	_ = m.saveStateLocked()
}

func (m *Manager) CloseAllPTYs() {
	m.closeAllPTYsInternal(true)
}

func (m *Manager) DetachAllPTYs() {
	m.closeAllPTYsInternal(false)
}

func (m *Manager) closeAllPTYsInternal(killWorkers bool) {
	if m.stopTicker != nil {
		select {
		case m.stopTicker <- struct{}{}:
		default:
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if killWorkers {
			_ = s.Close()
		} else {
			_ = s.Detach()
		}
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
			ID:         s.ID,
			Name:       s.Name,
			Cwd:        s.Cwd,
			Command:    s.Command,
			BubbleID:   s.BubbleID,
			Role:       s.Role,
			Cols:       s.Cols,
			Rows:       s.Rows,
			BufferB64:  base64.StdEncoding.EncodeToString(buf),
			CreatedAt:  s.CreatedAt,
			SocketPath: s.SocketPath,
			WorkerPID:  s.WorkerPID,
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

		// 1. Check if an active PTY worker socket exists from a previous run (Hot update / restart)
		if st.SocketPath != "" {
			if _, statErr := os.Stat(st.SocketPath); statErr == nil {
				if rp, dialErr := pty.DialRemotePTY(st.ID, st.SocketPath, 1*time.Second); dialErr == nil {
					ring := newRingBuffer(m.maxHistoryKB * 1024)
					if len(st.BufferB64) > 0 {
						bufBytes, _ := base64.StdEncoding.DecodeString(st.BufferB64)
						if len(bufBytes) > 0 {
							ring.Write(bufBytes)
						}
					}

					s := &Session{
						ID:         st.ID,
						Name:       st.Name,
						Cwd:        st.Cwd,
						Command:    cmd,
						PID:        rp.ProcessID(),
						IsRunning:  true,
						CreatedAt:  st.CreatedAt,
						BubbleID:   st.BubbleID,
						Role:       st.Role,
						Cols:       cols,
						Rows:       rows,
						SocketPath: st.SocketPath,
						WorkerPID:  rp.WorkerPID(),
						pty:        rp,
						buffer:     ring,
						listeners:  make(map[chan []byte]struct{}),
						onWrite:    func() { m.dirty.Store(true) },
					}

					m.sessions[s.ID] = s
					go s.readLoop()
					go s.waitLoop()
					restoredCount++
					log.Printf("[Session Restore] Reconnected to LIVE PTY Worker %s (%s) [PID %d] in %s", s.ID, s.Name, s.PID, s.Cwd)
					continue
				} else {
					log.Printf("[Session Restore] Failed to reconnect to worker at %s: %v", st.SocketPath, dialErr)
				}
				// Stale socket, cleanup
				_ = os.Remove(st.SocketPath)
			}
		}

		// 2. Fallback: Worker is not alive (e.g. system reboot or killed).
		// Start a fresh shell preserving history buffer.
		isAgent := st.Role == "claude" || st.Role == "agy" || cmd == "claude" || cmd == "agy"
		launchCmd := cmd
		if isAgent {
			launchCmd = "powershell.exe"
		}

		p, sockPath, workerPID, err := m.startPTY(st.ID, launchCmd, st.Cwd, cols, rows)
		if err != nil {
			log.Printf("[Session Restore] Could not restart PTY for session %s: %v", st.ID, err)
			continue
		}

		ring := newRingBuffer(m.maxHistoryKB * 1024)
		if len(st.BufferB64) > 0 {
			bufBytes, _ := base64.StdEncoding.DecodeString(st.BufferB64)
			if len(bufBytes) > 0 {
				ring.Write(bufBytes)
			}
		}
		if isAgent {
			ring.Write([]byte("\r\n\x1b[36m--- [Sesión restaurada. Escribe 'claude' para iniciar el agente cuando lo necesites] ---\x1b[0m\r\n"))
		} else {
			ring.Write([]byte("\r\n\x1b[36m--- [Sesión restaurada tras reiniciar Remote PC] ---\x1b[0m\r\n"))
		}

		s := &Session{
			ID:         st.ID,
			Name:       st.Name,
			Cwd:        st.Cwd,
			Command:    cmd,
			PID:        p.ProcessID(),
			IsRunning:  true,
			CreatedAt:  st.CreatedAt,
			BubbleID:   st.BubbleID,
			Role:       st.Role,
			Cols:       cols,
			Rows:       rows,
			SocketPath: sockPath,
			WorkerPID:  workerPID,
			pty:        p,
			buffer:     ring,
			listeners:  make(map[chan []byte]struct{}),
			onWrite:    func() { m.dirty.Store(true) },
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
			if s.onWrite != nil {
				s.onWrite()
			}

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

func (s *Session) GetHistory() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffer.Bytes()
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

type DetachablePTY interface {
	Detach() error
}

func (s *Session) Detach() error {
	s.mu.Lock()
	s.IsRunning = false
	s.mu.Unlock()

	if dp, ok := s.pty.(DetachablePTY); ok {
		return dp.Detach()
	}
	return s.pty.Close()
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
	sort.Slice(res, func(i, j int) bool {
		if res[i].CreatedAt.Equal(res[j].CreatedAt) {
			return res[i].ID < res[j].ID
		}
		return res[i].CreatedAt.Before(res[j].CreatedAt)
	})
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
	m.dirty.Store(true)
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
