package doctor

import (
	"strings"
	"testing"

	"orbit/internal/config"
)

func TestRunDiagnostics(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		DataDir: t.TempDir(),
	}

	report := RunDiagnostics(cfg, "1.0.0")
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if len(report.Items) < 8 {
		t.Errorf("expected at least 8 diagnostic items, got %d", len(report.Items))
	}

	total := report.Passed + report.Warnings + report.Errors
	if total != len(report.Items) {
		t.Errorf("sum of statuses (%d) != total items (%d)", total, len(report.Items))
	}

	cliText := FormatCLI(report)
	if !strings.Contains(cliText, "Orbit Doctor") {
		t.Errorf("expected 'Orbit Doctor' header in CLI output")
	}
	if !strings.Contains(cliText, "Sistema Operativo") {
		t.Errorf("expected 'Sistema Operativo' in CLI output")
	}
}
