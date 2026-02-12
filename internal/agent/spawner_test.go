package agent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func TestContainerCmd(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{
			name:   "simple prompt",
			prompt: "Review this merge request",
		},
		{
			name:   "prompt with single quotes",
			prompt: "Review the user's code",
		},
		{
			name:   "prompt with special characters",
			prompt: "Check for $variables and `backticks`",
		},
		{
			name:   "multiline prompt",
			prompt: "## Context\n- Repository: foo/bar\n\n## Task\nReview this PR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, env := containerCmd(tt.prompt)

			// Should produce shell command via /bin/sh -c
			if len(cmd) != 2 || cmd[0] != "-c" {
				t.Fatalf("cmd = %v, want [-c <command>]", cmd)
			}

			// Command should copy credentials from /claude-auth-src to /home/agent/.claude
			if !strings.Contains(cmd[1], "mkdir -p /home/agent/.claude") {
				t.Error("command should create /home/agent/.claude directory")
			}
			if !strings.Contains(cmd[1], "cp /claude-auth-src/.credentials.json /home/agent/.claude/") {
				t.Error("command should copy .credentials.json from /claude-auth-src")
			}
			if !strings.Contains(cmd[1], "cp /claude-auth-src/settings.json /home/agent/.claude/") {
				t.Error("command should copy settings.json from /claude-auth-src")
			}

			// Command should invoke claude with --dangerously-skip-permissions and -p (print mode)
			if !strings.Contains(cmd[1], "claude --dangerously-skip-permissions -p") {
				t.Error("command should invoke claude with --dangerously-skip-permissions -p")
			}

			// Command should use tmux
			if !strings.Contains(cmd[1], "tmux new-session") {
				t.Error("command should use tmux")
			}

			// Command should NOT embed the raw prompt text (use env var instead)
			if strings.Contains(cmd[1], tt.prompt) {
				t.Error("command should not embed raw prompt text; should use env var")
			}

			// Env should contain the prompt
			found := false
			for _, e := range env {
				if e == "FAMILIAR_PROMPT="+tt.prompt {
					found = true
					break
				}
			}
			if !found {
				t.Error("env should contain FAMILIAR_PROMPT with the prompt value")
			}
		})
	}
}

func TestNewSpawner_WarnsOnEmptyClaudeAuthDir(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	spawner, err := NewSpawner(SpawnerConfig{
		Image:         "alpine:latest",
		ClaudeAuthDir: "",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	if !strings.Contains(buf.String(), "claude_auth_dir not configured") {
		t.Errorf("expected warning about claude_auth_dir, got log output: %q", buf.String())
	}
}

func TestNewSpawner_NoWarningWhenClaudeAuthDirSet(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	spawner, err := NewSpawner(SpawnerConfig{
		Image:         "alpine:latest",
		ClaudeAuthDir: "/some/path",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	if strings.Contains(buf.String(), "claude_auth_dir not configured") {
		t.Errorf("unexpected warning when ClaudeAuthDir is set, got log output: %q", buf.String())
	}
}

func TestSpawner_Spawn_SetsContainerUser(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	expectedUID := fmt.Sprintf("%d", os.Getuid())

	spawner, err := NewSpawner(SpawnerConfig{
		Image: "alpine:latest",
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	worktreeDir := t.TempDir()

	session, err := spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "test-uid-agent",
		WorktreePath: worktreeDir,
		WorkDir:      "/workspace",
		Prompt:       "echo test",
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	defer spawner.Stop(context.Background(), session.ID)

	// Verify the container is running as the current process UID
	if session.ContainerUser != expectedUID {
		t.Errorf("session.ContainerUser = %q, want %q", session.ContainerUser, expectedUID)
	}
}

func TestSpawner_Spawn_UsesTmpfsHome(t *testing.T) {
	// Skip if Docker not available
	if os.Getenv("DOCKER_HOST") == "" && os.Getenv("CI") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			t.Skip("Docker not available")
		}
	}

	// Create temp directories
	claudeAuthDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create fake credentials file
	if err := os.WriteFile(filepath.Join(claudeAuthDir, ".credentials.json"), []byte(`{"test":"creds"}`), 0644); err != nil {
		t.Fatal(err)
	}

	spawner, err := NewSpawner(SpawnerConfig{
		Image:         "alpine:latest",
		ClaudeAuthDir: claudeAuthDir,
	})
	if err != nil {
		t.Fatalf("NewSpawner() error = %v", err)
	}
	defer spawner.Close()

	session, err := spawner.Spawn(context.Background(), SpawnRequest{
		ID:           "test-tmpfs-home",
		WorktreePath: worktreeDir,
		WorkDir:      "/workspace",
		Prompt:       "echo test",
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	defer spawner.Stop(context.Background(), session.ID)

	// Container should be created successfully
	if session.ContainerID == "" {
		t.Error("session.ContainerID should not be empty")
	}
}

func TestResolveContainerUser_UsesProcessUID(t *testing.T) {
	expected := fmt.Sprintf("%d", os.Getuid())
	got := resolveContainerUser()
	if got != expected {
		t.Errorf("resolveContainerUser() = %q, want %q", got, expected)
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
