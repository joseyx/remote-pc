package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"orbit/internal/bubble"
	"orbit/internal/config"
	"orbit/internal/daemon"
	"orbit/internal/mcp"
	"orbit/internal/obsidian"
	"orbit/internal/vpn"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
)

type CheckItem struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Status      Status `json:"status"` // "ok", "warning", "error"
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

type Report struct {
	Timestamp      string      `json:"timestamp"`
	OrbitVersion   string      `json:"orbitVersion"`
	Passed         int         `json:"passed"`
	Warnings       int         `json:"warnings"`
	Errors         int         `json:"errors"`
	IsReady        bool        `json:"isReady"`
	Items          []CheckItem `json:"items"`
}

// RunDiagnostics executes comprehensive environment and integration checks.
func RunDiagnostics(cfg *config.Config, version string) *Report {
	if cfg == nil {
		cfg = config.LoadConfig()
	}

	report := &Report{
		Timestamp:    time.Now().Format(time.RFC3339),
		OrbitVersion: version,
		Items:        make([]CheckItem, 0, 10),
	}

	// 1. Operating System
	report.Items = append(report.Items, checkOS(version))

	// 2. Terminal PTY Engine
	report.Items = append(report.Items, checkPTYPlatform())

	// 3. Git CLI
	report.Items = append(report.Items, checkGit())

	// 4. Claude Code CLI MCP
	report.Items = append(report.Items, checkClaudeCode())

	// 5. Claude Desktop MCP
	report.Items = append(report.Items, checkClaudeDesktop())

	// 6. Obsidian Vaults
	report.Items = append(report.Items, checkObsidian())

	// 7. Network & Port
	report.Items = append(report.Items, checkNetwork(cfg))

	// 8. Windscribe VPN CLI
	report.Items = append(report.Items, checkVPN())

	// 9. Data Directory & Permissions
	report.Items = append(report.Items, checkStorage(cfg))

	// 10. Orbit Daemon Active Status
	report.Items = append(report.Items, checkDaemon(cfg))

	// 11. Universal Workspaces Abstraction
	report.Items = append(report.Items, checkWorkspaces(cfg))

	// Count metrics
	for _, it := range report.Items {
		switch it.Status {
		case StatusOK:
			report.Passed++
		case StatusWarning:
			report.Warnings++
		case StatusError:
			report.Errors++
		}
	}
	report.IsReady = report.Errors == 0

	return report
}

func checkOS(version string) CheckItem {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}
	osDesc := fmt.Sprintf("%s (%s)", runtime.GOOS, runtime.GOARCH)
	switch runtime.GOOS {
	case "windows":
		osDesc = fmt.Sprintf("Windows (%s)", runtime.GOARCH)
	case "darwin":
		osDesc = fmt.Sprintf("macOS (%s)", runtime.GOARCH)
	case "linux":
		osDesc = fmt.Sprintf("Linux (%s)", runtime.GOARCH)
	}

	return CheckItem{
		Category: "Sistema",
		Name:     "Sistema Operativo y Arquitectura",
		Status:   StatusOK,
		Message:  fmt.Sprintf("%s - Host: %s", osDesc, hostname),
		Details:  fmt.Sprintf("Orbit v%s compilado con %s", version, runtime.Version()),
	}
}

func checkGit() CheckItem {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return CheckItem{
			Category:    "Herramientas",
			Name:        "Git CLI",
			Status:      StatusWarning,
			Message:     "Git no encontrado en PATH",
			Details:     "Las funciones de ramas, worktrees y diffs no estarán disponibles",
			Remediation: "Instala Git desde https://git-scm.com/ y agrégalo al PATH del sistema",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gitPath, "--version")
	out, err := cmd.Output()
	versionStr := strings.TrimSpace(string(out))
	if err != nil || versionStr == "" {
		versionStr = "Git detectado en " + gitPath
	}

	return CheckItem{
		Category: "Herramientas",
		Name:     "Git CLI",
		Status:   StatusOK,
		Message:  versionStr,
		Details:  fmt.Sprintf("Ruta binaria: %s", gitPath),
	}
}

func checkClaudeCode() CheckItem {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("USERPROFILE")
	}

	if home == "" {
		return CheckItem{
			Category:    "Agentes IA (MCP)",
			Name:        "Claude Code CLI",
			Status:      StatusWarning,
			Message:     "No se pudo determinar directorio home del usuario",
			Remediation: "Verifica las variables HOME o USERPROFILE",
		}
	}

	claudeJsonPath := filepath.Join(home, ".claude.json")
	data, err := os.ReadFile(claudeJsonPath)
	if err != nil {
		return CheckItem{
			Category:    "Agentes IA (MCP)",
			Name:        "Claude Code CLI",
			Status:      StatusWarning,
			Message:     "No se detectó configuración previa en ~/.claude.json",
			Details:     "Claude Code CLI no tiene registrado el servidor Orbit MCP",
			Remediation: "Ejecuta 'orbit setup-mcp' para registrar el servidor automáticamente",
		}
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return CheckItem{
			Category:    "Agentes IA (MCP)",
			Name:        "Claude Code CLI",
			Status:      StatusWarning,
			Message:     "Archivo ~/.claude.json contiene JSON inválido",
			Remediation: "Revisa la sintaxis de ~/.claude.json o ejecuta 'orbit setup-mcp'",
		}
	}

	mcpServers, _ := root["mcpServers"].(map[string]interface{})
	hasOrbit := false
	if mcpServers != nil {
		if _, ok := mcpServers["orbit"]; ok {
			hasOrbit = true
		} else if _, ok := mcpServers["remote-pc"]; ok {
			hasOrbit = true
		}
	}

	if hasOrbit {
		return CheckItem{
			Category: "Agentes IA (MCP)",
			Name:     "Claude Code CLI",
			Status:   StatusOK,
			Message:  "Servidor Orbit MCP registrado en ~/.claude.json",
			Details:  "Claude Code puede orquestar terminales, Git y Obsidian sin configuración extra",
		}
	}

	return CheckItem{
		Category:    "Agentes IA (MCP)",
		Name:        "Claude Code CLI",
		Status:      StatusWarning,
		Message:     "Servidor Orbit MCP no registrado en ~/.claude.json",
		Details:     "Archivo presente pero entrada 'orbit' no configurada",
		Remediation: "Ejecuta 'orbit setup-mcp' para agregarlo automáticamente",
	}
}

func checkClaudeDesktop() CheckItem {
	configPath := mcp.GetClaudeDesktopConfigPath()
	if configPath == "" {
		return CheckItem{
			Category: "Agentes IA (MCP)",
			Name:     "Claude Desktop",
			Status:   StatusWarning,
			Message:  "Ruta de configuración de Claude Desktop no determinada para este SO",
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return CheckItem{
			Category:    "Agentes IA (MCP)",
			Name:        "Claude Desktop",
			Status:      StatusWarning,
			Message:     "Claude Desktop no configurado o no instalado",
			Details:     fmt.Sprintf("Archivo no encontrado: %s", configPath),
			Remediation: "Si utilizas Claude Desktop, ejecuta 'orbit setup-mcp' para vincularlo",
		}
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return CheckItem{
			Category:    "Agentes IA (MCP)",
			Name:        "Claude Desktop",
			Status:      StatusWarning,
			Message:     "Formato JSON inválido en claude_desktop_config.json",
			Remediation: "Revisa la sintaxis o ejecuta 'orbit setup-mcp'",
		}
	}

	mcpServers, _ := root["mcpServers"].(map[string]interface{})
	hasOrbit := false
	if mcpServers != nil {
		if _, ok := mcpServers["orbit"]; ok {
			hasOrbit = true
		} else if _, ok := mcpServers["remote-pc"]; ok {
			hasOrbit = true
		}
	}

	if hasOrbit {
		return CheckItem{
			Category: "Agentes IA (MCP)",
			Name:     "Claude Desktop",
			Status:   StatusOK,
			Message:  "Servidor Orbit MCP registrado en Claude Desktop",
			Details:  fmt.Sprintf("Configurado en %s", configPath),
		}
	}

	return CheckItem{
		Category:    "Agentes IA (MCP)",
		Name:        "Claude Desktop",
		Status:      StatusWarning,
		Message:     "Claude Desktop instalado pero sin Orbit MCP vinculado",
		Details:     fmt.Sprintf("Configuración: %s", configPath),
		Remediation: "Ejecuta 'orbit setup-mcp' para vincularlo con un solo comando",
	}
}

func checkObsidian() CheckItem {
	mgr := obsidian.NewManager()
	vaults := mgr.ListVaults()

	if len(vaults) > 0 {
		names := make([]string, 0, len(vaults))
		for _, v := range vaults {
			names = append(names, v.Name)
		}
		return CheckItem{
			Category: "Obsidian",
			Name:     "Bóvedas de Notas",
			Status:   StatusOK,
			Message:  fmt.Sprintf("%d bóvedas detectadas (%s)", len(vaults), strings.Join(names, ", ")),
			Details:  "Indexación y búsqueda de notas técnicas habilitadas",
		}
	}

	return CheckItem{
		Category: "Obsidian",
		Name:     "Bóvedas de Notas",
		Status:   StatusOK,
		Message:  "No se detectaron bóvedas de Obsidian en rutas estándar",
		Details:  "Si usas Obsidian en una ruta personalizada, configúrala en server.config.json",
	}
}

func checkNetwork(cfg *config.Config) CheckItem {
	lanIps := getLANIPs()
	ipSummary := "localhost"
	if len(lanIps) > 0 {
		ipSummary = strings.Join(lanIps, ", ")
	}

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return CheckItem{
			Category: "Red",
			Name:     "Puertos y Conectividad",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Puerto %s en uso (LAN: %s)", port, ipSummary),
			Details:  "El puerto está abierto y listo para conexiones remotas",
		}
	}

	return CheckItem{
		Category: "Red",
		Name:     "Puertos y Conectividad",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Puerto %s disponible para escuchar (LAN: %s)", port, ipSummary),
		Details:  "El puerto local está libre para iniciar el daemon",
	}
}

func checkVPN() CheckItem {
	mgr := vpn.NewManager()
	st := mgr.Status()

	if st.Installed {
		msg := "Windscribe CLI instalada"
		if st.Connected {
			msg = fmt.Sprintf("Windscribe CLI conectada (%s - %s)", st.Location, st.IP)
		} else {
			msg = "Windscribe CLI detectada (Túnel actualmente desconectado)"
		}
		return CheckItem{
			Category: "VPN",
			Name:     "Windscribe Tunnel",
			Status:   StatusOK,
			Message:  msg,
			Details:  "Control seguro de túnel habilitado desde Cockpit y Android",
		}
	}

	return CheckItem{
		Category:    "VPN",
		Name:        "Windscribe Tunnel",
		Status:      StatusWarning,
		Message:     "CLI de Windscribe no detectada en rutas estándar",
		Details:     "La integración con túnel VPN estará en modo manual",
		Remediation: "Instala Windscribe Desktop para habilitar el control de VPN integrado",
	}
}

func checkStorage(cfg *config.Config) CheckItem {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = config.GetDataDir()
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return CheckItem{
			Category:    "Almacenamiento",
			Name:        "Directorio de Datos",
			Status:      StatusError,
			Message:     fmt.Sprintf("No se pudo crear carpeta de datos: %v", err),
			Details:     fmt.Sprintf("Ruta: %s", dataDir),
			Remediation: "Verifica permisos de usuario en el disco",
		}
	}

	probeFile := filepath.Join(dataDir, ".orbit_probe_write")
	if err := os.WriteFile(probeFile, []byte("ok"), 0644); err != nil {
		return CheckItem{
			Category:    "Almacenamiento",
			Name:        "Directorio de Datos",
			Status:      StatusError,
			Message:     fmt.Sprintf("Fallo de escritura en datos: %v", err),
			Details:     fmt.Sprintf("Ruta: %s", dataDir),
			Remediation: "Otorga permisos de escritura al directorio de datos",
		}
	}
	_ = os.Remove(probeFile)

	absPath, _ := filepath.Abs(dataDir)
	return CheckItem{
		Category: "Almacenamiento",
		Name:     "Directorio de Datos",
		Status:   StatusOK,
		Message:  "Permisos de lectura y escritura verificados",
		Details:  fmt.Sprintf("Almacén central: %s", absPath),
	}
}

func checkDaemon(cfg *config.Config) CheckItem {
	st := daemon.GetStatus(cfg, "")
	port := st.Port

	if st.Running {
		details := "Orbit Cockpit y la API REST están operando en línea"
		if st.Installed {
			details = fmt.Sprintf("Servicio registrado para autoarranque (%s). Proceso activo.", st.ServiceType)
		} else {
			details = "Ejecutándose en primer plano (Usa 'orbit service install' para autoarranque silencioso)."
		}
		msg := fmt.Sprintf("Servidor activo y respondiendo en http://localhost:%s/", port)
		if st.PID > 0 {
			msg = fmt.Sprintf("Servidor activo en http://localhost:%s/ (PID: %d)", port, st.PID)
		}
		return CheckItem{
			Category: "Orbit",
			Name:     "Daemon Server",
			Status:   StatusOK,
			Message:  msg,
			Details:  details,
		}
	}

	remediation := "Ejecuta 'orbit service start' para iniciar en segundo plano o 'orbit' en consola"
	if !st.Installed {
		remediation = "Ejecuta 'orbit service install' para autoarranque con el sistema o 'orbit' en consola"
	}

	return CheckItem{
		Category:    "Orbit",
		Name:        "Daemon Server",
		Status:      StatusWarning,
		Message:     fmt.Sprintf("Daemon no está corriendo en el puerto %s", port),
		Details:     "La interfaz web y los sockets no están activos en este momento",
		Remediation: remediation,
	}
}

func getLANIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
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
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

func checkWorkspaces(cfg *config.Config) CheckItem {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = config.GetDataDir()
	}

	wsFile := filepath.Join(dataDir, "workspaces.json")
	data, err := os.ReadFile(wsFile)
	if err != nil {
		return CheckItem{
			Category:    "Workspaces",
			Name:        "Abstracción de Workspaces",
			Status:      StatusWarning,
			Message:     "workspaces.json no inicializado aún",
			Details:     "Los proyectos se cargarán en modo por defecto o con 'orbit migrate'",
			Remediation: "Ejecuta 'orbit migrate' para auto-descubrir y registrar tus repositorios en disco",
		}
	}

	var list []bubble.WorkspaceInfo
	if err := json.Unmarshal(data, &list); err != nil {
		return CheckItem{
			Category:    "Workspaces",
			Name:        "Abstracción de Workspaces",
			Status:      StatusError,
			Message:     fmt.Sprintf("Error leyendo workspaces.json: %v", err),
			Details:     fmt.Sprintf("Ruta: %s", wsFile),
			Remediation: "Verifica la sintaxis JSON de workspaces.json o ejecuta 'orbit migrate'",
		}
	}

	names := make([]string, 0, len(list))
	for _, ws := range list {
		names = append(names, ws.Name)
	}

	return CheckItem{
		Category: "Workspaces",
		Name:     "Abstracción de Workspaces",
		Status:   StatusOK,
		Message:  fmt.Sprintf("%d proyectos registrados en workspaces.json", len(list)),
		Details:  fmt.Sprintf("Proyectos activos: %s", strings.Join(names, ", ")),
	}
}

// FormatCLI formats the diagnostic report as a clean ANSI console text.
func FormatCLI(report *Report) string {
	var b strings.Builder

	b.WriteString("=====================================================\n")
	b.WriteString("            Orbit Doctor - Diagnóstico del Entorno    \n")
	b.WriteString("=====================================================\n\n")

	for _, item := range report.Items {
		symbol := "[✓]"
		switch item.Status {
		case StatusWarning:
			symbol = "[!]"
		case StatusError:
			symbol = "[✗]"
		}

		b.WriteString(fmt.Sprintf("%s %-25s : %s\n", symbol, item.Name, item.Message))
		if item.Details != "" && item.Status != StatusOK {
			b.WriteString(fmt.Sprintf("    Detalle    : %s\n", item.Details))
		}
		if item.Remediation != "" {
			b.WriteString(fmt.Sprintf("    Sugerencia : %s\n", item.Remediation))
		}
	}

	b.WriteString("\n-----------------------------------------------------\n")
	if report.Errors == 0 && report.Warnings == 0 {
		b.WriteString(fmt.Sprintf("✓ Diagnóstico: %d verificaciones superadas al 100%%. ¡Todo listo!\n", report.Passed))
	} else if report.Errors == 0 {
		b.WriteString(fmt.Sprintf("✓ Diagnóstico: %d superadas, %d advertencias, 0 errores.\n", report.Passed, report.Warnings))
		b.WriteString("  Orbit está completamente operativo (las advertencias son opcionales).\n")
	} else {
		b.WriteString(fmt.Sprintf("✗ Diagnóstico: %d superadas, %d advertencias, %d errores.\n", report.Passed, report.Warnings, report.Errors))
		b.WriteString("  Por favor revisa los errores señalados con [✗] para operar.\n")
	}
	b.WriteString("=====================================================\n")

	return b.String()
}
