# Phase 5: Docker Agent Spawning Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-4. Review previous implementations before starting.

**Goal:** Spawn isolated Claude Code agents in Docker containers with mounted worktrees, tmux sessions for debugging, concurrency management, and session tracking.

**Architecture:** Agent spawner creates containers with worktrees mounted, Claude auth from host, runs Claude Code in tmux. Session manager tracks active agents, enforces concurrency limits, queues overflow.

**Tech Stack:** Go 1.25, Docker SDK (github.com/docker/docker), existing repocache package

**Prerequisites:** Phases 1-4 complete

---

## Task 1: Create Agent Container Dockerfile

**Files:**
- Create: `docker/agent/Dockerfile`

**Step 1: Create Dockerfile**

Create `docker/agent/Dockerfile`:
```dockerfile
FROM golang:1.25-alpine

# Install dependencies
RUN apk add --no-cache \
    git \
    tmux \
    openssh-client \
    ca-certificates \
    curl \
    bash

# Install Claude Code CLI
# Note: Update installation method based on actual Claude Code distribution
RUN curl -fsSL https://claude.ai/install.sh | sh || \
    echo "Claude Code installation placeholder - update when available"

# Create non-root user for agent
RUN adduser -D -h /home/agent -s /bin/bash agent

# Create directories
RUN mkdir -p /home/agent/.claude /workspace && \
    chown -R agent:agent /home/agent /workspace

USER agent
WORKDIR /workspace

# Default entrypoint - will be overridden
ENTRYPOINT ["/bin/bash"]
```

**Step 2: Create .dockerignore**

Create `docker/agent/.dockerignore`:
```
*.md
*.txt
```

**Step 3: Commit**

```bash
git add docker/
git commit -m "feat(docker): add agent container Dockerfile"
```

---

## Task 2: Docker Client Wrapper

**Files:**
- Create: `internal/docker/client.go`
- Create: `internal/docker/client_test.go`

**Step 1: Write failing test**

Create `internal/docker/client_test.go`:
```go
package docker

import (
	"context"
	"testing"
)

func TestClient_Ping(t *testing.T) {
	// Skip if Docker not available
	client, err := NewClient()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer client.Close()

	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestClient_ImageExists(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer client.Close()

	// Alpine should exist on most systems or be quick to pull
	exists, err := client.ImageExists(context.Background(), "alpine:latest")
	if err != nil {
		t.Logf("ImageExists() error = %v (may need to pull image)", err)
	}
	_ = exists // Just checking it doesn't panic
}
```

**Step 2: Implement client wrapper**

Create `internal/docker/client.go`:
```go
package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// Client wraps the Docker client with convenience methods.
type Client struct {
	cli *client.Client
}

// NewClient creates a new Docker client.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close closes the Docker client.
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping checks if Docker daemon is accessible.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

// ImageExists checks if an image exists locally.
func (c *Client) ImageExists(ctx context.Context, image string) (bool, error) {
	images, err := c.cli.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", image)),
	})
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

// PullImage pulls an image if it doesn't exist.
func (c *Client) PullImage(ctx context.Context, image string) error {
	exists, err := c.ImageExists(ctx, image)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	reader, err := c.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer reader.Close()

	// Consume the output
	_, err = io.Copy(io.Discard, reader)
	return err
}

// ContainerConfig holds configuration for creating a container.
type ContainerConfig struct {
	Name         string
	Image        string
	WorkDir      string
	Mounts       []Mount
	Env          []string
	Labels       map[string]string
	Cmd          []string
	Entrypoint   []string
}

// Mount represents a bind mount.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// CreateContainer creates a new container.
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	mounts := make([]mount.Mount, len(cfg.Mounts))
	for i, m := range cfg.Mounts {
		mounts[i] = mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		}
	}

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:      cfg.Image,
			WorkingDir: cfg.WorkDir,
			Env:        cfg.Env,
			Labels:     cfg.Labels,
			Cmd:        cfg.Cmd,
			Entrypoint: cfg.Entrypoint,
			Tty:        true,
			OpenStdin:  true,
		},
		&container.HostConfig{
			Mounts: mounts,
		},
		nil, nil, cfg.Name,
	)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

// StopContainer stops a container.
func (c *Client) StopContainer(ctx context.Context, id string, timeout int) error {
	t := timeout
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

// ExecInContainer runs a command in a container.
func (c *Client) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	exec, err := c.cli.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	return c.cli.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
}

// GetContainerLogs returns container logs.
func (c *Client) GetContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
	})
}
```

**Step 3: Add Docker SDK dependency**

```bash
go get github.com/docker/docker
go mod tidy
```

**Step 4: Run tests**

```bash
go test ./internal/docker/... -v
```

**Step 5: Commit**

```bash
git add internal/docker/ go.mod go.sum
git commit -m "feat(docker): add Docker client wrapper"
```

---

## Task 3: Agent Spawner (TDD)

**Files:**
- Create: `internal/agent/spawner.go`
- Create: `internal/agent/spawner_test.go`

**Step 1: Write failing test**

Create `internal/agent/spawner_test.go`:
```go
package agent

import (
	"context"
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
		Image:        "alpine:latest", // Use alpine for testing
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
```

**Step 2: Implement spawner**

Create `internal/agent/spawner.go`:
```go
package agent

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/drewdunne/familiar/internal/docker"
)

// SpawnerConfig configures the agent spawner.
type SpawnerConfig struct {
	Image         string
	ClaudeAuthDir string
	MaxAgents     int
	QueueSize     int
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
	ID          string
	ContainerID string
	WorktreePath string
	StartedAt   time.Time
	Status      string
}

// Spawner manages agent container lifecycle.
type Spawner struct {
	cfg      SpawnerConfig
	client   *docker.Client
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSpawner creates a new agent spawner.
func NewSpawner(cfg SpawnerConfig) (*Spawner, error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
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
		if _, err := os.Stat(s.cfg.ClaudeAuthDir); err == nil {
			mounts = append(mounts, docker.Mount{
				Source:   s.cfg.ClaudeAuthDir,
				Target:   "/home/agent/.claude",
				ReadOnly: true,
			})
		}
	}

	// Prepare environment
	env := []string{}
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

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
		// Start tmux session with claude command
		Cmd: []string{"-c", fmt.Sprintf(
			"tmux new-session -d -s claude '%s' && tmux wait-for claude",
			req.Prompt,
		)},
		Entrypoint: []string{"/bin/bash"},
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
```

**Step 3: Run tests**

```bash
go test ./internal/agent/... -v
```

**Step 4: Commit**

```bash
git add internal/agent/
git commit -m "feat(agent): add agent spawner with Docker integration"
```

---

## Task 4: Session Manager with Queue (TDD)

**Files:**
- Create: `internal/agent/manager.go`
- Create: `internal/agent/manager_test.go`

**Step 1: Write failing test**

Create `internal/agent/manager_test.go`:
```go
package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestManager_Queue(t *testing.T) {
	var spawnCount int32

	// Mock spawner behavior
	manager := NewManager(ManagerConfig{
		MaxConcurrent: 2,
		QueueSize:     5,
	})

	// Spawn function that tracks calls
	spawnFn := func(ctx context.Context, req SpawnRequest) error {
		atomic.AddInt32(&spawnCount, 1)
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// Queue 4 requests (2 should run, 2 should queue)
	for i := 0; i < 4; i++ {
		manager.Enqueue(SpawnRequest{ID: fmt.Sprintf("agent-%d", i)}, spawnFn)
	}

	// Give time for processing
	time.Sleep(200 * time.Millisecond)

	if count := atomic.LoadInt32(&spawnCount); count != 4 {
		t.Errorf("spawnCount = %d, want 4", count)
	}
}

func TestManager_QueueFull(t *testing.T) {
	manager := NewManager(ManagerConfig{
		MaxConcurrent: 1,
		QueueSize:     1,
	})

	// Block the single slot
	blocking := make(chan struct{})
	manager.Enqueue(SpawnRequest{ID: "blocking"}, func(ctx context.Context, req SpawnRequest) error {
		<-blocking
		return nil
	})

	// Fill the queue
	manager.Enqueue(SpawnRequest{ID: "queued"}, func(ctx context.Context, req SpawnRequest) error {
		return nil
	})

	// This should fail - queue full
	err := manager.Enqueue(SpawnRequest{ID: "overflow"}, func(ctx context.Context, req SpawnRequest) error {
		return nil
	})

	if err == nil {
		t.Error("Expected queue full error")
	}

	close(blocking)
}
```

**Step 2: Implement manager**

Create `internal/agent/manager.go`:
```go
package agent

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull is returned when the queue is at capacity.
var ErrQueueFull = errors.New("agent queue is full")

// ManagerConfig configures the session manager.
type ManagerConfig struct {
	MaxConcurrent int
	QueueSize     int
}

// SpawnFunc is a function that spawns an agent.
type SpawnFunc func(ctx context.Context, req SpawnRequest) error

// queuedRequest represents a queued spawn request.
type queuedRequest struct {
	req     SpawnRequest
	spawnFn SpawnFunc
}

// Manager manages agent concurrency and queueing.
type Manager struct {
	cfg       ManagerConfig
	queue     chan queuedRequest
	semaphore chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new session manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 20
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		cfg:       cfg,
		queue:     make(chan queuedRequest, cfg.QueueSize),
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start worker
	go m.worker()

	return m
}

// Enqueue adds a spawn request to the queue.
func (m *Manager) Enqueue(req SpawnRequest, spawnFn SpawnFunc) error {
	select {
	case m.queue <- queuedRequest{req: req, spawnFn: spawnFn}:
		return nil
	default:
		return ErrQueueFull
	}
}

// worker processes the queue.
func (m *Manager) worker() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case queued := <-m.queue:
			// Wait for semaphore
			m.semaphore <- struct{}{}

			m.wg.Add(1)
			go func(q queuedRequest) {
				defer m.wg.Done()
				defer func() { <-m.semaphore }()

				q.spawnFn(m.ctx, q.req)
			}(queued)
		}
	}
}

// QueueLength returns current queue length.
func (m *Manager) QueueLength() int {
	return len(m.queue)
}

// ActiveCount returns number of currently running agents.
func (m *Manager) ActiveCount() int {
	return len(m.semaphore)
}

// Shutdown stops the manager and waits for agents to complete.
func (m *Manager) Shutdown() {
	m.cancel()
	m.wg.Wait()
}
```

**Step 3: Run tests**

```bash
go test ./internal/agent/... -v
```

**Step 4: Commit**

```bash
git add internal/agent/
git commit -m "feat(agent): add session manager with queueing"
```

---

## Task 5: Claude Command Builder

**Files:**
- Create: `internal/agent/command.go`
- Create: `internal/agent/command_test.go`

**Step 1: Write failing test**

Create `internal/agent/command_test.go`:
```go
package agent

import "testing"

func TestBuildClaudeCommand(t *testing.T) {
	cmd := BuildClaudeCommand(ClaudeCommandConfig{
		Prompt:     "Review this PR",
		WorkDir:    "/workspace",
		Autonomous: true,
	})

	// Should include claude command
	if cmd == "" {
		t.Error("Command should not be empty")
	}

	// Should include the prompt
	if !strings.Contains(cmd, "Review this PR") {
		t.Error("Command should contain the prompt")
	}
}
```

**Step 2: Implement command builder**

Create `internal/agent/command.go`:
```go
package agent

import (
	"fmt"
	"strings"
)

// ClaudeCommandConfig holds configuration for the Claude command.
type ClaudeCommandConfig struct {
	Prompt     string
	WorkDir    string
	Autonomous bool
}

// BuildClaudeCommand builds the command to run Claude Code in the container.
func BuildClaudeCommand(cfg ClaudeCommandConfig) string {
	// Escape the prompt for shell
	escapedPrompt := strings.ReplaceAll(cfg.Prompt, "'", "'\"'\"'")

	var args []string
	args = append(args, "claude")

	if cfg.Autonomous {
		// Run in autonomous mode without permission prompts
		args = append(args, "--dangerously-skip-permissions")
	}

	// Add the prompt
	args = append(args, fmt.Sprintf("'%s'", escapedPrompt))

	return strings.Join(args, " ")
}

// BuildTmuxCommand wraps a command in a tmux session.
func BuildTmuxCommand(sessionName, innerCmd string) string {
	return fmt.Sprintf(
		"tmux new-session -d -s %s '%s' && tmux wait-for %s",
		sessionName,
		innerCmd,
		sessionName,
	)
}
```

**Step 3: Run tests**

```bash
go test ./internal/agent/... -v -run TestBuildClaudeCommand
```

**Step 4: Commit**

```bash
git add internal/agent/
git commit -m "feat(agent): add Claude command builder"
```

---

## Task 6: Integration - Wire Spawner to Event Handler

**Files:**
- Modify: `internal/server/server.go` (or create handler)
- Create: `internal/handler/agent.go`

**Step 1: Create agent handler**

Create `internal/handler/agent.go`:
```go
package handler

import (
	"context"
	"fmt"
	"log"

	"github.com/drewdunne/familiar/internal/agent"
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
	"github.com/drewdunne/familiar/internal/repocache"
)

// AgentHandler handles events by spawning agents.
type AgentHandler struct {
	spawner   *agent.Spawner
	repoCache *repocache.Cache
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(spawner *agent.Spawner, repoCache *repocache.Cache) *AgentHandler {
	return &AgentHandler{
		spawner:   spawner,
		repoCache: repoCache,
	}
}

// Handle processes an event by spawning an agent.
func (h *AgentHandler) Handle(ctx context.Context, evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) error {
	// Generate unique agent ID
	agentID := fmt.Sprintf("%s-%s-%d-%d", evt.Provider, evt.RepoName, evt.MRNumber, evt.Timestamp.Unix())

	// Ensure repo is cached and create worktree
	repoPath, err := h.repoCache.EnsureRepo(ctx, evt.RepoURL, evt.RepoOwner, evt.RepoName)
	if err != nil {
		return fmt.Errorf("ensuring repo: %w", err)
	}

	worktreePath, err := h.repoCache.CreateWorktree(ctx, evt.RepoOwner, evt.RepoName, evt.SourceBranch, agentID)
	if err != nil {
		return fmt.Errorf("creating worktree: %w", err)
	}

	// Build prompt (Phase 6 will enhance this)
	prompt := buildPrompt(evt, cfg, parsedIntent)

	// Spawn agent
	_, err = h.spawner.Spawn(ctx, agent.SpawnRequest{
		ID:           agentID,
		WorktreePath: worktreePath,
		WorkDir:      "/workspace", // TODO: Phase 6 will calculate LCA
		Prompt:       prompt,
	})
	if err != nil {
		// Cleanup worktree on failure
		h.repoCache.RemoveWorktree(ctx, evt.RepoOwner, evt.RepoName, agentID)
		return fmt.Errorf("spawning agent: %w", err)
	}

	log.Printf("Spawned agent %s for %s/%s MR #%d", agentID, evt.RepoOwner, evt.RepoName, evt.MRNumber)
	return nil
}

func buildPrompt(evt *event.Event, cfg *config.MergedConfig, parsedIntent *intent.ParsedIntent) string {
	// Simple prompt for now - Phase 6 will build full prompt
	var prompt string

	switch evt.Type {
	case event.TypeMROpened:
		prompt = cfg.Prompts.MROpened
	case event.TypeMRComment:
		prompt = cfg.Prompts.MRComment
	case event.TypeMRUpdated:
		prompt = cfg.Prompts.MRUpdated
	case event.TypeMention:
		prompt = cfg.Prompts.Mention
	}

	if parsedIntent != nil && parsedIntent.Instructions != "" {
		prompt += "\n\nUser instructions: " + parsedIntent.Instructions
	}

	return prompt
}
```

**Step 2: Commit**

```bash
git add internal/handler/
git commit -m "feat(handler): add agent handler for event processing"
```

---

## Task 7: Run Full Test Suite

**Step 1: Run all tests with coverage**

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Step 2: Verify coverage >= 80%**

**Step 3: Commit if needed**

---

## Summary

| Task | Component | Tests Added |
|------|-----------|-------------|
| 1 | Agent Dockerfile | - |
| 2 | Docker client | 2 |
| 3 | Agent spawner | 1 |
| 4 | Session manager | 2 |
| 5 | Claude command builder | 1 |
| 6 | Agent handler integration | - |
| 7 | Coverage verification | - |

**Total: 7 tasks, ~6 tests**
