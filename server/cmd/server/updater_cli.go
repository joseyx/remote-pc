package main

import (
	"fmt"
	"os"

	"orbit/internal/updater"
)

func checkUpdateCLI() {
	fmt.Printf("Buscando actualizaciones para Orbit (versión actual: v%s)...\n", Version)
	info, err := updater.CheckUpdate(Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error verificando actualizaciones: %v\n", err)
		os.Exit(1)
	}

	if info.UpdateAvailable {
		fmt.Printf("\n✨ ¡Nueva versión disponible: v%s!\n", info.LatestVersion)
		if info.ReleaseName != "" && info.ReleaseName != "v"+info.LatestVersion {
			fmt.Printf("   %s\n", info.ReleaseName)
		}
		if info.Changelog != "" {
			fmt.Println("\nNotas de la versión:")
			fmt.Println(info.Changelog)
		}
		fmt.Println("\nPara actualizar a la última versión ejecuta:")
		fmt.Println("   orbit update")
	} else {
		fmt.Printf("✓ Orbit está al día (v%s es la versión más reciente).\n", Version)
	}
}

func updateCLI() {
	force := false
	for _, arg := range os.Args[2:] {
		if arg == "--force" || arg == "-f" {
			force = true
		}
	}

	fmt.Printf("Verificando actualizaciones para Orbit (versión actual: v%s)...\n", Version)
	info, err := updater.CheckUpdate(Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error verificando actualizaciones: %v\n", err)
		os.Exit(1)
	}

	if !info.UpdateAvailable && !force {
		fmt.Printf("✓ Orbit ya está en la versión más reciente (v%s).\n", Version)
		fmt.Println("Si deseas forzar la reinstalación, ejecuta: orbit update --force")
		return
	}

	fmt.Printf("Descargando e instalando Orbit v%s (%s)...\n", info.LatestVersion, info.AssetName)
	if err := updater.ApplyUpdate(info); err != nil {
		fmt.Fprintf(os.Stderr, "Error al actualizar Orbit: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ ¡Orbit se actualizó exitosamente a v%s!\n", info.LatestVersion)
}
