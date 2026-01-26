package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

	if rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("GET /health body = %q, want %q", rec.Body.String(), `{"status":"ok"}`)
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
