//go:build !windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BroadcastSettingChange is a no-op on POSIX systems
func BroadcastSettingChange() {
	// Not applicable on Unix
}

// AddToUserPath installs remotepc-server into ~/.local/bin and ensures PATH is configured
func AddToUserPath(dir string) error {
	dir = filepath.Clean(dir)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("no se pudo determinar el directorio home: %w", err)
	}

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", localBin, err)
	}

	// Symlink to ~/.local/bin/remotepc-server
	targetSymlink := filepath.Join(localBin, "remotepc-server")
	sourceExe := filepath.Join(dir, "remotepc-server")
	if _, err := os.Stat(sourceExe); os.IsNotExist(err) {
		// If binary name differs or called directly with executable path
		if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
			sourceExe = dir
		}
	}

	_ = os.Remove(targetSymlink) // Remove previous symlink if present
	if err := os.Symlink(sourceExe, targetSymlink); err != nil {
		fmt.Printf("[PATH] Aviso: no se pudo crear symlink en %s (%v). Copiando binario...\n", targetSymlink, err)
		data, readErr := os.ReadFile(sourceExe)
		if readErr == nil {
			_ = os.WriteFile(targetSymlink, data, 0755)
		}
	} else {
		fmt.Printf("[PATH] ✓ Symlink creado en: %s -> %s\n", targetSymlink, sourceExe)
	}

	// Check if localBin is in current PATH
	envPath := os.Getenv("PATH")
	inPath := false
	for _, p := range filepath.SplitList(envPath) {
		if filepath.Clean(p) == localBin || filepath.Clean(p) == dir {
			inPath = true
			break
		}
	}

	if inPath {
		fmt.Printf("[PATH] ✓ %s ya está en tu variable PATH.\n", localBin)
		return nil
	}

	// Append export to shell rc files
	pathExportLine := `export PATH="$HOME/.local/bin:$PATH"`
	rcFiles := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".profile"),
	}

	updatedAny := false
	for _, rc := range rcFiles {
		if _, err := os.Stat(rc); err == nil {
			content, err := os.ReadFile(rc)
			if err == nil && !strings.Contains(string(content), "$HOME/.local/bin") {
				newContent := string(content) + "\n# Remote PC Server CLI\n" + pathExportLine + "\n"
				if err := os.WriteFile(rc, []byte(newContent), 0644); err == nil {
					fmt.Printf("[PATH] ✓ Se agregó %s a: %s\n", localBin, rc)
					updatedAny = true
				}
			}
		}
	}

	if !updatedAny {
		// If neither .zshrc nor .bashrc existed, create the appropriate one based on OS default
		targetRc := filepath.Join(home, ".bashrc")
		if runtime.GOOS == "darwin" {
			targetRc = filepath.Join(home, ".zshrc")
		}
		newContent := "# Remote PC Server CLI\n" + pathExportLine + "\n"
		_ = os.WriteFile(targetRc, []byte(newContent), 0644)
		fmt.Printf("[PATH] ✓ Creado y configurado PATH en: %s\n", targetRc)
	}

	fmt.Printf("[PATH] Recuerda ejecutar 'source ~/.zshrc' o 'source ~/.bashrc' para actualizar tu sesión actual.\n")
	return nil
}

// RemoveFromUserPath removes the symlink and shell RC modifications
func RemoveFromUserPath(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	targetSymlink := filepath.Join(home, ".local", "bin", "remotepc-server")
	if _, err := os.Lstat(targetSymlink); err == nil {
		_ = os.Remove(targetSymlink)
		fmt.Printf("[PATH] ✓ Eliminado: %s\n", targetSymlink)
	}

	rcFiles := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".profile"),
	}

	for _, rc := range rcFiles {
		contentBytes, err := os.ReadFile(rc)
		if err != nil {
			continue
		}
		lines := strings.Split(string(contentBytes), "\n")
		var filtered []string
		modified := false
		for _, line := range lines {
			if strings.Contains(line, "Remote PC Server CLI") || strings.Contains(line, `$HOME/.local/bin:$PATH`) {
				modified = true
				continue
			}
			filtered = append(filtered, line)
		}
		if modified {
			_ = os.WriteFile(rc, []byte(strings.Join(filtered, "\n")), 0644)
			fmt.Printf("[PATH] ✓ Removida configuración de PATH en: %s\n", rc)
		}
	}

	return nil
}

// CreateDesktopShortcut creates a desktop or app entry on Linux/macOS
func CreateDesktopShortcut(exePath string) error {
	exePath = filepath.Clean(exePath)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if runtime.GOOS == "darwin" {
		// macOS: Symlink on Desktop if Desktop exists
		desktop := filepath.Join(home, "Desktop")
		if _, err := os.Stat(desktop); err == nil {
			link := filepath.Join(desktop, "Remote PC")
			_ = os.Remove(link)
			_ = os.Symlink(exePath, link)
			fmt.Printf("[Shortcut] ✓ Acceso directo creado en: %s\n", link)
		}
		return nil
	}

	// Linux .desktop entry
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Remote PC
Comment=Remote PC Host & Web Dashboard
Exec=%s
Terminal=false
Categories=Development;Utility;
`, exePath)

	appsDir := filepath.Join(home, ".local", "share", "applications")
	_ = os.MkdirAll(appsDir, 0755)
	appFile := filepath.Join(appsDir, "remotepc.desktop")
	_ = os.WriteFile(appFile, []byte(desktopContent), 0644)

	desktopDir := filepath.Join(home, "Desktop")
	if _, err := os.Stat(desktopDir); err == nil {
		deskFile := filepath.Join(desktopDir, "remotepc.desktop")
		_ = os.WriteFile(deskFile, []byte(desktopContent), 0755)
		fmt.Printf("[Shortcut] ✓ Acceso directo de escritorio creado en: %s\n", deskFile)
	}

	return nil
}

// RemoveDesktopShortcut removes the desktop launcher
func RemoveDesktopShortcut() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	if runtime.GOOS == "darwin" {
		link := filepath.Join(home, "Desktop", "Remote PC")
		_ = os.Remove(link)
		return
	}

	_ = os.Remove(filepath.Join(home, "Desktop", "remotepc.desktop"))
	_ = os.Remove(filepath.Join(home, ".local", "share", "applications", "remotepc.desktop"))
}
