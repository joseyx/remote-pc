package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"remotepc-server/internal/bubble"
	"remotepc-server/internal/calendar"
	"remotepc-server/internal/config"
	"remotepc-server/internal/git"
	"remotepc-server/internal/obsidian"
	"remotepc-server/internal/services"
	"remotepc-server/internal/session"
	"remotepc-server/internal/system"
	"remotepc-server/internal/vpn"
	"remotepc-server/internal/web"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow mobile/remote origins
	},
}

type Server struct {
	cfg         *config.Config
	manager     *session.Manager
	vpnMgr      *vpn.Manager
	bubbleMgr   *bubble.Manager
	obsidianMgr *obsidian.Manager
	calendarMgr *calendar.Manager
	startedAt   time.Time
}

func NewServer(cfg *config.Config, manager *session.Manager) *Server {
	return &Server{
		cfg:         cfg,
		manager:     manager,
		vpnMgr:      vpn.NewManager(),
		bubbleMgr:   bubble.NewManager(),
		obsidianMgr: obsidian.NewManager(),
		calendarMgr: calendar.NewManager(),
		startedAt:   time.Now(),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/server-info", s.handleServerInfo)
	mux.HandleFunc("GET /api/system/stats", s.handleSystemStats)
	mux.HandleFunc("POST /api/system/process/kill", s.withAuth(s.handleKillProcess))
	mux.HandleFunc("POST /api/services/{id}/start", s.withAuth(s.handleStartService))
	mux.HandleFunc("POST /api/services/{id}/stop", s.withAuth(s.handleStopService))
	mux.HandleFunc("GET /api/connections", s.withAuth(s.handleListConnections))
	mux.HandleFunc("GET /api/fs/directories", s.withAuth(s.handleListDirectories))
	mux.HandleFunc("GET /api/sessions", s.withAuth(s.handleListSessions))
	mux.HandleFunc("POST /api/sessions", s.withAuth(s.handleCreateSession))
	mux.HandleFunc("PATCH /api/sessions/{id}", s.withAuth(s.handleRenameSession))
	mux.HandleFunc("DELETE /api/sessions/{id}", s.withAuth(s.handleDeleteSession))
	mux.HandleFunc("/ws/session/{id}", s.handleWebSocket)

	// VPN Controls
	mux.HandleFunc("GET /api/vpn/status", s.withAuth(s.handleVPNStatus))
	mux.HandleFunc("GET /api/vpn/options", s.withAuth(s.handleVPNOptions))
	mux.HandleFunc("POST /api/vpn/connect", s.withAuth(s.handleVPNConnect))
	mux.HandleFunc("POST /api/vpn/disconnect", s.withAuth(s.handleVPNDisconnect))

	// Eco & Bubble Routes
	mux.HandleFunc("GET /api/workspaces", s.withAuth(s.handleListWorkspaces))
	mux.HandleFunc("GET /api/bubbles", s.withAuth(s.handleListBubbles))
	mux.HandleFunc("POST /api/bubbles", s.withAuth(s.handleCreateBubble))
	mux.HandleFunc("PATCH /api/bubbles/{id}", s.withAuth(s.handleUpdateBubbleMetadata))
	mux.HandleFunc("POST /api/bubbles/reorder", s.withAuth(s.handleReorderBubbles))
	mux.HandleFunc("DELETE /api/bubbles/{id}", s.withAuth(s.handleDeleteBubble))
	mux.HandleFunc("POST /api/bubbles/{id}/dev/start", s.withAuth(s.handleStartDevServer))
	mux.HandleFunc("POST /api/bubbles/{id}/dev/stop", s.withAuth(s.handleStopDevServer))
	mux.HandleFunc("GET /api/bubbles/{id}/git/status", s.withAuth(s.handleGetGitStatus))
	mux.HandleFunc("GET /api/bubbles/{id}/git/diff", s.withAuth(s.handleGetGitDiff))
	mux.HandleFunc("GET /api/bubbles/{id}/git/log", s.withAuth(s.handleGetGitLog))
	mux.HandleFunc("GET /api/bubbles/{id}/git/branches", s.withAuth(s.handleListGitBranches))
	mux.HandleFunc("POST /api/bubbles/{id}/git/checkout", s.withAuth(s.handleCheckoutGitBranch))

	// Obsidian Vault Routes
	mux.HandleFunc("GET /api/vaults", s.withAuth(s.handleListVaults))
	mux.HandleFunc("GET /api/vaults/{vault}/notes", s.withAuth(s.handleSearchNotes))
	mux.HandleFunc("GET /api/vaults/{vault}/note", s.withAuth(s.handleReadNote))
	mux.HandleFunc("POST /api/vaults/{vault}/note", s.withAuth(s.handleSaveNote))

	// Calendar & Daily Notes Routes
	mux.HandleFunc("GET /api/calendar/month", s.withAuth(s.handleCalendarMonth))
	mux.HandleFunc("GET /api/calendar/note", s.withAuth(s.handleCalendarGetNote))
	mux.HandleFunc("PUT /api/calendar/note", s.withAuth(s.handleCalendarSaveNote))
	mux.HandleFunc("POST /api/calendar/handoff", s.withAuth(s.handleCalendarHandoff))
	mux.HandleFunc("GET /api/calendar/standup", s.withAuth(s.handleCalendarStandup))

	// Embedded Desktop Web Dashboard
	mux.Handle("/", web.Handler())

	return mux
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimPrefix(token, "Bearer ")
		} else {
			token = r.URL.Query().Get("token")
		}

		if s.cfg.AuthToken != "" && token != s.cfg.AuthToken {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	stats := system.GetStats()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleKillProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PID uint32 `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PID <= 4 {
		http.Error(w, `{"error": "Invalid PID"}`, http.StatusBadRequest)
		return
	}

	if err := system.KillProcess(req.PID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "pid": req.PID})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "pid": req.PID})
}

func (s *Server) handleStartService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error": "Missing service id"}`, http.StatusBadRequest)
		return
	}
	if err := services.StartService(id); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "id": id})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "id": id, "action": "started"})
}

func (s *Server) handleStopService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error": "Missing service id"}`, http.StatusBadRequest)
		return
	}
	if err := services.StopService(id); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "id": id})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "id": id, "action": "stopped"})
}

type ServerInfoResponse struct {
	Hostname     string      `json:"hostname"`
	Port         string      `json:"port"`
	LanIps       []string    `json:"lanIps"`
	AuthToken    string      `json:"authToken"`
	Uptime       string      `json:"uptime"`
	StartedAt    time.Time   `json:"startedAt"`
	SessionCount int         `json:"sessionCount"`
	ClientCount  int         `json:"clientCount"`
	VPN          *vpn.Status `json:"vpn,omitempty"`
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	uptimeDuration := time.Since(s.startedAt)
	uptimeStr := fmt.Sprintf("%dh %dm", int(uptimeDuration.Hours()), int(uptimeDuration.Minutes())%60)
	if uptimeDuration.Hours() < 1 {
		uptimeStr = fmt.Sprintf("%dm %ds", int(uptimeDuration.Minutes()), int(uptimeDuration.Seconds())%60)
	}

	vpnStatus := s.vpnMgr.Status()

	info := ServerInfoResponse{
		Hostname:     hostname,
		Port:         s.cfg.Port,
		LanIps:       getLANIPs(),
		AuthToken:    s.cfg.AuthToken,
		Uptime:       uptimeStr,
		StartedAt:    s.startedAt,
		SessionCount: len(s.manager.ListSessions()),
		ClientCount:  len(s.manager.ListConnections()),
		VPN:          &vpnStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleVPNStatus(w http.ResponseWriter, r *http.Request) {
	st := s.vpnMgr.Status()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) handleVPNOptions(w http.ResponseWriter, r *http.Request) {
	opts := s.vpnMgr.GetOptions()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(opts)
}

func (s *Server) handleVPNConnect(w http.ResponseWriter, r *http.Request) {
	var req vpn.ConnectOptions
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	st, err := s.vpnMgr.Connect(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) handleVPNDisconnect(w http.ResponseWriter, r *http.Request) {
	st, err := s.vpnMgr.Disconnect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	conns := s.manager.ListConnections()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(conns)
}

func getLANIPs() []string {
	var lan192 []string
	var lanPrivate []string
	var others []string

	ifaces, err := net.Interfaces()
	if err != nil {
		return lan192
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		isVirtual := strings.Contains(name, "vethernet") ||
			strings.Contains(name, "wsl") ||
			strings.Contains(name, "virtual") ||
			strings.Contains(name, "pseudo") ||
			strings.Contains(name, "windscribe") ||
			strings.Contains(name, "wireguard") ||
			strings.Contains(name, "wintun") ||
			strings.Contains(name, "tap")

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				ipStr := ip.String()
				if !isVirtual && strings.HasPrefix(ipStr, "192.168.") {
					lan192 = append(lan192, ipStr)
				} else if !isVirtual && (strings.HasPrefix(ipStr, "10.") || strings.HasPrefix(ipStr, "172.")) {
					lanPrivate = append(lanPrivate, ipStr)
				} else {
					others = append(others, ipStr)
				}
			}
		}
	}
	result := append(lan192, lanPrivate...)
	return append(result, others...)
}

type DirItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *Server) handleListDirectories(w http.ResponseWriter, r *http.Request) {
	targetPath := r.URL.Query().Get("path")
	if targetPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			targetPath = home
		} else {
			targetPath = "C:\\"
		}
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var dirs []DirItem
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, DirItem{
				Name: entry.Name(),
				Path: filepath.Join(targetPath, entry.Name()),
			})
		}
	}

	parent := filepath.Dir(targetPath)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"current": targetPath,
		"parent":  parent,
		"dirs":    dirs,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.manager.ListSessions()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

type CreateSessionReq struct {
	Name     string `json:"name"`
	Cwd      string `json:"cwd"`
	Command  string `json:"command"`
	Cols     uint16 `json:"cols"`
	Rows     uint16 `json:"rows"`
	BubbleID string `json:"bubbleId,omitempty"`
	Role     string `json:"role,omitempty"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Cwd == "" {
		req.Cwd, _ = os.UserHomeDir()
	} else {
		// Verify directory exists
		if stat, err := os.Stat(req.Cwd); err != nil || !stat.IsDir() {
			http.Error(w, "invalid working directory", http.StatusBadRequest)
			return
		}
	}

	if req.Command == "" {
		req.Command = s.cfg.DefaultShell
	}
	if req.Role == "" {
		req.Role = "shell"
	}

	sess, err := s.manager.CreateBubbleSession(req.Name, req.Cwd, req.Command, req.BubbleID, req.Role, req.Cols, req.Rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sess.ToInfo())
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.manager.DeleteSession(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type RenameSessionRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleRenameSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error": "id is required"}`, http.StatusBadRequest)
		return
	}
	var req RenameSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid json body"}`, http.StatusBadRequest)
		return
	}
	trimmed := strings.TrimSpace(req.Name)
	if trimmed == "" {
		http.Error(w, `{"error": "name cannot be empty"}`, http.StatusBadRequest)
		return
	}
	if err := s.manager.RenameSession(id, trimmed); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": id, "name": trimmed})
}

type WsIncomingMsg struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data"` // string data for input
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.URL.Query().Get("token")
	if s.cfg.AuthToken != "" && token != s.cfg.AuthToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sess, ok := s.manager.GetSession(id)
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS %s] Upgrade error: %v", id, err)
		return
	}
	defer conn.Close()

	// Track connection in manager
	connID := uuid.New().String()
	clientConn := &session.ClientConnection{
		ID:          connID,
		SessionID:   id,
		SessionName: sess.Name,
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		ConnectedAt: time.Now(),
	}
	s.manager.RegisterConnection(clientConn)
	defer s.manager.UnregisterConnection(connID)

	ch, history := sess.Subscribe()

	// Send initial terminal history
	if len(history) > 0 {
		if err := conn.WriteMessage(websocket.BinaryMessage, history); err != nil {
			sess.Unsubscribe(ch)
			return
		}
	}

	// Goroutine for sending PTY output to WebSocket
	done := make(chan struct{})
	go func() {
		defer close(done)
		for data := range ch {
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return
			}
		}
	}()

	// Loop for reading client input/resizing from WebSocket
	for {
		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if msgType == websocket.BinaryMessage {
			_ = sess.WriteInput(payload)
		} else if msgType == websocket.TextMessage {
			var msg WsIncomingMsg
			if err := json.Unmarshal(payload, &msg); err == nil {
				switch msg.Type {
				case "input":
					_ = sess.WriteInput([]byte(msg.Data))
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						_ = sess.Resize(msg.Cols, msg.Rows)
					}
				}
			} else {
				// If not JSON, treat raw text as input
				_ = sess.WriteInput(payload)
			}
		}
	}

	sess.Unsubscribe(ch)
	<-done
}

// Bubble & Eco Handlers

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	ws := s.bubbleMgr.ListWorkspaces()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ws)
}

func (s *Server) handleListBubbles(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("scan") == "true" {
		s.bubbleMgr.ScanExistingBubbles()
	}
	bubbles := s.bubbleMgr.ListBubbles()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bubbles)
}

func (s *Server) handleCreateBubble(w http.ResponseWriter, r *http.Request) {
	var req bubble.CreateBubbleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	b, err := s.bubbleMgr.CreateBubble(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(b)
}

func (s *Server) handleDeleteBubble(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.manager.DeleteBubbleSessions(id)
	if err := s.bubbleMgr.DeleteBubble(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type UpdateBubbleMetaReq struct {
	Priority   *string `json:"priority,omitempty"`
	CustomRank *int    `json:"customRank,omitempty"`
}

func (s *Server) handleUpdateBubbleMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing bubble id", http.StatusBadRequest)
		return
	}
	var req UpdateBubbleMetaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.bubbleMgr.UpdateBubbleMetadata(id, req.Priority, req.CustomRank); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b, ok := s.bubbleMgr.GetBubble(id)
	if !ok {
		http.Error(w, "bubble not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}

type ReorderBubblesReq struct {
	Order []string `json:"order"`
}

func (s *Server) handleReorderBubbles(w http.ResponseWriter, r *http.Request) {
	var req ReorderBubblesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.bubbleMgr.ReorderBubbles(req.Order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleStartDevServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pid, err := s.bubbleMgr.StartDevServer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "started", "pid": pid})
}

func (s *Server) handleStopDevServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.bubbleMgr.StopDevServer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "stopped"})
}

func (s *Server) handleGetGitStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, ok := s.bubbleMgr.GetBubble(id)
	if !ok {
		http.Error(w, "bubble not found", http.StatusNotFound)
		return
	}
	st, err := git.GetStatus(b.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) handleGetGitDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, ok := s.bubbleMgr.GetBubble(id)
	if !ok {
		http.Error(w, "bubble not found", http.StatusNotFound)
		return
	}
	filePath := r.URL.Query().Get("file")
	diff, err := git.GetDiff(b.Path, filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"diff": diff})
}

func (s *Server) handleGetGitLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, ok := s.bubbleMgr.GetBubble(id)
	if !ok {
		http.Error(w, "bubble not found", http.StatusNotFound)
		return
	}
	commits, err := git.GetRecentLog(b.Path, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(commits)
}

func (s *Server) handleListGitBranches(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, ok := s.bubbleMgr.GetBubble(id)
	if !ok {
		http.Error(w, "bubble not found", http.StatusNotFound)
		return
	}
	branches, err := git.ListBranches(b.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(branches)
}

func (s *Server) handleCheckoutGitBranch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, ok := s.bubbleMgr.GetBubble(id)
	if !ok {
		http.Error(w, "bubble not found", http.StatusNotFound)
		return
	}
	var body struct {
		Branch string `json:"branch"`
		Create bool   `json:"create"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := git.CheckoutBranch(b.Path, body.Branch, body.Create); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b.Branch = body.Branch
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "branch": body.Branch})
}

// Obsidian Handlers

func (s *Server) handleListVaults(w http.ResponseWriter, r *http.Request) {
	vaults := s.obsidianMgr.ListVaults()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(vaults)
}

func (s *Server) handleSearchNotes(w http.ResponseWriter, r *http.Request) {
	vault := r.PathValue("vault")
	q := r.URL.Query().Get("q")
	notes, err := s.obsidianMgr.SearchNotes(vault, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(notes)
}

func (s *Server) handleReadNote(w http.ResponseWriter, r *http.Request) {
	vault := r.PathValue("vault")
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path query parameter is required", http.StatusBadRequest)
		return
	}
	note, err := s.obsidianMgr.ReadNote(vault, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(note)
}

func (s *Server) handleSaveNote(w http.ResponseWriter, r *http.Request) {
	vault := r.PathValue("vault")
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.obsidianMgr.SaveNote(vault, body.Path, body.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (s *Server) handleCalendarMonth(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if y, err := strconv.Atoi(yearStr); err == nil && y > 2000 {
		year = y
	}
	if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
		month = m
	}

	summary, err := s.calendarMgr.GetMonthSummary(year, month)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleCalendarGetNote(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	note, err := s.calendarMgr.GetDayNote(date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(note)
}

func (s *Server) handleCalendarSaveNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date    string `json:"date"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Date == "" {
		body.Date = time.Now().Format("2006-01-02")
	}

	if err := s.calendarMgr.SaveDayNote(body.Date, body.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved", "date": body.Date})
}

func (s *Server) handleCalendarHandoff(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date     string `json:"date"`
		BubbleID string `json:"bubbleId"`
		TaskName string `json:"taskName"`
		Agent    string `json:"agent"`
		Status   string `json:"status"`
		Summary  string `json:"summary"`
		Pending  string `json:"pending"`
		Blockers string `json:"blockers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	workDir := ""
	if body.BubbleID != "" {
		if b, ok := s.bubbleMgr.GetBubble(body.BubbleID); ok && b != nil {
			workDir = b.Path
			if body.TaskName == "" {
				body.TaskName = b.Name
			}
		}
	}

	item := calendar.HandoffItem{
		Date:     body.Date,
		BubbleID: body.BubbleID,
		TaskName: body.TaskName,
		Agent:    body.Agent,
		Status:   body.Status,
		Summary:  body.Summary,
		Pending:  body.Pending,
		Blockers: body.Blockers,
	}

	created, err := s.calendarMgr.AddHandoff(item, workDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleCalendarStandup(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	standup, err := s.calendarMgr.GetStandup(date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(standup)
}

// Helpers
var _ = fs.ErrNotExist
