package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orbit/internal/plugin"
)

// Tool represents an MCP Tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// All registered MCP Tools
var Tools = []Tool{
	{
		Name: "save_daily_handoff",
		Description: "Registra el cierre de tarea / handoff del día actual en el Calendario de Remote PC. " +
			"Usa esta herramienta cuando el usuario pida cerrar el día, hacer el handoff, registrar el progreso diario, " +
			"o al finalizar la sesión de trabajo. Redacta tú mismo el resumen con base en todo lo avanzado en esta conversación.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Qué se logró hoy formateado en Markdown estructurado (usa viñetas con saltos de línea '- ...', backticks para `commits/tickets` y negritas; NO un solo párrafo continuo).",
				},
				"pending": map[string]interface{}{
					"type":        "string",
					"description": "Qué queda pendiente para continuar mañana formateado en Markdown (usa lista numerada con saltos de línea '1. ...\\n2. ...' o checkboxes '- [ ] ...'; NUNCA un párrafo corrido).",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"in_progress", "completed", "blocked"},
					"description": "Estado de la tarea o ticket: in_progress (en curso), completed (finalizado), o blocked (bloqueado).",
				},
				"blockers": map[string]interface{}{
					"type":        "string",
					"description": "Bloqueos, dudas de negocio, APIs de terceros caídas o credenciales faltantes si los hay.",
				},
				"task_name": map[string]interface{}{
					"type":        "string",
					"description": "Nombre de la tarea o ticket (opcional; si se omite se detecta automáticamente por el directorio actual).",
				},
				"agent": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"claude", "agy", "developer"},
					"description": "Agente responsable que redacta el handoff (por defecto claude).",
				},
			},
			"required": []string{"summary", "pending"},
		},
	},
	{
		Name: "get_standup_pending",
		Description: "Obtiene los puntos pendientes dejados el día anterior o en el último cierre de tareas registrado en Remote PC. " +
			"Útil al inicio de la jornada o cuando el usuario pregunta qué quedó pendiente de ayer.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Fecha de referencia YYYY-MM-DD (opcional; por defecto la fecha actual).",
				},
			},
		},
	},
	{
		Name:        "get_today_note",
		Description: "Consulta la nota diaria y los handoffs registrados en el Calendario de Remote PC para hoy (o una fecha dada).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Fecha YYYY-MM-DD (opcional; por defecto la fecha de hoy).",
				},
			},
		},
	},
	{
		Name: "update_daily_handoff",
		Description: "Modifica o actualiza un handoff existente en el Calendario de Remote PC. " +
			"Si no se proporciona id, busca automáticamente el handoff correspondiente al proyecto o tarea actual de hoy.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "ID del handoff a actualizar (opcional si se puede auto-detectar por la tarea actual).",
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Nuevo resumen de lo completado hoy.",
				},
				"pending": map[string]interface{}{
					"type":        "string",
					"description": "Nuevas tareas pendientes para mañana.",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"in_progress", "completed", "blocked"},
					"description": "Nuevo estado de la tarea.",
				},
				"blockers": map[string]interface{}{
					"type":        "string",
					"description": "Puntos de atención o bloqueos.",
				},
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Fecha YYYY-MM-DD (opcional; por defecto hoy).",
				},
				"task_name": map[string]interface{}{
					"type":        "string",
					"description": "Nombre de la tarea (opcional).",
				},
			},
		},
	},
	{
		Name:        "delete_daily_handoff",
		Description: "Elimina un handoff registrado del Calendario de Remote PC.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "ID del handoff a eliminar.",
				},
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Fecha YYYY-MM-DD del handoff (opcional).",
				},
			},
			"required": []string{"id"},
		},
	},
	{
		Name: "add_daily_todo",
		Description: "Agrega una tarea o ítem accionable (con checkbox [ ] o [x]) a la sección de Notas Generales " +
			"del Calendario de Remote PC para el día de hoy (o una fecha específica). " +
			"Ideal cuando el usuario pide anotar un pendiente rápido o marcar una micro-tarea como lista.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Texto del to-do o tarea.",
				},
				"checked": map[string]interface{}{
					"type":        "boolean",
					"description": "Si la tarea ya está completada (true genera '- [x]', false genera '- [ ]'; por defecto false).",
				},
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Fecha YYYY-MM-DD (opcional; por defecto hoy).",
				},
			},
			"required": []string{"text"},
		},
	},
	{
		Name: "append_daily_note",
		Description: "Anexa texto, notas libres, comandos o apuntes a la nota diaria del Calendario de Remote PC " +
			"sin crear un bloque de handoff.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Texto o markdown a anexar a la nota diaria.",
				},
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Fecha YYYY-MM-DD (opcional; por defecto hoy).",
				},
			},
			"required": []string{"text"},
		},
	},
	{
		Name: "list_workspace_bubbles",
		Description: "Lista todos los workspace bubbles y entornos de tareas (Aditum, Brif, etc.) configurados en Remote PC " +
			"con su estado, rama Git actual, puerto de desarrollo y ruta.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"all", "aditum", "brif"},
					"description": "Filtrar por espacio de trabajo (por defecto 'all').",
				},
				"scan": map[string]interface{}{
					"type":        "boolean",
					"description": "Reescanear el sistema de archivos para descubrir nuevos worktrees (por defecto true).",
				},
			},
		},
	},
	{
		Name: "get_bubble_context",
		Description: "Obtiene el contexto detallado de una tarea o bubble de Remote PC: rama activa, status de Git, " +
			"archivos modificados, puerto dev asignado y schema de base de datos. " +
			"Si se omite 'bubble_id', se auto-detecta según el directorio actual.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"bubble_id": map[string]interface{}{
					"type":        "string",
					"description": "ID del bubble o nombre de tarea (opcional; se auto-detecta del directorio actual).",
				},
			},
		},
	},
	{
		Name: "create_workspace_bubble",
		Description: "Crea un nuevo workspace bubble y entorno de trabajo (git worktree aislado, branch y configuración) " +
			"directamente desde el agente. Soporta espacios como Aditum y Brif.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"aditum", "brif"},
					"description": "Espacio de trabajo donde crear la tarea ('aditum' o 'brif').",
				},
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "Identificador del ticket o tarea (ej: 'TAR-1150' para Aditum, o 'dialogos-mx' para Brif).",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Nombre legible para la tarea (opcional; por defecto igual a task_id).",
				},
				"base_branch": map[string]interface{}{
					"type":        "string",
					"description": "Rama base para crear el worktree (opcional; ej: 'development').",
				},
				"use_slim": map[string]interface{}{
					"type":        "boolean",
					"description": "Solo para Aditum: usar seed ligera de base de datos (por defecto true).",
				},
				"is_worktree": map[string]interface{}{
					"type":        "boolean",
					"description": "Crear como git worktree aislado (true) o branch lineal en el repo principal (false). Por defecto true.",
				},
			},
			"required": []string{"workspace", "task_id"},
		},
	},
	{
		Name: "start_project_server",
		Description: "Inicia el servidor de desarrollo del proyecto (ej: Aditum Spring Boot backend, Brif Django dev server, etc.) en segundo plano dentro de Orbit / Remote PC. " +
			"Crea una sesión de terminal activa con rol 'server' que se visualiza en tiempo real en el Web Cockpit y en la app móvil Orbit. " +
			"Por defecto inicia el backend de 'aditum' o auto-detecta el proyecto y worktree según el directorio actual. " +
			"Si el servidor ya está corriendo, informa su estado o permite reiniciarlo con 'restart: true'.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "Espacio de trabajo o proyecto a correr ('aditum', 'brif', etc. Opcional; por defecto 'aditum' o auto-detectado).",
				},
				"bubble_id": map[string]interface{}{
					"type":        "string",
					"description": "ID de la tarea o worktree específico (opcional; ej: 'TAR-1125', 'aditum-hotfix-tar-1125').",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Comando personalizado para iniciar el servidor (opcional; si se omite usa el comando/script configurado del proyecto).",
				},
				"restart": map[string]interface{}{
					"type":        "boolean",
					"description": "Si es true y el servidor ya está corriendo, lo detiene y lo reinicia limpiamente (por defecto false).",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Nombre legible para la sesión (opcional; ej: 'Aditum Backend').",
				},
			},
		},
	},
	{
		Name: "get_project_server_status",
		Description: "Consulta si el servidor de desarrollo del proyecto (Aditum, Brif, etc.) está corriendo actualmente en Orbit / Remote PC, " +
			"junto con su ID de sesión, estado, puerto y logs recientes de arranque.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "Espacio de trabajo a consultar ('aditum', 'brif', etc. Opcional; por defecto 'aditum').",
				},
				"bubble_id": map[string]interface{}{
					"type":        "string",
					"description": "ID de la tarea o worktree específico (opcional).",
				},
			},
		},
	},
	{
		Name: "stop_project_server",
		Description: "Detiene el servidor de desarrollo en ejecución del proyecto especificado (ej: Aditum Spring Boot o Brif) en Orbit / Remote PC.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "Espacio de trabajo del servidor a detener ('aditum', 'brif', etc. Opcional; por defecto 'aditum').",
				},
				"bubble_id": map[string]interface{}{
					"type":        "string",
					"description": "ID de la tarea o worktree (opcional).",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "ID específico de la sesión a terminar si se conoce (opcional).",
				},
			},
		},
	},
}

// Config representation for connecting to Remote PC daemon
type clientConfig struct {
	Port      string `json:"port"`
	AuthToken string `json:"authToken"`
}

func getClientConfig() clientConfig {
	cfg := clientConfig{
		Port:      "8080",
		AuthToken: "",
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	localApp := os.Getenv("LOCALAPPDATA")
	userProfile := os.Getenv("USERPROFILE")

	paths := []string{
		filepath.Join("data", "server.config.json"),
		filepath.Join(exeDir, "data", "server.config.json"),
		filepath.Join(exeDir, "server.config.json"),
		filepath.Join(localApp, "RemotePC", "data", "server.config.json"),
		filepath.Join(localApp, "RemotePC", "server.config.json"),
		filepath.Join(userProfile, "Documents", "Personal Projects", "Remote PC", "data", "server.config.json"),
		filepath.Join(userProfile, "Documents", "Personal Projects", "Remote PC", "server.config.json"),
		filepath.Join(userProfile, "Documents", "Personal Projects", "Remote PC", "server", "server.config.json"),
		"server.config.json",
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var parsed struct {
				Port      string `json:"port"`
				AuthToken string `json:"authToken"`
			}
			if err := json.Unmarshal(data, &parsed); err == nil {
				if parsed.Port != "" {
					cfg.Port = parsed.Port
				}
				if parsed.AuthToken != "" {
					cfg.AuthToken = parsed.AuthToken
				}
				return cfg
			}
		}
	}

	if envPort := os.Getenv("REMOTE_PC_PORT"); envPort != "" {
		cfg.Port = envPort
	}
	if envToken := os.Getenv("REMOTE_PC_TOKEN"); envToken != "" {
		cfg.AuthToken = envToken
	}

	return cfg
}

func ensureServerRunning(port string) bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exeDir := filepath.Dir(exePath)
	home, _ := os.UserHomeDir()

	candidatePaths := []string{
		exePath,
		filepath.Join(exeDir, "orbit.exe"),
		filepath.Join(exeDir, "orbit"),
		filepath.Join(exeDir, "remotepc-server.exe"),
		filepath.Join(exeDir, "remotepc-server"),
	}

	if home != "" {
		candidatePaths = append(candidatePaths,
			filepath.Join(home, ".local", "bin", "orbit"),
			filepath.Join(home, ".local", "bin", "remotepc-server"),
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "orbit.exe"),
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "orbit"),
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "server", "orbit.exe"),
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "server", "orbit"),
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "remotepc-server.exe"),
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "remotepc-server"),
		)
	}

	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		candidatePaths = append(candidatePaths,
			filepath.Join(localApp, "Orbit", "orbit.exe"),
			filepath.Join(localApp, "RemotePC", "remotepc-server.exe"),
		)
	}

	for _, p := range candidatePaths {
		if _, err := os.Stat(p); err == nil {
			if err := startBackgroundDaemon(p); err == nil {
				healthURL := fmt.Sprintf("http://127.0.0.1:%s/api/health", port)
				client := &http.Client{Timeout: 500 * time.Millisecond}
				for i := 0; i < 15; i++ {
					time.Sleep(200 * time.Millisecond)
					resp, err := client.Get(healthURL)
					if err == nil && resp.StatusCode == 200 {
						_ = resp.Body.Close()
						return true
					}
					if resp != nil {
						_ = resp.Body.Close()
					}
				}
				return false
			}
		}
	}
	return false
}

func apiRequest(path, method string, body interface{}, target interface{}, retried bool) error {
	cfg := getClientConfig()
	url := fmt.Sprintf("http://127.0.0.1:%s%s", cfg.Port, path)

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("error serializing request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	}

	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if !retried && (strings.Contains(err.Error(), "refused") || strings.Contains(err.Error(), "10061") || strings.Contains(err.Error(), "dial tcp")) {
			if ensureServerRunning(cfg.Port) {
				return apiRequest(path, method, body, target, true)
			}
		}
		return fmt.Errorf("error conectando con Orbit en %s: %w. Asegúrate de que 'orbit' esté en ejecución", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("error parsing response: %w", err)
		}
	}
	return nil
}

// ExecuteTool handles tool dispatching
func ExecuteTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "save_daily_handoff":
		return handleSaveDailyHandoff(args)
	case "get_standup_pending":
		return handleGetStandupPending(args)
	case "get_today_note":
		return handleGetTodayNote(args)
	case "update_daily_handoff":
		return handleUpdateDailyHandoff(args)
	case "delete_daily_handoff":
		return handleDeleteDailyHandoff(args)
	case "add_daily_todo":
		return handleAddDailyTodo(args)
	case "append_daily_note":
		return handleAppendDailyNote(args)
	case "list_workspace_bubbles":
		return handleListWorkspaceBubbles(args)
	case "get_bubble_context":
		return handleGetBubbleContext(args)
	case "create_workspace_bubble":
		return handleCreateWorkspaceBubble(args)
	case "start_project_server":
		return handleStartProjectServer(args)
	case "get_project_server_status":
		return handleGetProjectServerStatus(args)
	case "stop_project_server":
		return handleStopProjectServer(args)
	case "transcribe_video":
		return handleTranscribeVideo(args)
	case "extract_audio":
		return handleExtractAudio(args)
	default:
		return "", fmt.Errorf("herramienta desconocida: %s", name)
	}
}

func handleSaveDailyHandoff(args map[string]interface{}) (string, error) {
	cwd, _ := os.Getwd()
	bubbleID := ""
	taskName, _ := args["task_name"].(string)

	var bubbles []map[string]interface{}
	_ = apiRequest("/api/bubbles", "GET", nil, &bubbles, false)
	cwdNorm := strings.ToLower(filepath.Clean(cwd))

	for _, b := range bubbles {
		bPath, _ := b["path"].(string)
		if bPath != "" {
			bNorm := strings.ToLower(filepath.Clean(bPath))
			if cwdNorm == bNorm || strings.HasPrefix(cwdNorm, bNorm) {
				if id, ok := b["id"].(string); ok {
					bubbleID = id
				}
				if taskName == "" {
					if name, ok := b["name"].(string); ok {
						taskName = name
					}
				}
				break
			}
		}
	}

	if taskName == "" {
		taskName = filepath.Base(cwd)
		if taskName == "" || taskName == "." {
			taskName = "Tarea"
		}
	}

	agent, _ := args["agent"].(string)
	if agent == "" {
		agent = "claude"
	}
	status, _ := args["status"].(string)
	if status == "" {
		status = "in_progress"
	}
	summary, _ := args["summary"].(string)
	pending, _ := args["pending"].(string)
	blockers, _ := args["blockers"].(string)

	payload := map[string]interface{}{
		"bubbleId": bubbleID,
		"workDir":  cwd,
		"taskName": taskName,
		"agent":    agent,
		"status":   status,
		"summary":  summary,
		"pending":  pending,
		"blockers": blockers,
	}

	var res map[string]interface{}
	if err := apiRequest("/api/calendar/handoff", "POST", payload, &res, false); err != nil {
		return "", fmt.Errorf("error al guardar handoff en Remote PC: %w", err)
	}

	statusLabel := "🟡 En Progreso"
	if resStatus, _ := res["status"].(string); resStatus == "completed" {
		statusLabel = "🟢 Completado"
	} else if resStatus == "blocked" {
		statusLabel = "🔴 Bloqueado"
	}

	lines := []string{
		"✓ **Cierre de jornada registrado exitosamente en Remote PC Calendar**",
		fmt.Sprintf("- **Fecha:** %v (%v)", res["date"], res["time"]),
		fmt.Sprintf("- **Tarea:** %v (%v)", res["taskName"], res["agent"]),
		fmt.Sprintf("- **Estado:** %s", statusLabel),
		fmt.Sprintf("- **Lo completado:** %v", res["summary"]),
		fmt.Sprintf("- **Pendiente para mañana:** %v", res["pending"]),
	}
	if b, ok := res["blockers"].(string); ok && b != "" {
		lines = append(lines, fmt.Sprintf("- **Bloqueos:** %s", b))
	}
	if br, ok := res["branch"].(string); ok && br != "" {
		dirty := 0
		if d, ok := res["dirtyFiles"].(float64); ok {
			dirty = int(d)
		}
		lines = append(lines, fmt.Sprintf("- **Git Snapshot:** Rama `%s` | %d archivo(s) sin commitear", br, dirty))
	}

	return strings.Join(lines, "\n"), nil
}

func handleGetStandupPending(args map[string]interface{}) (string, error) {
	date, _ := args["date"].(string)
	path := "/api/calendar/standup"
	if date != "" {
		path += "?date=" + date
	}

	var res struct {
		ReferenceDate string                   `json:"referenceDate"`
		PendingItems  []map[string]interface{} `json:"pendingItems"`
	}
	if err := apiRequest(path, "GET", nil, &res, false); err != nil {
		return "", fmt.Errorf("error al consultar standup: %w", err)
	}

	if len(res.PendingItems) == 0 {
		ref := res.ReferenceDate
		if ref == "" {
			ref = "reciente"
		}
		return fmt.Sprintf("No hay pendientes registrados para la fecha de referencia %s.", ref), nil
	}

	lines := []string{fmt.Sprintf("📋 **Pendientes de standup (Fecha de referencia: %s)**:", res.ReferenceDate)}
	for _, it := range res.PendingItems {
		icon := "🟡"
		if it["status"] == "blocked" {
			icon = "🔴"
		}
		lines = append(lines, fmt.Sprintf("\n### %s %v (%v)", icon, it["taskName"], it["agent"]))
		if p, ok := it["pending"].(string); ok && p != "" {
			lines = append(lines, fmt.Sprintf("- **Pendiente:** %s", p))
		}
		if s, ok := it["summary"].(string); ok && s != "" {
			lines = append(lines, fmt.Sprintf("- **Último avance:** %s", s))
		}
		if br, ok := it["branch"].(string); ok && br != "" {
			lines = append(lines, fmt.Sprintf("- **Rama:** `%s`", br))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func handleGetTodayNote(args map[string]interface{}) (string, error) {
	date, _ := args["date"].(string)
	path := "/api/calendar/note"
	if date != "" {
		path += "?date=" + date
	}

	var res struct {
		Date     string        `json:"date"`
		Content  string        `json:"content"`
		Handoffs []interface{} `json:"handoffs"`
	}
	if err := apiRequest(path, "GET", nil, &res, false); err != nil {
		return "", fmt.Errorf("error al consultar nota diaria: %w", err)
	}

	return fmt.Sprintf("📅 **Nota Diaria (%s)**:\n\n%s\n\n*Handoffs registrados:* %d", res.Date, res.Content, len(res.Handoffs)), nil
}

func handleUpdateDailyHandoff(args map[string]interface{}) (string, error) {
	handoffID, _ := args["id"].(string)
	date, _ := args["date"].(string)

	if handoffID == "" {
		path := "/api/calendar/note"
		if date != "" {
			path += "?date=" + date
		}
		var noteRes struct {
			Handoffs []map[string]interface{} `json:"handoffs"`
		}
		_ = apiRequest(path, "GET", nil, &noteRes, false)
		if len(noteRes.Handoffs) > 0 {
			cwd, _ := os.Getwd()
			cwdName := strings.ToLower(filepath.Base(cwd))
			var matched map[string]interface{}
			for _, h := range noteRes.Handoffs {
				tName, _ := h["taskName"].(string)
				tLower := strings.ToLower(tName)
				if strings.Contains(cwdName, tLower) || strings.Contains(tLower, cwdName) {
					matched = h
					break
				}
			}
			if matched == nil && len(noteRes.Handoffs) == 1 {
				matched = noteRes.Handoffs[0]
			}
			if matched != nil {
				if id, ok := matched["id"].(string); ok {
					handoffID = id
				}
				if date == "" {
					if d, ok := matched["date"].(string); ok {
						date = d
					}
				}
			}
		}
	}

	if handoffID == "" {
		return "", fmt.Errorf("error: No se encontró el ID del handoff a actualizar. Especifica el parámetro 'id'")
	}

	payload := map[string]interface{}{
		"id":   handoffID,
		"date": date,
	}
	for _, field := range []string{"summary", "pending", "status", "blockers"} {
		if val, ok := args[field].(string); ok {
			payload[field] = val
		}
	}
	if tName, ok := args["task_name"].(string); ok {
		payload["taskName"] = tName
	}

	var res map[string]interface{}
	if err := apiRequest("/api/calendar/handoff", "PUT", payload, &res, false); err != nil {
		return "", fmt.Errorf("error al actualizar handoff: %w", err)
	}

	return fmt.Sprintf("✓ **Handoff actualizado correctamente** (Tarea: %v, Estado: %v)", res["taskName"], res["status"]), nil
}

func handleDeleteDailyHandoff(args map[string]interface{}) (string, error) {
	handoffID, _ := args["id"].(string)
	date, _ := args["date"].(string)
	if handoffID == "" {
		return "", fmt.Errorf("error: Se requiere el parámetro 'id' para eliminar el handoff")
	}

	path := fmt.Sprintf("/api/calendar/handoff?id=%s", handoffID)
	if date != "" {
		path += "&date=" + date
	}

	if err := apiRequest(path, "DELETE", nil, nil, false); err != nil {
		return "", fmt.Errorf("error al eliminar handoff: %w", err)
	}

	return fmt.Sprintf("✓ **Handoff eliminado exitosamente** (ID: %s)", handoffID), nil
}

func handleAddDailyTodo(args map[string]interface{}) (string, error) {
	text, _ := args["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("error: Se requiere el parámetro 'text' con la descripción del to-do")
	}
	date, _ := args["date"].(string)
	checked := false
	if c, ok := args["checked"].(bool); ok {
		checked = c
	}

	payload := map[string]interface{}{
		"text":    text,
		"checked": checked,
		"date":    date,
	}

	var res map[string]interface{}
	if err := apiRequest("/api/calendar/todo", "POST", payload, &res, false); err != nil {
		return "", fmt.Errorf("error al agregar to-do en Remote PC: %w", err)
	}

	box := "[ ]"
	if checked {
		box = "[x]"
	}

	return fmt.Sprintf("✓ **To-do registrado en la nota diaria (%v)**\n- %s %s", res["date"], box, text), nil
}

func handleAppendDailyNote(args map[string]interface{}) (string, error) {
	text, _ := args["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("error: Se requiere el parámetro 'text' para anexar a la nota diaria")
	}
	date, _ := args["date"].(string)

	payload := map[string]interface{}{
		"text": text,
		"date": date,
	}

	var res map[string]interface{}
	if err := apiRequest("/api/calendar/note/append", "POST", payload, &res, false); err != nil {
		return "", fmt.Errorf("error al anexar texto en la nota diaria: %w", err)
	}

	return fmt.Sprintf("✓ **Nota diaria actualizada exitosamente (%v)**", res["date"]), nil
}

func handleListWorkspaceBubbles(args map[string]interface{}) (string, error) {
	workspace := "all"
	if ws, ok := args["workspace"].(string); ok && ws != "" {
		workspace = strings.ToLower(ws)
	}
	scan := true
	if s, ok := args["scan"].(bool); ok {
		scan = s
	}

	path := "/api/bubbles"
	if scan {
		path += "?scan=true"
	}

	var bubbles []map[string]interface{}
	if err := apiRequest(path, "GET", nil, &bubbles, false); err != nil {
		return "", fmt.Errorf("error al listar bubbles: %w", err)
	}

	if len(bubbles) == 0 {
		return "No hay workspace bubbles registrados en Remote PC.", nil
	}

	var filtered []map[string]interface{}
	for _, b := range bubbles {
		ws, _ := b["workspace"].(string)
		if workspace == "all" || strings.ToLower(ws) == workspace {
			filtered = append(filtered, b)
		}
	}

	if len(filtered) == 0 {
		return fmt.Sprintf("No se encontraron workspace bubbles para el espacio '%s'.", workspace), nil
	}

	lines := []string{fmt.Sprintf("🌐 **Workspace Bubbles en Remote PC (%d encontrados)**:", len(filtered))}
	for _, b := range filtered {
		statusIcon := "⚪"
		if st, _ := b["status"].(string); st == "running" {
			statusIcon = "🟢"
		} else if dr, _ := b["devRunning"].(bool); dr {
			statusIcon = "🟢"
		}

		ws := "general"
		if w, _ := b["workspace"].(string); w != "" {
			ws = strings.ToUpper(w)
		}
		name, _ := b["name"].(string)
		if name == "" {
			name, _ = b["id"].(string)
		}
		branch, _ := b["branch"].(string)
		if branch == "" {
			branch = "sin rama"
		}
		portInfo := ""
		if p, ok := b["port"].(float64); ok && p > 0 {
			portInfo = fmt.Sprintf(" | Puerto :%.0f", p)
		}
		schemaInfo := ""
		if s, ok := b["dbSchema"].(string); ok && s != "" {
			schemaInfo = fmt.Sprintf(" | DB: `%s`", s)
		}
		worktreeBadge := ""
		if wt, ok := b["isWorktree"].(bool); ok && wt {
			worktreeBadge = " (Worktree)"
		}

		lines = append(lines, fmt.Sprintf("\n- %s **%s** [%s]%s", statusIcon, name, ws, worktreeBadge))
		lines = append(lines, fmt.Sprintf("  - **ID:** `%v` | **Rama:** `%s`%s%s", b["id"], branch, portInfo, schemaInfo))
		lines = append(lines, fmt.Sprintf("  - **Ruta:** `%v`", b["path"]))
	}

	return strings.Join(lines, "\n"), nil
}

func handleGetBubbleContext(args map[string]interface{}) (string, error) {
	bubbleID, _ := args["bubble_id"].(string)
	bubbleID = strings.TrimSpace(bubbleID)

	var bubbles []map[string]interface{}
	if err := apiRequest("/api/bubbles", "GET", nil, &bubbles, false); err != nil {
		return "", fmt.Errorf("error al consultar bubbles: %w", err)
	}

	var matched map[string]interface{}
	if bubbleID != "" {
		for _, b := range bubbles {
			id, _ := b["id"].(string)
			name, _ := b["name"].(string)
			if id == bubbleID || strings.Contains(strings.ToLower(name), strings.ToLower(bubbleID)) {
				matched = b
				break
			}
		}
	} else {
		cwd, _ := os.Getwd()
		cwdNorm := strings.ToLower(filepath.Clean(cwd))
		for _, b := range bubbles {
			bPath, _ := b["path"].(string)
			if bPath != "" {
				bNorm := strings.ToLower(filepath.Clean(bPath))
				if cwdNorm == bNorm || strings.HasPrefix(cwdNorm, bNorm) {
					matched = b
					break
				}
			}
		}
	}

	if matched == nil {
		var available []string
		for i, b := range bubbles {
			if i >= 10 {
				break
			}
			available = append(available, fmt.Sprintf("`%v`", b["id"]))
		}
		return fmt.Sprintf("No se pudo determinar el bubble actual a partir del directorio ni del parámetro 'bubble_id'.\nBubbles disponibles: %s", strings.Join(available, ", ")), nil
	}

	bID, _ := matched["id"].(string)
	var gitSt struct {
		Branch  string `json:"branch"`
		IsDirty bool   `json:"isDirty"`
		Ahead   int    `json:"ahead"`
		Behind  int    `json:"behind"`
		Changes []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
			Staged bool   `json:"staged"`
		} `json:"changes"`
	}
	_ = apiRequest(fmt.Sprintf("/api/bubbles/%s/git/status", bID), "GET", nil, &gitSt, false)

	ws, _ := matched["workspace"].(string)
	lines := []string{
		fmt.Sprintf("🎯 **Contexto del Bubble:** `%v` (`%s`)", matched["name"], bID),
		fmt.Sprintf("- **Espacio:** %s", strings.ToUpper(ws)),
		fmt.Sprintf("- **Ruta de trabajo:** `%v`", matched["path"]),
	}

	if p, ok := matched["port"].(float64); ok && p > 0 {
		lines = append(lines, fmt.Sprintf("- **Puerto dev:** %.0f", p))
	} else {
		lines = append(lines, "- **Puerto dev:** No asignado")
	}

	if s, ok := matched["dbSchema"].(string); ok && s != "" {
		lines = append(lines, fmt.Sprintf("- **Esquema DB:** `%s`", s))
	}
	if pr, ok := matched["priority"].(float64); ok {
		lines = append(lines, fmt.Sprintf("- **Prioridad:** %.0f", pr))
	}

	if gitSt.Branch != "" {
		branch := gitSt.Branch
		dirtyMsg := "✨ Directorio limpio"
		if gitSt.IsDirty {
			dirtyMsg = "⚠️ Cambios sin commitear"
		}

		lines = append(lines, "\n**Estado Git:**")
		lines = append(lines, fmt.Sprintf("- **Rama:** `%s`", branch))
		lines = append(lines, fmt.Sprintf("- **Commits:** Ahead: %d | Behind: %d", gitSt.Ahead, gitSt.Behind))
		lines = append(lines, fmt.Sprintf("- **Estado de trabajo:** %s", dirtyMsg))

		if len(gitSt.Changes) > 0 {
			lines = append(lines, fmt.Sprintf("- **Archivos modificados (%d):**", len(gitSt.Changes)))
			limit := 8
			if len(gitSt.Changes) < limit {
				limit = len(gitSt.Changes)
			}
			for i := 0; i < limit; i++ {
				c := gitSt.Changes[i]
				st := c.Status
				if st == "" {
					st = "M"
				}
				staged := ""
				if c.Staged {
					staged = " (staged)"
				}
				lines = append(lines, fmt.Sprintf("  - `[%s]` %s%s", st, c.Path, staged))
			}
			if len(gitSt.Changes) > limit {
				lines = append(lines, fmt.Sprintf("  - ... y %d archivo(s) más", len(gitSt.Changes)-limit))
			}
		}
	} else if br, ok := matched["branch"].(string); ok && br != "" {
		lines = append(lines, fmt.Sprintf("- **Rama actual:** `%s`", br))
	}

	return strings.Join(lines, "\n"), nil
}

func handleCreateWorkspaceBubble(args map[string]interface{}) (string, error) {
	workspace, _ := args["workspace"].(string)
	workspace = strings.TrimSpace(strings.ToLower(workspace))
	taskID, _ := args["task_id"].(string)
	taskID = strings.TrimSpace(taskID)

	if workspace == "" || taskID == "" {
		return "", fmt.Errorf("error: Se requieren 'workspace' ('aditum' o 'brif') y 'task_id' (ej: 'TAR-1150')")
	}

	name, _ := args["name"].(string)
	if name == "" {
		name = taskID
	}
	baseBranch, _ := args["base_branch"].(string)
	useSlim := true
	if us, ok := args["use_slim"].(bool); ok {
		useSlim = us
	}
	isWorktree := true
	if wt, ok := args["is_worktree"].(bool); ok {
		isWorktree = wt
	}

	payload := map[string]interface{}{
		"workspace":  workspace,
		"taskId":     taskID,
		"name":       name,
		"baseBranch": baseBranch,
		"useSlim":    useSlim,
		"isWorktree": isWorktree,
	}

	var res map[string]interface{}
	if err := apiRequest("/api/bubbles", "POST", payload, &res, false); err != nil {
		return "", fmt.Errorf("error al crear bubble en Remote PC: %w", err)
	}

	lines := []string{
		"🎉 **Workspace Bubble creado y configurado exitosamente**",
		fmt.Sprintf("- **Tarea:** %v", res["name"]),
		fmt.Sprintf("- **ID:** `%v`", res["id"]),
		fmt.Sprintf("- **Espacio:** %v", strings.ToUpper(fmt.Sprintf("%v", res["workspace"]))),
		fmt.Sprintf("- **Ruta:** `%v`", res["path"]),
		fmt.Sprintf("- **Rama:** `%v`", res["branch"]),
	}
	if p, ok := res["port"].(float64); ok && p > 0 {
		lines = append(lines, fmt.Sprintf("- **Puerto dev:** %.0f", p))
	}
	if s, ok := res["dbSchema"].(string); ok && s != "" {
		lines = append(lines, fmt.Sprintf("- **Esquema DB:** `%s`", s))
	}

	return strings.Join(lines, "\n"), nil
}

func handleStartProjectServer(args map[string]interface{}) (string, error) {
	workspace, _ := args["workspace"].(string)
	workspace = strings.TrimSpace(strings.ToLower(workspace))

	bubbleID, _ := args["bubble_id"].(string)
	bubbleID = strings.TrimSpace(bubbleID)

	customCmd, _ := args["command"].(string)
	customCmd = strings.TrimSpace(customCmd)

	restart, _ := args["restart"].(bool)
	nameArg, _ := args["name"].(string)
	nameArg = strings.TrimSpace(nameArg)

	// 1. Fetch bubbles to match bubble and workspace
	var bubbles []map[string]interface{}
	_ = apiRequest("/api/bubbles?scan=true", "GET", nil, &bubbles, false)

	var matchedBubble map[string]interface{}
	cwd, _ := os.Getwd()
	cwdNorm := strings.ToLower(filepath.Clean(cwd))

	if bubbleID != "" {
		for _, b := range bubbles {
			id, _ := b["id"].(string)
			name, _ := b["name"].(string)
			if id == bubbleID || strings.EqualFold(id, bubbleID) || strings.EqualFold(name, bubbleID) ||
				strings.Contains(strings.ToLower(id), strings.ToLower(bubbleID)) ||
				strings.Contains(strings.ToLower(name), strings.ToLower(bubbleID)) {
				matchedBubble = b
				break
			}
		}
	} else {
		// Try matching by cwd
		for _, b := range bubbles {
			bPath, _ := b["path"].(string)
			if bPath != "" {
				bNorm := strings.ToLower(filepath.Clean(bPath))
				if cwdNorm == bNorm || strings.HasPrefix(cwdNorm, bNorm) {
					matchedBubble = b
					break
				}
			}
		}
	}

	// If no matchedBubble yet, and workspace is empty, default to "aditum"
	if matchedBubble == nil && workspace == "" {
		workspace = "aditum"
	}

	targetWs := workspace
	targetCwd := ""
	targetBubbleID := ""
	targetBubbleName := ""
	targetPort := 0

	if matchedBubble != nil {
		targetBubbleID, _ = matchedBubble["id"].(string)
		targetBubbleName, _ = matchedBubble["name"].(string)
		targetCwd, _ = matchedBubble["path"].(string)
		if ws, ok := matchedBubble["workspace"].(string); ok && ws != "" {
			targetWs = strings.ToLower(ws)
		}
		if p, ok := matchedBubble["port"].(float64); ok && p > 0 {
			targetPort = int(p)
		}
	} else {
		// Look for any bubble of that workspace
		for _, b := range bubbles {
			ws, _ := b["workspace"].(string)
			if strings.EqualFold(ws, targetWs) {
				// prefer root or main bubble if present
				isWt, _ := b["isWorktree"].(bool)
				if !isWt || matchedBubble == nil {
					matchedBubble = b
					if !isWt {
						break
					}
				}
			}
		}

		if matchedBubble != nil {
			targetBubbleID, _ = matchedBubble["id"].(string)
			targetBubbleName, _ = matchedBubble["name"].(string)
			targetCwd, _ = matchedBubble["path"].(string)
			if p, ok := matchedBubble["port"].(float64); ok && p > 0 {
				targetPort = int(p)
			}
		}
	}

	// If still no path, fallback to workspace definitions
	if targetCwd == "" {
		var workspaces []map[string]interface{}
		_ = apiRequest("/api/workspaces", "GET", nil, &workspaces, false)
		for _, ws := range workspaces {
			id, _ := ws["id"].(string)
			wsType, _ := ws["type"].(string)
			if strings.EqualFold(id, targetWs) || strings.EqualFold(wsType, targetWs) {
				if bp, ok := ws["basePath"].(string); ok && bp != "" {
					targetCwd = bp
					break
				}
			}
		}
	}

	// Absolute fallbacks for Jose's environment if nothing found
	if targetCwd == "" {
		if targetWs == "aditum" {
			targetCwd = `C:\Users\Jose\Documents\Work\Aditum`
		} else if targetWs == "brif" {
			targetCwd = `C:\Users\Jose\Documents\Work\brif`
		}
	}

	if targetCwd != "" {
		targetCwd = filepath.Clean(targetCwd)
	}

	if targetCwd == "" {
		return "", fmt.Errorf("no se pudo determinar el directorio de trabajo para el proyecto '%s'", targetWs)
	}

	if _, err := os.Stat(targetCwd); err != nil {
		return "", fmt.Errorf("el directorio '%s' no existe en el sistema de archivos", targetCwd)
	}

	// 2. Check existing sessions
	var sessions []map[string]interface{}
	_ = apiRequest("/api/sessions", "GET", nil, &sessions, false)

	var existingServerSession map[string]interface{}
	targetCwdLower := strings.ToLower(targetCwd)
	for _, s := range sessions {
		role, _ := s["role"].(string)
		isRunning, _ := s["isRunning"].(bool)
		bID, _ := s["bubbleId"].(string)
		sCwd, _ := s["cwd"].(string)
		sCwdLower := strings.ToLower(filepath.Clean(sCwd))

		if role == "server" && isRunning {
			if (targetBubbleID != "" && bID == targetBubbleID) || (sCwdLower == targetCwdLower) {
				existingServerSession = s
				break
			}
		}
	}

	if existingServerSession != nil {
		sessID, _ := existingServerSession["id"].(string)
		if !restart {
			var hist struct {
				Snippet string `json:"snippet"`
				Length  int    `json:"length"`
			}
			if sessID != "" {
				_ = apiRequest(fmt.Sprintf("/api/sessions/%s/history", sessID), "GET", nil, &hist, false)
			}

			lines := []string{
				"🟢 **El servidor de desarrollo ya está en ejecución en Orbit / Remote PC**",
				fmt.Sprintf("- **Sesión:** `%v` (`%s`)", existingServerSession["name"], sessID),
				fmt.Sprintf("- **Directorio:** `%v`", existingServerSession["cwd"]),
				fmt.Sprintf("- **PID:** `%v`", existingServerSession["pid"]),
			}
			if targetBubbleName != "" {
				lines = append(lines, fmt.Sprintf("- **Tarea / Bubble:** `%s` (`%s`)", targetBubbleName, targetBubbleID))
			}
			if targetPort > 0 {
				lines = append(lines, fmt.Sprintf("- **Puerto:** `%d`", targetPort))
			}
			lines = append(lines, "\n📱 **Visualización:** Visible en tiempo real en Web Cockpit y Orbit Mobile.")
			if strings.TrimSpace(hist.Snippet) != "" {
				tail := hist.Snippet
				if len(tail) > 1000 {
					tail = tail[len(tail)-1000:]
				}
				lines = append(lines, fmt.Sprintf("\n**Últimos logs emitidos:**\n```\n%s\n```", tail))
			}
			lines = append(lines, "\n*(Nota: Si deseas reiniciarlo limpiamente, pasa `restart: true` o pide 'reinicia el servidor'.)*")
			return strings.Join(lines, "\n"), nil
		}

		// If restart is true, terminate existing session
		_ = apiRequest(fmt.Sprintf("/api/sessions/%s", sessID), "DELETE", nil, nil, false)
		time.Sleep(800 * time.Millisecond)
	}

	// 3. Determine start command
	startCmd := customCmd
	if startCmd == "" {
		switch targetWs {
		case "aditum":
			startBackendScript := filepath.Join(targetCwd, "scripts", "start-backend.sh")
			if _, err := os.Stat(startBackendScript); err == nil {
				startCmd = `powershell.exe -NoExit -Command "bash ./scripts/start-backend.sh"`
			} else if _, err := os.Stat(filepath.Join(targetCwd, "mvnw.cmd")); err == nil {
				startCmd = `powershell.exe -NoExit -Command ".\mvnw.cmd spring-boot:run -Dmaven.test.skip=true"`
			} else {
				startCmd = `powershell.exe -NoExit -Command "mvn spring-boot:run -Dmaven.test.skip=true"`
			}
		case "brif":
			managePath := filepath.Join(targetCwd, "brif-scrapper", "manage.py")
			if _, err := os.Stat(managePath); err == nil {
				startCmd = `powershell.exe -NoExit -Command "cd brif-scrapper; python manage.py runserver 0.0.0.0:8000"`
			} else {
				startCmd = `powershell.exe -NoExit -Command "python manage.py runserver 0.0.0.0:8000"`
			}
		default:
			startCmd = `powershell.exe -NoExit`
		}
	}

	// 4. Session Name
	sessionName := nameArg
	if sessionName == "" {
		if targetWs == "aditum" {
			if targetBubbleName != "" && targetBubbleName != "aditum" {
				sessionName = fmt.Sprintf("⚙️ Aditum Backend (%s)", targetBubbleName)
			} else {
				sessionName = "⚙️ Aditum Backend"
			}
		} else if targetWs == "brif" {
			sessionName = "⚙️ Brif Server"
		} else {
			if targetBubbleName != "" {
				sessionName = fmt.Sprintf("⚙️ %s Server", targetBubbleName)
			} else {
				sessionName = "⚙️ Dev Server"
			}
		}
	}

	// 5. Create Session in Orbit API
	createPayload := map[string]interface{}{
		"name":     sessionName,
		"cwd":      targetCwd,
		"command":  startCmd,
		"bubbleId": targetBubbleID,
		"role":     "server",
		"cols":     120,
		"rows":     30,
	}

	var newSess map[string]interface{}
	if err := apiRequest("/api/sessions", "POST", createPayload, &newSess, false); err != nil {
		return "", fmt.Errorf("error al iniciar servidor en Orbit: %w", err)
	}

	sessID, _ := newSess["id"].(string)

	// Wait briefly so initial stdout/stderr starts filling the PTY ring buffer
	time.Sleep(1200 * time.Millisecond)

	var hist struct {
		Snippet string `json:"snippet"`
		Length  int    `json:"length"`
	}
	if sessID != "" {
		_ = apiRequest(fmt.Sprintf("/api/sessions/%s/history", sessID), "GET", nil, &hist, false)
	}

	lines := []string{
		"🚀 **Servidor de desarrollo iniciado exitosamente en Orbit**",
		fmt.Sprintf("- **Sesión:** `%s` (`%v`)", sessionName, sessID),
		"- **Rol:** `server` (destacado con ⚙️ en Web Cockpit y Orbit Mobile)",
		fmt.Sprintf("- **Directorio:** `%s`", targetCwd),
		fmt.Sprintf("- **Comando:** `%s`", startCmd),
		fmt.Sprintf("- **PID:** `%v`", newSess["pid"]),
	}
	if targetBubbleName != "" {
		lines = append(lines, fmt.Sprintf("- **Tarea / Bubble:** `%s` (`%s`)", targetBubbleName, targetBubbleID))
	}
	if targetPort > 0 {
		lines = append(lines, fmt.Sprintf("- **Puerto:** `%d`", targetPort))
	}

	lines = append(lines, "\n📱 **Visualización en tiempo real:**\nEl proceso está activo y emitiendo logs. Puedes abrir la app Orbit Mobile o el Web Cockpit para ver el streaming de la terminal.")

	if strings.TrimSpace(hist.Snippet) != "" {
		tail := hist.Snippet
		if len(tail) > 1000 {
			tail = tail[len(tail)-1000:]
		}
		lines = append(lines, fmt.Sprintf("\n**Salida inicial de la terminal:**\n```\n%s\n```", tail))
	}

	return strings.Join(lines, "\n"), nil
}

func handleGetProjectServerStatus(args map[string]interface{}) (string, error) {
	workspace, _ := args["workspace"].(string)
	workspace = strings.TrimSpace(strings.ToLower(workspace))

	bubbleID, _ := args["bubble_id"].(string)
	bubbleID = strings.TrimSpace(bubbleID)

	var sessions []map[string]interface{}
	if err := apiRequest("/api/sessions", "GET", nil, &sessions, false); err != nil {
		return "", fmt.Errorf("error al consultar sesiones: %w", err)
	}

	var matchedServers []map[string]interface{}
	for _, s := range sessions {
		role, _ := s["role"].(string)
		if role != "server" {
			continue
		}
		bID, _ := s["bubbleId"].(string)
		sName, _ := s["name"].(string)
		sCwd, _ := s["cwd"].(string)

		if bubbleID != "" {
			if bID == bubbleID || strings.EqualFold(bID, bubbleID) || strings.Contains(strings.ToLower(sName), strings.ToLower(bubbleID)) {
				matchedServers = append(matchedServers, s)
				continue
			}
		}

		if workspace != "" && workspace != "all" {
			if strings.Contains(strings.ToLower(sName), workspace) || strings.Contains(strings.ToLower(sCwd), workspace) || strings.Contains(strings.ToLower(bID), workspace) {
				matchedServers = append(matchedServers, s)
				continue
			}
		}

		if bubbleID == "" && (workspace == "" || workspace == "all") {
			matchedServers = append(matchedServers, s)
		}
	}

	if len(matchedServers) == 0 {
		msg := "⚪ **No hay servidores de desarrollo corriendo actualmente** en Orbit"
		if bubbleID != "" {
			msg += fmt.Sprintf(" para la tarea/bubble `%s`", bubbleID)
		} else if workspace != "" {
			msg += fmt.Sprintf(" para el workspace `%s`", workspace)
		}
		msg += ".\n\nPuedes iniciar uno solicitando: *'inicia el servidor del proyecto'* o llamando a `start_project_server`."
		return msg, nil
	}

	lines := []string{fmt.Sprintf("🟢 **Servidores de desarrollo encontrados en Orbit (%d):**", len(matchedServers))}

	for _, s := range matchedServers {
		id, _ := s["id"].(string)
		name, _ := s["name"].(string)
		cwd, _ := s["cwd"].(string)
		cmd, _ := s["command"].(string)
		pid := s["pid"]
		isRunning, _ := s["isRunning"].(bool)
		bID, _ := s["bubbleId"].(string)

		statusIcon := "🟢"
		statusText := "Corriendo"
		if !isRunning {
			statusIcon = "⚪"
			statusText = "Detenido"
		}

		lines = append(lines, fmt.Sprintf("\n%s **%s** (`%s`)", statusIcon, name, id))
		lines = append(lines, fmt.Sprintf("  - **Estado:** %s | **PID:** `%v`", statusText, pid))
		lines = append(lines, fmt.Sprintf("  - **Directorio:** `%s`", cwd))
		lines = append(lines, fmt.Sprintf("  - **Comando:** `%s`", cmd))
		if bID != "" {
			lines = append(lines, fmt.Sprintf("  - **Bubble ID:** `%s`", bID))
		}

		var hist struct {
			Snippet string `json:"snippet"`
			Length  int    `json:"length"`
		}
		if id != "" {
			_ = apiRequest(fmt.Sprintf("/api/sessions/%s/history", id), "GET", nil, &hist, false)
		}

		if strings.TrimSpace(hist.Snippet) != "" {
			tail := hist.Snippet
			if len(tail) > 800 {
				tail = tail[len(tail)-800:]
			}
			lines = append(lines, fmt.Sprintf("  - **Logs recientes:**\n```\n%s\n```", tail))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func handleStopProjectServer(args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	sessionID = strings.TrimSpace(sessionID)

	workspace, _ := args["workspace"].(string)
	workspace = strings.TrimSpace(strings.ToLower(workspace))

	bubbleID, _ := args["bubble_id"].(string)
	bubbleID = strings.TrimSpace(bubbleID)

	var sessions []map[string]interface{}
	if err := apiRequest("/api/sessions", "GET", nil, &sessions, false); err != nil {
		return "", fmt.Errorf("error al consultar sesiones: %w", err)
	}

	var toStop []map[string]interface{}
	for _, s := range sessions {
		id, _ := s["id"].(string)
		role, _ := s["role"].(string)
		if role != "server" && sessionID == "" {
			continue
		}

		if sessionID != "" {
			if id == sessionID {
				toStop = append(toStop, s)
				break
			}
			continue
		}

		bID, _ := s["bubbleId"].(string)
		sName, _ := s["name"].(string)
		sCwd, _ := s["cwd"].(string)

		if bubbleID != "" {
			if bID == bubbleID || strings.EqualFold(bID, bubbleID) || strings.Contains(strings.ToLower(sName), strings.ToLower(bubbleID)) {
				toStop = append(toStop, s)
				continue
			}
		}

		if workspace != "" && workspace != "all" {
			if strings.Contains(strings.ToLower(sName), workspace) || strings.Contains(strings.ToLower(sCwd), workspace) || strings.Contains(strings.ToLower(bID), workspace) {
				toStop = append(toStop, s)
				continue
			}
		}

		if bubbleID == "" && (workspace == "" || workspace == "all") {
			toStop = append(toStop, s)
		}
	}

	if len(toStop) == 0 {
		return "No se encontró ningún servidor de desarrollo activo que coincida con los criterios para detener.", nil
	}

	stopped := []string{}
	for _, s := range toStop {
		id, _ := s["id"].(string)
		name, _ := s["name"].(string)
		_ = apiRequest(fmt.Sprintf("/api/sessions/%s", id), "DELETE", nil, nil, false)
		stopped = append(stopped, fmt.Sprintf("- `%s` (%s)", name, id))
	}

	return fmt.Sprintf("🛑 **Servidor(es) de desarrollo detenido(s) exitosamente:**\n%s\n\nEl proceso y la terminal fueron cerrados en Orbit.", strings.Join(stopped, "\n")), nil
}

// GroqWhisperTools defines MCP tools exposed when the Groq Whisper plugin is active
var GroqWhisperTools = []Tool{
	{
		Name: "transcribe_video",
		Description: "Extrae el audio de un archivo de video o audio local (.mp4, .mkv, .mov, .webm, .mp3, .wav, etc.) con FFmpeg y lo transcribe a texto a velocidad ultra-rápida (LPU) usando la API de Groq Whisper. Ideal para procesar reuniones, grabaciones, demos, tutoriales y notas de voz.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Ruta absoluta al archivo de video o audio en el disco local (ej. C:/Users/.../video.mp4).",
				},
				"with_timestamps": map[string]interface{}{
					"type":        "boolean",
					"description": "Opcional. Si es true, devuelve la transcripción formateada con marcas de tiempo [MM:SS - MM:SS] por segmento.",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Opcional. Código de idioma ISO (ej. 'es', 'en', 'fr') o 'auto' para autodetección.",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Opcional. Contexto previo, vocabulario técnico o nombres propios para mejorar la precisión del modelo.",
				},
			},
			"required": []string{"file_path"},
		},
	},
	{
		Name: "extract_audio",
		Description: "Extrae únicamente la pista de audio de un archivo de video local a formato MP3 optimizado y ligero usando FFmpeg, sin transcribirlo.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"video_path": map[string]interface{}{
					"type":        "string",
					"description": "Ruta absoluta al archivo de video local.",
				},
				"output_path": map[string]interface{}{
					"type":        "string",
					"description": "Opcional. Ruta destino para guardar el archivo .mp3. Si se omite, se guarda junto al archivo de video.",
				},
				"bitrate": map[string]interface{}{
					"type":        "string",
					"description": "Opcional. Bitrate del MP3 (por defecto '48k' mono a 16kHz).",
				},
			},
			"required": []string{"video_path"},
		},
	},
}

// GetTools returns default MCP tools plus tools from any enabled plugins
func GetTools() []Tool {
	mgr := plugin.GetGlobalManager()
	if mgr != nil {
		mgr.Reload()
	}
	base := make([]Tool, len(Tools))
	copy(base, Tools)
	if mgr != nil && mgr.IsEnabled("groq_whisper") {
		base = append(base, GroqWhisperTools...)
	}
	return base
}

func handleTranscribeVideo(args map[string]interface{}) (string, error) {
	mgr := plugin.GetGlobalManager()
	if mgr == nil || !mgr.IsEnabled("groq_whisper") {
		return "", fmt.Errorf("el plugin 'Groq Whisper Video Transcriber' no está habilitado en Orbit. Actívalo en Orbit > Plugins")
	}

	filePath, _ := args["file_path"].(string)
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("el parámetro 'file_path' es obligatorio")
	}

	cfg := mgr.GetConfig("groq_whisper")
	apiKey, _ := cfg["api_key"].(string)
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("el plugin Groq Whisper está activado pero falta configurar la API Key. Configúrala en Orbit > Plugins")
	}

	model, _ := cfg["model"].(string)
	if model == "" {
		model = "whisper-large-v3-turbo"
	}

	lang, _ := args["language"].(string)
	if lang == "" {
		lang, _ = cfg["language"].(string)
	}

	withTimestamps := false
	if wt, ok := args["with_timestamps"].(bool); ok {
		withTimestamps = wt
	} else if defWt, ok := cfg["with_timestamps"].(bool); ok {
		withTimestamps = defWt
	}

	prompt, _ := args["prompt"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	res, err := plugin.TranscribeMedia(ctx, filePath, withTimestamps, prompt, apiKey, model, lang)
	if err != nil {
		return "", fmt.Errorf("error al transcribir archivo multimedia: %w", err)
	}

	summary := fmt.Sprintf("🎙️ **Transcripción completada con Groq Whisper (%s):**\n\n"+
		"- **Archivo:** `%s`\n"+
		"- **Idioma detectado:** `%s`\n"+
		"- **Duración:** `%.1fs` (~%.1f min)\n\n"+
		"### Transcripción:\n\n%s",
		model, filepath.Base(filePath), res.Language, res.Duration, res.Duration/60.0, res.FormattedText)

	return summary, nil
}

func handleExtractAudio(args map[string]interface{}) (string, error) {
	mgr := plugin.GetGlobalManager()
	if mgr == nil || !mgr.IsEnabled("groq_whisper") {
		return "", fmt.Errorf("el plugin 'Groq Whisper Video Transcriber' no está habilitado en Orbit. Actívalo en Orbit > Plugins")
	}

	videoPath, _ := args["video_path"].(string)
	videoPath = strings.TrimSpace(videoPath)
	if videoPath == "" || strings.HasPrefix(videoPath, "-") {
		return "", fmt.Errorf("el parámetro 'video_path' es inválido o no puede comenzar con '-'")
	}

	fi, err := os.Stat(videoPath)
	if err != nil || fi.IsDir() {
		return "", fmt.Errorf("archivo de video no encontrado o no es un archivo válido: %s", videoPath)
	}

	outputPath, _ := args["output_path"].(string)
	outputPath = strings.TrimSpace(outputPath)
	if outputPath != "" && strings.HasPrefix(outputPath, "-") {
		return "", fmt.Errorf("el parámetro 'output_path' no puede comenzar con '-'")
	}
	if outputPath != "" {
		if _, err := os.Stat(outputPath); err == nil {
			return "", fmt.Errorf("el archivo de destino '%s' ya existe. Especifica otro nombre para no sobrescribir", outputPath)
		}
	} else {
		ext := filepath.Ext(videoPath)
		baseOut := strings.TrimSuffix(videoPath, ext) + ".mp3"
		outputPath = baseOut
		if _, err := os.Stat(outputPath); err == nil {
			outputPath = fmt.Sprintf("%s_extracted_%d.mp3", strings.TrimSuffix(videoPath, ext), time.Now().Unix())
		}
	}

	bitrate, _ := args["bitrate"].(string)
	if bitrate == "" {
		bitrate = "48k"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := plugin.ExtractAudio(ctx, videoPath, outputPath, bitrate); err != nil {
		return "", fmt.Errorf("error extrayendo audio con ffmpeg: %w", err)
	}

	outFi, _ := os.Stat(outputPath)
	sizeMB := float64(0)
	if outFi != nil {
		sizeMB = float64(outFi.Size()) / (1024 * 1024)
	}

	return fmt.Sprintf("🎵 **Audio extraído exitosamente con FFmpeg:**\n\n"+
		"- **Video original:** `%s`\n"+
		"- **Audio generado:** `%s` (%.2f MB)\n"+
		"- **Bitrate:** `%s` (mono 16kHz optimizado para Whisper)",
		filepath.Base(videoPath), outputPath, sizeMB, bitrate), nil
}
