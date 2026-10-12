//go:build darwin

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

const (
	plistLabel = "com.orbit.daemon"
)

func getServiceTypeDescription() string {
	return "launchd user agent (~/Library/LaunchAgents/com.orbit.daemon.plist)"
}

func getPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

func isServiceInstalled() bool {
	p, err := getPlistPath()
	if err != nil {
		return false
	}
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}

// Install registers a launchd user agent for Orbit
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

	plistPath, err := getPlistPath()
	if err != nil {
		return fmt.Errorf("no se pudo determinar directorio home: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("error al crear directorio LaunchAgents: %w", err)
	}

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, "Library", "Logs", "Orbit")
	_ = os.MkdirAll(logDir, 0755)

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>-no-open</string>
    </array>
    <key>WorkingDirectory</key>
    <string>%s</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>%s/daemon-err.log</string>
</dict>
</plist>
`, plistLabel, exePath, exeDir, logDir, logDir)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("error escribiendo plist de launchd: %w", err)
	}

	_ = exec.Command("launchctl", "unload", plistPath).Run()
	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("error cargando agente launchd: %w", err)
	}

	_ = exec.Command("launchctl", "start", plistLabel).Run()
	return nil
}

// Uninstall unloads and removes the launchd user agent
func Uninstall(cfg *config.Config) error {
	plistPath, err := getPlistPath()
	if err == nil {
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)
	}
	_ = Stop(cfg)
	return nil
}

// Start triggers launchd start or fallback process
func Start(cfg *config.Config, exePath string) error {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	if pid := findPIDOnPort(port); pid > 0 {
		return fmt.Errorf("Orbit Daemon ya está ejecutándose en el puerto %s (PID: %d)", port, pid)
	}

	if isServiceInstalled() {
		_ = exec.Command("launchctl", "start", plistLabel).Run()
		return waitForHealth(port)
	}

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

// Stop stops the launchd service and kills process on port
func Stop(cfg *config.Config) error {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	if isServiceInstalled() {
		_ = exec.Command("launchctl", "stop", plistLabel).Run()
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
