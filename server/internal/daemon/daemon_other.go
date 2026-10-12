//go:build !windows && !linux && !darwin

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
	return "Proceso en Segundo Plano Standalone"
}

func isServiceInstalled() bool {
	return false
}

func Install(cfg *config.Config, exePath string) error {
	return fmt.Errorf("la instalación de autoarranque como servicio no está soportada en esta plataforma")
}

func Uninstall(cfg *config.Config) error {
	return Stop(cfg)
}

func Start(cfg *config.Config, exePath string) error {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	if pid := findPIDOnPort(port); pid > 0 {
		return fmt.Errorf("Orbit Daemon ya está ejecutándose en el puerto %s (PID: %d)", port, pid)
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

func Stop(cfg *config.Config) error {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	pid := findPIDOnPort(port)
	if pid > 0 {
		_ = exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run()
		return nil
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
