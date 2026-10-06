package git

import (
	"testing"
)

func TestGetStatusMonorepo(t *testing.T) {
	st, err := GetStatus(`C:\Users\Jose\Documents\Work\brif`)
	if err != nil {
		t.Fatalf("GetStatus error: %v", err)
	}
	t.Logf("Monorepo status branch: %s, isDirty: %v, total changes: %d", st.Branch, st.IsDirty, len(st.Changes))
	for _, c := range st.Changes {
		t.Logf("  [%s] %s (staged=%v)", c.Status, c.Path, c.Staged)
	}
}
