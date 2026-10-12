package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SetupMCP configures the MCP server for Claude Code CLI and Claude Desktop
func SetupMCP(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("no se pudo determinar la ruta del ejecutable: %w", err)
		}
	}
	exePath = filepath.Clean(exePath)

	fmt.Printf("[MCP] Configurando Orbit MCP con ejecutable: %s\n", exePath)

	// 1. Configurar Claude Code CLI
	setupClaudeCode(exePath)

	// 2. Configurar Claude Desktop
	setupClaudeDesktop(exePath)

	return nil
}

// UninstallMCP removes the MCP server configuration from Claude Code and Claude Desktop
func UninstallMCP() error {
	fmt.Println("[MCP] Desinstalando Orbit MCP de Claude Code y Claude Desktop...")

	uninstallClaudeCode()
	uninstallClaudeDesktop()

	return nil
}

func getClaudeDesktopConfigPath() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" && home != "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if appData != "" {
			return filepath.Join(appData, "Claude", "claude_desktop_config.json")
		}
	case "darwin":
		if home != "" {
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
		}
	default: // Linux and other POSIX
		if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
			return filepath.Join(configHome, "Claude", "claude_desktop_config.json")
		}
		if home != "" {
			return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
		}
	}
	return ""
}

func setupClaudeCode(exePath string) {
	// Intentar primero con el comando oficial de Claude CLI si está disponible
	if claudeCmd, err := exec.LookPath("claude"); err == nil && claudeCmd != "" {
		fmt.Println("[MCP] Ejecutando: claude mcp add --scope user orbit ...")
		cmd := exec.Command("claude", "mcp", "add", "--scope", "user", "orbit", "--", exePath, "mcp")
		out, err := cmd.CombinedOutput()
		if err == nil {
			fmt.Printf("[MCP] ✓ Claude Code configurado vía CLI exitosamente: %s\n", string(out))
			return
		}
		fmt.Printf("[MCP] Aviso: 'claude mcp add' falló (%v: %s), configurando directamente en .claude.json...\n", err, string(out))
	}

	// Configuración directa en ~/.claude.json
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		fmt.Println("[MCP] Aviso: No se encontró directorio home para configurar Claude Code.")
		return
	}

	claudeJsonPath := filepath.Join(home, ".claude.json")
	var root map[string]interface{}

	if data, err := os.ReadFile(claudeJsonPath); err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			root = make(map[string]interface{})
		}
	} else {
		root = make(map[string]interface{})
	}

	mcpServers, ok := root["mcpServers"].(map[string]interface{})
	if !ok || mcpServers == nil {
		mcpServers = make(map[string]interface{})
		root["mcpServers"] = mcpServers
	}

	serverConfig := map[string]interface{}{
		"type":    "stdio",
		"command": exePath,
		"args":    []string{"mcp"},
	}

	// Register Orbit as primary and remote-pc as compatibility alias
	mcpServers["orbit"] = serverConfig
	mcpServers["remote-pc"] = serverConfig

	if data, err := json.MarshalIndent(root, "", "  "); err == nil {
		if err := os.WriteFile(claudeJsonPath, data, 0644); err == nil {
			fmt.Printf("[MCP] ✓ Registrado 'orbit' en %s\n", claudeJsonPath)
		} else {
			fmt.Printf("[MCP] Error al escribir en %s: %v\n", claudeJsonPath, err)
		}
	}
}

func setupClaudeDesktop(exePath string) {
	configPath := getClaudeDesktopConfigPath()
	if configPath == "" {
		return
	}

	claudeDir := filepath.Dir(configPath)
	_ = os.MkdirAll(claudeDir, 0755)

	var root map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			root = make(map[string]interface{})
		}
	} else {
		root = make(map[string]interface{})
	}

	mcpServers, ok := root["mcpServers"].(map[string]interface{})
	if !ok || mcpServers == nil {
		mcpServers = make(map[string]interface{})
		root["mcpServers"] = mcpServers
	}

	serverConfig := map[string]interface{}{
		"command": exePath,
		"args":    []string{"mcp"},
	}

	// Register Orbit as primary and remote-pc as compatibility alias
	mcpServers["orbit"] = serverConfig
	mcpServers["remote-pc"] = serverConfig

	if data, err := json.MarshalIndent(root, "", "  "); err == nil {
		if err := os.WriteFile(configPath, data, 0644); err == nil {
			fmt.Printf("[MCP] ✓ Registrado 'orbit' en Claude Desktop: %s\n", configPath)
		} else {
			fmt.Printf("[MCP] Error al escribir en Claude Desktop (%s): %v\n", configPath, err)
		}
	}
}

func uninstallClaudeCode() {
	if claudeCmd, err := exec.LookPath("claude"); err == nil && claudeCmd != "" {
		_ = exec.Command("claude", "mcp", "remove", "--scope", "user", "orbit").Run()
		_ = exec.Command("claude", "mcp", "remove", "--scope", "user", "remote-pc").Run()
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return
	}

	claudeJsonPath := filepath.Join(home, ".claude.json")
	data, err := os.ReadFile(claudeJsonPath)
	if err != nil {
		return
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}

	if mcpServers, ok := root["mcpServers"].(map[string]interface{}); ok && mcpServers != nil {
		delete(mcpServers, "orbit")
		delete(mcpServers, "remote-pc")
		if newData, err := json.MarshalIndent(root, "", "  "); err == nil {
			_ = os.WriteFile(claudeJsonPath, newData, 0644)
			fmt.Printf("[MCP] ✓ Eliminado 'orbit' de %s\n", claudeJsonPath)
		}
	}
}

func uninstallClaudeDesktop() {
	configPath := getClaudeDesktopConfigPath()
	if configPath == "" {
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}

	if mcpServers, ok := root["mcpServers"].(map[string]interface{}); ok && mcpServers != nil {
		delete(mcpServers, "orbit")
		delete(mcpServers, "remote-pc")
		if newData, err := json.MarshalIndent(root, "", "  "); err == nil {
			_ = os.WriteFile(configPath, newData, 0644)
			fmt.Printf("[MCP] ✓ Eliminado 'orbit' de Claude Desktop (%s)\n", configPath)
		}
	}
}
