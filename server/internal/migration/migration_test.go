package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateData(t *testing.T) {
	tempRoot, err := os.MkdirTemp("", "orbit-migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	srcDir := filepath.Join(tempRoot, "src-data")
	dstDir := filepath.Join(tempRoot, "dst-data")

	// 1. Create simulated previous version data in srcDir
	if err := os.MkdirAll(filepath.Join(srcDir, "calendar_notes"), 0755); err != nil {
		t.Fatalf("failed to create src calendar_notes: %v", err)
	}

	// Daily note
	noteContent := "# Daily Note 2026-10-08\nTest note"
	_ = os.WriteFile(filepath.Join(srcDir, "calendar_notes", "2026-10-08.md"), []byte(noteContent), 0644)

	// Handoff registry
	srcHandoffs := map[string][]genericHandoff{
		"2026-10-08": {
			{
				ID:       "handoff-1",
				Date:     "2026-10-08",
				Time:     "18:00",
				BubbleID: "aditum-aditum",
				TaskName: "TAR-1108",
				Status:   "completed",
				Summary:  "Initial summary",
			},
		},
	}
	hBytes, _ := json.MarshalIndent(srcHandoffs, "", "  ")
	_ = os.WriteFile(filepath.Join(srcDir, "calendar_notes", "handoffs_registry.json"), hBytes, 0644)

	// Bubbles metadata
	metaContent := `{ "aditum-aditum": { "priority": "P0", "customRank": 1 } }`
	_ = os.WriteFile(filepath.Join(srcDir, "bubbles_metadata.json"), []byte(metaContent), 0644)

	// Server config
	cfgContent := `{ "port": "8080", "authToken": "test-token-123" }`
	_ = os.WriteFile(filepath.Join(srcDir, "server.config.json"), []byte(cfgContent), 0644)

	// Run migration
	summary, err := MigrateData(dstDir, srcDir)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}

	// Verify daily note copied
	dstNote := filepath.Join(dstDir, "calendar_notes", "2026-10-08.md")
	if data, err := os.ReadFile(dstNote); err != nil || string(data) != noteContent {
		t.Fatalf("note file not migrated correctly: %v", err)
	}

	// Verify handoffs merged
	dstHandoffFile := filepath.Join(dstDir, "calendar_notes", "handoffs_registry.json")
	var dstHandoffs map[string][]genericHandoff
	dstHBytes, err := os.ReadFile(dstHandoffFile)
	if err != nil {
		t.Fatalf("failed to read dst handoffs: %v", err)
	}
	if err := json.Unmarshal(dstHBytes, &dstHandoffs); err != nil || len(dstHandoffs["2026-10-08"]) != 1 {
		t.Fatalf("unexpected dst handoffs: %+v", dstHandoffs)
	}
	if dstHandoffs["2026-10-08"][0].ID != "handoff-1" {
		t.Fatalf("handoff id mismatch: %s", dstHandoffs["2026-10-08"][0].ID)
	}

	// Verify bubbles metadata
	dstMetaFile := filepath.Join(dstDir, "bubbles_metadata.json")
	if _, err := os.Stat(dstMetaFile); err != nil {
		t.Fatalf("bubbles_metadata.json missing in dst: %v", err)
	}

	// Verify workspaces.json was generated
	dstWsFile := filepath.Join(dstDir, "workspaces.json")
	if _, err := os.Stat(dstWsFile); err != nil {
		t.Fatalf("workspaces.json missing in dst: %v", err)
	}

	// Test idempotency: run migration again and ensure nothing is corrupted or duplicated
	summary2, err := MigrateData(dstDir, srcDir)
	if err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}
	if summary2.HandoffsCount != 1 {
		t.Fatalf("expected 1 handoff after second run, got %d", summary2.HandoffsCount)
	}

	// Test CLI formatter output
	cliOutput := FormatCLI(summary)
	if len(cliOutput) == 0 {
		t.Fatalf("expected formatted CLI output, got empty")
	}
}
