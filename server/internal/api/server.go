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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"remotepc-server/internal/config"
	"remotepc-server/internal/session"
	"remotepc-server/internal/vpn"
	"remotepc-server/internal/web"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow mobile/remote origins
	},
}

type Server struct {
	cfg       *config.Config
	manager   *session.Manager
	vpnMgr    *vpn.Manager
	startedAt time.Time
}

func NewServer(cfg *config.Config, manager *session.Manager) *Server {
	return &Server{
		cfg:       cfg,
		manager:   manager,
		vpnMgr:    vpn.NewManager(),
		startedAt: time.Now(),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/server-info", s.handleServerInfo)
	mux.HandleFunc("GET /api/connections", s.withAuth(s.handleListConnections))
	mux.HandleFunc("GET /api/fs/directories", s.withAuth(s.handleListDirectories))
	mux.HandleFunc("GET /api/sessions", s.withAuth(s.handleListSessions))
	mux.HandleFunc("POST /api/sessions", s.withAuth(s.handleCreateSession))
	mux.HandleFunc("DELETE /api/sessions/{id}", s.withAuth(s.handleDeleteSession))
	mux.HandleFunc("/ws/session/{id}", s.handleWebSocket)

	// VPN Controls
	mux.HandleFunc("GET /api/vpn/status", s.withAuth(s.handleVPNStatus))
	mux.HandleFunc("GET /api/vpn/options", s.withAuth(s.handleVPNOptions))
	mux.HandleFunc("POST /api/vpn/connect", s.withAuth(s.handleVPNConnect))
	mux.HandleFunc("POST /api/vpn/disconnect", s.withAuth(s.handleVPNDisconnect))

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
	Name    string `json:"name"`
	Cwd     string `json:"cwd"`
	Command string `json:"command"`
	Cols    uint16 `json:"cols"`
	Rows    uint16 `json:"rows"`
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

	sess, err := s.manager.CreateSession(req.Name, req.Cwd, req.Command, req.Cols, req.Rows)
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

// Helpers
var _ = fs.ErrNotExist
