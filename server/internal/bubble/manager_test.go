package bubble

import (
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
