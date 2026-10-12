//go:build linux

package daemon

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"orbit/internal/config"
)

func getServiceTypeDescription() string {
	return "systemd user service (~/.config/systemd/user/orbit.service)"
}

func getServiceUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "orbit.service"), nil
}

func isServiceInstalled() bool {
	p, err := getServiceUnitPath()
	if err != nil {
		return false
	}
	if _, err := os.Stat(p); err == nil {
		return true
	}
	out, err := exec.Command("systemctl", "--user", "is-enabled", "orbit.service").Output()
	return err == nil && strings.TrimSpace(string(out)) == "enabled"
}

// Install creates and enables a systemd user unit for Orbit
func Install(cfg *config.Config, exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	exePath, _ = filepath.Abs(filepath.Clean(exePath))
	exeDir := filepath.Dir(exePath)

	unitPath, err := getServiceUnitPath()
	if err != nil {
		return fmt.Errorf("no se pudo determinar directorio home: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return fmt.Errorf("error al crear directorio systemd user: %w", err)
	}

	unitContent := fmt.Sprintf(`[Unit]
Description=Orbit Developer Cockpit & Workspace Hub
After=network.target

[Service]
Type=simple
ExecStart=%s -no-open
WorkingDirectory=%s
Restart=on-failure
RestartSec=5
Environment=PATH=%%h/.local/bin:%%h/bin:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=default.target
`, exePath, exeDir)

	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("error escribiendo archivo de unidad systemd: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", "orbit.service").Run(); err != nil {
		return fmt.Errorf("error habilitando servicio systemd user: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "start", "orbit.service").Run()
	return nil
}

// Uninstall disables and removes the systemd user unit
func Uninstall(cfg *config.Config) error {
	_ = exec.Command("systemctl", "--user", "stop", "orbit.service").Run()
	_ = exec.Command("systemctl", "--user", "disable", "orbit.service").Run()

	unitPath, err := getServiceUnitPath()
	if err == nil {
		_ = os.Remove(unitPath)
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// Start initiates the systemd user service or background process
func Start(cfg *config.Config, exePath string) error {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	if pid := findPIDOnPort(port); pid > 0 {
		return fmt.Errorf("Orbit Daemon ya está ejecutándose en el puerto %s (PID: %d)", port, pid)
	}

	if isServiceInstalled() {
		if err := exec.Command("systemctl", "--user", "start", "orbit.service").Run(); err == nil {
			return waitForHealth(port)
		}
	}

	// Standalone fallback if systemd is not active (e.g. containers / WSL without systemd)
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	exePath, _ = filepath.Abs(filepath.Clean(exePath))
	cmd := exec.Command(exePath, "-no-open")
	cmd.Dir = filepath.Dir(exePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error al iniciar proceso: %w", err)
	}

	return waitForHealth(port)
}

// Stop halts the systemd user service or process
func Stop(cfg *config.Config) error {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	if isServiceInstalled() {
		_ = exec.Command("systemctl", "--user", "stop", "orbit.service").Run()
	}

	pid := findPIDOnPort(port)
	if pid > 0 {
		_ = exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run()
		time.Sleep(300 * time.Millisecond)
		if findPIDOnPort(port) > 0 {
			_ = exec.Command("kill", "-KILL", strconv.Itoa(pid)).Run()
		}
		return nil
	}

	return nil
}

func waitForHealth(port string) error {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/api/health", port))
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	return nil
}

func findPIDOnPort(port string) int {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf(":%s", port)).Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		myPID := os.Getpid()
		for _, line := range lines {
			if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && pid > 0 && pid != myPID {
				return pid
			}
		}
	}
	return 0
}
