package server

import (
	"log"
	"net/http"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/webhook"
)

// Server is the HTTP server for Familiar.
type Server struct {
	cfg        *config.Config
	mux        *http.ServeMux
	httpServer *httpServer
}

// New creates a new Server with the given config.
func New(cfg *config.Config) *Server {
	s := &Server{
		cfg: cfg,
		mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// routes sets up the HTTP routes.
func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)

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
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// handleGitHubEvent processes a GitHub webhook event.
func (s *Server) handleGitHubEvent(event *webhook.GitHubEvent) error {
	log.Printf("Received GitHub event: %s, action: %s", event.EventType, event.Action)
	// TODO: Route to event processor in future phases
	return nil
}

// handleGitLabEvent processes a GitLab webhook event.
func (s *Server) handleGitLabEvent(event *webhook.GitLabEvent) error {
	log.Printf("Received GitLab event: %s, kind: %s", event.EventType, event.ObjectKind)
	// TODO: Route to event processor in future phases
	return nil
}
