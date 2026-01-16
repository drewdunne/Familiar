# Phase 9: Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-8.

**Goal:** Production hardening with timeout enforcement, graceful shutdown, error recovery, and optional metrics.

**Tech Stack:** Go 1.25, existing packages

**Prerequisites:** Phases 1-8 complete

---

## Task 1: Agent Timeout Enforcement (TDD)

**Files:**
- Modify: `internal/agent/spawner.go`
- Create: `internal/agent/timeout_test.go`

**Step 1: Write failing test**

```go
func TestSpawner_Timeout(t *testing.T) {
	// Create spawner with short timeout
	spawner, _ := NewSpawner(SpawnerConfig{
		Image:          "alpine:latest",
		TimeoutMinutes: 1, // 1 minute for test
	})
	defer spawner.Close()

	// Track if timeout was triggered
	var timedOut bool
	spawner.OnTimeout = func(session *Session) {
		timedOut = true
	}

	// Spawn agent that would run forever
	session, _ := spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "timeout-test",
		WorktreePath: t.TempDir(),
		Prompt:       "sleep 3600", // 1 hour
	})

	// Wait for timeout (slightly longer than configured)
	time.Sleep(70 * time.Second)

	if !timedOut {
		t.Error("Timeout should have been triggered")
	}

	// Session should be cleaned up
	if _, ok := spawner.GetSession(session.ID); ok {
		t.Error("Session should be removed after timeout")
	}
}
```

**Step 2: Implement timeout**

Add to spawner:

```go
type SpawnerConfig struct {
	// ... existing fields
	TimeoutMinutes int
}

type Spawner struct {
	// ... existing fields
	OnTimeout func(*Session)
}

// startTimeoutWatcher starts a goroutine to enforce timeouts.
func (s *Spawner) startTimeoutWatcher() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.checkTimeouts()
		}
	}()
}

func (s *Spawner) checkTimeouts() {
	s.mu.Lock()
	defer s.mu.Unlock()

	timeout := time.Duration(s.cfg.TimeoutMinutes) * time.Minute
	now := time.Now()

	for id, session := range s.sessions {
		if now.Sub(session.StartedAt) > timeout {
			// Kill the container
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			s.client.StopContainer(ctx, session.ContainerID, 5)
			s.client.RemoveContainer(ctx, session.ContainerID, true)
			cancel()

			delete(s.sessions, id)

			if s.OnTimeout != nil {
				s.OnTimeout(session)
			}
		}
	}
}
```

**Commit**

---

## Task 2: Graceful Shutdown (TDD)

**Files:**
- Create: `internal/server/shutdown.go`
- Create: `internal/server/shutdown_test.go`

**Step 1: Write failing test**

```go
func TestGracefulShutdown(t *testing.T) {
	server := New(&config.Config{})

	// Start server in background
	go server.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
```

**Step 2: Implement graceful shutdown**

```go
package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func (s *Server) ListenAndServeWithShutdown() error {
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port),
		Handler: s.Handler(),
	}

	// Channel to signal shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start server
	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	log.Printf("Server started on %s", httpServer.Addr)

	// Wait for shutdown signal
	<-shutdown
	log.Println("Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop accepting new webhooks
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	// Wait for active agents to complete (or timeout)
	s.drainAgents(ctx)

	// Stop cleanup scheduler
	if s.cleanupScheduler != nil {
		s.cleanupScheduler.Stop()
	}

	log.Println("Shutdown complete")
	return nil
}

func (s *Server) drainAgents(ctx context.Context) {
	// Wait for agents to complete or context to expire
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown timeout, force stopping remaining agents")
			s.spawner.StopAll(context.Background())
			return
		case <-ticker.C:
			if s.spawner.ActiveCount() == 0 {
				return
			}
			log.Printf("Waiting for %d agents to complete...", s.spawner.ActiveCount())
		}
	}
}
```

**Commit**

---

## Task 3: Error Recovery (TDD)

**Files:**
- Create: `internal/agent/recovery.go`
- Create: `internal/agent/recovery_test.go`

Implement recovery for:
- Container dies unexpectedly (restart or cleanup)
- Git operations fail (retry with backoff)
- API rate limiting (backoff and retry)

**Step 1: Write failing test**

```go
func TestRecovery_ContainerDied(t *testing.T) {
	// Test that unexpected container death is handled
}

func TestRecovery_GitRetry(t *testing.T) {
	// Test that git operations retry on transient failure
}
```

**Step 2: Implement recovery**

```go
package agent

import (
	"context"
	"time"
)

// RecoveryConfig configures error recovery behavior.
type RecoveryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// WithRetry wraps a function with retry logic.
func WithRetry(ctx context.Context, cfg RecoveryConfig, fn func() error) error {
	var err error
	backoff := cfg.InitialBackoff

	for i := 0; i <= cfg.MaxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i == cfg.MaxRetries {
			break
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff
		backoff *= 2
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return err
}

// IsTransientError checks if an error is transient and should be retried.
func IsTransientError(err error) bool {
	// Check for network errors, rate limits, etc.
	// Implementation depends on specific error types
	return false
}
```

**Commit**

---

## Task 4: Health Check Enhancements

**Files:**
- Modify: `internal/server/server.go`

Add detailed health check:

```go
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "ok",
		"checks": map[string]interface{}{
			"docker":       s.checkDocker(),
			"active_agents": s.spawner.ActiveCount(),
			"queue_length":  s.manager.QueueLength(),
		},
	}

	// Determine overall status
	if !health["checks"].(map[string]interface{})["docker"].(bool) {
		health["status"] = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) checkDocker() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.dockerClient.Ping(ctx) == nil
}
```

**Commit**

---

## Task 5: Optional Metrics Endpoint

**Files:**
- Create: `internal/metrics/metrics.go`

```go
package metrics

import (
	"sync/atomic"
)

// Metrics tracks operational metrics.
type Metrics struct {
	AgentsSpawned   uint64
	AgentsCompleted uint64
	AgentsFailed    uint64
	AgentsTimedOut  uint64
	WebhooksReceived uint64
	WebhooksProcessed uint64
}

var global = &Metrics{}

func AgentSpawned()   { atomic.AddUint64(&global.AgentsSpawned, 1) }
func AgentCompleted() { atomic.AddUint64(&global.AgentsCompleted, 1) }
func AgentFailed()    { atomic.AddUint64(&global.AgentsFailed, 1) }
func AgentTimedOut()  { atomic.AddUint64(&global.AgentsTimedOut, 1) }
func WebhookReceived() { atomic.AddUint64(&global.WebhooksReceived, 1) }
func WebhookProcessed() { atomic.AddUint64(&global.WebhooksProcessed, 1) }

func Get() Metrics {
	return Metrics{
		AgentsSpawned:    atomic.LoadUint64(&global.AgentsSpawned),
		AgentsCompleted:  atomic.LoadUint64(&global.AgentsCompleted),
		AgentsFailed:     atomic.LoadUint64(&global.AgentsFailed),
		AgentsTimedOut:   atomic.LoadUint64(&global.AgentsTimedOut),
		WebhooksReceived: atomic.LoadUint64(&global.WebhooksReceived),
		WebhooksProcessed: atomic.LoadUint64(&global.WebhooksProcessed),
	}
}
```

Add `/metrics` endpoint to server (Prometheus format optional).

**Commit**

---

## Task 6: Run Full Test Suite

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Verify coverage >= 80%

---

## Task 7: Final Integration Test

Run E2E test with full setup:
1. Start Familiar
2. Send test webhook
3. Verify agent spawns
4. Verify logs captured
5. Graceful shutdown

**Commit any fixes**

---

## Summary

| Task | Component | Tests |
|------|-----------|-------|
| 1 | Timeout enforcement | 1 |
| 2 | Graceful shutdown | 1 |
| 3 | Error recovery | 2 |
| 4 | Health check | - |
| 5 | Metrics | - |
| 6 | Coverage | - |
| 7 | E2E test | 1 |

**Total: 7 tasks, ~5 tests**
