package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
)

type Config struct {
	Port         string `json:"port"`
	AuthToken    string `json:"authToken"`
	Host         string `json:"host"`
	DefaultShell string `json:"defaultShell"`
	MaxHistoryKB int    `json:"maxHistoryKB"`
}

func LoadConfig() *Config {
	cfg := &Config{
		Port:         "8080",
		Host:         "0.0.0.0",
		DefaultShell: "powershell.exe",
		MaxHistoryKB: 2048, // 2 MB ring buffer per session
	}

	configPath := "server.config.json"
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
	} else if envPort := os.Getenv("REMOTE_PC_PORT"); envPort != "" {
		cfg.Port = envPort
	}

	if *hostFlag != "" {
		cfg.Host = *hostFlag
	}

	if *tokenFlag != "" {
		cfg.AuthToken = *tokenFlag
	} else if envToken := os.Getenv("REMOTE_PC_TOKEN"); envToken != "" {
		cfg.AuthToken = envToken
	} else if cfg.AuthToken == "" {
		// Generate random secure token if none specified
		bytes := make([]byte, 16)
		_, _ = rand.Read(bytes)
		cfg.AuthToken = hex.EncodeToString(bytes)
	}

	// Persist configuration to disk
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = os.WriteFile(configPath, data, 0644)
	}

	return cfg
}
