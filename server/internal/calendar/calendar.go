package calendar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"orbit/internal/git"
)

type HandoffItem struct {
	ID         string    `json:"id"`
	Date       string    `json:"date"`     // "YYYY-MM-DD"
	Time       string    `json:"time"`     // "HH:MM"
	BubbleID   string    `json:"bubbleId"` // e.g. "aditum-feature-tar-842"
	TaskName   string    `json:"taskName"` // e.g. "Aditum (TAR-842)"
	Agent      string    `json:"agent"`    // "claude", "agy", "developer"
	Status     string    `json:"status"`   // "completed", "in_progress", "blocked"
	Summary    string    `json:"summary"`  // What was completed today
	Pending    string    `json:"pending"`  // What remains pending for tomorrow
	Blockers   string    `json:"blockers,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	LastCommit string    `json:"lastCommit,omitempty"`
	DirtyFiles int       `json:"dirtyFiles,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type DaySummary struct {
	Date         string   `json:"date"` // "YYYY-MM-DD"
	HasNote      bool     `json:"hasNote"`
	HandoffCount int      `json:"handoffCount"`
	Tasks        []string `json:"tasks"`
}

type DayNote struct {
	Date     string        `json:"date"`
	Content  string        `json:"content"`
	Handoffs []HandoffItem `json:"handoffs"`
	Exists   bool          `json:"exists"`
	Path     string        `json:"path"`
}

type StandupSummary struct {
	ReferenceDate string        `json:"referenceDate"`
	PendingItems  []HandoffItem `json:"pendingItems"`
}

type HandoffDraft struct {
	BubbleID   string `json:"bubbleId"`
	TaskName   string `json:"taskName"`
	Agent      string `json:"agent"`
	Status     string `json:"status"` // "in_progress", "completed", "blocked"
	Summary    string `json:"summary"`
	Pending    string `json:"pending"`
	Blockers   string `json:"blockers"`
	Branch     string `json:"branch"`
	LastCommit string `json:"lastCommit"`
	DirtyFiles int    `json:"dirtyFiles"`
}

type Manager struct {
	mu        sync.RWMutex
	notesDir  string
	stateFile string
	handoffs  map[string][]HandoffItem // date -> items
}

var handoffMetaRegex = regexp.MustCompile(`<!--\s*HANDOFF_META:\s*({.*?})\s*-->`)

func NewManager(notesDir ...string) *Manager {
	dir := "calendar_notes"
	if len(notesDir) > 0 && notesDir[0] != "" {
		dir = notesDir[0]
	} else if fi, err := os.Stat(filepath.Join("data", "calendar_notes")); err == nil && fi.IsDir() {
		dir = filepath.Join("data", "calendar_notes")
	}

	_ = os.MkdirAll(dir, 0755)

	m := &Manager{
		notesDir:  dir,
		stateFile: filepath.Join(dir, "handoffs_registry.json"),
		handoffs:  make(map[string][]HandoffItem),
	}
	m.loadState()
	return m
}

func (m *Manager) loadState() {
	data, err := os.ReadFile(m.stateFile)
	if err != nil && m.stateFile != filepath.Join("calendar_notes", "handoffs_registry.json") {
		data, err = os.ReadFile(filepath.Join("calendar_notes", "handoffs_registry.json"))
	}
	if err != nil {
		return
	}
	var raw map[string][]HandoffItem
	if err := json.Unmarshal(data, &raw); err == nil {
		m.handoffs = raw
	}
}

func (m *Manager) saveState() error {
	data, err := json.MarshalIndent(m.handoffs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.stateFile, data, 0644)
}

func (m *Manager) getNoteFilePath(date string) string {
	return filepath.Join(m.notesDir, fmt.Sprintf("%s.md", date))
}

// GetDayNote loads the markdown note and associated structured handoffs for date (YYYY-MM-DD)
func (m *Manager) GetDayNote(date string) (*DayNote, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filePath := m.getNoteFilePath(date)
	content := ""
	exists := false

	if data, err := os.ReadFile(filePath); err == nil {
		content = string(data)
		exists = true
	} else {
		// Provide default template for new day
		content = fmt.Sprintf("# 📅 Diario de Trabajo - %s\n\n## 📌 Notas Generales\n- [ ] \n\n## 🌙 Handoffs de Tareas & Agentes\n", date)
	}

	handoffs := m.handoffs[date]
	if handoffs == nil {
		handoffs = make([]HandoffItem, 0)
	}

	return &DayNote{
		Date:     date,
		Content:  content,
		Handoffs: handoffs,
		Exists:   exists,
		Path:     filePath,
	}, nil
}

// SaveDayNote persists the raw markdown content for a given date
func (m *Manager) SaveDayNote(date string, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := m.getNoteFilePath(date)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save daily note: %w", err)
	}
	return nil
}

// AddDailyTodo appends or inserts a checklist item into the "Notas Generales" section of the daily note
func (m *Manager) AddDailyTodo(date string, text string, checked bool) (*DayNote, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	note, err := m.GetDayNote(date)
	if err != nil {
		return nil, err
	}

	checkbox := "- [ ] "
	if checked {
		checkbox = "- [x] "
	}
	todoLine := checkbox + strings.TrimSpace(text)

	content := note.Content
	targetHeader := "## 📌 Notas Generales"

	if idx := strings.Index(content, targetHeader); idx != -1 {
		rest := content[idx+len(targetHeader):]
		trimmedRest := strings.TrimLeft(rest, "\r\n ")
		if strings.HasPrefix(trimmedRest, "- [ ]") && (len(trimmedRest) == 5 || trimmedRest[5] == '\r' || trimmedRest[5] == '\n') {
			content = content[:idx+len(targetHeader)] + "\n" + todoLine + trimmedRest[5:]
		} else {
			content = content[:idx+len(targetHeader)] + "\n" + todoLine + rest
		}
	} else {
		content = content + "\n\n" + targetHeader + "\n" + todoLine + "\n"
	}

	if err := m.SaveDayNote(date, content); err != nil {
		return nil, err
	}
	return m.GetDayNote(date)
}

// AppendDailyNote appends text/notes under the "Notas Generales" section
func (m *Manager) AppendDailyNote(date string, text string) (*DayNote, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	note, err := m.GetDayNote(date)
	if err != nil {
		return nil, err
	}

	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return note, nil
	}

	content := note.Content
	targetHeader := "## 📌 Notas Generales"

	if idx := strings.Index(content, targetHeader); idx != -1 {
		rest := content[idx+len(targetHeader):]
		content = content[:idx+len(targetHeader)] + "\n" + trimmedText + rest
	} else {
		content = content + "\n\n" + targetHeader + "\n" + trimmedText + "\n"
	}

	if err := m.SaveDayNote(date, content); err != nil {
		return nil, err
	}
	return m.GetDayNote(date)
}

// AddHandoff records a structured end-of-day handoff and updates the daily note markdown
func (m *Manager) AddHandoff(item HandoffItem, workDir string) (*HandoffItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.Date == "" {
		item.Date = time.Now().Format("2006-01-02")
	}
	if item.Time == "" {
		item.Time = time.Now().Format("15:04")
	}
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	// Capture automatic Git snapshot if directory is provided
	if workDir != "" {
		if branch, err := git.GetBranch(workDir); err == nil && branch != "" {
			item.Branch = branch
		}
		if logs, err := git.GetRecentLog(workDir, 1); err == nil && len(logs) > 0 {
			item.LastCommit = fmt.Sprintf("%s (%s)", logs[0].Hash, logs[0].Message)
		}
		if st, err := git.GetStatus(workDir); err == nil && st != nil {
			item.DirtyFiles = len(st.Changes)
		}
	}

	// Append to registry
	m.handoffs[item.Date] = append(m.handoffs[item.Date], item)
	_ = m.saveState()
	m.syncNoteMarkdown(item.Date)

	return &item, nil
}

func indentLines(text string, indent string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			lines[i] = indent + l
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Manager) renderHandoffBlock(item HandoffItem) string {
	statusEmoji := "🟡 En Progreso"
	switch item.Status {
	case "completed":
		statusEmoji = "🟢 Completado"
	case "blocked":
		statusEmoji = "🔴 Bloqueado"
	}

	agentIcon := "🤖 Claude Code"
	switch item.Agent {
	case "agy":
		agentIcon = "🚀 Antigravity CLI"
	case "developer":
		agentIcon = "👤 Desarrollador"
	}

	var block strings.Builder
	block.WriteString(fmt.Sprintf("\n### %s | %s (%s)\n", item.TaskName, agentIcon, item.Time))
	block.WriteString(fmt.Sprintf("- **Estado:** %s\n", statusEmoji))
	if item.Summary != "" {
		if strings.Contains(item.Summary, "\n") {
			block.WriteString("- **Lo completado hoy:**\n" + indentLines(item.Summary, "  ") + "\n")
		} else {
			block.WriteString(fmt.Sprintf("- **Lo completado hoy:** %s\n", item.Summary))
		}
	}
	if item.Pending != "" {
		if strings.Contains(item.Pending, "\n") {
			block.WriteString("- **Pendiente para mañana:**\n" + indentLines(item.Pending, "  ") + "\n")
		} else {
			block.WriteString(fmt.Sprintf("- **Pendiente para mañana:** %s\n", item.Pending))
		}
	}
	if item.Blockers != "" {
		block.WriteString(fmt.Sprintf("- **Bloqueos / Atención:** %s\n", item.Blockers))
	}
	if item.Branch != "" || item.LastCommit != "" {
		gitInfo := fmt.Sprintf("- **Git Snapshot:** Rama `%s`", item.Branch)
		if item.LastCommit != "" {
			gitInfo += fmt.Sprintf(" | Commit `%s`", item.LastCommit)
		}
		if item.DirtyFiles > 0 {
			gitInfo += fmt.Sprintf(" | %d archivo(s) sin commitear", item.DirtyFiles)
		} else {
			gitInfo += " | Árbol limpio"
		}
		block.WriteString(gitInfo + "\n")
	}
	return block.String()
}

func (m *Manager) syncNoteMarkdown(date string) {
	filePath := m.getNoteFilePath(date)
	existingContent := ""
	if data, err := os.ReadFile(filePath); err == nil {
		existingContent = string(data)
	} else {
		existingContent = fmt.Sprintf("# 📅 Diario de Trabajo - %s\n\n## 📌 Notas Generales\n- [ ] \n\n## 🌙 Handoffs de Tareas & Agentes\n", date)
	}

	separator := "## 🌙 Handoffs de Tareas & Agentes"
	baseContent := existingContent
	if idx := strings.Index(existingContent, separator); idx != -1 {
		baseContent = strings.TrimRight(existingContent[:idx], " \r\n") + "\n\n"
	} else {
		baseContent = strings.TrimRight(existingContent, " \r\n") + "\n\n"
	}

	var sb strings.Builder
	sb.WriteString(baseContent)
	sb.WriteString(separator + "\n")

	items := m.handoffs[date]
	for _, it := range items {
		sb.WriteString(m.renderHandoffBlock(it))
	}

	_ = os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// UpdateHandoff modifies an existing handoff entry and updates the daily note
func (m *Manager) UpdateHandoff(item HandoffItem) (*HandoffItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetDate := item.Date
	if targetDate == "" {
		for d, list := range m.handoffs {
			for _, h := range list {
				if h.ID == item.ID {
					targetDate = d
					break
				}
			}
			if targetDate != "" {
				break
			}
		}
	}

	if targetDate == "" {
		return nil, fmt.Errorf("handoff with id %s not found", item.ID)
	}

	items, ok := m.handoffs[targetDate]
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("no handoffs found for date %s", targetDate)
	}

	foundIdx := -1
	for i, h := range items {
		if h.ID == item.ID {
			foundIdx = i
			break
		}
	}
	if foundIdx == -1 {
		return nil, fmt.Errorf("handoff with id %s not found in date %s", item.ID, targetDate)
	}

	target := &items[foundIdx]
	if item.TaskName != "" {
		target.TaskName = item.TaskName
	}
	if item.Agent != "" {
		target.Agent = item.Agent
	}
	if item.Status != "" {
		target.Status = item.Status
	}
	target.Summary = item.Summary
	target.Pending = item.Pending
	target.Blockers = item.Blockers
	if item.Time != "" {
		target.Time = item.Time
	}
	if item.Branch != "" {
		target.Branch = item.Branch
	}
	if item.LastCommit != "" {
		target.LastCommit = item.LastCommit
	}
	if item.DirtyFiles != 0 {
		target.DirtyFiles = item.DirtyFiles
	}

	_ = m.saveState()
	m.syncNoteMarkdown(targetDate)

	res := *target
	return &res, nil
}

// DeleteHandoff removes a handoff entry by ID and updates the daily note
func (m *Manager) DeleteHandoff(date string, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetDate := date
	if targetDate == "" {
		for d, list := range m.handoffs {
			for _, h := range list {
				if h.ID == id {
					targetDate = d
					break
				}
			}
			if targetDate != "" {
				break
			}
		}
	}

	if targetDate == "" {
		return fmt.Errorf("handoff with id %s not found", id)
	}

	items, ok := m.handoffs[targetDate]
	if !ok || len(items) == 0 {
		return fmt.Errorf("no handoffs found for date %s", targetDate)
	}

	newItems := make([]HandoffItem, 0, len(items))
	found := false
	for _, h := range items {
		if h.ID == id {
			found = true
			continue
		}
		newItems = append(newItems, h)
	}

	if !found {
		return fmt.Errorf("handoff with id %s not found in date %s", id, targetDate)
	}

	m.handoffs[targetDate] = newItems
	_ = m.saveState()
	m.syncNoteMarkdown(targetDate)

	return nil
}

// GetMonthSummary returns days with notes or handoffs for the specified year and month
func (m *Manager) GetMonthSummary(year int, month int) ([]DaySummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := fmt.Sprintf("%04d-%02d-", year, month)
	daysMap := make(map[string]*DaySummary)

	// Check files on disk
	entries, err := os.ReadDir(m.notesDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".md") {
				date := strings.TrimSuffix(e.Name(), ".md")
				daysMap[date] = &DaySummary{
					Date:         date,
					HasNote:      true,
					HandoffCount: 0,
					Tasks:        make([]string, 0),
				}
			}
		}
	}

	// Cross-reference with handoffs registry
	for date, items := range m.handoffs {
		if strings.HasPrefix(date, prefix) {
			summary, ok := daysMap[date]
			if !ok {
				summary = &DaySummary{
					Date:         date,
					HasNote:      false,
					HandoffCount: 0,
					Tasks:        make([]string, 0),
				}
				daysMap[date] = summary
			}
			summary.HandoffCount = len(items)
			taskSet := make(map[string]struct{})
			for _, item := range items {
				if item.TaskName != "" {
					taskSet[item.TaskName] = struct{}{}
				}
			}
			for t := range taskSet {
				summary.Tasks = append(summary.Tasks, t)
			}
			sort.Strings(summary.Tasks)
		}
	}

	result := make([]DaySummary, 0, len(daysMap))
	for _, s := range daysMap {
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result, nil
}

// GetStandup finds the most recent day with handoffs prior to or on referenceDate
// and returns the pending items for morning standup
func (m *Manager) GetStandup(referenceDate ...string) (*StandupSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targetDate := time.Now().Format("2006-01-02")
	if len(referenceDate) > 0 && referenceDate[0] != "" {
		targetDate = referenceDate[0]
	}

	// Gather sorted list of dates with handoffs
	var dates []string
	for d, items := range m.handoffs {
		if len(items) > 0 {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)

	if len(dates) == 0 {
		return &StandupSummary{
			ReferenceDate: "",
			PendingItems:  make([]HandoffItem, 0),
		}, nil
	}

	// Find the most recent date <= targetDate (or if today has no handoffs yet, the latest previous day)
	bestDate := ""
	for i := len(dates) - 1; i >= 0; i-- {
		d := dates[i]
		if d < targetDate {
			bestDate = d
			break
		} else if d == targetDate {
			// If today already has handoffs, check if there are pending items
			bestDate = d
			break
		}
	}

	if bestDate == "" {
		// If no date was <= targetDate, pick the oldest available
		bestDate = dates[0]
	}

	items := m.handoffs[bestDate]
	pending := make([]HandoffItem, 0)
	for _, it := range items {
		if it.Status != "completed" || it.Pending != "" {
			pending = append(pending, it)
		}
	}

	return &StandupSummary{
		ReferenceDate: bestDate,
		PendingItems:  pending,
	}, nil
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\r`)

func cleanTerminalAnsi(input string) string {
	cleaned := ansiRegex.ReplaceAllString(input, "")
	return strings.TrimSpace(cleaned)
}

// GenerateDraft builds an intelligent, automated handoff proposal based on Git changes and terminal activity
func (m *Manager) GenerateDraft(bubbleID, bubbleName, workDir, agentRole, terminalSnippet string) (*HandoffDraft, error) {
	draft := &HandoffDraft{
		BubbleID: bubbleID,
		TaskName: bubbleName,
		Agent:    "claude",
		Status:   "in_progress",
	}

	if agentRole == "agy" {
		draft.Agent = "agy"
	} else if agentRole == "developer" {
		draft.Agent = "developer"
	} else if agentRole == "claude" {
		draft.Agent = "claude"
	}

	var branch string
	var commits []git.CommitInfo
	var status *git.StatusResult

	if workDir != "" {
		if b, err := git.GetBranch(workDir); err == nil && b != "" {
			branch = b
			draft.Branch = b
		}
		if logs, err := git.GetRecentLog(workDir, 5); err == nil && len(logs) > 0 {
			commits = logs
			draft.LastCommit = fmt.Sprintf("%s (%s)", logs[0].Hash, logs[0].Message)
		}
		if st, err := git.GetStatus(workDir); err == nil && st != nil {
			status = st
			draft.DirtyFiles = len(st.Changes)
		}
	}

	// Status determination
	if draft.DirtyFiles > 0 {
		draft.Status = "in_progress"
	} else if len(commits) > 0 {
		draft.Status = "completed"
	}

	// Build Summary
	var sumParts []string
	if len(commits) > 0 {
		var commitLines []string
		for i, c := range commits {
			if i >= 3 {
				break
			}
			commitLines = append(commitLines, fmt.Sprintf("• %s (%s)", c.Message, c.Hash))
		}
		sumParts = append(sumParts, "Commits recientes:\n"+strings.Join(commitLines, "\n"))
	}

	if status != nil && len(status.Changes) > 0 {
		var fileNames []string
		for i, ch := range status.Changes {
			if i >= 4 {
				fileNames = append(fileNames, fmt.Sprintf("... y %d más", len(status.Changes)-4))
				break
			}
			fileNames = append(fileNames, filepath.Base(ch.Path))
		}
		sumParts = append(sumParts, fmt.Sprintf("Archivos modificados (%d): %s", len(status.Changes), strings.Join(fileNames, ", ")))
	}

	// Look at terminal snippet if available
	cleanedTerm := cleanTerminalAnsi(terminalSnippet)
	if cleanedTerm != "" {
		lines := strings.Split(cleanedTerm, "\n")
		var meaningful []string
		for i := len(lines) - 1; i >= 0 && len(meaningful) < 3; i-- {
			l := strings.TrimSpace(lines[i])
			if len(l) > 15 && !strings.HasPrefix(l, "PS ") && !strings.HasPrefix(l, "C:\\") && !strings.Contains(l, "webpack") && !strings.Contains(l, "node_modules") {
				meaningful = append([]string{l}, meaningful...)
			}
		}
		if len(meaningful) > 0 {
			sumParts = append(sumParts, "Última actividad en consola:\n• "+strings.Join(meaningful, "\n• "))
		}
	}

	if len(sumParts) == 0 {
		if branch != "" {
			draft.Summary = "Avance general en la rama " + branch
		} else {
			draft.Summary = "Avance general en la tarea " + bubbleName
		}
	} else {
		draft.Summary = strings.Join(sumParts, "\n\n")
	}

	// Build Pending
	var pendParts []string
	if draft.DirtyFiles > 0 {
		pendParts = append(pendParts, fmt.Sprintf("• Revisar y commitear %d archivos pendientes de cambios locales.", draft.DirtyFiles))
	}
	if branch != "" && branch != "main" && branch != "master" {
		pendParts = append(pendParts, fmt.Sprintf("• Continuar desarrollo y pruebas en la rama '%s'.", branch))
	} else {
		pendParts = append(pendParts, "• Continuar con el siguiente ticket o integración.")
	}
	draft.Pending = strings.Join(pendParts, "\n")

	return draft, nil
}

