//go:build windows

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

// CreateDesktopShortcut creates a shortcut to Orbit on the user's Desktop
func CreateDesktopShortcut(exePath string) error {
	exePath = filepath.Clean(exePath)
	exeDir := filepath.Dir(exePath)

	userProfile := os.Getenv("USERPROFILE")
	desktopDir := filepath.Join(userProfile, "Desktop")
	shortcutPath := filepath.Join(desktopDir, "Orbit.lnk")

	// Clean legacy shortcut if present
	_ = os.Remove(filepath.Join(desktopDir, "Remote PC.lnk"))

	psScript := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%s'); $s.TargetPath = '%s'; $s.WorkingDirectory = '%s'; $s.Description = 'Orbit - Developer Cockpit & MCP Host'; $s.Save()`,
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
	desktopDir := filepath.Join(userProfile, "Desktop")
	for _, name := range []string{"Orbit.lnk", "Remote PC.lnk"} {
		p := filepath.Join(desktopDir, name)
		if _, err := os.Stat(p); err == nil {
			_ = os.Remove(p)
			fmt.Printf("[Shortcut] ✓ Acceso directo eliminado: %s\n", p)
		}
	}
}
