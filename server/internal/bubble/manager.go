package bubble

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu         sync.RWMutex
	workspaces map[WorkspaceType]WorkspaceInfo
	bubbles    map[string]*Bubble
	devCmds    map[string]*exec.Cmd
}

func NewManager() *Manager {
	m := &Manager{
		workspaces: make(map[WorkspaceType]WorkspaceInfo),
		bubbles:    make(map[string]*Bubble),
		devCmds:    make(map[string]*exec.Cmd),
	}

	m.workspaces[WorkspaceAditum] = WorkspaceInfo{
		ID:          "aditum",
		Name:        "Aditum",
		Type:        WorkspaceAditum,
		BasePath:    `C:\Users\Jose\Documents\Work\Aditum`,
		VaultAlias:  "aditum",
		DefaultBase: "development",
		Description: "Java / Spring Boot + MySQL 8.0 local (multi-schema) + Liquibase",
	}

	m.workspaces[WorkspaceBrif] = WorkspaceInfo{
		ID:          "brif",
		Name:        "Brif",
		Type:        WorkspaceBrif,
		BasePath:    `C:\Users\Jose\Documents\Work\brif`,
		VaultAlias:  "brif",
		DefaultBase: "develop",
		Description: "Django + Scrapy + Celery + GitLab (monorepo y worktrees)",
	}

	m.workspaces[WorkspaceGeneral] = WorkspaceInfo{
		ID:          "general",
		Name:        "General",
		Type:        WorkspaceGeneral,
		BasePath:    `C:\Users\Jose\Documents\Work`,
		VaultAlias:  "aditum",
		DefaultBase: "main",
		Description: "Herramientas y proyectos generales",
	}

	m.ScanExistingBubbles()
	return m
}

func (m *Manager) ListWorkspaces() []WorkspaceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]WorkspaceInfo, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		res = append(res, ws)
	}
	return res
}

func (m *Manager) ListBubbles() []*Bubble {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*Bubble, 0, len(m.bubbles))
	for _, b := range m.bubbles {
		res = append(res, b)
	}
	return res
}

func (m *Manager) GetBubble(id string) (*Bubble, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.bubbles[id]
	return b, ok
}

func (m *Manager) ScanExistingBubbles() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Scan Aditum worktrees
	aditumWS := m.workspaces[WorkspaceAditum]
	if _, err := os.Stat(aditumWS.BasePath); err == nil {
		m.scanGitWorktrees(WorkspaceAditum, aditumWS.BasePath)
	}

	// 2. Scan Brif workspace and worktrees
	brifWS := m.workspaces[WorkspaceBrif]
	if _, err := os.Stat(brifWS.BasePath); err == nil {
		scrapperPath := filepath.Join(brifWS.BasePath, "brif-scrapper")
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

		// Register the monorepo root bubble as the primary bubble
		m.bubbles["brif-main"] = &Bubble{
			ID:         "brif-main",
			Name:       "Brif",
			Workspace:  WorkspaceBrif,
			Path:       brifWS.BasePath,
			Branch:     brifBranch,
			Port:       8000,
			Status:     StatusIdle,
			CreatedAt:  time.Now(),
			IsWorktree: false,
		}

		// Scan worktrees belonging to brif-scrapper if it exists
		if _, err := os.Stat(scrapperPath); err == nil {
			m.scanGitWorktrees(WorkspaceBrif, scrapperPath)
		}
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

	case WorkspaceGeneral:
		return nil, fmt.Errorf("unsupported workspace type for auto-creation")
	}

	return nil, fmt.Errorf("failed to create bubble")
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
		cmd := exec.Command("git", "worktree", "remove", "--force", b.Path)
		_ = cmd.Run()
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
		return 0, fmt.Errorf("no dev server command configured for this workspace")
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
