package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/logging"
	"github.com/drewdunne/familiar/internal/metrics"
)

// TestIntegration_FullServerLifecycle tests the complete server lifecycle:
// 1. Start server with graceful shutdown
// 2. Send webhook requests
// 3. Verify health and metrics endpoints
// 4. Verify logging works
// 5. Graceful shutdown
func TestIntegration_FullServerLifecycle(t *testing.T) {
	// Reset metrics for clean test
	metrics.Reset()

	// Create temp directory for logs
	logDir := t.TempDir()

	// Create server config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use random available port
		},
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{
				WebhookSecret: "test-secret-github",
			},
			GitLab: config.GitLabConfig{
				WebhookSecret: "test-secret-gitlab",
			},
		},
		Logging: config.LoggingConfig{
			Dir:           logDir,
			RetentionDays: 30,
		},
	}

	// Create and start server
	srv := New(cfg)

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServeWithShutdown()
	}()

	// Wait for server to be ready
	select {
	case <-srv.Ready():
		// Server is ready
	case err := <-serverErr:
		t.Fatalf("Server failed to start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to start within timeout")
	}

	addr := srv.Addr()
	if addr == "" {
		t.Fatal("Server address is empty")
	}
	baseURL := fmt.Sprintf("http://%s", addr)

	// Test 1: Health endpoint works
	t.Run("health_endpoint", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("Failed to get health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var health HealthResponse
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("Failed to decode health response: %v", err)
		}

		if health.Status != "ok" && health.Status != "degraded" {
			t.Errorf("Health status = %q, want ok or degraded", health.Status)
		}
	})

	// Test 2: Metrics endpoint works
	t.Run("metrics_endpoint_initial", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/metrics")
		if err != nil {
			t.Fatalf("Failed to get metrics: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Metrics status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var m metrics.Metrics
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			t.Fatalf("Failed to decode metrics response: %v", err)
		}
	})

	// Test 3: Send GitHub webhook
	t.Run("github_webhook", func(t *testing.T) {
		payload := `{"action":"opened","number":42}`
		signature := signPayload(payload, "test-secret-github")

		req, err := http.NewRequest(http.MethodPost, baseURL+"/webhook/github", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("X-Hub-Signature-256", signature)
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to send webhook: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GitHub webhook status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	// Test 4: Send GitLab webhook
	t.Run("gitlab_webhook", func(t *testing.T) {
		payload := `{"object_kind":"merge_request"}`

		req, err := http.NewRequest(http.MethodPost, baseURL+"/webhook/gitlab", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("X-Gitlab-Token", "test-secret-gitlab")
		req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to send webhook: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GitLab webhook status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	// Test 5: Verify logging system
	t.Run("logging_system", func(t *testing.T) {
		writer := logging.NewWriter(logDir)

		entry := logging.LogEntry{
			AgentID:   "test-agent-1",
			RepoOwner: "testowner",
			RepoName:  "testrepo",
			MRNumber:  123,
			EventType: "mr_opened",
			Timestamp: time.Now(),
		}

		logPath, err := writer.Create(entry)
		if err != nil {
			t.Fatalf("Failed to create log: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("Log file was not created")
		}

		// Verify we can append to it
		if err := writer.Append(logPath, []byte("test log content\n")); err != nil {
			t.Errorf("Failed to append to log: %v", err)
		}

		// Verify content was written
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read log: %v", err)
		}
		if !strings.Contains(string(content), "test log content") {
			t.Error("Log content not found")
		}
	})

	// Test 6: Graceful shutdown
	t.Run("graceful_shutdown", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown error: %v", err)
		}

		// Verify server is no longer accepting connections
		_, err := http.Get(baseURL + "/health")
		if err == nil {
			t.Error("Server still accepting connections after shutdown")
		}
	})

	// Wait for server goroutine to complete
	select {
	case err := <-serverErr:
		if err != nil {
			t.Errorf("Server returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Server goroutine did not complete within timeout")
	}
}

// TestIntegration_LogCleanup tests the log cleanup functionality.
func TestIntegration_LogCleanup(t *testing.T) {
	logDir := t.TempDir()

	// Create log writer and cleaner
	writer := logging.NewWriter(logDir)
	cleaner := logging.NewCleaner(logDir, 1) // 1 day retention

	// Create a log file with old timestamp
	entry := logging.LogEntry{
		AgentID:   "old-agent",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  1,
		EventType: "mr_opened",
		Timestamp: time.Now().AddDate(0, 0, -2), // 2 days ago
	}

	logPath, err := writer.Create(entry)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}

	// Set modification time to 2 days ago
	oldTime := time.Now().AddDate(0, 0, -2)
	if err := os.Chtimes(logPath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	// Create a recent log file
	recentEntry := logging.LogEntry{
		AgentID:   "recent-agent",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  2,
		EventType: "mr_opened",
		Timestamp: time.Now(),
	}

	recentLogPath, err := writer.Create(recentEntry)
	if err != nil {
		t.Fatalf("Failed to create recent log: %v", err)
	}

	// Run cleanup
	if _, err := cleaner.Cleanup(); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify old log is deleted
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("Old log file should have been deleted")
	}

	// Verify recent log still exists
	if _, err := os.Stat(recentLogPath); os.IsNotExist(err) {
		t.Error("Recent log file should still exist")
	}
}

// TestIntegration_ServerRecovery tests that the server handles errors gracefully.
func TestIntegration_ServerRecovery(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0,
		},
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{
				WebhookSecret: "test-secret",
			},
		},
	}

	srv := New(cfg)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServeWithShutdown()
	}()

	select {
	case <-srv.Ready():
	case err := <-serverErr:
		t.Fatalf("Server failed to start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to start within timeout")
	}

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Send invalid webhook (missing signature)
	t.Run("invalid_webhook_rejected", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/webhook/github", strings.NewReader(`{}`))
		req.Header.Set("X-GitHub-Event", "pull_request")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Invalid webhook status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	// Verify server still healthy after bad request
	t.Run("server_healthy_after_bad_request", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	select {
	case <-serverErr:
	case <-time.After(5 * time.Second):
		t.Error("Server goroutine did not complete")
	}
}

// TestIntegration_MetricsAccumulation tests that metrics accumulate correctly.
func TestIntegration_MetricsAccumulation(t *testing.T) {
	metrics.Reset()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0,
		},
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{
				WebhookSecret: "test-secret",
			},
		},
	}

	srv := New(cfg)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServeWithShutdown()
	}()

	select {
	case <-srv.Ready():
	case err := <-serverErr:
		t.Fatalf("Server failed to start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to start within timeout")
	}

	baseURL := fmt.Sprintf("http://%s", srv.Addr())

	// Send multiple webhooks
	for i := 0; i < 3; i++ {
		payload := fmt.Sprintf(`{"action":"opened","number":%d}`, i)
		signature := signPayload(payload, "test-secret")

		req, _ := http.NewRequest(http.MethodPost, baseURL+"/webhook/github", strings.NewReader(payload))
		req.Header.Set("X-Hub-Signature-256", signature)
		req.Header.Set("X-GitHub-Event", "pull_request")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// Simulate agent metrics
	metrics.AgentSpawned()
	metrics.AgentSpawned()
	metrics.AgentCompleted()
	metrics.AgentFailed()

	// Check metrics endpoint
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	var m metrics.Metrics
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("Failed to decode metrics: %v", err)
	}

	if m.AgentsSpawned != 2 {
		t.Errorf("AgentsSpawned = %d, want 2", m.AgentsSpawned)
	}
	if m.AgentsCompleted != 1 {
		t.Errorf("AgentsCompleted = %d, want 1", m.AgentsCompleted)
	}
	if m.AgentsFailed != 1 {
		t.Errorf("AgentsFailed = %d, want 1", m.AgentsFailed)
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	select {
	case <-serverErr:
	case <-time.After(5 * time.Second):
	}
}

// TestIntegration_LogDirectory tests that log directories are created correctly.
func TestIntegration_LogDirectory(t *testing.T) {
	logDir := t.TempDir()
	writer := logging.NewWriter(logDir)

	entries := []logging.LogEntry{
		{
			AgentID:   "agent-1",
			RepoOwner: "org1",
			RepoName:  "repo1",
			MRNumber:  1,
			EventType: "mr_opened",
			Timestamp: time.Now(),
		},
		{
			AgentID:   "agent-2",
			RepoOwner: "org1",
			RepoName:  "repo2",
			MRNumber:  2,
			EventType: "mr_comment",
			Timestamp: time.Now(),
		},
		{
			AgentID:   "agent-3",
			RepoOwner: "org2",
			RepoName:  "repo1",
			MRNumber:  1,
			EventType: "mention",
			Timestamp: time.Now(),
		},
	}

	for _, entry := range entries {
		logPath, err := writer.Create(entry)
		if err != nil {
			t.Fatalf("Failed to create log for %s: %v", entry.AgentID, err)
		}

		// Verify directory structure
		expectedDir := filepath.Join(logDir, entry.RepoOwner, entry.RepoName, fmt.Sprint(entry.MRNumber))
		actualDir := filepath.Dir(logPath)
		if actualDir != expectedDir {
			t.Errorf("Log directory = %q, want %q", actualDir, expectedDir)
		}

		// Verify file exists
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Errorf("Log file not created: %s", logPath)
		}
	}
}

// signPayload creates a GitHub-style HMAC signature for the payload.
func signPayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
