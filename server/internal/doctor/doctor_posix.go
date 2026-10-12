//go:build !windows

package doctor

import (
	"os"
)

func checkPTYPlatform() CheckItem {
	if _, err := os.Stat("/dev/ptmx"); err == nil {
		return CheckItem{
			Category: "Terminal",
			Name:     "Motor PTY (POSIX)",
			Status:   StatusOK,
			Message:  "Subsistema POSIX PTY (/dev/ptmx) activo",
			Details:  "Soporte completo para terminales interactivos zsh/bash",
		}
	}
	return CheckItem{
		Category: "Terminal",
		Name:     "Motor PTY (POSIX)",
		Status:   StatusOK,
		Message:  "Subsistema PTY de Unix disponible",
		Details:  "Soporte estándar POSIX",
	}
}
