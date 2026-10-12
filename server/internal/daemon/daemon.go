package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orbit/internal/config"
)

// Status represents the operational and registration status of the background daemon
type Status struct {
	Installed    bool      `json:"installed"`
	Running      bool      `json:"running"`
	PID          int       `json:"pid,omitempty"`
	Port         string    `json:"port"`
	Uptime       string    `json:"uptime,omitempty"`
	Version      string    `json:"version,omitempty"`
	SessionCount int       `json:"sessionCount,omitempty"`
	ClientCount  int       `json:"clientCount,omitempty"`
	ServiceType  string    `json:"serviceType"`
	ExePath      string    `json:"exePath"`
	Message      string    `json:"message"`
	Details      string    `json:"details,omitempty"`
	CheckedAt    time.Time `json:"checkedAt"`
}

// FormatCLI renders a human-friendly console summary of the daemon status
func FormatCLI(st Status) string {
	var sb strings.Builder
	sb.WriteString("=====================================================\n")
	sb.WriteString("            Orbit - Background Service               \n")
	sb.WriteString("=====================================================\n")

	instStr := "[!] No registrado para autoarranque"
	if st.Installed {
		instStr = "[✓] Instalado (Inicio automático con el sistema)"
	}
	sb.WriteString(fmt.Sprintf(" Estado Servicio  : %s\n", instStr))
	sb.WriteString(fmt.Sprintf(" Tipo de Servicio : %s\n", st.ServiceType))

	runStr := "[🔴 Inactivo]"
	if st.Running {
		if st.PID > 0 {
			runStr = fmt.Sprintf("[🟢 Activo] (PID: %d)", st.PID)
		} else {
			runStr = "[🟢 Activo]"
		}
	}
	sb.WriteString(fmt.Sprintf(" Estado Proceso   : %s\n", runStr))
	sb.WriteString(fmt.Sprintf(" Puerto HTTP      : %s (http://localhost:%s/)\n", st.Port, st.Port))

	if st.Running {
		if st.Uptime != "" {
			sb.WriteString(fmt.Sprintf(" Uptime           : %s\n", st.Uptime))
		}
		if st.Version != "" {
			sb.WriteString(fmt.Sprintf(" Versión Servidor : v%s\n", st.Version))
		}
		sb.WriteString(fmt.Sprintf(" Sesiones Activas : %d\n", st.SessionCount))
		sb.WriteString(fmt.Sprintf(" Clientes Web     : %d\n", st.ClientCount))
	}

	if st.ExePath != "" {
		sb.WriteString(fmt.Sprintf(" Ruta Ejecutable  : %s\n", st.ExePath))
	}

	if st.Details != "" {
		sb.WriteString(fmt.Sprintf(" Información      : %s\n", st.Details))
	}

	sb.WriteString("-----------------------------------------------------\n")
	if !st.Installed {
		sb.WriteString(" 👉 Para autoarranque silencioso: 'orbit service install'\n")
	} else if !st.Running {
		sb.WriteString(" 👉 Para iniciar en segundo plano: 'orbit service start'\n")
	} else {
		sb.WriteString(" 👉 Para detener el proceso: 'orbit service stop'\n")
		sb.WriteString(" 👉 Para desinstalar el servicio: 'orbit service uninstall'\n")
	}
	sb.WriteString("=====================================================\n")
	return sb.String()
}

// GetStatus checks both daemon installation and running state
func GetStatus(cfg *config.Config, currentVersion string) Status {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	exePath, _ = filepath.Abs(exePath)

	st := Status{
		Installed:   isServiceInstalled(),
		ServiceType: getServiceTypeDescription(),
		Port:        port,
		ExePath:     exePath,
		CheckedAt:   time.Now(),
	}

	// 1. Health check over HTTP
	client := &http.Client{Timeout: 750 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/api/health", port))
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		st.Running = true
		st.Message = "Orbit Daemon está en línea y respondiendo."

		// Query server-info for extra telemetry
		infoResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/api/server-info", port))
		if err == nil && infoResp.StatusCode == http.StatusOK {
			defer infoResp.Body.Close()
			var info struct {
				Version      string `json:"version"`
				Uptime       string `json:"uptime"`
				SessionCount int    `json:"sessionCount"`
				ClientCount  int    `json:"clientCount"`
			}
			if err := json.NewDecoder(infoResp.Body).Decode(&info); err == nil {
				st.Version = info.Version
				st.Uptime = info.Uptime
				st.SessionCount = info.SessionCount
				st.ClientCount = info.ClientCount
			}
		}

		// Find PID
		st.PID = findPIDOnPort(port)
	} else {
		if resp != nil {
			_ = resp.Body.Close()
		}
		st.Running = false
		st.Message = fmt.Sprintf("Orbit Daemon no está escuchando en el puerto %s.", port)
	}

	return st
}

// Restart stops and starts the daemon
func Restart(cfg *config.Config, exePath string) error {
	_ = Stop(cfg)
	time.Sleep(500 * time.Millisecond)
	return Start(cfg, exePath)
}
