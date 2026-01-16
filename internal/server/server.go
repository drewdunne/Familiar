package server

import (
	"net/http"

	"github.com/drewdunne/familiar/internal/config"
)

// Server is the HTTP server for Familiar.
type Server struct {
	cfg *config.Config
	mux *http.ServeMux
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
}

// handleHealth responds with server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
