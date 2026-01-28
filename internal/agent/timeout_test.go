package agent

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSpawnerConfig_TimeoutMinutes(t *testing.T) {
	cfg := SpawnerConfig{
		Image:          "alpine:latest",
		TimeoutMinutes: 30,
	}

	if cfg.TimeoutMinutes != 30 {
		t.Errorf("TimeoutMinutes = %d, want 30", cfg.TimeoutMinutes)
	}
}

func TestSpawner_OnTimeoutCallback(t *testing.T) {
	// This test verifies the OnTimeout callback field exists and can be set
	var callbackCalled bool
	callback := func(s *Session) {
		callbackCalled = true
	}

	spawner := &Spawner{
		OnTimeout: callback,
		sessions:  make(map[string]*Session),
	}

	// Simulate calling the callback
	if spawner.OnTimeout != nil {
		spawner.OnTimeout(&Session{ID: "test"})
	}

	if !callbackCalled {
		t.Error("OnTimeout callback was not invoked")
	}
}

func TestSpawner_CheckTimeouts_ExpiredSession(t *testing.T) {
	// Create a spawner with a short timeout
	spawner := &Spawner{
		cfg: SpawnerConfig{
			TimeoutMinutes: 1, // 1 minute timeout
		},
		sessions: make(map[string]*Session),
	}

	var timedOutSessions []*Session
	var mu sync.Mutex
	callbackDone := make(chan struct{}, 1)
	spawner.OnTimeout = func(s *Session) {
		mu.Lock()
		timedOutSessions = append(timedOutSessions, s)
		mu.Unlock()
		callbackDone <- struct{}{}
	}

	// Add an expired session (started 2 minutes ago)
	expiredSession := &Session{
		ID:        "expired-session",
		StartedAt: time.Now().Add(-2 * time.Minute),
		Status:    "running",
	}
	spawner.sessions["expired-session"] = expiredSession

	// Add a non-expired session (started 30 seconds ago)
	activeSession := &Session{
		ID:        "active-session",
		StartedAt: time.Now().Add(-30 * time.Second),
		Status:    "running",
	}
	spawner.sessions["active-session"] = activeSession

	// Check timeouts - should mark expired session for timeout
	spawner.checkTimeouts()

	// Wait for the callback to complete (it runs in a goroutine)
	select {
	case <-callbackDone:
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for OnTimeout callback")
	}

	// Verify only the expired session triggered the callback
	mu.Lock()
	defer mu.Unlock()
	if len(timedOutSessions) != 1 {
		t.Errorf("Expected 1 timed out session, got %d", len(timedOutSessions))
	}
	if len(timedOutSessions) > 0 && timedOutSessions[0].ID != "expired-session" {
		t.Errorf("Timed out session ID = %q, want %q", timedOutSessions[0].ID, "expired-session")
	}
}

func TestSpawner_CheckTimeouts_NoTimeout(t *testing.T) {
	// Create a spawner with no timeout configured (0 means no timeout)
	spawner := &Spawner{
		cfg: SpawnerConfig{
			TimeoutMinutes: 0,
		},
		sessions: make(map[string]*Session),
	}

	var callbackCount int
	spawner.OnTimeout = func(s *Session) {
		callbackCount++
	}

	// Add a session that's been running for a long time
	oldSession := &Session{
		ID:        "old-session",
		StartedAt: time.Now().Add(-24 * time.Hour),
		Status:    "running",
	}
	spawner.sessions["old-session"] = oldSession

	// Check timeouts - should not trigger callback since TimeoutMinutes is 0
	spawner.checkTimeouts()

	if callbackCount != 0 {
		t.Errorf("Callback called %d times, expected 0 (no timeout configured)", callbackCount)
	}
}

func TestSpawner_CheckTimeouts_NoCallback(t *testing.T) {
	// Create a spawner with timeout but no callback
	spawner := &Spawner{
		cfg: SpawnerConfig{
			TimeoutMinutes: 1,
		},
		sessions:  make(map[string]*Session),
		OnTimeout: nil, // No callback set
	}

	// Add an expired session
	expiredSession := &Session{
		ID:        "expired-session",
		StartedAt: time.Now().Add(-2 * time.Minute),
		Status:    "running",
	}
	spawner.sessions["expired-session"] = expiredSession

	// Check timeouts - should not panic even with no callback
	spawner.checkTimeouts()

	// Verify session is marked as timed_out
	if spawner.sessions["expired-session"].Status != "timed_out" {
		t.Errorf("Session status = %q, want %q", spawner.sessions["expired-session"].Status, "timed_out")
	}
}

func TestSpawner_StopAll(t *testing.T) {
	// Skip if Docker not available - we need real containers for StopAll
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image:     "alpine:latest",
		MaxAgents: 5,
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	worktreeDir := t.TempDir()

	// Spawn multiple agents
	for i := 0; i < 3; i++ {
		_, err := spawner.Spawn(context.Background(), SpawnRequest{
			ID:           testSessionID(t, i),
			WorktreePath: worktreeDir,
			WorkDir:      "/workspace",
			Prompt:       "sleep 60",
		})
		if err != nil {
			t.Fatalf("Spawn() agent %d error = %v", i, err)
		}
	}

	// Verify we have 3 sessions
	if spawner.ActiveCount() != 3 {
		t.Fatalf("ActiveCount() = %d, want 3", spawner.ActiveCount())
	}

	// Stop all sessions
	spawner.StopAll(context.Background())

	// Verify all sessions are stopped
	if spawner.ActiveCount() != 0 {
		t.Errorf("ActiveCount() after StopAll = %d, want 0", spawner.ActiveCount())
	}
}

func TestSpawner_StopAll_Empty(t *testing.T) {
	// Test StopAll with no sessions - should not panic
	spawner := &Spawner{
		sessions: make(map[string]*Session),
	}

	// Should not panic
	spawner.StopAll(context.Background())

	if spawner.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d, want 0", spawner.ActiveCount())
	}
}

func TestSpawner_StartTimeoutWatcher(t *testing.T) {
	// Test that the timeout watcher can be started and stopped
	spawner := &Spawner{
		cfg: SpawnerConfig{
			TimeoutMinutes: 1,
		},
		sessions: make(map[string]*Session),
	}

	// Start the watcher
	stop := spawner.startTimeoutWatcher()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop the watcher
	stop()

	// If we get here without panic/hang, the test passes
}

// Helper function to check if Docker is available
func dockerAvailable() bool {
	_, err := NewSpawner(SpawnerConfig{Image: "alpine:latest"})
	return err == nil
}

// Helper function to generate unique session IDs for tests
func testSessionID(t *testing.T, index int) string {
	return t.Name() + "-" + string(rune('a'+index))
}
