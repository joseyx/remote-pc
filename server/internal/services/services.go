package services

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	brifScrapperDir = `c:\Users\Jose\Documents\Work\brif\brif-scrapper`
	mu              sync.Mutex
	runningCommands = make(map[string]*exec.Cmd)
)

type ServiceStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Running bool   `json:"running"`
	PingMs  int64  `json:"pingMs"`
}

func StartService(id string) error {
	mu.Lock()
	defer mu.Unlock()

	switch id {
	case "redis":
		cmd := exec.Command("wsl", "-u", "root", "systemctl", "start", "redis-server")
		return cmd.Run()

	case "flower":
		if isPortOpen(5555) {
			return nil
		}
		cmd := exec.Command(brifScrapperDir+`\venv\Scripts\celery.exe`, "-A", "config", "flower", "--port=5555")
		cmd.Dir = brifScrapperDir
		if err := cmd.Start(); err != nil {
			return err
		}
		runningCommands["flower"] = cmd
		for i := 0; i < 15; i++ {
			time.Sleep(200 * time.Millisecond)
			if isPortOpen(5555) {
				break
			}
		}
		return nil

	case "celery-worker":
		cmd := exec.Command(brifScrapperDir+`\venv\Scripts\celery.exe`, "-A", "config", "worker", "--loglevel=info", "-E", "--pool=solo", "-n", "brif-worker", "-Q", "celery,fetch,articles")
		cmd.Dir = brifScrapperDir
		if err := cmd.Start(); err != nil {
			return err
		}
		runningCommands["celery-worker"] = cmd
		return nil

	case "celery-beat":
		cmd := exec.Command(brifScrapperDir+`\venv\Scripts\celery.exe`, "-A", "config", "beat", "--loglevel=info", "--scheduler", "django_celery_beat.schedulers:DatabaseScheduler")
		cmd.Dir = brifScrapperDir
		if err := cmd.Start(); err != nil {
			return err
		}
		runningCommands["celery-beat"] = cmd
		return nil

	default:
		return fmt.Errorf("servicio desconocido: %s", id)
	}
}

func StopService(id string) error {
	mu.Lock()
	defer mu.Unlock()

	switch id {
	case "redis":
		cmd := exec.Command("wsl", "-u", "root", "systemctl", "stop", "redis-server")
		return cmd.Run()

	case "flower":
		if cmd, ok := runningCommands["flower"]; ok && cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			delete(runningCommands, "flower")
		}
		killProcessOnPort(5555)
		return nil

	case "celery-worker":
		if cmd, ok := runningCommands["celery-worker"]; ok && cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			delete(runningCommands, "celery-worker")
		}
		_ = exec.Command("taskkill", "/F", "/IM", "celery.exe").Run()
		return nil

	case "celery-beat":
		if cmd, ok := runningCommands["celery-beat"]; ok && cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			delete(runningCommands, "celery-beat")
		}
		return nil

	default:
		return fmt.Errorf("servicio desconocido: %s", id)
	}
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 80*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func killProcessOnPort(port int) {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return
	}
	target := fmt.Sprintf(":%d", port)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, target) && strings.Contains(line, "LISTENING") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				pidStr := fields[len(fields)-1]
				if pid, err := strconv.Atoi(pidStr); err == nil && pid > 4 {
					_ = exec.Command("taskkill", "/F", "/PID", pidStr).Run()
				}
			}
		}
	}
}
