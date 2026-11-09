package bubble

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanExistingBubbles(t *testing.T) {
	mgr := NewManager()
	bubbles := mgr.ListBubbles()
	t.Logf("Found %d bubbles across workspaces:", len(bubbles))
	for _, b := range bubbles {
		t.Logf(" - [%s] %s | path: %s | branch: %s | port: %d | isWorktree: %v",
			b.Workspace, b.Name, b.Path, b.Branch, b.Port, b.IsWorktree)
	}
}

func TestWorkspaceCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orbit-ws-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// Test AddWorkspace
	projectDir := filepath.Join(tmpDir, "sample-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	ws := WorkspaceInfo{
		ID:          "sample",
		Name:        "Sample Project",
		Type:        "sample",
		BasePath:    projectDir,
		DefaultBase: "main",
		DevCommand:  "npm run dev",
		DefaultPort: 3000,
	}

	if err := mgr.AddWorkspace(ws); err != nil {
		t.Fatalf("AddWorkspace failed: %v", err)
	}

	fetched, ok := mgr.GetWorkspace("sample")
	if !ok {
		t.Fatalf("expected workspace sample to exist")
	}
	if fetched.Name != "Sample Project" || fetched.DefaultPort != 3000 {
		t.Fatalf("unexpected fetched workspace: %+v", fetched)
	}

	// Test UpdateWorkspace
	if err := mgr.UpdateWorkspace("sample", WorkspaceInfo{
		DevCommand:  "yarn dev",
		DefaultPort: 3005,
	}); err != nil {
		t.Fatalf("UpdateWorkspace failed: %v", err)
	}

	updated, _ := mgr.GetWorkspace("sample")
	if updated.DevCommand != "yarn dev" || updated.DefaultPort != 3005 {
		t.Fatalf("updated values mismatch: %+v", updated)
	}

	// Test DeleteWorkspace
	if err := mgr.DeleteWorkspace("sample"); err != nil {
		t.Fatalf("DeleteWorkspace failed: %v", err)
	}
	if _, ok := mgr.GetWorkspace("sample"); ok {
		t.Fatalf("expected workspace to be deleted")
	}
}

func TestGenericGitBubbleLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orbit-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a dummy git repository
	repoDir := filepath.Join(tmpDir, "my-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	initCmd := exec.Command("git", "-C", repoDir, "init", "-b", "main")
	if err := initCmd.Run(); err != nil {
		// Fallback for older git
		_ = exec.Command("git", "-C", repoDir, "init").Run()
		_ = exec.Command("git", "-C", repoDir, "checkout", "-b", "main").Run()
	}
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@orbit.local").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Orbit Tester").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "initial commit").Run()

	mgr := NewManager(tmpDir)

	ws := WorkspaceInfo{
		ID:          "myrepo",
		Name:        "My Repo",
		Type:        "myrepo",
		BasePath:    repoDir,
		DefaultBase: "main",
		DevCommand:  "echo hello",
		DefaultPort: 4000,
	}

	if err := mgr.AddWorkspace(ws); err != nil {
		t.Fatalf("AddWorkspace failed: %v", err)
	}

	// Verify root bubble was discovered
	rootBubble, ok := mgr.GetBubble("myrepo-root")
	if !ok {
		t.Fatalf("expected root bubble myrepo-root to be created")
	}
	if rootBubble.Branch != "main" {
		t.Logf("root branch is %s", rootBubble.Branch)
	}

	// Create a generic worktree bubble
	b, err := mgr.CreateBubble(CreateBubbleRequest{
		Workspace:  "myrepo",
		TaskID:     "feat-auth",
		Name:       "Feature Auth",
		BaseBranch: "main",
		IsWorktree: true,
		Port:       4001,
	})
	if err != nil {
		t.Fatalf("CreateBubble failed: %v", err)
	}
	if b == nil {
		t.Fatalf("expected created bubble, got nil")
	}
	if !b.IsWorktree {
		t.Fatalf("expected IsWorktree to be true")
	}
	if b.Port != 4001 {
		t.Fatalf("expected port 4001, got %d", b.Port)
	}

	// Verify .worktree.env
	envPath := filepath.Join(b.Path, ".worktree.env")
	if envData, err := os.ReadFile(envPath); err != nil || len(envData) == 0 {
		t.Fatalf("expected .worktree.env to be written at %s", envPath)
	}

	// Delete worktree bubble
	if err := mgr.DeleteBubble(b.ID); err != nil {
		t.Fatalf("DeleteBubble failed: %v", err)
	}

	if _, ok := mgr.GetBubble(b.ID); ok {
		t.Fatalf("expected bubble %s to be deleted", b.ID)
	}
}

func TestCreateBubbleWithPriorityAndTaskBriefing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orbit-priority-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo-test")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	initCmd := exec.Command("git", "-C", repoDir, "init", "-b", "main")
	if err := initCmd.Run(); err != nil {
		_ = exec.Command("git", "-C", repoDir, "init").Run()
		_ = exec.Command("git", "-C", repoDir, "checkout", "-b", "main").Run()
	}
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@orbit.local").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Orbit Tester").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "initial commit").Run()

	mgr := NewManager(tmpDir)
	ws := WorkspaceInfo{
		ID:          "repotest",
		Name:        "Repo Test",
		Type:        "repotest",
		BasePath:    repoDir,
		DefaultBase: "main",
	}
	if err := mgr.AddWorkspace(ws); err != nil {
		t.Fatalf("AddWorkspace failed: %v", err)
	}

	b, err := mgr.CreateBubble(CreateBubbleRequest{
		Workspace:       "repotest",
		TaskID:          "tar-999",
		Name:            "TAR-999 Fix Billing",
		Priority:        "P0",
		TaskDescription: "Fix billing calculation and negative amount handling.",
		Requirements:    []string{"Validate non-negative balance", "Unit test edge cases"},
		IsWorktree:      true,
	})
	if err != nil {
		t.Fatalf("CreateBubble failed: %v", err)
	}
	if b == nil {
		t.Fatalf("expected bubble, got nil")
	}

	if b.Priority != "P0" {
		t.Errorf("expected priority P0, got '%s'", b.Priority)
	}

	// Verify TASK.md was written
	taskPath := filepath.Join(b.Path, "TASK.md")
	taskData, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("failed to read TASK.md: %v", err)
	}
	taskStr := string(taskData)
	if !strings.Contains(taskStr, "Prioridad:** P0") {
		t.Errorf("TASK.md missing priority P0")
	}
	if !strings.Contains(taskStr, "Fix billing calculation") {
		t.Errorf("TASK.md missing task description")
	}
	if !strings.Contains(taskStr, "- [ ] Validate non-negative balance") {
		t.Errorf("TASK.md missing requirement checkbox")
	}

	// Verify TASK.md is excluded from Git (.git/info/exclude)
	mainExclude := filepath.Join(repoDir, ".git", "info", "exclude")
	exData, err := os.ReadFile(mainExclude)
	if err != nil {
		t.Fatalf("failed to read main repo exclude: %v", err)
	}
	if !strings.Contains(string(exData), "TASK.md") {
		t.Errorf("expected TASK.md to be present in %s", mainExclude)
	}

	// Clean up
	_ = mgr.DeleteBubble(b.ID)
}

func TestUpdateBubbleMetadataTaskStatus(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	testBubble := &Bubble{
		ID:        "test-status-bubble",
		Name:      "Test Status Bubble",
		Workspace: WorkspaceGeneral,
		Path:      tmpDir,
		Status:    StatusIdle,
	}
	mgr.bubbles[testBubble.ID] = testBubble

	prio := "P1"
	rank := 3
	status := "in_progress"
	if err := mgr.UpdateBubbleMetadata(testBubble.ID, &prio, &rank, &status); err != nil {
		t.Fatalf("UpdateBubbleMetadata failed: %v", err)
	}

	b, ok := mgr.GetBubble(testBubble.ID)
	if !ok {
		t.Fatalf("bubble not found")
	}
	if b.Priority != "P1" {
		t.Errorf("expected P1, got %s", b.Priority)
	}
	if b.CustomRank != 3 {
		t.Errorf("expected rank 3, got %d", b.CustomRank)
	}
	if b.TaskStatus != "in_progress" {
		t.Errorf("expected in_progress, got %s", b.TaskStatus)
	}

	metaData, err := os.ReadFile(filepath.Join(tmpDir, "bubbles_metadata.json"))
	if err != nil {
		t.Fatalf("failed to read bubbles_metadata.json: %v", err)
	}
	if !strings.Contains(string(metaData), `"taskStatus": "in_progress"`) {
		t.Errorf("expected taskStatus in metadata json, got %s", string(metaData))
	}
}

func TestMultiRepoWorktreeDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup multi-repo workspace structure (like Brif):
	// wsParent is NOT a git repo, but contains sub-repos (brif-scrapper)
	wsParent := filepath.Join(tmpDir, "brif-ws")
	if err := os.MkdirAll(wsParent, 0755); err != nil {
		t.Fatalf("failed to create wsParent: %v", err)
	}

	scrapperRepo := filepath.Join(wsParent, "brif-scrapper")
	if err := os.MkdirAll(scrapperRepo, 0755); err != nil {
		t.Fatalf("failed to create scrapperRepo: %v", err)
	}

	_ = exec.Command("git", "-C", scrapperRepo, "init", "-b", "develop").Run()
	_ = exec.Command("git", "-C", scrapperRepo, "config", "user.email", "tester@orbit.local").Run()
	_ = exec.Command("git", "-C", scrapperRepo, "config", "user.name", "Orbit Tester").Run()
	_ = exec.Command("git", "-C", scrapperRepo, "commit", "--allow-empty", "-m", "initial commit").Run()

	mgr := NewManager(tmpDir)
	ws := WorkspaceInfo{
		ID:          "brif",
		Name:        "Brif",
		Type:        WorkspaceBrif,
		BasePath:    wsParent,
		DefaultBase: "develop",
		DefaultPort: 8000,
	}
	if err := mgr.AddWorkspace(ws); err != nil {
		t.Fatalf("AddWorkspace failed: %v", err)
	}

	// 2. Create a worktree outside the repo (e.g. C:\Users\...\brif-scrapper-feat-x)
	wtDir := filepath.Join(tmpDir, "brif-scrapper-feat-x")
	cmd := exec.Command("git", "-C", scrapperRepo, "worktree", "add", "-b", "feat/feat-x", wtDir, "develop")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create git worktree: %v\nOutput: %s", err, string(out))
	}

	// 3. Scan worktrees
	mgr.scanGitWorktrees(WorkspaceBrif, scrapperRepo)

	bubbleID := fmt.Sprintf("%s-%s", WorkspaceBrif, strings.ToLower(filepath.Base(wtDir)))
	b, ok := mgr.GetBubble(bubbleID)
	if !ok {
		t.Fatalf("expected bubble %s to be registered", bubbleID)
	}
	if !b.IsWorktree {
		t.Fatalf("expected bubble to be a worktree")
	}

	// Verify resolveGitRepo resolves scrapperRepo instead of wsParent
	resolvedRepo := mgr.resolveGitRepo(b)
	if !strings.EqualFold(filepath.Clean(resolvedRepo), filepath.Clean(scrapperRepo)) {
		t.Fatalf("expected resolved repo %s, got %s", scrapperRepo, resolvedRepo)
	}

	// 4. Delete bubble
	if err := mgr.DeleteBubble(bubbleID); err != nil {
		t.Fatalf("DeleteBubble failed: %v", err)
	}

	// Bubble must be removed from memory
	if _, ok := mgr.GetBubble(bubbleID); ok {
		t.Fatalf("expected bubble %s to be deleted from manager", bubbleID)
	}

	// Worktree directory on disk must be removed
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Fatalf("expected worktree directory %s to be removed from disk", wtDir)
	}

	// Git worktree must no longer be registered in scrapperRepo
	wtListCmd := exec.Command("git", "-C", scrapperRepo, "worktree", "list", "--porcelain")
	out, err := wtListCmd.Output()
	if err != nil {
		t.Fatalf("failed to run git worktree list: %v", err)
	}
	if strings.Contains(string(out), filepath.ToSlash(wtDir)) {
		t.Fatalf("expected worktree %s to be removed from git worktree list, but found in:\n%s", wtDir, string(out))
	}
}

func TestScanGitWorktreesIgnoresPrunableAndNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "init", "-b", "main").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "tester@orbit.local").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Orbit Tester").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "init").Run()

	wtDir := filepath.Join(tmpDir, "repo-wt")
	if out, err := exec.Command("git", "-C", repoDir, "worktree", "add", "-b", "wt-branch", wtDir, "main").CombinedOutput(); err != nil {
		t.Fatalf("failed to add worktree: %v (%s)", err, string(out))
	}

	// Delete wtDir directly from filesystem without git worktree remove
	if err := os.RemoveAll(wtDir); err != nil {
		t.Fatalf("failed to remove wtDir: %v", err)
	}

	mgr := NewManager(tmpDir)
	// scanGitWorktrees should detect that wtDir does not exist / is prunable and NOT register it
	mgr.scanGitWorktrees("repo", repoDir)

	bubbleID := fmt.Sprintf("repo-%s", strings.ToLower(filepath.Base(wtDir)))
	if _, ok := mgr.GetBubble(bubbleID); ok {
		t.Fatalf("expected non-existent / prunable worktree bubble %s NOT to be registered", bubbleID)
	}
}

