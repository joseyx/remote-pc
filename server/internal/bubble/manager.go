package bubble

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type BubbleMeta struct {
	Priority   string `json:"priority,omitempty"`
	CustomRank int    `json:"customRank,omitempty"`
}

type Manager struct {
	mu             sync.RWMutex
	dataDir        string
	workspacesPath string
	workspaces     map[WorkspaceType]WorkspaceInfo
	bubbles        map[string]*Bubble
	devCmds        map[string]*exec.Cmd
	metaPath       string
	metadata       map[string]BubbleMeta
}

func NewManager(dataDir ...string) *Manager {
	dir := "data"
	if len(dataDir) > 0 && dataDir[0] != "" {
		dir = dataDir[0]
	}

	metaPath := filepath.Join(dir, "bubbles_metadata.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		for _, fb := range []string{
			"bubbles_metadata.json",
			filepath.Join("data", "bubbles_metadata.json"),
			filepath.Join("..", "data", "bubbles_metadata.json"),
		} {
			if _, errFb := os.Stat(fb); errFb == nil {
				metaPath = fb
				break
			}
		}
	}

	wsPath := filepath.Join(dir, "workspaces.json")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		for _, fb := range []string{
			"workspaces.json",
			filepath.Join("data", "workspaces.json"),
			filepath.Join("..", "data", "workspaces.json"),
		} {
			if _, errFb := os.Stat(fb); errFb == nil {
				wsPath = fb
				break
			}
		}
	}

	m := &Manager{
		dataDir:        dir,
		workspacesPath: wsPath,
		workspaces:     make(map[WorkspaceType]WorkspaceInfo),
		bubbles:        make(map[string]*Bubble),
		devCmds:        make(map[string]*exec.Cmd),
		metaPath:       metaPath,
		metadata:       make(map[string]BubbleMeta),
	}
	m.loadMetadata()
	m.loadWorkspaces()
	m.ScanExistingBubbles()
	return m
}

func (m *Manager) loadWorkspaces() {
	data, err := os.ReadFile(m.workspacesPath)
	if err == nil {
		var list []WorkspaceInfo
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			for _, ws := range list {
				m.workspaces[ws.Type] = ws
			}
			return
		}
	}

	// Seed defaults if no workspaces.json exists
	m.seedDefaultWorkspaces()
	_ = m.saveWorkspacesLocked()
}

func (m *Manager) seedDefaultWorkspaces() {
	// If Jose's personal Work directories exist, configure Aditum and Brif
	if fi, err := os.Stat(`C:\Users\Jose\Documents\Work\Aditum`); err == nil && fi.IsDir() {
		m.workspaces[WorkspaceAditum] = WorkspaceInfo{
			ID:          "aditum",
			Name:        "Aditum",
			Type:        WorkspaceAditum,
			BasePath:    `C:\Users\Jose\Documents\Work\Aditum`,
			VaultAlias:  "aditum",
			DefaultBase: "development",
			Description: "Java / Spring Boot + MySQL 8.0 local (multi-schema) + Liquibase",
			DefaultPort: 8080,
		}
	}

	if fi, err := os.Stat(`C:\Users\Jose\Documents\Work\brif`); err == nil && fi.IsDir() {
		m.workspaces[WorkspaceBrif] = WorkspaceInfo{
			ID:          "brif",
			Name:        "Brif",
			Type:        WorkspaceBrif,
			BasePath:    `C:\Users\Jose\Documents\Work\brif`,
			VaultAlias:  "brif",
			DefaultBase: "develop",
			Description: "Django + Scrapy + Celery + GitLab (monorepo y worktrees)",
			DefaultPort: 8000,
		}
	}

	if fi, err := os.Stat(`C:\Users\Jose\Documents\Work`); err == nil && fi.IsDir() {
		m.workspaces[WorkspaceGeneral] = WorkspaceInfo{
			ID:          "general",
			Name:        "General",
			Type:        WorkspaceGeneral,
			BasePath:    `C:\Users\Jose\Documents\Work`,
			VaultAlias:  "aditum",
			DefaultBase: "main",
			Description: "Herramientas y proyectos generales",
		}
	}
}

func (m *Manager) saveWorkspacesLocked() error {
	if m.workspacesPath == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(m.workspacesPath), 0755)
	list := make([]WorkspaceInfo, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		list = append(list, ws)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.workspacesPath, data, 0644)
}

func (m *Manager) AddWorkspace(ws WorkspaceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ws.ID == "" {
		ws.ID = strings.ToLower(strings.ReplaceAll(ws.Name, " ", "-"))
	}
	if ws.Type == "" {
		ws.Type = WorkspaceType(ws.ID)
	}
	if ws.BasePath == "" {
		return fmt.Errorf("se requiere la ruta base (basePath) del workspace")
	}

	ws.BasePath = filepath.Clean(ws.BasePath)
	if _, err := os.Stat(ws.BasePath); err != nil {
		return fmt.Errorf("la ruta no existe: %s", ws.BasePath)
	}

	m.workspaces[ws.Type] = ws
	if err := m.saveWorkspacesLocked(); err != nil {
		return err
	}

	m.scanWorkspaceLocked(ws)
	return nil
}

func (m *Manager) UpdateWorkspace(id string, ws WorkspaceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wsKey := WorkspaceType(id)
	existing, ok := m.workspaces[wsKey]
	if !ok {
		for k, v := range m.workspaces {
			if v.ID == id {
				wsKey = k
				existing = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("workspace no encontrado: %s", id)
	}

	if ws.Name != "" {
		existing.Name = ws.Name
	}
	if ws.BasePath != "" {
		existing.BasePath = filepath.Clean(ws.BasePath)
	}
	if ws.DefaultBase != "" {
		existing.DefaultBase = ws.DefaultBase
	}
	if ws.DevCommand != "" {
		existing.DevCommand = ws.DevCommand
	}
	if ws.DefaultPort > 0 {
		existing.DefaultPort = ws.DefaultPort
	}
	if ws.Description != "" {
		existing.Description = ws.Description
	}
	if ws.VaultAlias != "" {
		existing.VaultAlias = ws.VaultAlias
	}

	m.workspaces[wsKey] = existing
	if err := m.saveWorkspacesLocked(); err != nil {
		return err
	}

	m.scanWorkspaceLocked(existing)
	return nil
}

func (m *Manager) DeleteWorkspace(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wsKey := WorkspaceType(id)
	if _, ok := m.workspaces[wsKey]; !ok {
		for k, v := range m.workspaces {
			if v.ID == id {
				wsKey = k
				break
			}
		}
	}

	delete(m.workspaces, wsKey)

	// Remove bubbles belonging to this workspace
	for bid, b := range m.bubbles {
		if b.Workspace == wsKey {
			delete(m.bubbles, bid)
		}
	}

	return m.saveWorkspacesLocked()
}

func (m *Manager) GetWorkspace(id string) (WorkspaceInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wsKey := WorkspaceType(id)
	if ws, ok := m.workspaces[wsKey]; ok {
		return ws, true
	}
	for _, ws := range m.workspaces {
		if ws.ID == id {
			return ws, true
		}
	}
	return WorkspaceInfo{}, false
}

func (m *Manager) ListWorkspaces() []WorkspaceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]WorkspaceInfo, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		res = append(res, ws)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	return res
}

func (m *Manager) ListBubbles() []*Bubble {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*Bubble, 0, len(m.bubbles))
	for _, b := range m.bubbles {
		res = append(res, b)
	}
	sort.Slice(res, func(i, j int) bool {
		nameI := strings.ToLower(res[i].Name)
		nameJ := strings.ToLower(res[j].Name)
		if nameI == nameJ {
			return res[i].ID < res[j].ID
		}
		return nameI < nameJ
	})
	return res
}

func (m *Manager) GetBubble(id string) (*Bubble, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.bubbles[id]
	return b, ok
}

func (m *Manager) loadMetadata() {
	if m.metaPath == "" {
		return
	}
	data, err := os.ReadFile(m.metaPath)
	if err != nil {
		for _, fb := range []string{
			filepath.Join("data", "bubbles_metadata.json"),
			filepath.Join("..", "data", "bubbles_metadata.json"),
			"bubbles_metadata.json",
		} {
			if fb != m.metaPath {
				if d, errFb := os.ReadFile(fb); errFb == nil {
					data = d
					err = nil
					break
				}
			}
		}
	}
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &m.metadata)
}

func (m *Manager) saveMetadataLocked() error {
	if m.metaPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.metaPath, data, 0644)
}

func (m *Manager) applyMetadata(b *Bubble) {
	if meta, ok := m.metadata[b.ID]; ok {
		b.Priority = meta.Priority
		b.CustomRank = meta.CustomRank
	}
}

func (m *Manager) UpdateBubbleMetadata(id string, priority *string, customRank *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta := m.metadata[id]
	if priority != nil {
		meta.Priority = *priority
	}
	if customRank != nil {
		meta.CustomRank = *customRank
	}
	m.metadata[id] = meta

	if b, ok := m.bubbles[id]; ok {
		b.Priority = meta.Priority
		b.CustomRank = meta.CustomRank
	}

	return m.saveMetadataLocked()
}

func (m *Manager) ReorderBubbles(orderedIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for rank, id := range orderedIDs {
		meta := m.metadata[id]
		meta.CustomRank = rank + 1
		m.metadata[id] = meta
		if b, ok := m.bubbles[id]; ok {
			b.CustomRank = rank + 1
		}
	}

	return m.saveMetadataLocked()
}

func (m *Manager) ScanExistingBubbles() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ws := range m.workspaces {
		m.scanWorkspaceLocked(ws)
	}
}

func (m *Manager) scanWorkspaceLocked(ws WorkspaceInfo) {
	if _, err := os.Stat(ws.BasePath); err != nil {
		return
	}

	// 1. Scan Aditum worktrees
	if ws.Type == WorkspaceAditum {
		m.scanGitWorktrees(WorkspaceAditum, ws.BasePath)
		return
	}

	// 2. Scan Brif workspace and worktrees
	if ws.Type == WorkspaceBrif {
		scrapperPath := filepath.Join(ws.BasePath, "brif-scrapper")
		var brifBranch string
		if _, err := os.Stat(scrapperPath); err == nil {
			cmd := exec.Command("git", "-C", scrapperPath, "rev-parse", "--abbrev-ref", "HEAD")
			if out, err := cmd.Output(); err == nil {
				brifBranch = strings.TrimSpace(string(out))
			}
		}
		if brifBranch == "" {
			brifBranch = "develop"
		}

		brifBubble := &Bubble{
			ID:         "brif-main",
			Name:       "Brif",
			Workspace:  WorkspaceBrif,
			Path:       ws.BasePath,
			Branch:     brifBranch,
			Port:       8000,
			Status:     StatusIdle,
			CreatedAt:  time.Now(),
			IsWorktree: false,
		}
		m.applyMetadata(brifBubble)
		m.bubbles["brif-main"] = brifBubble

		if _, err := os.Stat(scrapperPath); err == nil {
			m.scanGitWorktrees(WorkspaceBrif, scrapperPath)
		}
		return
	}

	// 3. Generic Git workspaces or custom workspaces
	cmd := exec.Command("git", "-C", ws.BasePath, "rev-parse", "--is-inside-work-tree")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
		rootBranch := ws.DefaultBase
		if rootBranch == "" {
			rootBranch = "main"
		}
		bCmd := exec.Command("git", "-C", ws.BasePath, "rev-parse", "--abbrev-ref", "HEAD")
		if bOut, err := bCmd.Output(); err == nil {
			bStr := strings.TrimSpace(string(bOut))
			if bStr != "" {
				rootBranch = bStr
			}
		}

		rootID := fmt.Sprintf("%s-root", ws.ID)
		port := ws.DefaultPort
		rootBubble := &Bubble{
			ID:         rootID,
			Name:       ws.Name,
			Workspace:  ws.Type,
			Path:       ws.BasePath,
			Branch:     rootBranch,
			Port:       port,
			Status:     StatusIdle,
			CreatedAt:  time.Now(),
			IsWorktree: false,
		}
		m.applyMetadata(rootBubble)
		m.bubbles[rootID] = rootBubble

		m.scanGitWorktrees(ws.Type, ws.BasePath)
	}
}

func (m *Manager) scanGitWorktrees(wsType WorkspaceType, repoPath string) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to list worktrees for %s: %v", repoPath, err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	var currentPath, currentHead, currentBranch string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			currentHead = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			currentBranch = strings.TrimPrefix(line, "branch ")
			currentBranch = strings.TrimPrefix(currentBranch, "refs/heads/")
		} else if line == "" && currentPath != "" {
			m.registerDiscoveredWorktree(wsType, repoPath, currentPath, currentHead, currentBranch)
			currentPath = ""
			currentHead = ""
			currentBranch = ""
		}
	}
	if currentPath != "" {
		m.registerDiscoveredWorktree(wsType, repoPath, currentPath, currentHead, currentBranch)
	}
}

func (m *Manager) registerDiscoveredWorktree(wsType WorkspaceType, repoPath, wtPath, head, branch string) {
	normPath := filepath.Clean(wtPath)
	normRepo := filepath.Clean(repoPath)
	isRoot := strings.EqualFold(normPath, normRepo)

	if wsType == WorkspaceBrif && isRoot {
		if b, ok := m.bubbles["brif-main"]; ok {
			b.Branch = branch
		}
		return
	}

	if wsType != WorkspaceAditum && isRoot {
		for _, b := range m.bubbles {
			if b.Workspace == wsType && !b.IsWorktree {
				b.Branch = branch
				return
			}
		}
	}

	dirName := filepath.Base(normPath)
	id := fmt.Sprintf("%s-%s", wsType, strings.ToLower(dirName))
	name := dirName

	port := 0
	schema := ""
	meta := make(map[string]string)

	// Read .worktree.env if present
	envFile := filepath.Join(normPath, ".worktree.env")
	if data, err := os.ReadFile(envFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SERVER_PORT=") {
				p, _ := strconv.Atoi(strings.TrimPrefix(line, "SERVER_PORT="))
				port = p
			} else if strings.HasPrefix(line, "PORT=") {
				p, _ := strconv.Atoi(strings.TrimPrefix(line, "PORT="))
				port = p
			} else if strings.HasPrefix(line, "DB_SCHEMA=") {
				schema = strings.TrimPrefix(line, "DB_SCHEMA=")
			} else if strings.HasPrefix(line, "SEED_SCHEMA=") {
				meta["seedSchema"] = strings.TrimPrefix(line, "SEED_SCHEMA=")
			}
		}
	}

	// Aditum main default
	if wsType == WorkspaceAditum && isRoot && port == 0 {
		port = 8080
		schema = "aditum"
	}

	// Brif main default
	if wsType == WorkspaceBrif && isRoot && port == 0 {
		port = 8000
	}

	// Generic workspace default port
	if port == 0 {
		if ws, ok := m.workspaces[wsType]; ok && ws.DefaultPort > 0 {
			port = ws.DefaultPort
		}
	}

	if wsType == WorkspaceAditum {
		if strings.Contains(dirName, "TAR-") {
			idx := strings.Index(dirName, "TAR-")
			name = dirName[idx:]
		}
	}

	b := &Bubble{
		ID:         id,
		Name:       name,
		Workspace:  wsType,
		Path:       normPath,
		Branch:     branch,
		Port:       port,
		DbSchema:   schema,
		Status:     StatusIdle,
		CreatedAt:  time.Now(),
		IsWorktree: !isRoot,
		Metadata:   meta,
	}

	m.applyMetadata(b)
	m.bubbles[id] = b
}

func (m *Manager) CreateBubble(req CreateBubbleRequest) (*Bubble, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bashPath := `C:\Program Files\Git\bin\bash.exe`
	if _, err := os.Stat(bashPath); err != nil {
		bashPath = "bash.exe"
	}

	switch req.Workspace {
	case WorkspaceAditum:
		startScript := `/c/Users/Jose/bin/start-worktree.sh`
		args := []string{startScript, req.TaskID}

		if req.Index > 0 {
			args = append(args, strconv.Itoa(req.Index))
		}
		if req.BaseBranch != "" {
			args = append(args, req.BaseBranch)
		}
		if !req.UseSlim {
			args = append(args, "--full")
		}
		if req.SeedSchema != "" {
			args = append(args, fmt.Sprintf("--seed=%s", req.SeedSchema))
		}

		cmd := exec.Command(bashPath, args...)
		cmd.Dir = `C:\Users\Jose\Documents\Work\Aditum`
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("start-worktree.sh error: %w\nOutput:\n%s", err, string(out))
		}

		// Rescan to discover the newly created worktree
		m.scanGitWorktrees(WorkspaceAditum, `C:\Users\Jose\Documents\Work\Aditum`)

		// Find created bubble matching taskID
		var created *Bubble
		for _, b := range m.bubbles {
			if strings.Contains(strings.ToLower(b.Path), strings.ToLower(req.TaskID)) {
				created = b
				break
			}
		}
		if created != nil {
			return created, nil
		}
		return nil, fmt.Errorf("worktree script completed but bubble could not be matched. Log:\n%s", string(out))

	case WorkspaceBrif:
		baseWS := m.workspaces[WorkspaceBrif]
		scrapperPath := filepath.Join(baseWS.BasePath, "brif-scrapper")
		baseBranch := req.BaseBranch
		if baseBranch == "" {
			baseBranch = "develop"
		}

		if req.IsWorktree {
			// Worktree mode for Brif
			targetDir := filepath.Join(`C:\Users\Jose\Documents\Work`, fmt.Sprintf("brif-scrapper-%s", req.TaskID))
			branchName := req.TaskID
			if !strings.HasPrefix(branchName, "feat/") && !strings.HasPrefix(branchName, "fix/") {
				branchName = "feat/" + branchName
			}

			cmd := exec.Command("git", "-C", scrapperPath, "worktree", "add", "-b", branchName, targetDir, fmt.Sprintf("origin/%s", baseBranch))
			out, err := cmd.CombinedOutput()
			if err != nil {
				// Retry checking out existing branch if origin/develop already has it
				cmd = exec.Command("git", "-C", scrapperPath, "worktree", "add", targetDir, branchName)
				out2, err2 := cmd.CombinedOutput()
				if err2 != nil {
					return nil, fmt.Errorf("git worktree add failed: %w (%s / %s)", err, string(out), string(out2))
				}
			}

			m.scanGitWorktrees(WorkspaceBrif, scrapperPath)
			for _, b := range m.bubbles {
				if filepath.Clean(b.Path) == filepath.Clean(targetDir) {
					return b, nil
				}
			}
		} else {
			// In-place linear mode for Brif (just checkout or create branch directly in brif-scrapper)
			branchName := req.TaskID
			if !strings.HasPrefix(branchName, "feat/") && !strings.HasPrefix(branchName, "fix/") {
				branchName = "feat/" + branchName
			}

			// Try checkout -b
			cmd := exec.Command("git", "-C", scrapperPath, "checkout", "-b", branchName, fmt.Sprintf("origin/%s", baseBranch))
			if err := cmd.Run(); err != nil {
				// Try plain checkout if branch already exists
				_ = exec.Command("git", "-C", scrapperPath, "checkout", branchName).Run()
			}

			if b, ok := m.bubbles["brif-main"]; ok {
				b.Branch = branchName
				return b, nil
			}
		}

	default:
		// Generic Git workspace handler
		ws, ok := m.workspaces[req.Workspace]
		if !ok {
			for _, v := range m.workspaces {
				if v.ID == string(req.Workspace) {
					ws = v
					ok = true
					break
				}
			}
		}
		if !ok {
			return nil, fmt.Errorf("workspace no encontrado: %s", req.Workspace)
		}

		baseBranch := req.BaseBranch
		if baseBranch == "" {
			baseBranch = ws.DefaultBase
		}
		if baseBranch == "" {
			baseBranch = "main"
		}

		branchName := req.TaskID
		if branchName == "" {
			return nil, fmt.Errorf("se requiere un identificador de tarea o rama (taskId)")
		}

		if req.IsWorktree {
			// Ensure .worktrees directory exists inside ws.BasePath
			worktreesBase := filepath.Join(ws.BasePath, ".worktrees")
			_ = os.MkdirAll(worktreesBase, 0755)

			// Add .worktrees/ to git exclude if not present
			excludePath := filepath.Join(ws.BasePath, ".git", "info", "exclude")
			if exData, err := os.ReadFile(excludePath); err == nil {
				if !strings.Contains(string(exData), ".worktrees") {
					_ = os.WriteFile(excludePath, []byte(string(exData)+"\n.worktrees/\n"), 0644)
				}
			}

			targetDir := filepath.Join(worktreesBase, req.TaskID)

			// Try git worktree add -b <branchName> <targetDir> <baseBranch>
			cmd := exec.Command("git", "-C", ws.BasePath, "worktree", "add", "-b", branchName, targetDir, baseBranch)
			out, err := cmd.CombinedOutput()
			if err != nil {
				// Retry with origin/<baseBranch> or existing branch
				cmd = exec.Command("git", "-C", ws.BasePath, "worktree", "add", "-b", branchName, targetDir, fmt.Sprintf("origin/%s", baseBranch))
				if out2, err2 := cmd.CombinedOutput(); err2 != nil {
					// Fallback to checking out existing local branch
					cmd = exec.Command("git", "-C", ws.BasePath, "worktree", "add", targetDir, branchName)
					if out3, err3 := cmd.CombinedOutput(); err3 != nil {
						return nil, fmt.Errorf("error al crear worktree git: %v (%s | %s | %s)", err, string(out), string(out2), string(out3))
					}
				}
			}

			// If custom port is requested or workspace has default port, write .worktree.env
			port := req.Port
			if port <= 0 {
				port = ws.DefaultPort
			}
			if port > 0 {
				envContent := fmt.Sprintf("PORT=%d\nSERVER_PORT=%d\n", port, port)
				_ = os.WriteFile(filepath.Join(targetDir, ".worktree.env"), []byte(envContent), 0644)
			}

			m.scanGitWorktrees(ws.Type, ws.BasePath)

			// Look up created bubble
			for _, b := range m.bubbles {
				if filepath.Clean(b.Path) == filepath.Clean(targetDir) {
					if req.Name != "" {
						b.Name = req.Name
					}
					if port > 0 {
						b.Port = port
					}
					return b, nil
				}
			}

			return nil, fmt.Errorf("worktree creado pero no se pudo registrar la bubble en %s", targetDir)
		} else {
			// In-place linear branch checkout
			cmd := exec.Command("git", "-C", ws.BasePath, "checkout", "-b", branchName, baseBranch)
			if err := cmd.Run(); err != nil {
				_ = exec.Command("git", "-C", ws.BasePath, "checkout", branchName).Run()
			}

			rootID := fmt.Sprintf("%s-root", ws.ID)
			if b, ok := m.bubbles[rootID]; ok {
				b.Branch = branchName
				return b, nil
			}
			// If not found, rescan and return root bubble
			m.scanWorkspaceLocked(ws)
			if b, ok := m.bubbles[rootID]; ok {
				return b, nil
			}
			return nil, fmt.Errorf("rama cambiada pero no se pudo actualizar la bubble root")
		}
	}

	return nil, fmt.Errorf("error inesperado al crear bubble")
}

func (m *Manager) DeleteBubble(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.bubbles[id]
	if !ok {
		return fmt.Errorf("bubble %s not found", id)
	}

	// 1. Stop dev server if running
	if cmd, ok := m.devCmds[id]; ok && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		delete(m.devCmds, id)
	}

	bashPath := `C:\Program Files\Git\bin\bash.exe`
	if _, err := os.Stat(bashPath); err != nil {
		bashPath = "bash.exe"
	}

	if b.Workspace == WorkspaceAditum && b.IsWorktree {
		// Use remove-worktree.sh
		removeScript := `/c/Users/Jose/bin/remove-worktree.sh`
		cmd := exec.Command(bashPath, removeScript, b.Branch, "--force")
		cmd.Dir = `C:\Users\Jose\Documents\Work\Aditum`
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("remove-worktree.sh failed: %w (%s)", err, string(out))
		}
	} else if b.IsWorktree {
		// Generic git worktree remove
		basePath := ""
		if ws, ok := m.workspaces[b.Workspace]; ok {
			basePath = ws.BasePath
		}
		if basePath != "" {
			cmd := exec.Command("git", "-C", basePath, "worktree", "remove", "--force", b.Path)
			_ = cmd.Run()
			_ = exec.Command("git", "-C", basePath, "worktree", "prune").Run()
		} else {
			cmd := exec.Command("git", "worktree", "remove", "--force", b.Path)
			_ = cmd.Run()
		}
		if _, err := os.Stat(b.Path); err == nil {
			_ = os.RemoveAll(b.Path)
		}
	}

	delete(m.bubbles, id)
	return nil
}

func (m *Manager) StartDevServer(id string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.bubbles[id]
	if !ok {
		return 0, fmt.Errorf("bubble %s not found", id)
	}

	if b.DevRunning {
		return b.DevPid, nil
	}

	var cmd *exec.Cmd

	switch b.Workspace {
	case WorkspaceAditum:
		startBackend := filepath.Join(b.Path, "scripts", "start-backend.sh")
		if _, err := os.Stat(startBackend); err == nil {
			bashPath := `C:\Program Files\Git\bin\bash.exe`
			cmd = exec.Command(bashPath, "./scripts/start-backend.sh")
			cmd.Dir = b.Path
		} else {
			// Fallback maven spring-boot:run
			cmd = exec.Command("cmd", "/c", ".\\mvnw.cmd", "spring-boot:run", "-Dmaven.test.skip=true")
			cmd.Dir = b.Path
		}

	case WorkspaceBrif:
		targetDir := b.Path
		if _, err := os.Stat(filepath.Join(b.Path, "brif-scrapper", "manage.py")); err == nil {
			targetDir = filepath.Join(b.Path, "brif-scrapper")
		}

		pyPath := filepath.Join(targetDir, "venv", "Scripts", "python.exe")
		if _, err := os.Stat(pyPath); err == nil {
			cmd = exec.Command(pyPath, "manage.py", "runserver", "0.0.0.0:8000")
			cmd.Dir = targetDir
		} else {
			cmd = exec.Command("python", "manage.py", "runserver", "0.0.0.0:8000")
			cmd.Dir = targetDir
		}

	default:
		ws, ok := m.workspaces[b.Workspace]
		if ok && strings.TrimSpace(ws.DevCommand) != "" {
			devCmd := strings.TrimSpace(ws.DevCommand)
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd", "/c", devCmd)
			} else {
				cmd = exec.Command("sh", "-c", devCmd)
			}
			cmd.Dir = b.Path
			cmd.Env = os.Environ()
			if b.Port > 0 {
				cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", b.Port), fmt.Sprintf("SERVER_PORT=%d", b.Port))
			}
		} else {
			return 0, fmt.Errorf("no hay comando de desarrollo configurado para este workspace")
		}
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start dev server: %w", err)
	}

	b.DevRunning = true
	b.DevPid = cmd.Process.Pid
	m.devCmds[id] = cmd

	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		b.DevRunning = false
		b.DevPid = 0
		delete(m.devCmds, id)
		m.mu.Unlock()
	}()

	return b.DevPid, nil
}

func (m *Manager) StopDevServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.bubbles[id]
	if !ok {
		return fmt.Errorf("bubble %s not found", id)
	}

	cmd, ok := m.devCmds[id]
	if !ok || cmd == nil || cmd.Process == nil {
		b.DevRunning = false
		b.DevPid = 0
		return nil
	}

	// Taskkill tree on Windows
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	_ = killCmd.Run()
	_ = cmd.Process.Kill()

	b.DevRunning = false
	b.DevPid = 0
	delete(m.devCmds, id)
	return nil
}
