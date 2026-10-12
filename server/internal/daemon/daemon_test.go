package daemon

import (
	"strings"
	"testing"
	"time"

	"orbit/internal/config"
)

func TestFormatCLI(t *testing.T) {
	t.Run("Stopped and Not Installed", func(t *testing.T) {
		st := Status{
			Installed:   false,
			Running:     false,
			Port:        "8080",
			ServiceType: "Registro de Inicio de Usuario",
			CheckedAt:   time.Now(),
		}
		out := FormatCLI(st)
		if !strings.Contains(out, "No registrado") {
			t.Errorf("expected output to mention 'No registrado', got:\n%s", out)
		}
		if !strings.Contains(out, "Inactivo") {
			t.Errorf("expected output to mention 'Inactivo', got:\n%s", out)
		}
		if !strings.Contains(out, "orbit service install") {
			t.Errorf("expected output to suggest 'orbit service install', got:\n%s", out)
		}
	})

	t.Run("Running and Installed", func(t *testing.T) {
		st := Status{
			Installed:    true,
			Running:      true,
			PID:          12345,
			Port:         "8080",
			Uptime:       "1h 30m",
			Version:      "1.0.0",
			SessionCount: 2,
			ClientCount:  1,
			ServiceType:  "systemd user service",
			CheckedAt:    time.Now(),
		}
		out := FormatCLI(st)
		if !strings.Contains(out, "[✓] Instalado") {
			t.Errorf("expected output to mention '[✓] Instalado', got:\n%s", out)
		}
		if !strings.Contains(out, "Activo") || !strings.Contains(out, "12345") {
			t.Errorf("expected output to mention 'Activo' and PID, got:\n%s", out)
		}
		if !strings.Contains(out, "orbit service stop") {
			t.Errorf("expected output to suggest 'orbit service stop', got:\n%s", out)
		}
	})
}

func TestGetStatusUnbound(t *testing.T) {
	cfg := &config.Config{
		Port: "59999", // Unlikely to be listening in test
	}
	st := GetStatus(cfg, "1.0.0")
	if st.Running {
		t.Errorf("expected port 59999 to not be running")
	}
	if st.Port != "59999" {
		t.Errorf("expected port to be 59999, got %s", st.Port)
	}
	if st.ServiceType == "" {
		t.Errorf("expected non-empty ServiceType")
	}
}
