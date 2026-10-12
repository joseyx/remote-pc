package bubble

import (
	"os"
	"os/exec"
	"path/filepath"
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

