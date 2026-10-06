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
	"syscall"
	"time"

	"remotepc-server/internal/api"
	"remotepc-server/internal/config"
	"remotepc-server/internal/session"
)

func main() {
	openFlag := flag.Bool("open", false, "Open web dashboard in browser on start")
	flag.Parse()

	cfg := config.LoadConfig()
	manager := session.NewManager(cfg.MaxHistoryKB, "sessions_state.json")
	if err := manager.RestoreSessions(); err != nil {
		log.Printf("Warning: failed to restore previous sessions: %v", err)
	}
	server := api.NewServer(cfg, manager)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	dashUrl := fmt.Sprintf("http://localhost:%s/", cfg.Port)

	fmt.Println("=====================================================")
	fmt.Println("       Remote PC Daemon - Terminal & Claude Host     ")
	fmt.Println("=====================================================")
	fmt.Printf(" Listening on : %s\n", addr)
	fmt.Printf(" Dashboard UI : %s\n", dashUrl)
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

	if *openFlag {
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = exec.Command("cmd", "/c", "start", dashUrl).Start()
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
