package obsidian

import (
	"testing"
)

func TestVaultDiscovery(t *testing.T) {
	mgr := NewManager()
	vaults := mgr.ListVaults()
	t.Logf("Discovered %d vaults:", len(vaults))
	for _, v := range vaults {
		t.Logf(" - Vault [%s] name: %s, path: %s", v.ID, v.Name, v.Path)
	}

	// Test note search in Aditum vault
	aditumNotes, err := mgr.SearchNotes("aditum", "worktree")
	if err != nil {
		t.Logf("Aditum search error: %v", err)
	} else {
		t.Logf("Found %d notes in Aditum matching 'worktree':", len(aditumNotes))
		for _, n := range aditumNotes {
			t.Logf("   * %s (%s)", n.Title, n.Relative)
		}
	}

	// Test note search in Brif vault
	brifNotes, err := mgr.SearchNotes("brif", "scrapper")
	if err != nil {
		t.Logf("Brif search error: %v", err)
	} else {
		t.Logf("Found %d notes in Brif matching 'scrapper':", len(brifNotes))
		for _, n := range brifNotes {
			t.Logf("   * %s (%s)", n.Title, n.Relative)
		}
	}
}
