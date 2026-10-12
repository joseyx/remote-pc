package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	Port         string `json:"port"`
	AuthToken    string `json:"authToken"`
	Host         string `json:"host"`
	DefaultShell string `json:"defaultShell"`
	MaxHistoryKB int    `json:"maxHistoryKB"`
	PIN          string `json:"pin,omitempty"`
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
	base := filepath.Base(exeDir)

	// Search order for data directory:
	var candidates []string

	// If binary is in server/ or bin/, check parent data dir FIRST
	if strings.EqualFold(base, "server") || strings.EqualFold(base, "bin") {
		candidates = append(candidates, filepath.Join(exeDir, "..", "data"))
	}

	candidates = append(candidates,
		filepath.Join(exeDir, "data"),
		"data",
		filepath.Join(exeDir, "..", "data"),
	)

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

	// Helper to check if a directory contains real project data
	hasData := func(dir string) bool {
		indicators := []string{"calendar_notes", "server.config.json", "bubbles_metadata.json", "sessions_state.json"}
		for _, ind := range indicators {
			if _, err := os.Stat(filepath.Join(dir, ind)); err == nil {
				return true
			}
		}
		return false
	}

	// 1. First pass: Return first candidate that actually contains existing data files
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			if hasData(c) {
				return filepath.Clean(c)
			}
		}
	}

	// 2. Second pass: If none has data, return first candidate directory that already exists
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return filepath.Clean(c)
		}
	}

	// 3. Fallback: choose the best location to initialize data
	var targetDir string
	if strings.EqualFold(base, "server") || strings.EqualFold(base, "bin") {
		targetDir = filepath.Join(exeDir, "..", "data")
	} else {
		targetDir = filepath.Join(exeDir, "data")
	}
	_ = os.MkdirAll(targetDir, 0755)
	return filepath.Clean(targetDir)
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

// Save persists the configuration to server.config.json in dataDir
func (c *Config) Save() error {
	dataDir := c.DataDir
	if dataDir == "" {
		dataDir = GetDataDir()
	}
	savePath := filepath.Join(dataDir, "server.config.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(savePath, data, 0644)
}

// RotateToken generates a new random token or sets a custom token and persists it
func (c *Config) RotateToken(customToken string) string {
	if customToken != "" {
		c.AuthToken = customToken
	} else {
		bytes := make([]byte, 16)
		_, _ = rand.Read(bytes)
		c.AuthToken = hex.EncodeToString(bytes)
	}
	_ = c.Save()
	return c.AuthToken
}

// SetPIN configures or updates the security PIN
func (c *Config) SetPIN(pin string) error {
	c.PIN = pin
	return c.Save()
}

// RemovePIN clears the configured security PIN
func (c *Config) RemovePIN() error {
	c.PIN = ""
	return c.Save()
}

// VerifyPIN checks whether the provided PIN matches
func (c *Config) VerifyPIN(pin string) bool {
	if c.PIN == "" {
		return true // No PIN configured
	}
	return c.PIN == pin
}
