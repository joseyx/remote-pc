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
	"remotepc-server/internal/git"
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

	// Append to markdown file
	filePath := m.getNoteFilePath(item.Date)
	existingContent := ""
	if data, err := os.ReadFile(filePath); err == nil {
		existingContent = string(data)
	} else {
		existingContent = fmt.Sprintf("# 📅 Diario de Trabajo - %s\n\n## 📌 Notas Generales\n- [ ] \n\n## 🌙 Handoffs de Tareas & Agentes\n", item.Date)
	}

	// Build markdown block for this handoff
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
		block.WriteString(fmt.Sprintf("- **Lo completado hoy:** %s\n", item.Summary))
	}
	if item.Pending != "" {
		block.WriteString(fmt.Sprintf("- **Pendiente para mañana:** %s\n", item.Pending))
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

	if !strings.Contains(existingContent, "## 🌙 Handoffs de Tareas & Agentes") {
		existingContent += "\n\n## 🌙 Handoffs de Tareas & Agentes\n"
	}
	updatedContent := existingContent + block.String()
	_ = os.WriteFile(filePath, []byte(updatedContent), 0644)

	return &item, nil
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
