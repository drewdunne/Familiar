# Phase 1: Project Skeleton + Webhook Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Establish project foundation with Go module, HTTP server, webhook endpoints with signature verification, config loading, and GitHub Actions CI.

**Architecture:** Standard Go project layout with `cmd/` for entrypoints and `internal/` for private packages. HTTP server using Go stdlib with middleware for signature verification. YAML config with environment variable substitution.

**Tech Stack:** Go 1.25, stdlib net/http, gopkg.in/yaml.v3, godotenv

---

## Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `go.sum`

**Step 1: Initialize Go module**

Run:
```bash
cd /home/drewdunne/repos/Familiar/.worktrees/phase-1
go mod init github.com/drewdunne/familiar
```

Expected: Creates `go.mod` with module name.

**Step 2: Verify module**

Run:
```bash
cat go.mod
```

Expected:
```
module github.com/drewdunne/familiar

go 1.25
```

**Step 3: Commit**

```bash
git add go.mod
git commit -m "chore: initialize Go module"
```

---

## Task 2: Create Main Entrypoint Structure

**Files:**
- Create: `cmd/familiar/main.go`

**Step 1: Create directory and main.go**

Create `cmd/familiar/main.go`:
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: familiar <command>")
		fmt.Println("Commands:")
		fmt.Println("  serve    Start the webhook server")
		fmt.Println("  version  Print version information")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Println("Starting server... (not implemented)")
	case "version":
		fmt.Println("familiar v0.1.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
```

**Step 2: Verify it compiles and runs**

Run:
```bash
go build -o familiar ./cmd/familiar
./familiar version
```

Expected: `familiar v0.1.0`

**Step 3: Commit**

```bash
git add cmd/
git commit -m "chore: add main entrypoint with CLI structure"
```

---

## Task 3: Config Package - Types and Loading (TDD)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing test for config loading**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

logging:
  dir: "/var/log/familiar"
  retention_days: 30
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Logging.Dir != "/var/log/familiar" {
		t.Errorf("Logging.Dir = %q, want %q", cfg.Logging.Dir, "/var/log/familiar")
	}
	if cfg.Logging.RetentionDays != 30 {
		t.Errorf("Logging.RetentionDays = %d, want %d", cfg.Logging.RetentionDays, 30)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file, got nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/... -v
```

Expected: FAIL - package doesn't exist yet.

**Step 3: Write minimal implementation**

Create `internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Dir           string `yaml:"dir"`
	RetentionDays int    `yaml:"retention_days"`
}

// Load reads and parses the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
```

**Step 4: Add yaml dependency**

Run:
```bash
go mod tidy
```

Expected: Adds `gopkg.in/yaml.v3` to go.mod.

**Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/... -v
```

Expected: PASS (2 tests)

**Step 6: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat(config): add config types and YAML loading"
```

---

## Task 4: Config - Environment Variable Substitution (TDD)

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing test for env var substitution**

Add to `internal/config/config_test.go`:
```go
func TestLoadConfig_EnvVarSubstitution(t *testing.T) {
	// Set env var for test
	os.Setenv("TEST_SECRET_TOKEN", "my-secret-value")
	defer os.Unsetenv("TEST_SECRET_TOKEN")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

providers:
  github:
    token: "${TEST_SECRET_TOKEN}"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Providers.GitHub.Token != "my-secret-value" {
		t.Errorf("Providers.GitHub.Token = %q, want %q", cfg.Providers.GitHub.Token, "my-secret-value")
	}
}

func TestLoadConfig_EnvVarNotSet(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("NONEXISTENT_VAR")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

providers:
  github:
    token: "${NONEXISTENT_VAR}"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should be empty string when env var not set
	if cfg.Providers.GitHub.Token != "" {
		t.Errorf("Providers.GitHub.Token = %q, want empty string", cfg.Providers.GitHub.Token)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/... -v
```

Expected: FAIL - Providers field doesn't exist.

**Step 3: Update implementation**

Update `internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Logging   LoggingConfig   `yaml:"logging"`
	Providers ProvidersConfig `yaml:"providers"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Dir           string `yaml:"dir"`
	RetentionDays int    `yaml:"retention_days"`
}

// ProvidersConfig holds git provider configurations.
type ProvidersConfig struct {
	GitHub GitHubConfig `yaml:"github"`
	GitLab GitLabConfig `yaml:"gitlab"`
}

// GitHubConfig holds GitHub-specific settings.
type GitHubConfig struct {
	AuthMethod    string `yaml:"auth_method"`
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// GitLabConfig holds GitLab-specific settings.
type GitLabConfig struct {
	AuthMethod    string `yaml:"auth_method"`
	Token         string `yaml:"token"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// envVarPattern matches ${VAR_NAME} patterns.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Substitute environment variables
	data = envVarPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := envVarPattern.FindSubmatch(match)[1]
		return []byte(os.Getenv(string(varName)))
	})

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/... -v
```

Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add env var substitution and provider configs"
```

---

## Task 5: Config - Default Values (TDD)

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing test for defaults**

Add to `internal/config/config_test.go`:
```go
func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config - should fill in defaults
	configContent := `
server:
  port: 9000
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Explicit value should be preserved
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9000)
	}

	// Default values should be applied
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Logging.RetentionDays != 30 {
		t.Errorf("Logging.RetentionDays = %d, want default %d", cfg.Logging.RetentionDays, 30)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/... -v -run TestLoadConfig_Defaults
```

Expected: FAIL - defaults not applied.

**Step 3: Update implementation**

Update `internal/config/config.go`, add `applyDefaults` function and call it in `Load`:
```go
// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Logging: LoggingConfig{
			Dir:           "/var/log/familiar",
			RetentionDays: 30,
		},
	}
}

// Load reads and parses the config file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Substitute environment variables
	data = envVarPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := envVarPattern.FindSubmatch(match)[1]
		return []byte(os.Getenv(string(varName)))
	})

	// Start with defaults
	cfg := DefaultConfig()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/... -v
```

Expected: PASS (5 tests)

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add default values"
```

---

## Task 6: HTTP Server Package - Basic Server (TDD)

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

**Step 1: Write failing test for server creation**

Create `internal/server/server_test.go`:
```go
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/server/... -v
```

Expected: FAIL - package doesn't exist.

**Step 3: Write minimal implementation**

Create `internal/server/server.go`:
```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/drewdunne/familiar/internal/config"
)

// Server is the HTTP server for Familiar.
type Server struct {
	cfg    *config.Config
	mux    *http.ServeMux
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
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/server/... -v
```

Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat(server): add basic HTTP server with health endpoint"
```

---

## Task 7: Webhook Handler - GitHub Signature Verification (TDD)

**Files:**
- Create: `internal/webhook/github.go`
- Create: `internal/webhook/github_test.go`

**Step 1: Write failing test for GitHub signature verification**

Create `internal/webhook/github_test.go`:
```go
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubHandler_ValidSignature(t *testing.T) {
	secret := "test-secret"
	payload := `{"action":"opened","number":1}`

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	handler := NewGitHubHandler(secret, func(event *GitHubEvent) error {
		if event.Action != "opened" {
			t.Errorf("event.Action = %q, want %q", event.Action, "opened")
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGitHubHandler_InvalidSignature(t *testing.T) {
	secret := "test-secret"
	payload := `{"action":"opened","number":1}`

	handler := NewGitHubHandler(secret, func(event *GitHubEvent) error {
		t.Error("handler should not be called with invalid signature")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGitHubHandler_MissingSignature(t *testing.T) {
	secret := "test-secret"
	payload := `{"action":"opened","number":1}`

	handler := NewGitHubHandler(secret, func(event *GitHubEvent) error {
		t.Error("handler should not be called with missing signature")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/webhook/... -v
```

Expected: FAIL - package doesn't exist.

**Step 3: Write minimal implementation**

Create `internal/webhook/github.go`:
```go
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// GitHubEvent represents a parsed GitHub webhook event.
type GitHubEvent struct {
	EventType string
	Action    string `json:"action"`
	Number    int    `json:"number"`
	RawPayload []byte
}

// GitHubEventHandler is called when a valid GitHub webhook is received.
type GitHubEventHandler func(event *GitHubEvent) error

// GitHubHandler handles GitHub webhook requests.
type GitHubHandler struct {
	secret  string
	handler GitHubEventHandler
}

// NewGitHubHandler creates a new GitHub webhook handler.
func NewGitHubHandler(secret string, handler GitHubEventHandler) *GitHubHandler {
	return &GitHubHandler{
		secret:  secret,
		handler: handler,
	}
}

// ServeHTTP implements http.Handler.
func (h *GitHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	if !h.verifySignature(body, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse event
	event := &GitHubEvent{
		EventType:  r.Header.Get("X-GitHub-Event"),
		RawPayload: body,
	}
	if err := json.Unmarshal(body, event); err != nil {
		http.Error(w, "failed to parse payload", http.StatusBadRequest)
		return
	}

	// Call handler
	if err := h.handler(event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// verifySignature verifies the GitHub webhook signature.
func (h *GitHubHandler) verifySignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/webhook/... -v
```

Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add internal/webhook/
git commit -m "feat(webhook): add GitHub webhook handler with signature verification"
```

---

## Task 8: Webhook Handler - GitLab Signature Verification (TDD)

**Files:**
- Create: `internal/webhook/gitlab.go`
- Create: `internal/webhook/gitlab_test.go`

**Step 1: Write failing test for GitLab signature verification**

Create `internal/webhook/gitlab_test.go`:
```go
package webhook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabHandler_ValidToken(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request","object_attributes":{"action":"open"}}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		if event.ObjectKind != "merge_request" {
			t.Errorf("event.ObjectKind = %q, want %q", event.ObjectKind, "merge_request")
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", secret)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestGitLabHandler_InvalidToken(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request"}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		t.Error("handler should not be called with invalid token")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Token", "wrong-token")
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGitLabHandler_MissingToken(t *testing.T) {
	secret := "test-secret-token"
	payload := `{"object_kind":"merge_request"}`

	handler := NewGitLabHandler(secret, func(event *GitLabEvent) error {
		t.Error("handler should not be called with missing token")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(payload))
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/webhook/... -v -run TestGitLab
```

Expected: FAIL - GitLabHandler doesn't exist.

**Step 3: Write minimal implementation**

Create `internal/webhook/gitlab.go`:
```go
package webhook

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
)

// GitLabEvent represents a parsed GitLab webhook event.
type GitLabEvent struct {
	EventType        string
	ObjectKind       string `json:"object_kind"`
	ObjectAttributes struct {
		Action string `json:"action"`
	} `json:"object_attributes"`
	RawPayload []byte
}

// GitLabEventHandler is called when a valid GitLab webhook is received.
type GitLabEventHandler func(event *GitLabEvent) error

// GitLabHandler handles GitLab webhook requests.
type GitLabHandler struct {
	secret  string
	handler GitLabEventHandler
}

// NewGitLabHandler creates a new GitLab webhook handler.
func NewGitLabHandler(secret string, handler GitLabEventHandler) *GitLabHandler {
	return &GitLabHandler{
		secret:  secret,
		handler: handler,
	}
}

// ServeHTTP implements http.Handler.
func (h *GitLabHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify token
	token := r.Header.Get("X-Gitlab-Token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(h.secret)) != 1 {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Parse event
	event := &GitLabEvent{
		EventType:  r.Header.Get("X-Gitlab-Event"),
		RawPayload: body,
	}
	if err := json.Unmarshal(body, event); err != nil {
		http.Error(w, "failed to parse payload", http.StatusBadRequest)
		return
	}

	// Call handler
	if err := h.handler(event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/webhook/... -v
```

Expected: PASS (6 tests)

**Step 5: Commit**

```bash
git add internal/webhook/
git commit -m "feat(webhook): add GitLab webhook handler with token verification"
```

---

## Task 9: Wire Webhook Handlers into Server

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Step 1: Write failing test for webhook routes**

Add to `internal/server/server_test.go`:
```go
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
```

Also add imports at top of test file:
```go
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/server/... -v -run TestServer_Webhook
```

Expected: FAIL - routes not wired up.

**Step 3: Update implementation**

Update `internal/server/server.go`:
```go
package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/webhook"
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
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/server/... -v
```

Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat(server): wire webhook handlers for GitHub and GitLab"
```

---

## Task 10: Update Main to Use Server

**Files:**
- Modify: `cmd/familiar/main.go`

**Step 1: Update main.go to start real server**

Update `cmd/familiar/main.go`:
```go
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
```

**Step 2: Add godotenv dependency**

Run:
```bash
go get github.com/joho/godotenv
go mod tidy
```

**Step 3: Verify it compiles**

Run:
```bash
go build -o familiar ./cmd/familiar
./familiar version
```

Expected: `familiar v0.1.0`

**Step 4: Commit**

```bash
git add cmd/familiar/ go.mod go.sum
git commit -m "feat(cli): wire serve command to config and server"
```

---

## Task 11: Add GitHub Actions CI

**Files:**
- Create: `.github/workflows/test.yml`

**Step 1: Create workflow file**

Create `.github/workflows/test.yml`:
```yaml
name: Test

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...

      - name: Check coverage
        run: |
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: ${coverage}%"
          if (( $(echo "$coverage < 80" | bc -l) )); then
            echo "Coverage ${coverage}% is below 80% threshold"
            exit 1
          fi

      - name: Build
        run: go build -o familiar ./cmd/familiar
```

**Step 2: Create directory**

Run:
```bash
mkdir -p .github/workflows
```

Then create the file above.

**Step 3: Commit**

```bash
git add .github/
git commit -m "ci: add GitHub Actions workflow with test and coverage"
```

---

## Task 12: Create Example Config

**Files:**
- Create: `config.example.yaml`

**Step 1: Create example config**

Create `config.example.yaml`:
```yaml
# Familiar Configuration
# Copy to config.yaml and customize

server:
  host: "0.0.0.0"
  port: 8080

logging:
  dir: "/var/log/familiar"
  retention_days: 30

concurrency:
  max_agents: 5
  queue_size: 20

agents:
  timeout_minutes: 30
  debounce_seconds: 10

repo_cache:
  dir: "/var/cache/familiar/repos"

providers:
  github:
    auth_method: "pat"
    token: "${GITHUB_TOKEN}"
    webhook_secret: "${GITHUB_WEBHOOK_SECRET}"
  gitlab:
    auth_method: "pat"
    token: "${GITLAB_TOKEN}"
    webhook_secret: "${GITLAB_WEBHOOK_SECRET}"

llm:
  strategy: "api"
  api:
    provider: "anthropic"
    model: "claude-sonnet-4-20250514"
    api_key: "${ANTHROPIC_API_KEY}"

# Default prompts per event type
prompts:
  mr_opened: |
    Review this merge request for bugs, security issues, and code quality.
    Provide actionable feedback as inline comments.

  mr_comment: |
    A user has commented on this merge request.
    Address their question or request directly.

  mr_updated: |
    New commits have been pushed to this merge request.
    Review the changes since your last review.

  mention: |
    You were mentioned in a comment.
    Follow the user's instructions precisely.

# Default permissions
permissions:
  merge: "never"
  approve: "never"
  push_commits: "on_request"
  dismiss_reviews: "never"

# Default enabled events
events:
  mr_opened: true
  mr_comment: true
  mr_updated: true
  mention: true
```

**Step 2: Commit**

```bash
git add config.example.yaml
git commit -m "docs: add example config file"
```

---

## Task 13: Update README with Setup Instructions

**Files:**
- Modify: `README.md`

**Step 1: Update README**

The README needs comprehensive setup instructions. Update the GitHub and GitLab setup sections with detailed steps. (Full content would be extensive - key sections to add include detailed PAT creation steps, webhook URL format, required scopes, and branch protection configuration steps for each provider.)

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add comprehensive setup instructions for GitHub and GitLab"
```

---

## Task 14: Run Full Test Suite and Verify Coverage

**Step 1: Run all tests with coverage**

Run:
```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Expected: All tests pass, coverage >= 80%

**Step 2: Fix any coverage gaps**

If coverage is below 80%, add tests for uncovered code paths.

**Step 3: Final commit if needed**

```bash
git add .
git commit -m "test: ensure 80% coverage threshold"
```

---

## Task 15: Create Pull Request

**Step 1: Push branch**

Run:
```bash
git push -u origin feature/phase-1-skeleton
```

**Step 2: Create PR**

Run:
```bash
gh pr create --title "Phase 1: Project skeleton with webhook server" --body "$(cat <<'EOF'
## Summary
- Initialize Go module and project structure
- Add config package with YAML loading, env var substitution, and defaults
- Add HTTP server with /health endpoint
- Add webhook handlers for GitHub and GitLab with signature verification
- Add GitHub Actions CI with 80% coverage requirement
- Add example config and comprehensive README setup instructions

## Test Plan
- [ ] `go test ./...` passes
- [ ] Coverage >= 80%
- [ ] `go build ./cmd/familiar` succeeds
- [ ] Manual test: `./familiar serve --config config.example.yaml` starts server
- [ ] Manual test: `/health` endpoint returns `{"status":"ok"}`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Summary

| Task | Component | Tests Added |
|------|-----------|-------------|
| 1 | Go module init | - |
| 2 | Main entrypoint | - |
| 3 | Config loading | 2 |
| 4 | Env var substitution | 2 |
| 5 | Config defaults | 1 |
| 6 | HTTP server + health | 2 |
| 7 | GitHub webhook handler | 3 |
| 8 | GitLab webhook handler | 3 |
| 9 | Wire webhooks to server | 2 |
| 10 | CLI serve command | - |
| 11 | GitHub Actions CI | - |
| 12 | Example config | - |
| 13 | README setup docs | - |
| 14 | Coverage verification | - |
| 15 | Pull request | - |

**Total: 15 tasks, ~15 tests, estimated 70-90 minutes**
