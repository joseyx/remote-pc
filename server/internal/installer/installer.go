package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"remotepc-server/internal/mcp"
)

var (
	moduser32               = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW = moduser32.NewProc("SendMessageTimeoutW")
)

const (
	HWND_BROADCAST   = 0xffff
	WM_SETTINGCHANGE = 0x001A
	SMTO_ABORTIFHUNG = 0x0002
)

// BroadcastSettingChange notifies Windows that environment variables have changed
func BroadcastSettingChange() {
	envStr, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}

	var result uintptr
	_, _, _ = procSendMessageTimeoutW.Call(
		HWND_BROADCAST,
		WM_SETTINGCHANGE,
		0,
		uintptr(unsafe.Pointer(envStr)),
		SMTO_ABORTIFHUNG,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}

// AddToUserPath adds dir to the current user's PATH environment variable in the registry
func AddToUserPath(dir string) error {
	dir = filepath.Clean(dir)

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("error al abrir clave de registro Environment: %w", err)
	}
	defer k.Close()

	curPath, valType, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("error al leer PATH: %w", err)
	}

	parts := strings.Split(curPath, ";")
	dirNorm := strings.ToLower(dir)
	for _, p := range parts {
		if strings.ToLower(filepath.Clean(strings.TrimSpace(p))) == dirNorm {
			fmt.Printf("[PATH] La ruta ya se encuentra en el PATH del usuario: %s\n", dir)
			return nil
		}
	}

	var newPath string
	if strings.TrimSpace(curPath) == "" {
		newPath = dir
	} else {
		newPath = strings.TrimRight(curPath, ";") + ";" + dir
	}

	if valType == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", newPath)
	} else {
		err = k.SetStringValue("Path", newPath)
	}
	if err != nil {
		return fmt.Errorf("error al actualizar registro PATH: %w", err)
	}

	BroadcastSettingChange()
	fmt.Printf("[PATH] ✓ Se agregó exitosamente al PATH del usuario: %s\n", dir)
	return nil
}

// RemoveFromUserPath removes dir from the current user's PATH environment variable in the registry
func RemoveFromUserPath(dir string) error {
	dir = filepath.Clean(dir)

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("error al abrir clave de registro Environment: %w", err)
	}
	defer k.Close()

	curPath, valType, err := k.GetStringValue("Path")
	if err != nil {
		return nil
	}

	parts := strings.Split(curPath, ";")
	dirNorm := strings.ToLower(dir)
	var filtered []string
	removed := false

	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if strings.ToLower(filepath.Clean(trimmed)) == dirNorm {
			removed = true
			continue
		}
		filtered = append(filtered, trimmed)
	}

	if !removed {
		return nil
	}

	newPath := strings.Join(filtered, ";")
	if valType == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", newPath)
	} else {
		err = k.SetStringValue("Path", newPath)
	}
	if err != nil {
		return fmt.Errorf("error al actualizar PATH en el registro: %w", err)
	}

	BroadcastSettingChange()
	fmt.Printf("[PATH] ✓ Se eliminó del PATH del usuario: %s\n", dir)
	return nil
}

// CreateDesktopShortcut creates a shortcut to Remote PC on the user's Desktop
func CreateDesktopShortcut(exePath string) error {
	exePath = filepath.Clean(exePath)
	exeDir := filepath.Dir(exePath)

	userProfile := os.Getenv("USERPROFILE")
	desktopDir := filepath.Join(userProfile, "Desktop")
	shortcutPath := filepath.Join(desktopDir, "Remote PC.lnk")

	psScript := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%s'); $s.TargetPath = '%s'; $s.WorkingDirectory = '%s'; $s.Description = 'Remote PC Host & Web Dashboard'; $s.Save()`,
		shortcutPath, exePath, exeDir,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error creando acceso directo en escritorio: %w", err)
	}

	fmt.Printf("[Shortcut] ✓ Acceso directo creado en: %s\n", shortcutPath)
	return nil
}

// RemoveDesktopShortcut deletes the desktop shortcut if it exists
func RemoveDesktopShortcut() {
	userProfile := os.Getenv("USERPROFILE")
	shortcutPath := filepath.Join(userProfile, "Desktop", "Remote PC.lnk")
	if _, err := os.Stat(shortcutPath); err == nil {
		_ = os.Remove(shortcutPath)
		fmt.Printf("[Shortcut] ✓ Acceso directo eliminado: %s\n", shortcutPath)
	}
}

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
	fmt.Println("             Remote PC - Self-Installer              ")
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
	fmt.Println(" ✓ Instalación completada con éxito.")
	fmt.Println(" Ahora puedes:")
	fmt.Println(" 1. Ejecutar 'remotepc-server' desde cualquier consola.")
	fmt.Println(" 2. Usar las herramientas MCP de Remote PC directamente en Claude Code.")
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
	fmt.Println("            Remote PC - Desinstalador                ")
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
