package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
	"github.com/drewdunne/familiar/internal/metrics"
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

func TestServer_MetricsEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)

	// Reset metrics to a known state
	metrics.Reset()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /metrics status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Parse the JSON response
	var m metrics.Metrics
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("Failed to parse metrics response: %v", err)
	}

	// All metrics should be zero after reset
	if m.AgentsSpawned != 0 {
		t.Errorf("AgentsSpawned = %d, want 0", m.AgentsSpawned)
	}
	if m.AgentsCompleted != 0 {
		t.Errorf("AgentsCompleted = %d, want 0", m.AgentsCompleted)
	}
	if m.AgentsFailed != 0 {
		t.Errorf("AgentsFailed = %d, want 0", m.AgentsFailed)
	}
	if m.AgentsTimedOut != 0 {
		t.Errorf("AgentsTimedOut = %d, want 0", m.AgentsTimedOut)
	}
	if m.WebhooksReceived != 0 {
		t.Errorf("WebhooksReceived = %d, want 0", m.WebhooksReceived)
	}
	if m.WebhooksProcessed != 0 {
		t.Errorf("WebhooksProcessed = %d, want 0", m.WebhooksProcessed)
	}
}

func TestServer_MetricsEndpoint_ContentType(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GET /metrics Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestServer_MetricsEndpoint_ReflectsIncrements(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	srv := New(cfg)

	// Reset and set specific metrics
	metrics.Reset()
	metrics.AgentSpawned()
	metrics.AgentSpawned()
	metrics.AgentCompleted()
	metrics.WebhookReceived()
	metrics.WebhookReceived()
	metrics.WebhookReceived()
	metrics.WebhookProcessed()
	metrics.WebhookProcessed()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	var m metrics.Metrics
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("Failed to parse metrics response: %v", err)
	}

	if m.AgentsSpawned != 2 {
		t.Errorf("AgentsSpawned = %d, want 2", m.AgentsSpawned)
	}
	if m.AgentsCompleted != 1 {
		t.Errorf("AgentsCompleted = %d, want 1", m.AgentsCompleted)
	}
	if m.WebhooksReceived != 3 {
		t.Errorf("WebhooksReceived = %d, want 3", m.WebhooksReceived)
	}
	if m.WebhooksProcessed != 2 {
		t.Errorf("WebhooksProcessed = %d, want 2", m.WebhooksProcessed)
	}
}

func TestServer_GitLabWebhook_RoutesToEventHandler(t *testing.T) {
	// Track whether the handler was called
	handlerCalled := false
	var receivedEvent *event.Event

	// Create a mock handler that records calls
	mockHandler := func(ctx context.Context, evt *event.Event, cfg *config.MergedConfig, intent *intent.ParsedIntent) error {
		handlerCalled = true
		receivedEvent = evt
		return nil
	}

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
		Events: config.ServerEventsConfig{
			MRComment: true,
		},
		Agents: config.AgentsConfig{
			DebounceSeconds: 0, // Disable debounce for testing
		},
	}

	// Create router with mock handler
	router := event.NewRouter(cfg, mockHandler, nil)

	// Create server with injected router
	srv := NewWithRouter(cfg, router)

	// Send a valid GitLab Note Hook webhook
	payload := `{
		"object_kind": "note",
		"object_attributes": {
			"id": 123,
			"note": "Please fix this bug",
			"noteable_type": "MergeRequest"
		},
		"merge_request": {
			"iid": 42
		},
		"project": {
			"path_with_namespace": "myorg/myrepo",
			"git_http_url": "https://gitlab.com/myorg/myrepo.git"
		},
		"user": {
			"username": "reviewer"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", "test-secret")
	req.Header.Set("X-Gitlab-Event", "Note Hook")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /webhook/gitlab status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify handler was called
	if !handlerCalled {
		t.Fatal("Event handler was not called - routing is not wired up")
	}

	// Verify event was normalized correctly
	if receivedEvent == nil {
		t.Fatal("Received event is nil")
	}
	if receivedEvent.Type != event.TypeMRComment {
		t.Errorf("Event type = %s, want %s", receivedEvent.Type, event.TypeMRComment)
	}
	if receivedEvent.MRNumber != 42 {
		t.Errorf("MR number = %d, want 42", receivedEvent.MRNumber)
	}
	if receivedEvent.RepoOwner != "myorg" {
		t.Errorf("RepoOwner = %s, want myorg", receivedEvent.RepoOwner)
	}
	if receivedEvent.RepoName != "myrepo" {
		t.Errorf("RepoName = %s, want myrepo", receivedEvent.RepoName)
	}
	if receivedEvent.CommentBody != "Please fix this bug" {
		t.Errorf("CommentBody = %s, want 'Please fix this bug'", receivedEvent.CommentBody)
	}
}
