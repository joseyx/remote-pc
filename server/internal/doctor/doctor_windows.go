//go:build windows

package doctor

import (
	"golang.org/x/sys/windows"
)

var (
	modkernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procCreatePseudoConsole = modkernel32.NewProc("CreatePseudoConsole")
)

func checkPTYPlatform() CheckItem {
	if procCreatePseudoConsole.Find() == nil {
		return CheckItem{
			Category: "Terminal",
			Name:     "Motor PTY (Pseudo-Console)",
			Status:   StatusOK,
			Message:  "Windows ConPTY API nativo disponible y activo",
			Details:  "Permite emulación ANSI/VT completa y resize fluido en Windows",
		}
	}
	return CheckItem{
		Category:    "Terminal",
		Name:        "Motor PTY (Pseudo-Console)",
		Status:      StatusWarning,
		Message:     "Windows ConPTY API no detectado en kernel32.dll",
		Details:     "Requiere Windows 10 build 17763 (v1809) o superior",
		Remediation: "Actualiza tu sistema a una versión reciente de Windows 10 u 11",
	}
}
