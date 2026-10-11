package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	fmt.Printf("[MCP] Configurando Remote PC MCP con ejecutable: %s\n", exePath)

	// 1. Configurar Claude Code CLI
	setupClaudeCode(exePath)

	// 2. Configurar Claude Desktop
	setupClaudeDesktop(exePath)

	return nil
}

// UninstallMCP removes the MCP server configuration from Claude Code and Claude Desktop
func UninstallMCP() error {
	fmt.Println("[MCP] Desinstalando Remote PC MCP de Claude Code y Claude Desktop...")

	uninstallClaudeCode()
	uninstallClaudeDesktop()

	return nil
}

func setupClaudeCode(exePath string) {
	// Intentar primero con el comando oficial de Claude CLI si está disponible
	if claudeCmd, err := exec.LookPath("claude"); err == nil && claudeCmd != "" {
		fmt.Println("[MCP] Ejecutando: claude mcp add --scope user remote-pc ...")
		cmd := exec.Command("claude", "mcp", "add", "--scope", "user", "remote-pc", "--", exePath, "mcp")
		out, err := cmd.CombinedOutput()
		if err == nil {
			fmt.Printf("[MCP] ✓ Claude Code configurado vía CLI exitosamente: %s\n", string(out))
			return
		}
		fmt.Printf("[MCP] Aviso: 'claude mcp add' falló (%v: %s), configurando directamente en .claude.json...\n", err, string(out))
	}

	// Configuración directa en ~/.claude.json
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		fmt.Println("[MCP] Aviso: No se encontró USERPROFILE para configurar Claude Code.")
		return
	}

	claudeJsonPath := filepath.Join(userProfile, ".claude.json")
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

	mcpServers["remote-pc"] = map[string]interface{}{
		"type":    "stdio",
		"command": exePath,
		"args":    []string{"mcp"},
	}

	if data, err := json.MarshalIndent(root, "", "  "); err == nil {
		if err := os.WriteFile(claudeJsonPath, data, 0644); err == nil {
			fmt.Printf("[MCP] ✓ Registrado en %s\n", claudeJsonPath)
		} else {
			fmt.Printf("[MCP] Error al escribir en %s: %v\n", claudeJsonPath, err)
		}
	}
}

func setupClaudeDesktop(exePath string) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return
	}

	claudeDir := filepath.Join(appData, "Claude")
	_ = os.MkdirAll(claudeDir, 0755)

	configPath := filepath.Join(claudeDir, "claude_desktop_config.json")
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

	mcpServers["remote-pc"] = map[string]interface{}{
		"command": exePath,
		"args":    []string{"mcp"},
	}

	if data, err := json.MarshalIndent(root, "", "  "); err == nil {
		if err := os.WriteFile(configPath, data, 0644); err == nil {
			fmt.Printf("[MCP] ✓ Registrado en Claude Desktop: %s\n", configPath)
		} else {
			fmt.Printf("[MCP] Error al escribir en Claude Desktop (%s): %v\n", configPath, err)
		}
	}
}

func uninstallClaudeCode() {
	if claudeCmd, err := exec.LookPath("claude"); err == nil && claudeCmd != "" {
		cmd := exec.Command("claude", "mcp", "remove", "--scope", "user", "remote-pc")
		_ = cmd.Run()
	}

	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return
	}

	claudeJsonPath := filepath.Join(userProfile, ".claude.json")
	data, err := os.ReadFile(claudeJsonPath)
	if err != nil {
		return
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}

	if mcpServers, ok := root["mcpServers"].(map[string]interface{}); ok && mcpServers != nil {
		if _, exists := mcpServers["remote-pc"]; exists {
			delete(mcpServers, "remote-pc")
			if newData, err := json.MarshalIndent(root, "", "  "); err == nil {
				_ = os.WriteFile(claudeJsonPath, newData, 0644)
				fmt.Printf("[MCP] ✓ Eliminado 'remote-pc' de %s\n", claudeJsonPath)
			}
		}
	}
}

func uninstallClaudeDesktop() {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return
	}

	configPath := filepath.Join(appData, "Claude", "claude_desktop_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return
	}

	if mcpServers, ok := root["mcpServers"].(map[string]interface{}); ok && mcpServers != nil {
		if _, exists := mcpServers["remote-pc"]; exists {
			delete(mcpServers, "remote-pc")
			if newData, err := json.MarshalIndent(root, "", "  "); err == nil {
				_ = os.WriteFile(configPath, newData, 0644)
				fmt.Printf("[MCP] ✓ Eliminado 'remote-pc' de Claude Desktop (%s)\n", configPath)
			}
		}
	}
}
