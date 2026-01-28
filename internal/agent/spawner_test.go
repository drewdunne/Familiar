package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSpawner_Spawn(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		// Check if Docker socket exists
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	// Create temp worktree
	worktreeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktreeDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create temp claude auth dir
	claudeAuthDir := t.TempDir()

	spawner, err := NewSpawner(SpawnerConfig{
		Image:         "alpine:latest", // Use alpine for testing
		ClaudeAuthDir: claudeAuthDir,
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	session, err := spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "test-agent-123",
		WorktreePath: worktreeDir,
		WorkDir:      "/workspace",
		Prompt:       "echo 'Hello from agent'",
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if session.ID != "test-agent-123" {
		t.Errorf("session.ID = %q, want %q", session.ID, "test-agent-123")
	}
	if session.ContainerID == "" {
		t.Error("session.ContainerID should not be empty")
	}

	// Cleanup
	if err := spawner.Stop(context.Background(), session.ID); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestSpawner_MaxAgentsLimit(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image:     "alpine:latest",
		MaxAgents: 2,
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	// Cleanup sessions after test
	defer func() {
		for _, session := range spawner.ListSessions() {
			spawner.Stop(context.Background(), session.ID)
		}
	}()

	worktreeDir := t.TempDir()

	// Spawn two agents (should succeed)
	for i := 0; i < 2; i++ {
		_, err := spawner.Spawn(context.Background(), SpawnRequest{
			ID:           fmt.Sprintf("agent-%d", i),
			WorktreePath: worktreeDir,
			WorkDir:      "/workspace",
			Prompt:       "sleep 60",
		})
		if err != nil {
			t.Fatalf("Spawn() agent %d error = %v", i, err)
		}
	}

	// Third agent should fail
	_, err = spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "agent-overflow",
		WorktreePath: worktreeDir,
		WorkDir:      "/workspace",
		Prompt:       "sleep 60",
	})
	if err == nil {
		t.Error("Expected error when exceeding max agents limit")
	}

	// Verify ActiveCount
	if spawner.ActiveCount() != 2 {
		t.Errorf("ActiveCount() = %d, want 2", spawner.ActiveCount())
	}
}

func TestSpawner_GetSession(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image: "alpine:latest",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	worktreeDir := t.TempDir()

	session, err := spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "test-get-session",
		WorktreePath: worktreeDir,
		WorkDir:      "/workspace",
		Prompt:       "sleep 60",
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	defer spawner.Stop(context.Background(), session.ID)

	// Test GetSession
	retrieved, ok := spawner.GetSession("test-get-session")
	if !ok {
		t.Error("GetSession() returned ok=false for existing session")
	}
	if retrieved.ID != session.ID {
		t.Errorf("GetSession().ID = %q, want %q", retrieved.ID, session.ID)
	}

	// Test GetSession for non-existent session
	_, ok = spawner.GetSession("non-existent")
	if ok {
		t.Error("GetSession() returned ok=true for non-existent session")
	}
}

func TestSpawner_StopNonExistent(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image: "alpine:latest",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	// Stop non-existent session should return error
	err = spawner.Stop(context.Background(), "non-existent-session")
	if err == nil {
		t.Error("Stop() expected error for non-existent session")
	}
}

func TestSpawner_CaptureAndStop(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image: "alpine:latest",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	worktreeDir := t.TempDir()

	// Spawn agent that produces output
	session, err := spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "test-capture",
		WorktreePath: worktreeDir,
		WorkDir:      "/workspace",
		Prompt:       "echo 'test log output'",
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	// Create log file path
	logPath := filepath.Join(t.TempDir(), "agent.log")

	// Capture and stop
	err = spawner.CaptureAndStop(context.Background(), session.ID, logPath)
	if err != nil {
		t.Fatalf("CaptureAndStop() error = %v", err)
	}

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}

	// Verify session is removed
	if _, ok := spawner.GetSession(session.ID); ok {
		t.Error("Session should be removed after CaptureAndStop")
	}
}

func TestSpawner_CaptureAndStop_NonExistent(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image: "alpine:latest",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	// CaptureAndStop non-existent session should return error
	err = spawner.CaptureAndStop(context.Background(), "non-existent-session", "/tmp/test.log")
	if err == nil {
		t.Error("CaptureAndStop() expected error for non-existent session")
	}
}
