package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"orbit/internal/mcp"
)

// Install performs self-installation: adds to PATH, configures MCP, and creates shortcuts
func Install(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	exePath = filepath.Clean(exePath)
	exeDir := filepath.Dir(exePath)

	fmt.Println("=====================================================")
	fmt.Println("             Orbit - Self-Installer                  ")
	fmt.Println("=====================================================")
	fmt.Printf(" Ejecutable : %s\n", exePath)
	fmt.Printf(" Directorio : %s\n", exeDir)
	fmt.Println("-----------------------------------------------------")

	// 1. Agregar a PATH
	if err := AddToUserPath(exeDir); err != nil {
		fmt.Printf("Error agregando a PATH: %v\n", err)
	}

	// 2. Configurar MCP
	if err := mcp.SetupMCP(exePath); err != nil {
		fmt.Printf("Error configurando MCP: %v\n", err)
	}

	// 3. Crear acceso directo
	if err := CreateDesktopShortcut(exePath); err != nil {
		fmt.Printf("Aviso acceso directo: %v\n", err)
	}

	fmt.Println("=====================================================")
	fmt.Println(" ✓ Instalación de Orbit completada con éxito.")
	fmt.Println(" Ahora puedes:")
	fmt.Println(" 1. Ejecutar 'orbit' (o 'remotepc-server') desde cualquier consola.")
	fmt.Println(" 2. Usar las herramientas MCP de Orbit directamente en Claude Code.")
	fmt.Println("=====================================================")
	return nil
}

// Uninstall performs self-uninstallation: removes from PATH, removes MCP, and removes shortcuts
func Uninstall(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	exePath = filepath.Clean(exePath)
	exeDir := filepath.Dir(exePath)

	fmt.Println("=====================================================")
	fmt.Println("            Orbit - Desinstalador                    ")
	fmt.Println("=====================================================")

	// 1. Remover de PATH
	_ = RemoveFromUserPath(exeDir)

	// 2. Remover MCP
	_ = mcp.UninstallMCP()

	// 3. Remover acceso directo
	RemoveDesktopShortcut()

	fmt.Println("=====================================================")
	fmt.Println(" ✓ Desinstalación completada.")
	fmt.Println("=====================================================")
	return nil
}
