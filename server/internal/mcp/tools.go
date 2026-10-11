package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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
		filepath.Join(exeDir, "server.config.json"),
		filepath.Join(localApp, "RemotePC", "server.config.json"),
		filepath.Join(userProfile, "Documents", "Personal Projects", "Remote PC", "server.config.json"),
		filepath.Join(userProfile, "Documents", "Personal Projects", "Remote PC", "server", "server.config.json"),
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

	candidatePaths := []string{
		exePath,
		filepath.Join(exeDir, "remotepc-server.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "RemotePC", "remotepc-server.exe"),
		filepath.Join(os.Getenv("USERPROFILE"), "Documents", "Personal Projects", "Remote PC", "remotepc-server.exe"),
		filepath.Join(os.Getenv("USERPROFILE"), "Documents", "Personal Projects", "Remote PC", "server", "remotepc-server.exe"),
	}

	for _, p := range candidatePaths {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command("cmd", "/c", "start", "", "/b", p, "-no-open")
			cmd.Dir = filepath.Dir(p)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CreationFlags: 0x08000000, // CREATE_NO_WINDOW
			}
			if err := cmd.Start(); err == nil {
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
		return fmt.Errorf("error conectando con Remote PC en %s: %w. Asegúrate de que remotepc-server esté en ejecución", url, err)
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
