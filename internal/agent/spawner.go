package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/drewdunne/familiar/internal/docker"
)

// SpawnerConfig configures the agent spawner.
type SpawnerConfig struct {
	Image          string
	ClaudeAuthDir  string
	MaxAgents      int
	TimeoutMinutes int // 0 means no timeout
}

// SpawnRequest contains parameters for spawning an agent.
type SpawnRequest struct {
	ID           string
	WorktreePath string
	WorkDir      string // Working directory inside container
	Prompt       string
	Env          map[string]string
}

// Session represents a running agent session.
type Session struct {
	ID           string
	ContainerID  string
	WorktreePath string
	StartedAt    time.Time
	Status       string
}

// Spawner manages agent container lifecycle.
type Spawner struct {
	cfg       SpawnerConfig
	client    *docker.Client
	sessions  map[string]*Session
	mu        sync.RWMutex
	OnTimeout func(*Session) // Called when a session times out
}

// NewSpawner creates a new agent spawner.
func NewSpawner(cfg SpawnerConfig) (*Spawner, error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	if cfg.ClaudeAuthDir == "" {
		log.Println("WARNING: claude_auth_dir not configured; agents will hit first-run prompts")
	}

	if cfg.MaxAgents == 0 {
		cfg.MaxAgents = 5
	}

	return &Spawner{
		cfg:      cfg,
		client:   client,
		sessions: make(map[string]*Session),
	}, nil
}

// Close closes the spawner.
func (s *Spawner) Close() error {
	return s.client.Close()
}

// Spawn creates and starts a new agent container.
func (s *Spawner) Spawn(ctx context.Context, req SpawnRequest) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check concurrency limit
	if len(s.sessions) >= s.cfg.MaxAgents {
		return nil, fmt.Errorf("max agents limit reached (%d)", s.cfg.MaxAgents)
	}

	// Prepare mounts
	mounts := []docker.Mount{
		{
			Source:   req.WorktreePath,
			Target:   "/workspace",
			ReadOnly: false,
		},
	}

	// Mount Claude auth if configured
	if s.cfg.ClaudeAuthDir != "" {
		mounts = append(mounts, docker.Mount{
			Source:   s.cfg.ClaudeAuthDir,
			Target:   "/home/agent/.claude",
			ReadOnly: true,
		})
	}

	// Prepare environment
	env := []string{}
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Build container command (claude CLI inside tmux, prompt via env var)
	cmd, cmdEnv := containerCmd(req.Prompt)
	env = append(env, cmdEnv...)

	// Create container
	containerID, err := s.client.CreateContainer(ctx, docker.ContainerConfig{
		Name:    "familiar-agent-" + req.ID,
		Image:   s.cfg.Image,
		WorkDir: req.WorkDir,
		Mounts:  mounts,
		Env:     env,
		Labels: map[string]string{
			"familiar.agent":    "true",
			"familiar.agent.id": req.ID,
		},
		Cmd:        cmd,
		Entrypoint: []string{"/bin/sh"},
	})
	if err != nil {
		return nil, fmt.Errorf("creating container: %w", err)
	}

	// Start container
	if err := s.client.StartContainer(ctx, containerID); err != nil {
		// Cleanup on failure
		s.client.RemoveContainer(ctx, containerID, true)
		return nil, fmt.Errorf("starting container: %w", err)
	}

	session := &Session{
		ID:           req.ID,
		ContainerID:  containerID,
		WorktreePath: req.WorktreePath,
		StartedAt:    time.Now(),
		Status:       "running",
	}

	s.sessions[req.ID] = session
	return session, nil
}

// Stop stops and removes an agent container.
func (s *Spawner) Stop(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Stop container (10 second timeout)
	if err := s.client.StopContainer(ctx, session.ContainerID, 10); err != nil {
		// Log but continue to cleanup
		log.Printf("warning: failed to stop container %s: %v", session.ContainerID, err)
	}

	// Remove container
	if err := s.client.RemoveContainer(ctx, session.ContainerID, true); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}

	delete(s.sessions, sessionID)
	return nil
}

// GetSession returns a session by ID.
func (s *Spawner) GetSession(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

// ListSessions returns all active sessions.
func (s *Spawner) ListSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// ActiveCount returns the number of active agents.
func (s *Spawner) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// CaptureAndStop captures container logs to a file and then stops the agent.
func (s *Spawner) CaptureAndStop(ctx context.Context, sessionID string, logPath string) error {
	session, ok := s.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Get container logs
	logs, err := s.client.GetContainerLogs(ctx, session.ContainerID)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}
	defer logs.Close()

	// Write to log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, logs); err != nil {
		return fmt.Errorf("writing logs: %w", err)
	}

	return s.Stop(ctx, sessionID)
}

// startTimeoutWatcher starts a goroutine that periodically checks for timed-out sessions.
// Returns a function to stop the watcher.
func (s *Spawner) startTimeoutWatcher() func() {
	ticker := time.NewTicker(30 * time.Second)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				s.checkTimeouts()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}

// checkTimeouts checks all sessions and marks/handles those that have exceeded the timeout.
func (s *Spawner) checkTimeouts() {
	// Skip if no timeout configured
	if s.cfg.TimeoutMinutes == 0 {
		return
	}

	timeout := time.Duration(s.cfg.TimeoutMinutes) * time.Minute
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.sessions {
		if session.Status != "running" {
			continue
		}

		if now.Sub(session.StartedAt) > timeout {
			// Mark session as timed out
			session.Status = "timed_out"

			// Call the timeout callback if set
			if s.OnTimeout != nil {
				// Call callback without holding lock to avoid deadlock
				// Make a copy of session for the callback
				sessionCopy := *session
				go s.OnTimeout(&sessionCopy)
			}
		}
	}
}

// containerCmd builds the container Cmd and extra env vars for running
// a Claude agent inside a tmux session. The prompt is passed via the
// FAMILIAR_PROMPT environment variable to avoid nested shell quoting issues.
func containerCmd(prompt string) (cmd []string, extraEnv []string) {
	return []string{"-c",
			`tmux new-session -d -s claude 'claude --dangerously-skip-permissions "$FAMILIAR_PROMPT"' && tmux wait-for claude`,
		},
		[]string{"FAMILIAR_PROMPT=" + prompt}
}

// StopAll stops all active sessions.
func (s *Spawner) StopAll(ctx context.Context) {
	s.mu.Lock()
	sessionIDs := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	s.mu.Unlock()

	for _, id := range sessionIDs {
		if err := s.Stop(ctx, id); err != nil {
			log.Printf("warning: failed to stop session %s: %v", id, err)
		}
	}
}
