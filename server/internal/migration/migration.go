package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"orbit/internal/bubble"
)

type MigrationSummary struct {
	TargetDir       string   `json:"targetDir"`
	SourceDirs      []string `json:"sourceDirs"`
	WorkspacesCount int      `json:"workspacesCount"`
	WorkspacesAdded []string `json:"workspacesAdded"`
	FilesCopied     []string `json:"filesCopied"`
	HandoffsCount   int      `json:"handoffsCount"`
	NotesCount      int      `json:"notesCount"`
	MetadataKeys    int      `json:"metadataKeys"`
	Errors          []string `json:"errors,omitempty"`
}

// FindPotentialSources returns a list of candidate folders containing existing Orbit data
func FindPotentialSources(targetDir string) []string {
	cleanTarget := filepath.Clean(targetDir)
	var candidates []string

	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, "Documents", "Personal Projects", "Remote PC", "data"),
			filepath.Join(home, "Documents", "Remote PC", "data"),
			filepath.Join(home, "Documents", "Orbit", "data"),
		)
	}

	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		candidates = append(candidates,
			filepath.Join(localApp, "Orbit", "data"),
			filepath.Join(localApp, "RemotePC", "data"),
		)
	}

	if appData := os.Getenv("APPDATA"); appData != "" {
		candidates = append(candidates,
			filepath.Join(appData, "Orbit", "data"),
			filepath.Join(appData, "RemotePC", "data"),
		)
	}

	// Working directory relative candidates
	candidates = append(candidates, "data", filepath.Join("..", "data"))

	// Claude Desktop configuration inspection
	if appData := os.Getenv("APPDATA"); appData != "" {
		claudeCfg := filepath.Join(appData, "Claude", "claude_desktop_config.json")
		if data, err := os.ReadFile(claudeCfg); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, "orbit") || strings.Contains(line, "remotepc") {
					cleanLine := strings.ReplaceAll(line, `\\`, `/`)
					for _, part := range strings.Fields(cleanLine) {
						part = strings.Trim(part, `",:[]{} `)
						if strings.HasSuffix(strings.ToLower(part), ".exe") {
							dir := filepath.Dir(part)
							candidates = append(candidates,
								filepath.Join(dir, "data"),
								filepath.Join(dir, "..", "data"),
							)
						}
					}
				}
			}
		}
	}

	seen := make(map[string]bool)
	var validSources []string

	hasRealData := func(dir string) bool {
		for _, name := range []string{"calendar_notes", "server.config.json", "bubbles_metadata.json", "sessions_state.json", "workspaces.json"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		return false
	}

	for _, c := range candidates {
		clean := filepath.Clean(c)
		if clean == cleanTarget || seen[clean] {
			continue
		}
		seen[clean] = true

		if fi, err := os.Stat(clean); err == nil && fi.IsDir() && hasRealData(clean) {
			validSources = append(validSources, clean)
		}
	}

	return validSources
}

// MigrateData merges previous configurations, notes, handoffs, and initializes workspaces.json
func MigrateData(targetDir string, explicitSources ...string) (*MigrationSummary, error) {
	if targetDir == "" {
		return nil, fmt.Errorf("targetDir no puede ser vacío")
	}

	cleanTarget := filepath.Clean(targetDir)
	_ = os.MkdirAll(cleanTarget, 0755)

	summary := &MigrationSummary{
		TargetDir: cleanTarget,
	}

	sources := explicitSources
	if len(sources) == 0 {
		sources = FindPotentialSources(cleanTarget)
	}
	summary.SourceDirs = sources

	// 1. Process data migration from each identified source
	for _, src := range sources {
		if filepath.Clean(src) == cleanTarget {
			continue
		}
		migrateFromSource(src, cleanTarget, summary)
	}

	// 2. Discover and seed workspaces in targetDir
	if err := migrateWorkspaces(cleanTarget, summary); err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("workspaces error: %v", err))
	}

	// 3. Count final handoffs and notes in targetDir
	countTargetRecords(cleanTarget, summary)

	return summary, nil
}

func migrateFromSource(src, dst string, s *MigrationSummary) {
	// a. Migrate calendar_notes folder (daily notes + handoffs)
	srcNotesDir := filepath.Join(src, "calendar_notes")
	dstNotesDir := filepath.Join(dst, "calendar_notes")
	if fi, err := os.Stat(srcNotesDir); err == nil && fi.IsDir() {
		_ = os.MkdirAll(dstNotesDir, 0755)

		entries, _ := os.ReadDir(srcNotesDir)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			srcFile := filepath.Join(srcNotesDir, entry.Name())
			dstFile := filepath.Join(dstNotesDir, entry.Name())

			if entry.Name() == "handoffs_registry.json" {
				mergeHandoffsRegistry(srcFile, dstFile, s)
			} else if strings.HasSuffix(entry.Name(), ".md") {
				// Copy if not exists in dst or if src is newer
				copyIfNewer(srcFile, dstFile, s)
			}
		}
	}

	// b. Merge bubbles_metadata.json
	mergeBubblesMetadata(filepath.Join(src, "bubbles_metadata.json"), filepath.Join(dst, "bubbles_metadata.json"), s)

	// c. Copy server.config.json if target doesn't have it
	copyIfNotExists(filepath.Join(src, "server.config.json"), filepath.Join(dst, "server.config.json"), s)

	// d. Copy sessions_state.json if target doesn't have it
	copyIfNotExists(filepath.Join(src, "sessions_state.json"), filepath.Join(dst, "sessions_state.json"), s)
}

func copyIfNewer(src, dst string, s *MigrationSummary) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return
	}

	dstInfo, err := os.Stat(dst)
	if os.IsNotExist(err) || (err == nil && srcInfo.ModTime().After(dstInfo.ModTime())) {
		data, err := os.ReadFile(src)
		if err == nil {
			if err := os.WriteFile(dst, data, 0644); err == nil {
				s.FilesCopied = append(s.FilesCopied, filepath.Base(dst))
			}
		}
	}
}

func copyIfNotExists(src, dst string, s *MigrationSummary) {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if data, err := os.ReadFile(src); err == nil {
			if err := os.WriteFile(dst, data, 0644); err == nil {
				s.FilesCopied = append(s.FilesCopied, filepath.Base(dst))
			}
		}
	}
}

type genericHandoff struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Time      string `json:"time"`
	BubbleID  string `json:"bubbleId"`
	TaskName  string `json:"taskName"`
	Agent     string `json:"agent"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Pending   string `json:"pending"`
	Blockers  string `json:"blockers"`
	Branch    string `json:"branch"`
	CreatedAt string `json:"createdAt"`
}

func mergeHandoffsRegistry(srcFile, dstFile string, s *MigrationSummary) {
	srcData, err := os.ReadFile(srcFile)
	if err != nil {
		return
	}

	var srcReg map[string][]genericHandoff
	if err := json.Unmarshal(srcData, &srcReg); err != nil {
		return
	}

	dstReg := make(map[string][]genericHandoff)
	if dstData, err := os.ReadFile(dstFile); err == nil {
		_ = json.Unmarshal(dstData, &dstReg)
	}

	// Merge by date and unique ID
	for date, items := range srcReg {
		existing := dstReg[date]
		seenIDs := make(map[string]bool)
		for _, it := range existing {
			seenIDs[it.ID] = true
		}

		for _, it := range items {
			if !seenIDs[it.ID] {
				existing = append(existing, it)
				seenIDs[it.ID] = true
			}
		}
		dstReg[date] = existing
	}

	out, err := json.MarshalIndent(dstReg, "", "  ")
	if err == nil {
		_ = os.WriteFile(dstFile, out, 0644)
	}
}

func mergeBubblesMetadata(srcFile, dstFile string, s *MigrationSummary) {
	srcData, err := os.ReadFile(srcFile)
	if err != nil {
		return
	}

	var srcMeta map[string]bubble.BubbleMeta
	if err := json.Unmarshal(srcData, &srcMeta); err != nil {
		return
	}

	dstMeta := make(map[string]bubble.BubbleMeta)
	if dstData, err := os.ReadFile(dstFile); err == nil {
		_ = json.Unmarshal(dstData, &dstMeta)
	}

	for k, v := range srcMeta {
		destV, exists := dstMeta[k]
		if !exists {
			dstMeta[k] = v
		} else {
			if destV.Priority == "" && v.Priority != "" {
				destV.Priority = v.Priority
			}
			if destV.CustomRank == 0 && v.CustomRank != 0 {
				destV.CustomRank = v.CustomRank
			}
			dstMeta[k] = destV
		}
	}

	out, err := json.MarshalIndent(dstMeta, "", "  ")
	if err == nil {
		_ = os.WriteFile(dstFile, out, 0644)
	}
	s.MetadataKeys = len(dstMeta)
}

func migrateWorkspaces(targetDir string, s *MigrationSummary) error {
	wsPath := filepath.Join(targetDir, "workspaces.json")
	var existingList []bubble.WorkspaceInfo

	if data, err := os.ReadFile(wsPath); err == nil {
		_ = json.Unmarshal(data, &existingList)
	}

	wsMap := make(map[string]bubble.WorkspaceInfo)
	for _, ws := range existingList {
		wsMap[ws.ID] = ws
	}

	// Auto-discovery of existing known workspaces
	home, _ := os.UserHomeDir()
	checkPaths := func(candidates ...string) string {
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && fi.IsDir() {
				return filepath.Clean(c)
			}
		}
		return ""
	}

	// 1. Aditum
	if _, ok := wsMap["aditum"]; !ok {
		aditumPath := checkPaths(
			`C:\Users\Jose\Documents\Work\Aditum`,
			filepath.Join(home, "Documents", "Work", "Aditum"),
			filepath.Join(home, "Work", "Aditum"),
		)
		if aditumPath != "" {
			ws := bubble.WorkspaceInfo{
				ID:          "aditum",
				Name:        "Aditum",
				Type:        bubble.WorkspaceAditum,
				BasePath:    aditumPath,
				VaultAlias:  "aditum",
				DefaultBase: "development",
				DefaultPort: 8080,
				DevCommand:  "./scripts/start-backend.sh",
				Description: "Java / Spring Boot + MySQL 8.0 local + Liquibase",
			}
			wsMap["aditum"] = ws
			s.WorkspacesAdded = append(s.WorkspacesAdded, "Aditum")
		}
	}

	// 2. Brif
	if _, ok := wsMap["brif"]; !ok {
		brifPath := checkPaths(
			`C:\Users\Jose\Documents\Work\brif`,
			filepath.Join(home, "Documents", "Work", "brif"),
			filepath.Join(home, "Work", "brif"),
		)
		if brifPath != "" {
			ws := bubble.WorkspaceInfo{
				ID:          "brif",
				Name:        "Brif",
				Type:        bubble.WorkspaceBrif,
				BasePath:    brifPath,
				VaultAlias:  "brif",
				DefaultBase: "develop",
				DefaultPort: 8000,
				DevCommand:  "python manage.py runserver 0.0.0.0:8000",
				Description: "Django + Scrapy + Celery + GitLab (monorepo y worktrees)",
			}
			wsMap["brif"] = ws
			s.WorkspacesAdded = append(s.WorkspacesAdded, "Brif")
		}
	}

	// 3. General
	if _, ok := wsMap["general"]; !ok {
		genPath := checkPaths(
			`C:\Users\Jose\Documents\Work`,
			filepath.Join(home, "Documents", "Work"),
			filepath.Join(home, "Work"),
		)
		if genPath != "" {
			ws := bubble.WorkspaceInfo{
				ID:          "general",
				Name:        "General",
				Type:        bubble.WorkspaceGeneral,
				BasePath:    genPath,
				VaultAlias:  "aditum",
				DefaultBase: "main",
				Description: "Herramientas y proyectos generales",
			}
			wsMap["general"] = ws
			s.WorkspacesAdded = append(s.WorkspacesAdded, "General")
		}
	}

	// 4. Auto-discover from sessions_state.json cwd entries
	sessionsFile := filepath.Join(targetDir, "sessions_state.json")
	if sessData, err := os.ReadFile(sessionsFile); err == nil {
		var sessions []struct {
			Cwd string `json:"cwd"`
		}
		if err := json.Unmarshal(sessData, &sessions); err == nil {
			for _, sess := range sessions {
				cwd := strings.TrimSpace(sess.Cwd)
				if cwd == "" {
					continue
				}
				cleanCwd := filepath.Clean(cwd)
				// Check if this cwd is a git repo and not already inside any known workspace
				if isGitRoot(cleanCwd) {
					baseName := filepath.Base(cleanCwd)
					slug := strings.ToLower(strings.ReplaceAll(baseName, " ", "-"))
					if _, exists := wsMap[slug]; !exists && !isInsideExistingWorkspace(cleanCwd, wsMap) {
						wsMap[slug] = bubble.WorkspaceInfo{
							ID:          slug,
							Name:        baseName,
							Type:        bubble.WorkspaceType(slug),
							BasePath:    cleanCwd,
							DefaultBase: "main",
							Description: "Proyecto descubierto automáticamente",
						}
						s.WorkspacesAdded = append(s.WorkspacesAdded, baseName)
					}
				}
			}
		}
	}

	// Convert map back to list
	finalList := make([]bubble.WorkspaceInfo, 0, len(wsMap))
	for _, ws := range wsMap {
		finalList = append(finalList, ws)
	}
	sort.Slice(finalList, func(i, j int) bool {
		return finalList[i].Name < finalList[j].Name
	})

	s.WorkspacesCount = len(finalList)
	out, err := json.MarshalIndent(finalList, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(wsPath, out, 0644)
}

func isGitRoot(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	if fi, err := os.Stat(gitDir); err == nil {
		return fi.IsDir() || !fi.IsDir() // .git can be a file in worktrees/submodules
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func isInsideExistingWorkspace(dir string, wsMap map[string]bubble.WorkspaceInfo) bool {
	cleanDir := strings.ToLower(filepath.Clean(dir))
	for _, ws := range wsMap {
		cleanWs := strings.ToLower(filepath.Clean(ws.BasePath))
		if cleanDir == cleanWs || strings.HasPrefix(cleanDir, cleanWs+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func countTargetRecords(targetDir string, s *MigrationSummary) {
	notesDir := filepath.Join(targetDir, "calendar_notes")
	if entries, err := os.ReadDir(notesDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") {
				s.NotesCount++
			}
		}
	}

	regFile := filepath.Join(notesDir, "handoffs_registry.json")
	if data, err := os.ReadFile(regFile); err == nil {
		var reg map[string][]genericHandoff
		if err := json.Unmarshal(data, &reg); err == nil {
			for _, items := range reg {
				s.HandoffsCount += len(items)
			}
		}
	}

	metaFile := filepath.Join(targetDir, "bubbles_metadata.json")
	if data, err := os.ReadFile(metaFile); err == nil {
		var meta map[string]bubble.BubbleMeta
		if err := json.Unmarshal(data, &meta); err == nil {
			s.MetadataKeys = len(meta)
		}
	}
}

// AutoMigrate is executed on server startup if workspaces.json does not exist
func AutoMigrate(targetDir string) (*MigrationSummary, error) {
	wsPath := filepath.Join(targetDir, "workspaces.json")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		return MigrateData(targetDir)
	}
	return nil, nil
}

// FormatCLI renders a clean, friendly CLI report
func FormatCLI(s *MigrationSummary) string {
	var sb strings.Builder
	sb.WriteString("\n======================================================\n")
	sb.WriteString("🚀 ORBIT - MIGRACIÓN DE DATOS & WORKSPACES UNIVERSALES\n")
	sb.WriteString("======================================================\n")
	sb.WriteString(fmt.Sprintf("• Directorio destino: %s\n", s.TargetDir))

	if len(s.SourceDirs) > 0 {
		sb.WriteString("• Fuentes de datos migradas:\n")
		for _, src := range s.SourceDirs {
			sb.WriteString(fmt.Sprintf("    - %s\n", src))
		}
	} else {
		sb.WriteString("• Fuentes previas: Ninguna externa (inicialización limpia)\n")
	}

	sb.WriteString(fmt.Sprintf("• Workspaces configurados (%d):\n", s.WorkspacesCount))
	if len(s.WorkspacesAdded) > 0 {
		for _, w := range s.WorkspacesAdded {
			sb.WriteString(fmt.Sprintf("    + Registrado: %s\n", w))
		}
	}

	sb.WriteString(fmt.Sprintf("• Handoffs de IA preservados: %d registros\n", s.HandoffsCount))
	sb.WriteString(fmt.Sprintf("• Notas diarias migradas: %d archivos (.md)\n", s.NotesCount))
	sb.WriteString(fmt.Sprintf("• Prioridades y Ranks Notion: %d tareas\n", s.MetadataKeys))

	if len(s.FilesCopied) > 0 {
		sb.WriteString(fmt.Sprintf("• Archivos copiados/actualizados: %d\n", len(s.FilesCopied)))
	}

	if len(s.Errors) > 0 {
		sb.WriteString("\n⚠️ Advertencias durante la migración:\n")
		for _, e := range s.Errors {
			sb.WriteString(fmt.Sprintf("    - %s\n", e))
		}
	}

	sb.WriteString("======================================================\n\n")
	return sb.String()
}
