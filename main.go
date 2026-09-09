package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dembaca/prox-mds/internal/config"
	"github.com/dembaca/prox-mds/internal/server"
)

// Version is set via ldflags during build
var Version = "dev"

func main() {
	configPath := flag.String("config", "", "Path to configuration file (if not provided, defaults are used)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	log.Printf("prox-mds version %s", Version)

	// Load configuration
	var cfg *config.Config
	var err error
	if *configPath == "" {
		cfg = config.DefaultConfig()
		log.Println("No config file specified, using default configuration")
	} else {
		cfg, err = config.Load(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Dump loaded config if debug logging is enabled
	if *debug {
		configJSON, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal config: %v", err)
		} else {
			log.Printf("Loaded config:\n%s", string(configJSON))
		}
	}

	// Create and start server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown and config reload
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for signals: SIGTERM/SIGINT for shutdown, SIGHUP for VM config reload
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.MDS.ListenAddr)
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Handle signals
	for sig := range sigChan {
		switch sig {
		case syscall.SIGHUP:
			// Reload VM config cache
			log.Println("Received SIGHUP, reloading VM config cache...")
			if err := server.RefreshActiveVMConfigCache(); err != nil {
				log.Printf("Error reloading VM config cache: %v", err)
			} else {
				log.Println("VM config cache reloaded successfully")
			}
		case os.Interrupt, syscall.SIGTERM:
			// Graceful shutdown
			log.Println("Shutting down server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
			defer shutdownCancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Printf("Error during shutdown: %v", err)
			} else {
				log.Println("Server stopped gracefully")
			}
			return
		}
	}
}
