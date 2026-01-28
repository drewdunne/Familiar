package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"sync"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/metrics"
	"github.com/drewdunne/familiar/internal/webhook"
)

// HealthResponse represents the health check response structure.
type HealthResponse struct {
	Status string                 `json:"status"`
	Checks map[string]interface{} `json:"checks"`
}

// Server is the HTTP server for Familiar.
type Server struct {
	cfg             *config.Config
	mux             *http.ServeMux
	httpServer      *httpServer
	httpServerMu    sync.RWMutex  // protects httpServer pointer
	ready           chan struct{} // closed when server is ready to accept connections
	dockerAvailable bool
	eventRouter     *event.Router
}

// New creates a new Server with the given config.
func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:             cfg,
		mux:             http.NewServeMux(),
		ready:           make(chan struct{}),
		dockerAvailable: checkDockerAvailable(),
	}
	s.routes()
	return s
}

// NewWithRouter creates a new Server with an injected event router.
// This allows dependency injection for testing and custom event handling.
func NewWithRouter(cfg *config.Config, router *event.Router) *Server {
	s := &Server{
		cfg:             cfg,
		mux:             http.NewServeMux(),
		ready:           make(chan struct{}),
		dockerAvailable: checkDockerAvailable(),
		eventRouter:     router,
	}
	s.routes()
	return s
}

// Ready returns a channel that is closed when the server is ready to accept connections.
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// checkDockerAvailable checks if Docker is available on the system.
func checkDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	return err == nil
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// routes sets up the HTTP routes.
func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/metrics", s.handleMetrics)

	// GitHub webhook
	if s.cfg.Providers.GitHub.WebhookSecret != "" {
		githubHandler := webhook.NewGitHubHandler(
			s.cfg.Providers.GitHub.WebhookSecret,
			s.handleGitHubEvent,
		)
		s.mux.Handle("/webhook/github", githubHandler)
	}

	// GitLab webhook
	if s.cfg.Providers.GitLab.WebhookSecret != "" {
		gitlabHandler := webhook.NewGitLabHandler(
			s.cfg.Providers.GitLab.WebhookSecret,
			s.handleGitLabEvent,
		)
		s.mux.Handle("/webhook/gitlab", gitlabHandler)
	}
}

// handleHealth responds with server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	checks := map[string]interface{}{
		"docker":        s.dockerAvailable,
		"active_agents": 0, // Would come from spawner if available
	}

	status := "ok"
	if !s.dockerAvailable {
		status = "degraded"
	}

	health := HealthResponse{
		Status: status,
		Checks: checks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleGitHubEvent processes a GitHub webhook event.
func (s *Server) handleGitHubEvent(event *webhook.GitHubEvent) error {
	log.Printf("Received GitHub event: %s, action: %s", event.EventType, event.Action)
	// TODO: Route to event processor in future phases
	return nil
}

// handleGitLabEvent processes a GitLab webhook event.
func (s *Server) handleGitLabEvent(glEvent *webhook.GitLabEvent) error {
	log.Printf("Received GitLab event: %s, kind: %s", glEvent.EventType, glEvent.ObjectKind)

	// If no router configured, just log and return (backwards compatible)
	if s.eventRouter == nil {
		return nil
	}

	// Normalize the webhook event
	normalizedEvent, err := event.NormalizeGitLabEvent(glEvent)
	if err != nil {
		log.Printf("Failed to normalize GitLab event: %v", err)
		return nil // Don't fail the webhook, just log
	}

	// Route the event
	if err := s.eventRouter.Route(context.Background(), normalizedEvent); err != nil {
		log.Printf("Failed to route event: %v", err)
		return nil // Don't fail the webhook, just log
	}

	return nil
}

// handleMetrics responds with current operational metrics.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	m := metrics.Get()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}
