package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/server"
	"github.com/joho/godotenv"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "version":
		fmt.Printf("familiar v%s\n", version)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: familiar <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve    Start the webhook server")
	fmt.Println("  version  Print version information")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "Path to config file")
	envFile := fs.String("env-file", "", "Path to .env file (optional)")
	fs.Parse(args)

	// Load .env file if specified or exists
	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			log.Printf("Warning: could not load env file %s: %v", *envFile, err)
		}
	} else {
		// Try default locations
		godotenv.Load(".env")
		godotenv.Load("/etc/familiar/familiar.env")
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and start server
	srv := server.New(cfg)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	log.Printf("Starting Familiar server on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
