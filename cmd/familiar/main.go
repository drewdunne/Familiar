package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/drewdunne/familiar/internal/agent"
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/handler"
	"github.com/drewdunne/familiar/internal/registry"
	"github.com/drewdunne/familiar/internal/repocache"
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

	// Create repo cache
	var repoCache *repocache.Cache
	if cfg.RepoCache.HostDir != "" {
		// Running in container with separate host/container paths
		repoCache = repocache.NewWithHostDir(cfg.RepoCache.Dir, cfg.RepoCache.HostDir)
	} else {
		// Running directly on host
		repoCache = repocache.New(cfg.RepoCache.Dir)
	}

	// Create provider registry
	reg := registry.New(cfg)

	// Create agent spawner
	spawner, err := agent.NewSpawner(agent.SpawnerConfig{
		Image:          cfg.Agents.Image,
		ClaudeAuthDir:  cfg.Agents.ClaudeAuthDir,
		MaxAgents:      cfg.Concurrency.MaxAgents,
		TimeoutMinutes: cfg.Agents.TimeoutMinutes,
	})
	if err != nil {
		log.Fatalf("Failed to create agent spawner: %v", err)
	}
	defer spawner.Close()

	// Create agent handler
	agentHandler := handler.NewAgentHandler(spawner, repoCache, reg, cfg.Logging.Dir, cfg.Logging.HostDir)

	// Create event router
	router := event.NewRouter(cfg, agentHandler.Handle, nil)

	// Create and start server with router
	srv := server.NewWithRouter(cfg, router)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	log.Printf("Starting Familiar server on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
