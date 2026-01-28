package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Parse the JSON response
	var health HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	// Check that the response has the expected structure
	if health.Status != "ok" && health.Status != "degraded" {
		t.Errorf("GET /health status = %q, want 'ok' or 'degraded'", health.Status)
	}

	if health.Checks == nil {
		t.Error("GET /health checks is nil, want non-nil")
	}

	// Verify docker check exists in response
	if _, ok := health.Checks["docker"]; !ok {
		t.Error("GET /health missing 'docker' in checks")
	}

	// Verify active_agents check exists in response
	if _, ok := health.Checks["active_agents"]; !ok {
		t.Error("GET /health missing 'active_agents' in checks")
	}
}

func TestServer_HealthEndpoint_ContentType(t *testing.T) {
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

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GET /health Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestServer_HealthEndpoint_DegradedStatus(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)
	// Simulate Docker being unavailable
	srv.dockerAvailable = false

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var health HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if health.Status != "degraded" {
		t.Errorf("GET /health status = %q, want 'degraded' when Docker unavailable", health.Status)
	}

	dockerCheck, ok := health.Checks["docker"].(bool)
	if !ok || dockerCheck {
		t.Error("GET /health docker check should be false when Docker unavailable")
	}
}

func TestServer_HealthEndpoint_OkStatus(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)
	// Simulate Docker being available
	srv.dockerAvailable = true

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	var health HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if health.Status != "ok" {
		t.Errorf("GET /health status = %q, want 'ok' when Docker available", health.Status)
	}

	dockerCheck, ok := health.Checks["docker"].(bool)
	if !ok || !dockerCheck {
		t.Error("GET /health docker check should be true when Docker available")
	}
}

func TestServer_WebhookGitHubEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{
				WebhookSecret: "test-secret",
			},
		},
	}

	srv := New(cfg)

	// Create valid signature
	payload := `{"action":"opened"}`
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /webhook/github status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestServer_WebhookGitLabEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Providers: config.ProvidersConfig{
			GitLab: config.GitLabConfig{
				WebhookSecret: "test-secret",
			},
		},
	}

	srv := New(cfg)

	payload := `{"object_kind":"merge_request"}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", "test-secret")
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /webhook/gitlab status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
