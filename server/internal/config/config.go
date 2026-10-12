package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Port         string `json:"port"`
	AuthToken    string `json:"authToken"`
	Host         string `json:"host"`
	DefaultShell string `json:"defaultShell"`
	MaxHistoryKB int    `json:"maxHistoryKB"`
	DataDir      string `json:"-"`
}

// GetDataDir determines the central storage folder for runtime data (notes, metadata, state)
func GetDataDir() string {
	if envDir := os.Getenv("ORBIT_DATA_DIR"); envDir != "" {
		_ = os.MkdirAll(envDir, 0755)
		return envDir
	}
	if envDir := os.Getenv("REMOTE_PC_DATA_DIR"); envDir != "" {
		_ = os.MkdirAll(envDir, 0755)
		return envDir
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	home, _ := os.UserHomeDir()

	// Search order for data directory:
	// 1. "data" in current working directory
	// 2. "data" in executable directory
	// 3. "../data" (if binary is in server/ or bin/)
	candidates := []string{
		"data",
		filepath.Join(exeDir, "data"),
		filepath.Join(exeDir, "..", "data"),
	}

	if runtime.GOOS == "darwin" && home != "" {
		candidates = append(candidates,
			filepath.Join(home, "Library", "Application Support", "Orbit", "data"),
			filepath.Join(home, "Library", "Application Support", "RemotePC", "data"),
		)
	} else if runtime.GOOS != "windows" && home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".local", "share", "orbit", "data"),
			filepath.Join(home, ".local", "share", "remotepc", "data"),
		)
	} else if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		candidates = append(candidates,
			filepath.Join(localApp, "Orbit", "data"),
			filepath.Join(localApp, "RemotePC", "data"),
		)
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return filepath.Clean(c)
		}
	}

	// Create "data" in current directory if none exists
	_ = os.MkdirAll("data", 0755)
	return "data"
}

func LoadConfig() *Config {
	dataDir := GetDataDir()

	defaultShell := "powershell.exe"
	if runtime.GOOS == "darwin" {
		defaultShell = "/bin/zsh"
	} else if runtime.GOOS != "windows" {
		if shell := os.Getenv("SHELL"); shell != "" {
			defaultShell = shell
		} else {
			defaultShell = "/bin/bash"
		}
	}

	cfg := &Config{
		Port:         "8080",
		Host:         "0.0.0.0",
		DefaultShell: defaultShell,
		MaxHistoryKB: 2048, // 2 MB ring buffer per session
		DataDir:      dataDir,
	}

	configPath := filepath.Join(dataDir, "server.config.json")
	// If config doesn't exist in dataDir, check root/exeDir as fallback
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fallbackPaths := []string{
			"server.config.json",
			filepath.Join(filepath.Dir(dataDir), "server.config.json"),
		}
		for _, fp := range fallbackPaths {
			if _, err := os.Stat(fp); err == nil {
				configPath = fp
				break
			}
		}
	}

	// Load existing persistent configuration if available
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	portFlag := flag.String("port", "", "Server port (default: 8080 or PORT env)")
	tokenFlag := flag.String("token", "", "Authentication token (or AUTH_TOKEN env)")
	hostFlag := flag.String("host", "", "Listen address (default: 0.0.0.0)")

	if !flag.Parsed() {
		flag.Parse()
	}

	if *portFlag != "" {
		cfg.Port = *portFlag
	} else if envPort := os.Getenv("ORBIT_PORT"); envPort != "" {
		cfg.Port = envPort
	} else if envPort := os.Getenv("REMOTE_PC_PORT"); envPort != "" {
		cfg.Port = envPort
	}

	if *hostFlag != "" {
		cfg.Host = *hostFlag
	}

	if *tokenFlag != "" {
		cfg.AuthToken = *tokenFlag
	} else if envToken := os.Getenv("ORBIT_TOKEN"); envToken != "" {
		cfg.AuthToken = envToken
	} else if envToken := os.Getenv("REMOTE_PC_TOKEN"); envToken != "" {
		cfg.AuthToken = envToken
	} else if cfg.AuthToken == "" {
		// Generate random secure token if none specified
		bytes := make([]byte, 16)
		_, _ = rand.Read(bytes)
		cfg.AuthToken = hex.EncodeToString(bytes)
	}

	// Persist configuration to disk in dataDir
	savePath := filepath.Join(dataDir, "server.config.json")
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = os.WriteFile(savePath, data, 0644)
	}

	return cfg
}
