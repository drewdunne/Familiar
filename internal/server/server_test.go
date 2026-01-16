package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drewdunne/familiar/internal/config"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("GET /health body = %q, want %q", rec.Body.String(), `{"status":"ok"}`)
	}
}
