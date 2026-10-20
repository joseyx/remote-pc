package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"orbit/internal/bubble"
	"orbit/internal/calendar"
	"orbit/internal/config"
	"orbit/internal/daemon"
	"orbit/internal/doctor"
	"orbit/internal/git"
	"orbit/internal/obsidian"
	"orbit/internal/plugin"
	"orbit/internal/services"
	"orbit/internal/session"
	"orbit/internal/system"
	"orbit/internal/updater"
	"orbit/internal/vpn"
	"orbit/internal/web"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow mobile/remote origins
	},
}

type Server struct {
	mu          sync.RWMutex
	cfg         *config.Config
	manager     *session.Manager
	vpnMgr      *vpn.Manager
	bubbleMgr   *bubble.Manager
	obsidianMgr *obsidian.Manager
	calendarMgr *calendar.Manager
	pluginMgr   *plugin.Manager
	startedAt   time.Time
	version     string
}

func NewServer(cfg *config.Config, manager *session.Manager, version string) *Server {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = config.GetDataDir()
	}
	pluginMgr := plugin.NewManager(dataDir)
	plugin.SetGlobalManager(pluginMgr)

	return &Server{
		cfg:         cfg,
		manager:     manager,
		vpnMgr:      vpn.NewManager(),
		bubbleMgr:   bubble.NewManager(dataDir),
		obsidianMgr: obsidian.NewManager(),
		calendarMgr: calendar.NewManager(filepath.Join(dataDir, "calendar_notes")),
		pluginMgr:   pluginMgr,
		startedAt:   time.Now(),
		version:     version,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/server-info", s.handleServerInfo)
	mux.HandleFunc("GET /api/system/doctor", s.handleSystemDoctor)
	mux.HandleFunc("GET /api/system/version", s.handleSystemVersion)
	mux.HandleFunc("POST /api/system/update", s.withAuth(s.handleSystemUpdate))
	mux.HandleFunc("GET /api/system/service", s.handleSystemServiceStatus)
	mux.HandleFunc("POST /api/system/service", s.withAuth(s.handleSystemServiceAction))
	mux.HandleFunc("GET /api/system/security", s.withAuth(s.handleGetSecurityStatus))
	mux.HandleFunc("POST /api/system/token/rotate", s.withAuth(s.handleRotateToken))
	mux.HandleFunc("POST /api/system/pin/set", s.withAuth(s.handleSetPIN))
	mux.HandleFunc("POST /api/system/pin/remove", s.withAuth(s.handleRemovePIN))
	mux.HandleFunc("POST /api/system/pin/verify", s.handleVerifyPIN)
	mux.HandleFunc("GET /api/system/stats", s.handleSystemStats)
	mux.HandleFunc("POST /api/system/process/kill", s.withAuth(s.handleKillProcess))
	mux.HandleFunc("POST /api/services/{id}/start", s.withAuth(s.handleStartService))
	mux.HandleFunc("POST /api/services/{id}/stop", s.withAuth(s.handleStopService))
	mux.HandleFunc("GET /api/connections", s.withAuth(s.handleListConnections))
	mux.HandleFunc("GET /api/fs/directories", s.withAuth(s.handleListDirectories))
	mux.HandleFunc("GET /api/sessions", s.withAuth(s.handleListSessions))
	mux.HandleFunc("POST /api/sessions", s.withAuth(s.handleCreateSession))
	mux.HandleFunc("GET /api/sessions/{id}/history", s.withAuth(s.handleGetSessionHistory))
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
	mux.HandleFunc("POST /api/workspaces", s.withAuth(s.handleCreateWorkspace))
	mux.HandleFunc("PUT /api/workspaces/{id}", s.withAuth(s.handleUpdateWorkspace))
	mux.HandleFunc("DELETE /api/workspaces/{id}", s.withAuth(s.handleDeleteWorkspace))
	mux.HandleFunc("GET /api/workspaces/{id}/branches", s.withAuth(s.handleGetWorkspaceBranches))
	mux.HandleFunc("POST /api/workspaces/{id}/link-repo", s.withAuth(s.handleLinkWorkspaceRepo))
	mux.HandleFunc("GET /api/system/sql-dumps", s.withAuth(s.handleListSqlDumps))
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
	mux.HandleFunc("POST /api/calendar/todo", s.withAuth(s.handleCalendarAddTodo))
	mux.HandleFunc("POST /api/calendar/note/append", s.withAuth(s.handleCalendarAppendNote))
	mux.HandleFunc("POST /api/calendar/handoff", s.withAuth(s.handleCalendarHandoff))
	mux.HandleFunc("PUT /api/calendar/handoff", s.withAuth(s.handleCalendarHandoffUpdate))
	mux.HandleFunc("DELETE /api/calendar/handoff", s.withAuth(s.handleCalendarHandoffDelete))
	mux.HandleFunc("GET /api/calendar/handoff/draft", s.withAuth(s.handleCalendarHandoffDraft))
	mux.HandleFunc("POST /api/calendar/handoff/auto", s.withAuth(s.handleCalendarHandoffAuto))
	mux.HandleFunc("POST /api/calendar/handoff/prompt-agent", s.withAuth(s.handleCalendarHandoffPromptAgent))
	mux.HandleFunc("GET /api/calendar/standup", s.withAuth(s.handleCalendarStandup))

	// Plugin Routes
	mux.HandleFunc("GET /api/plugins", s.withAuth(s.handleListPlugins))
	mux.HandleFunc("POST /api/plugins/{id}/toggle", s.withAuth(s.handleTogglePlugin))
	mux.HandleFunc("POST /api/plugins/{id}/config", s.withAuth(s.handleUpdatePluginConfig))
	mux.HandleFunc("POST /api/plugins/groq-whisper/transcribe", s.withAuth(s.handleGroqWhisperTranscribe))

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

		s.mu.RLock()
		expectedToken := s.cfg.AuthToken
		s.mu.RUnlock()

		if expectedToken != "" && token != expectedToken {
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
	withTop := r.URL.Query().Get("top") == "true"
	stats := system.GetStats(withTop)
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
	Version      string      `json:"version"`
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

	s.mu.RLock()
	curToken := s.cfg.AuthToken
	s.mu.RUnlock()

	info := ServerInfoResponse{
		Version:      s.version,
		Hostname:     hostname,
		Port:         s.cfg.Port,
		LanIps:       getLANIPs(),
		AuthToken:    curToken,
		Uptime:       uptimeStr,
		StartedAt:    s.startedAt,
		SessionCount: len(s.manager.ListSessions()),
		ClientCount:  len(s.manager.ListConnections()),
		VPN:          &vpnStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleSystemDoctor(w http.ResponseWriter, r *http.Request) {
	report := doctor.RunDiagnostics(s.cfg, s.version)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

func (s *Server) handleSystemVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	check := r.URL.Query().Get("check") != "false"

	if !check {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentVersion":  s.version,
			"updateAvailable": false,
		})
		return
	}

	info, err := updater.CheckUpdate(s.version)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentVersion":  s.version,
			"latestVersion":   s.version,
			"updateAvailable": false,
			"error":           err.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleSystemUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Force bool `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	info, err := updater.CheckUpdate(s.version)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": fmt.Sprintf("Error al verificar actualizaciones: %v", err),
		})
		return
	}

	if !info.UpdateAvailable && !req.Force {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"message": fmt.Sprintf("Orbit ya está en la versión más reciente (v%s)", s.version),
			"version": s.version,
		})
		return
	}

	if err := updater.ApplyUpdate(info); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": fmt.Sprintf("Error aplicando actualización: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Actualización completada a v%s. Reiniciando servidor...", info.LatestVersion),
		"version": info.LatestVersion,
	})

	// Graceful restart in background
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if err := s.manager.SaveState(); err != nil {
			log.Printf("Error saving session state prior to update restart: %v", err)
		}
		s.manager.CloseAllPTYs()

		exePath, err := os.Executable()
		if err != nil {
			exePath = os.Args[0]
		}

		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Start(); err != nil {
			log.Printf("Error restarting after update: %v", err)
			return
		}
		os.Exit(0)
	}()
}

func (s *Server) handleSystemServiceStatus(w http.ResponseWriter, r *http.Request) {
	st := daemon.GetStatus(s.cfg, s.version)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) handleSystemServiceAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"` // "install", "uninstall", "start", "stop", "restart"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "Payload JSON inválido"})
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}

	var actionErr error
	var msg string

	switch req.Action {
	case "install":
		actionErr = daemon.Install(s.cfg, exePath)
		msg = "Orbit registrado exitosamente para autoarranque con el sistema."
	case "uninstall":
		actionErr = daemon.Uninstall(s.cfg)
		msg = "Servicio de autoarranque de Orbit desinstalado exitosamente."
	case "start":
		actionErr = daemon.Start(s.cfg, exePath)
		msg = "Orbit Daemon iniciado en segundo plano."
	case "stop":
		actionErr = daemon.Stop(s.cfg)
		msg = "Orbit Daemon detenido."
	case "restart":
		actionErr = daemon.Restart(s.cfg, exePath)
		msg = "Orbit Daemon reiniciado."
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("Acción desconocida: %s", req.Action)})
		return
	}

	if actionErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   actionErr.Error(),
		})
		return
	}

	st := daemon.GetStatus(s.cfg, s.version)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": msg,
		"status":  st,
	})
}

func (s *Server) handleGetSecurityStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":   s.cfg.AuthToken,
		"hasPin":  s.cfg.PIN != "",
		"port":    s.cfg.Port,
		"success": true,
	})
}

func (s *Server) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PIN         string `json:"pin"`
		CustomToken string `json:"customToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.PIN != "" && req.PIN != s.cfg.PIN {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "PIN de seguridad incorrecto",
		})
		return
	}

	newToken := s.cfg.RotateToken(strings.TrimSpace(req.CustomToken))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"token":   newToken,
		"message": "Token de autenticación rotado exitosamente.",
	})
}

func (s *Server) handleSetPIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPIN string `json:"currentPin"`
		NewPIN     string `json:"newPin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "Payload JSON inválido"})
		return
	}

	req.NewPIN = strings.TrimSpace(req.NewPIN)
	if len(req.NewPIN) < 4 || len(req.NewPIN) > 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "El PIN debe tener entre 4 y 8 caracteres o dígitos"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.PIN != "" && req.CurrentPIN != s.cfg.PIN {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "El PIN actual no coincide",
		})
		return
	}

	if err := s.cfg.SetPIN(req.NewPIN); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"hasPin":  true,
		"message": "PIN de seguridad configurado exitosamente.",
	})
}

func (s *Server) handleRemovePIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PIN string `json:"pin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.PIN != "" && req.PIN != s.cfg.PIN {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "PIN de seguridad incorrecto",
		})
		return
	}

	_ = s.cfg.RemovePIN()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"hasPin":  false,
		"message": "PIN de seguridad removido exitosamente.",
	})
}

func (s *Server) handleVerifyPIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PIN string `json:"pin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	s.mu.RLock()
	defer s.mu.RUnlock()

	valid := false
	if s.cfg.PIN != "" {
		valid = req.PIN == s.cfg.PIN
	} else if s.cfg.AuthToken != "" {
		valid = req.PIN == s.cfg.AuthToken
	} else {
		valid = true
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"valid":   valid,
		"hasPin":  s.cfg.PIN != "",
		"message": map[bool]string{true: "PIN verificado correctamente", false: "PIN incorrecto"}[valid],
	})
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

	if req.Role == "server" && req.BubbleID != "" {
		s.bubbleMgr.SetDevRunning(req.BubbleID, true, sess.PID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sess.ToInfo())
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if sess, ok := s.manager.GetSession(id); ok && sess != nil {
		if sess.Role == "server" && sess.BubbleID != "" {
			s.bubbleMgr.SetDevRunning(sess.BubbleID, false, 0)
		}
	}
	if err := s.manager.DeleteSession(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetSessionHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.manager.GetSession(id)
	if !ok || sess == nil {
		http.Error(w, `{"error": "session not found"}`, http.StatusNotFound)
		return
	}
	hist := sess.GetHistory()
	tailLen := 4000
	if len(hist) < tailLen {
		tailLen = len(hist)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      id,
		"snippet": string(hist[len(hist)-tailLen:]),
		"length":  len(hist),
	})
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
	s.mu.RLock()
	expectedToken := s.cfg.AuthToken
	s.mu.RUnlock()

	if expectedToken != "" && token != expectedToken {
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
				_ = conn.Close() // Ensure conn.ReadMessage() unblocks immediately
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

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var ws bubble.WorkspaceInfo
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.bubbleMgr.AddWorkspace(ws); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var ws bubble.WorkspaceInfo
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.bubbleMgr.UpdateWorkspace(id, ws); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.bubbleMgr.DeleteWorkspace(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleGetWorkspaceBranches(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	branches, err := s.bubbleMgr.GetWorkspaceBranches(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(branches)
}

type LinkRepoReq struct {
	RepoURL string `json:"repoUrl"`
}

func (s *Server) handleLinkWorkspaceRepo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req LinkRepoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.RepoURL == "" {
		http.Error(w, "repoUrl is required", http.StatusBadRequest)
		return
	}
	if err := s.bubbleMgr.LinkRepo(id, req.RepoURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "repoUrl": req.RepoURL})
}

func (s *Server) handleListSqlDumps(w http.ResponseWriter, r *http.Request) {
	dumps := s.bubbleMgr.DiscoverSqlDumps()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dumps)
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

func (s *Server) handleCalendarAddTodo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date    string `json:"date"`
		Text    string `json:"text"`
		Checked bool   `json:"checked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		http.Error(w, "invalid request body: text is required", http.StatusBadRequest)
		return
	}
	note, err := s.calendarMgr.AddDailyTodo(body.Date, body.Text, body.Checked)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(note)
}

func (s *Server) handleCalendarAppendNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string `json:"date"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		http.Error(w, "invalid request body: text is required", http.StatusBadRequest)
		return
	}
	note, err := s.calendarMgr.AppendDailyNote(body.Date, body.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(note)
}

func (s *Server) handleCalendarHandoff(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date     string `json:"date"`
		BubbleID string `json:"bubbleId"`
		WorkDir  string `json:"workDir"`
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
	if workDir == "" && body.WorkDir != "" {
		workDir = body.WorkDir
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

func (s *Server) handleCalendarHandoffUpdate(w http.ResponseWriter, r *http.Request) {
	var item calendar.HandoffItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if item.ID == "" {
		http.Error(w, "missing handoff id", http.StatusBadRequest)
		return
	}

	updated, err := s.calendarMgr.UpdateHandoff(item)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleCalendarHandoffDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	date := r.URL.Query().Get("date")

	if id == "" {
		var body struct {
			ID   string `json:"id"`
			Date string `json:"date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			id = body.ID
			if date == "" {
				date = body.Date
			}
		}
	}

	if id == "" {
		http.Error(w, "missing handoff id", http.StatusBadRequest)
		return
	}

	if err := s.calendarMgr.DeleteHandoff(date, id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": true,
		"id":      id,
	})
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

func (s *Server) handleCalendarHandoffDraft(w http.ResponseWriter, r *http.Request) {
	bubbleID := r.URL.Query().Get("bubbleId")
	workDir := ""
	bubbleName := "Tarea General"
	agentRole := "claude"
	terminalSnippet := ""

	if bubbleID != "" {
		if b, ok := s.bubbleMgr.GetBubble(bubbleID); ok && b != nil {
			workDir = b.Path
			bubbleName = b.Name
		}
	} else {
		// Pick first active running bubble
		for _, sess := range s.manager.ListSessions() {
			if sess.BubbleID != "" && sess.IsRunning {
				if b, ok := s.bubbleMgr.GetBubble(sess.BubbleID); ok && b != nil {
					bubbleID = b.ID
					workDir = b.Path
					bubbleName = b.Name
					agentRole = sess.Role
					break
				}
			}
		}
	}

	// Look up active session in this bubble
	for _, sessInfo := range s.manager.ListSessions() {
		if sessInfo.BubbleID == bubbleID && sessInfo.IsRunning {
			if sessInfo.Role == "claude" || sessInfo.Role == "agy" {
				agentRole = sessInfo.Role
			}
			if sess, ok := s.manager.GetSession(sessInfo.ID); ok && sess != nil {
				hist := sess.GetHistory()
				if len(hist) > 0 {
					tailLen := 3000
					if len(hist) < tailLen {
						tailLen = len(hist)
					}
					terminalSnippet = string(hist[len(hist)-tailLen:])
				}
			}
			break
		}
	}

	draft, err := s.calendarMgr.GenerateDraft(bubbleID, bubbleName, workDir, agentRole, terminalSnippet)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(draft)
}

func (s *Server) handleCalendarHandoffAuto(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BubbleID string `json:"bubbleId"`
		Date     string `json:"date"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	bubbleID := body.BubbleID
	workDir := ""
	bubbleName := "Tarea General"
	agentRole := "claude"
	terminalSnippet := ""

	if bubbleID != "" {
		if b, ok := s.bubbleMgr.GetBubble(bubbleID); ok && b != nil {
			workDir = b.Path
			bubbleName = b.Name
		}
	} else {
		for _, sess := range s.manager.ListSessions() {
			if sess.BubbleID != "" && sess.IsRunning {
				if b, ok := s.bubbleMgr.GetBubble(sess.BubbleID); ok && b != nil {
					bubbleID = b.ID
					workDir = b.Path
					bubbleName = b.Name
					agentRole = sess.Role
					break
				}
			}
		}
	}

	for _, sessInfo := range s.manager.ListSessions() {
		if sessInfo.BubbleID == bubbleID && sessInfo.IsRunning {
			if sessInfo.Role == "claude" || sessInfo.Role == "agy" {
				agentRole = sessInfo.Role
			}
			if sess, ok := s.manager.GetSession(sessInfo.ID); ok && sess != nil {
				hist := sess.GetHistory()
				if len(hist) > 0 {
					tailLen := 3000
					if len(hist) < tailLen {
						tailLen = len(hist)
					}
					terminalSnippet = string(hist[len(hist)-tailLen:])
				}
			}
			break
		}
	}

	draft, err := s.calendarMgr.GenerateDraft(bubbleID, bubbleName, workDir, agentRole, terminalSnippet)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date := body.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	item := calendar.HandoffItem{
		Date:     date,
		BubbleID: draft.BubbleID,
		TaskName: draft.TaskName,
		Agent:    draft.Agent,
		Status:   draft.Status,
		Summary:  draft.Summary,
		Pending:  draft.Pending,
		Blockers: draft.Blockers,
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

func (s *Server) handleCalendarHandoffPromptAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BubbleID string `json:"bubbleId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	bubbleID := body.BubbleID
	if bubbleID == "" {
		http.Error(w, `{"error": "bubbleId is required"}`, http.StatusBadRequest)
		return
	}

	// Find running session for this bubble
	var targetSession *session.Session
	var targetSessionInfo *session.SessionInfo

	for _, sessInfo := range s.manager.ListSessions() {
		if sessInfo.BubbleID == bubbleID && sessInfo.IsRunning {
			if sessInfo.Role == "claude" || sessInfo.Role == "agy" {
				if sess, ok := s.manager.GetSession(sessInfo.ID); ok && sess != nil {
					targetSession = sess
					targetSessionInfo = sessInfo
					break
				}
			} else if targetSession == nil {
				if sess, ok := s.manager.GetSession(sessInfo.ID); ok && sess != nil {
					targetSession = sess
					targetSessionInfo = sessInfo
				}
			}
		}
	}

	if targetSession == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "No hay ninguna terminal activa en esta tarea para solicitar el resumen.",
		})
		return
	}

	// Prompt the agent in its terminal
	agentPrompt := "\r\nPor favor redacta el resumen de cierre de jornada (handoff) de esta tarea con tu analisis y guardalo ejecutando el comando: remotepc-handoff --status in_progress --summary \"<lo completado>\" --pending \"<lo pendiente>\"\r\n"

	if err := targetSession.WriteInput([]byte(agentPrompt)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "prompted",
		"sessionId":   targetSessionInfo.ID,
		"sessionName": targetSessionInfo.Name,
		"agent":       targetSessionInfo.Role,
	})
}

// Helpers
var _ = fs.ErrNotExist

// ============================================================================
// Plugins & Extensions Handlers
// ============================================================================

func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := s.pluginMgr.ListPlugins(true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plugins)
}

func (s *Server) handleTogglePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID de plugin requerido", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON inválido", http.StatusBadRequest)
		return
	}

	if err := s.pluginMgr.Toggle(id, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p, _ := s.pluginMgr.GetPlugin(id, true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"plugin":          p,
		"requiresRestart": p != nil && p.RequiresRestart,
	})
}

func (s *Server) handleUpdatePluginConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID de plugin requerido", http.StatusBadRequest)
		return
	}

	var req struct {
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON inválido", http.StatusBadRequest)
		return
	}

	if err := s.pluginMgr.UpdateConfig(id, req.Config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p, _ := s.pluginMgr.GetPlugin(id, true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"plugin":          p,
		"message":         "Configuración actualizada correctamente",
		"requiresRestart": p != nil && p.RequiresRestart,
	})
}

func (s *Server) handleGroqWhisperTranscribe(w http.ResponseWriter, r *http.Request) {
	if !s.pluginMgr.IsEnabled("groq_whisper") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "El plugin Groq Whisper Transcriber no está habilitado. Actívalo en Orbit > Plugins.",
		})
		return
	}

	cfg := s.pluginMgr.GetConfig("groq_whisper")
	apiKey, _ := cfg["api_key"].(string)
	if strings.TrimSpace(apiKey) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "Falta configurar la Groq API Key en la configuración del plugin.",
		})
		return
	}

	model, _ := cfg["model"].(string)
	if model == "" {
		model = "whisper-large-v3-turbo"
	}

	var targetPath string
	var withTimestamps bool
	var language string
	var prompt string
	var isTempFile bool

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(150 << 20); err != nil { // 150 MB max
			http.Error(w, "Error al procesar archivo cargado: "+err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No se encontró el archivo 'file' en el formulario", http.StatusBadRequest)
			return
		}
		defer file.Close()

		tempUploadPath := filepath.Join(os.TempDir(), fmt.Sprintf("orbit_web_upload_%d_%s", time.Now().UnixNano(), header.Filename))
		outFile, err := os.Create(tempUploadPath)
		if err != nil {
			http.Error(w, "Error al crear archivo temporal: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = io.Copy(outFile, file)
		_ = outFile.Close()
		if err != nil {
			_ = os.Remove(tempUploadPath)
			http.Error(w, "Error al guardar archivo temporal: "+err.Error(), http.StatusInternalServerError)
			return
		}

		targetPath = tempUploadPath
		isTempFile = true
		withTimestamps = r.FormValue("withTimestamps") == "true"
		language = r.FormValue("language")
		prompt = r.FormValue("prompt")
	} else {
		var req struct {
			FilePath       string `json:"filePath"`
			WithTimestamps *bool  `json:"withTimestamps"`
			Language       string `json:"language"`
			Prompt         string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Payload JSON inválido", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(req.FilePath) == "" {
			http.Error(w, "El campo 'filePath' es requerido", http.StatusBadRequest)
			return
		}
		targetPath = req.FilePath
		if req.WithTimestamps != nil {
			withTimestamps = *req.WithTimestamps
		} else if defWt, ok := cfg["with_timestamps"].(bool); ok {
			withTimestamps = defWt
		}
		language = req.Language
		prompt = req.Prompt
	}

	if isTempFile {
		defer os.Remove(targetPath)
	}

	if language == "" {
		if defLang, ok := cfg["language"].(string); ok {
			language = defLang
		}
	}

	res, err := plugin.TranscribeMedia(r.Context(), targetPath, withTimestamps, prompt, apiKey, model, language)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":       true,
		"text":          res.Text,
		"formattedText": res.FormattedText,
		"language":      res.Language,
		"duration":      res.Duration,
		"segments":      res.Segments,
	})
}


