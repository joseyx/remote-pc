package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"orbit/internal/api"
	"orbit/internal/config"
	"orbit/internal/doctor"
	"orbit/internal/installer"
	"orbit/internal/mcp"
	"orbit/internal/session"
	"orbit/internal/updater"
)

var Version = "1.0.0"

func main() {
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	updater.CleanOldBinaries(exePath)

	if len(os.Args) > 1 {
		subcommand := os.Args[1]
		switch subcommand {
		case "doctor":
			cfg := config.LoadConfig()
			report := doctor.RunDiagnostics(cfg, Version)
			fmt.Print(doctor.FormatCLI(report))
			return

		case "check-update":
			checkUpdateCLI()
			return

		case "update":
			updateCLI()
			return

		case "mcp":
			// Crucial: Preserve caller's current working directory for git/bubble auto-detection!
			if err := mcp.RunServer(); err != nil {
				fmt.Fprintf(os.Stderr, "MCP Server error: %v\n", err)
				os.Exit(1)
			}
			return

		case "setup-mcp":
			if err := mcp.SetupMCP(exePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error configurando MCP: %v\n", err)
				os.Exit(1)
			}
			return

		case "uninstall-mcp":
			if err := mcp.UninstallMCP(); err != nil {
				fmt.Fprintf(os.Stderr, "Error desinstalando MCP: %v\n", err)
				os.Exit(1)
			}
			return

		case "add-path":
			if err := installer.AddToUserPath(filepath.Dir(exePath)); err != nil {
				fmt.Fprintf(os.Stderr, "Error agregando al PATH: %v\n", err)
				os.Exit(1)
			}
			return

		case "remove-path":
			if err := installer.RemoveFromUserPath(filepath.Dir(exePath)); err != nil {
				fmt.Fprintf(os.Stderr, "Error removiendo del PATH: %v\n", err)
				os.Exit(1)
			}
			return

		case "install":
			if err := installer.Install(exePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error instalando Orbit: %v\n", err)
				os.Exit(1)
			}
			return

		case "uninstall":
			if err := installer.Uninstall(exePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error desinstalando Orbit: %v\n", err)
				os.Exit(1)
			}
			return

		case "version", "-v", "--version":
			fmt.Printf("Orbit v%s - Developer Cockpit & MCP Orchestrator\n", Version)
			return

		case "help", "-h", "--help":
			printUsage()
			return
		}
	}

	// Ensure working directory is the executable's directory for normal server daemon run
	exeDir := filepath.Dir(exePath)
	_ = os.Chdir(exeDir)

	openFlag := flag.Bool("open", true, "Open web dashboard in browser on start (default: true)")
	noOpenFlag := flag.Bool("no-open", false, "Do not open web dashboard in browser on start")
	flag.Parse()

	shouldOpen := *openFlag && !*noOpenFlag

	cfg := config.LoadConfig()
	sessionsFile := filepath.Join(cfg.DataDir, "sessions_state.json")
	if _, err := os.Stat(sessionsFile); os.IsNotExist(err) {
		if _, err := os.Stat("sessions_state.json"); err == nil {
			sessionsFile = "sessions_state.json"
		}
	}
	manager := session.NewManager(cfg.MaxHistoryKB, sessionsFile)
	if err := manager.RestoreSessions(); err != nil {
		log.Printf("Warning: failed to restore previous sessions: %v", err)
	}
	server := api.NewServer(cfg, manager, Version)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	dashUrl := fmt.Sprintf("http://localhost:%s/", cfg.Port)

	fmt.Println("=====================================================")
	fmt.Println("       Orbit Daemon - Developer Cockpit & MCP Host   ")
	fmt.Println("=====================================================")
	fmt.Printf(" Listening on : %s\n", addr)
	fmt.Printf(" Cockpit UI   : %s\n", dashUrl)
	fmt.Printf(" Auth Token   : %s\n", cfg.AuthToken)
	fmt.Println("-----------------------------------------------------")
	fmt.Println(" Connect from Android with:")
	fmt.Printf("   Host/IP : <Your PC LAN or Tailscale IP>\n")
	fmt.Printf("   Port    : %s\n", cfg.Port)
	fmt.Printf("   Token   : %s\n", cfg.AuthToken)
	fmt.Println("=====================================================")

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Routes(),
	}

	if shouldOpen {
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = openBrowser(dashUrl)
		}()
	}

	// Listen for OS interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Persist active terminal sessions state for the next run
	if err := manager.SaveState(); err != nil {
		log.Printf("Error saving session state: %v", err)
	} else {
		fmt.Println("Saved active terminal sessions state to disk.")
	}
	manager.CloseAllPTYs()

	_ = httpServer.Shutdown(ctx)
	fmt.Println("Server stopped.")
}

func printUsage() {
	fmt.Println("Orbit - Developer Cockpit & MCP Suite")
	fmt.Println()
	fmt.Println("Uso:")
	fmt.Println("  orbit [comando] [opciones]")
	fmt.Println("  (alias retrocompatible: remotepc-server [comando] [opciones])")
	fmt.Println()
	fmt.Println("Comandos disponibles:")
	fmt.Println("  (sin comando)      Inicia el servidor daemon y el Cockpit web")
	fmt.Println("  doctor             Ejecuta un diagnóstico completo del entorno e integraciones")
	fmt.Println("  check-update       Verifica si existe una versión más reciente de Orbit")
	fmt.Println("  update [--force]   Actualiza Orbit automáticamente a la última versión")
	fmt.Println("  mcp                Ejecuta el servidor MCP JSON-RPC 2.0 por stdio (para Claude)")
	fmt.Println("  setup-mcp          Configura el servidor MCP en Claude Code y Claude Desktop")
	fmt.Println("  uninstall-mcp      Remueve la configuración MCP de Claude Code y Claude Desktop")
	fmt.Println("  add-path           Agrega el directorio del ejecutable al PATH del usuario")
	fmt.Println("  remove-path        Remueve el directorio del ejecutable del PATH del usuario")
	fmt.Println("  install            Instala Orbit en el PATH del usuario y configura el MCP")
	fmt.Println("  uninstall          Desinstala Orbit del PATH y remueve la configuración MCP")
	fmt.Println("  version            Muestra la versión de Orbit")
	fmt.Println("  help               Muestra esta ayuda")
	fmt.Println()
	fmt.Println("Opciones del servidor:")
	fmt.Println("  -open              Abre el Cockpit web en el navegador al iniciar (por defecto: true)")
	fmt.Println("  -no-open           No abre el Cockpit web automáticamente")
	fmt.Println("  -port <puerto>     Puerto HTTP a utilizar (por defecto: 8080)")
	fmt.Println("  -token <token>     Token de autenticación Bearer")
	fmt.Println("  -host <ip>         Dirección IP para escuchar (por defecto: 0.0.0.0)")
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, bsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
