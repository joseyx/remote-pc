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
	"strings"
	"syscall"
	"time"

	"orbit/internal/api"
	"orbit/internal/config"
	"orbit/internal/daemon"
	"orbit/internal/doctor"
	"orbit/internal/installer"
	"orbit/internal/mcp"
	"orbit/internal/migration"
	"orbit/internal/session"
	"orbit/internal/updater"
)

var Version = "1.3.0"

func main() {
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	updater.CleanOldBinaries(exePath)

	if len(os.Args) > 1 {
		subcommand := os.Args[1]
		switch subcommand {
		case "migrate":
			cfg := config.LoadConfig()
			summary, err := migration.MigrateData(cfg.DataDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error durante la migración: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(migration.FormatCLI(summary))
			return

		case "doctor":
			cfg := config.LoadConfig()
			report := doctor.RunDiagnostics(cfg, Version)
			fmt.Print(doctor.FormatCLI(report))
			return

		case "service", "daemon":
			serviceCLI(exePath)
			return

		case "token":
			tokenCLI()
			return

		case "pin":
			pinCLI()
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
	if sum, err := migration.AutoMigrate(cfg.DataDir); err == nil && sum != nil && sum.WorkspacesCount > 0 {
		log.Printf("[Migration] Auto-migración completada: %d workspaces configurados", sum.WorkspacesCount)
	}
	sessionsFile := filepath.Join(cfg.DataDir, "sessions_state.json")
	if _, err := os.Stat(sessionsFile); os.IsNotExist(err) {
		for _, fb := range []string{
			filepath.Join("data", "sessions_state.json"),
			filepath.Join("..", "data", "sessions_state.json"),
			"sessions_state.json",
		} {
			if _, errFb := os.Stat(fb); errFb == nil {
				sessionsFile = fb
				break
			}
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
	fmt.Println("  service [comando]  Gestiona el demonio de fondo (install, uninstall, start, stop, status)")
	fmt.Println("  token [comando]    Muestra o rota el token de autenticación (show, rotate, set)")
	fmt.Println("  pin [comando]      Gestiona el PIN de seguridad (status, set, remove)")
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

func serviceCLI(exePath string) {
	cfg := config.LoadConfig()

	action := "status"
	if len(os.Args) > 2 {
		action = strings.ToLower(os.Args[2])
	}

	switch action {
	case "install":
		fmt.Println("Registrando Orbit como servicio en segundo plano...")
		if err := daemon.Install(cfg, exePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error instalando servicio: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Orbit se ha instalado exitosamente para autoarranque con el sistema.")
		st := daemon.GetStatus(cfg, Version)
		fmt.Print(daemon.FormatCLI(st))

	case "uninstall", "remove":
		fmt.Println("Desinstalando servicio de Orbit...")
		if err := daemon.Uninstall(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error desinstalando servicio: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Servicio de Orbit desinstalado exitosamente.")

	case "start":
		fmt.Println("Iniciando Orbit Daemon en segundo plano...")
		if err := daemon.Start(cfg, exePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error iniciando servicio: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Orbit Daemon iniciado correctamente.")
		st := daemon.GetStatus(cfg, Version)
		fmt.Print(daemon.FormatCLI(st))

	case "stop":
		fmt.Println("Deteniendo Orbit Daemon...")
		if err := daemon.Stop(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error deteniendo servicio: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Orbit Daemon detenido.")

	case "restart":
		fmt.Println("Reiniciando Orbit Daemon...")
		if err := daemon.Restart(cfg, exePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error reiniciando servicio: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Orbit Daemon reiniciado.")
		st := daemon.GetStatus(cfg, Version)
		fmt.Print(daemon.FormatCLI(st))

	case "status":
		st := daemon.GetStatus(cfg, Version)
		fmt.Print(daemon.FormatCLI(st))

	case "help", "-h", "--help":
		printServiceUsage()

	default:
		fmt.Fprintf(os.Stderr, "Acción desconocida '%s'.\n\n", action)
		printServiceUsage()
		os.Exit(1)
	}
}

func printServiceUsage() {
	fmt.Println("Uso: orbit service [comando]")
	fmt.Println()
	fmt.Println("Comandos de servicio:")
	fmt.Println("  status      Muestra el estado del servicio y métricas de salud (por defecto)")
	fmt.Println("  install     Registra Orbit para autoarranque en segundo plano con el sistema")
	fmt.Println("  uninstall   Desregistra el servicio y detiene el proceso en segundo plano")
	fmt.Println("  start       Inicia el demonio en segundo plano silenciosamente")
	fmt.Println("  stop        Detiene el proceso en segundo plano")
	fmt.Println("  restart     Reinicia el proceso en segundo plano")
	fmt.Println("  help        Muestra esta ayuda")
}

func tokenCLI() {
	cfg := config.LoadConfig()

	action := "show"
	if len(os.Args) > 2 {
		action = strings.ToLower(os.Args[2])
	}

	switch action {
	case "show", "get", "status":
		fmt.Printf("Token de Autenticación de Orbit: %s\n", cfg.AuthToken)

	case "rotate":
		oldToken := cfg.AuthToken
		newToken := cfg.RotateToken("")
		fmt.Println("✓ Token rotado exitosamente.")
		fmt.Printf("  Token anterior : %s\n", oldToken)
		fmt.Printf("  Nuevo Token    : %s\n", newToken)
		fmt.Println("  Recuerda actualizar tu app Orbit Mobile escaneando el nuevo código QR.")

	case "set":
		if len(os.Args) < 4 || strings.TrimSpace(os.Args[3]) == "" {
			fmt.Println("Uso: orbit token set <nuevo-token>")
			os.Exit(1)
		}
		custom := strings.TrimSpace(os.Args[3])
		cfg.RotateToken(custom)
		fmt.Printf("✓ Token de autenticación establecido a: %s\n", custom)

	case "help", "-h", "--help":
		fmt.Println("Uso: orbit token [comando]")
		fmt.Println("  show              Muestra el token actual (por defecto)")
		fmt.Println("  rotate            Genera y guarda un nuevo token aleatorio seguro")
		fmt.Println("  set <token>       Configura un token personalizado específico")

	default:
		fmt.Printf("Token actual: %s\n", cfg.AuthToken)
		fmt.Println("Comandos: 'orbit token rotate' o 'orbit token set <token>'")
	}
}

func pinCLI() {
	cfg := config.LoadConfig()

	action := "status"
	if len(os.Args) > 2 {
		action = strings.ToLower(os.Args[2])
	}

	switch action {
	case "status", "show":
		if cfg.PIN != "" {
			fmt.Println("Estado del PIN de Seguridad: [✓ Configurado]")
		} else {
			fmt.Println("Estado del PIN de Seguridad: [! No configurado]")
			fmt.Println("Para configurar un PIN: 'orbit pin set <pin_4_a_8_digitos>'")
		}

	case "set":
		if len(os.Args) < 4 || strings.TrimSpace(os.Args[3]) == "" {
			fmt.Println("Uso: orbit pin set <nuevo_pin>")
			os.Exit(1)
		}
		newPin := strings.TrimSpace(os.Args[3])
		if len(newPin) < 4 || len(newPin) > 8 {
			fmt.Println("Error: El PIN debe tener entre 4 y 8 caracteres o dígitos.")
			os.Exit(1)
		}
		if err := cfg.SetPIN(newPin); err != nil {
			fmt.Fprintf(os.Stderr, "Error guardando PIN: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ PIN de seguridad establecido exitosamente.")

	case "remove", "delete":
		if cfg.PIN == "" {
			fmt.Println("No hay ningún PIN de seguridad configurado actualmente.")
			return
		}
		if err := cfg.RemovePIN(); err != nil {
			fmt.Fprintf(os.Stderr, "Error removiendo PIN: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ PIN de seguridad removido exitosamente.")

	case "help", "-h", "--help":
		fmt.Println("Uso: orbit pin [comando]")
		fmt.Println("  status            Muestra si hay un PIN de seguridad activo")
		fmt.Println("  set <pin>         Establece un nuevo PIN (4 a 8 caracteres)")
		fmt.Println("  remove            Elimina el PIN de seguridad configurado")

	default:
		fmt.Println("Comando desconocido. Usa 'orbit pin help'")
	}
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
