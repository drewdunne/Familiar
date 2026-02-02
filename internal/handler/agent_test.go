package handler

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/drewdunne/familiar/internal/agent"
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/event"
	"github.com/drewdunne/familiar/internal/intent"
	"github.com/drewdunne/familiar/internal/lca"
	"github.com/drewdunne/familiar/internal/logging"
	"github.com/drewdunne/familiar/internal/provider"
)

func TestNewAgentHandler(t *testing.T) {
	handler := NewAgentHandler(nil, nil, nil, "/var/log/familiar", "/host/path/logs")

	if handler == nil {
		t.Fatal("NewAgentHandler() returned nil")
	}

	if handler.spawner != nil {
		t.Error("handler.spawner should be nil when nil is passed")
	}

	if handler.repoCache != nil {
		t.Error("handler.repoCache should be nil when nil is passed")
	}

	if handler.registry != nil {
		t.Error("handler.registry should be nil when nil is passed")
	}

	if handler.promptBuilder == nil {
		t.Error("handler.promptBuilder should be initialized")
	}

	if handler.logWriter == nil {
		t.Error("handler.logWriter should be initialized when logDir is set")
	}

	if handler.logDir != "/var/log/familiar" {
		t.Errorf("handler.logDir = %q, want %q", handler.logDir, "/var/log/familiar")
	}

	if handler.logHostDir != "/host/path/logs" {
		t.Errorf("handler.logHostDir = %q, want %q", handler.logHostDir, "/host/path/logs")
	}
}

func TestNewAgentHandler_NoLogDir(t *testing.T) {
	handler := NewAgentHandler(nil, nil, nil, "", "")

	if handler.logWriter != nil {
		t.Error("handler.logWriter should be nil when logDir is empty")
	}
}

func TestHostLogPath(t *testing.T) {
	tests := []struct {
		name          string
		logDir        string
		logHostDir    string
		containerPath string
		want          string
	}{
		{
			name:          "swaps container prefix for host prefix",
			logDir:        "/var/log/familiar",
			logHostDir:    "/home/user/project/logs",
			containerPath: "/var/log/familiar/owner/repo/2/2025-01-30T12-00-00-mr_comment-agent1.log",
			want:          "/home/user/project/logs/owner/repo/2/2025-01-30T12-00-00-mr_comment-agent1.log",
		},
		{
			name:          "falls back to container path when no host dir",
			logDir:        "/var/log/familiar",
			logHostDir:    "",
			containerPath: "/var/log/familiar/owner/repo/2/file.log",
			want:          "/var/log/familiar/owner/repo/2/file.log",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAgentHandler(nil, nil, nil, tc.logDir, tc.logHostDir)
			got := h.hostLogPath(tc.containerPath)
			if got != tc.want {
				t.Errorf("hostLogPath(%q) = %q, want %q", tc.containerPath, got, tc.want)
			}
		})
	}
}

func TestCreateLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	writer := logging.NewWriter(tmpDir)

	entry := logging.LogEntry{
		AgentID:   "test-agent-1",
		RepoOwner: "drewdunne",
		RepoName:  "tinyhost",
		MRNumber:  2,
		EventType: "mr_comment",
	}
	path, err := writer.Create(entry)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Path should be under baseDir/owner/repo/mr/
	if !strings.HasPrefix(path, tmpDir) {
		t.Errorf("path %q does not start with %q", path, tmpDir)
	}

	// File should exist
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if info.IsDir() {
		t.Error("expected a file, not a directory")
	}
}

func TestLCAIntegration_FilePaths(t *testing.T) {
	// Test that LCA calculation works correctly with file paths
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "single file",
			files:    []string{"internal/handler/agent.go"},
			expected: "internal/handler",
		},
		{
			name:     "same directory",
			files:    []string{"internal/handler/agent.go", "internal/handler/agent_test.go"},
			expected: "internal/handler",
		},
		{
			name:     "sibling directories",
			files:    []string{"internal/handler/agent.go", "internal/config/config.go"},
			expected: "internal",
		},
		{
			name:     "root files",
			files:    []string{"README.md", "go.mod"},
			expected: ".",
		},
		{
			name:     "different trees",
			files:    []string{"cmd/main.go", "internal/handler/agent.go"},
			expected: ".",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := lca.FindLCA(tc.files)
			if result != tc.expected {
				t.Errorf("FindLCA(%v) = %q, want %q", tc.files, result, tc.expected)
			}
		})
	}
}

func TestLCAToWorkDir(t *testing.T) {
	// Test the logic for converting LCA to container workDir
	tests := []struct {
		lcaResult   string
		expectedDir string
	}{
		{".", "/workspace"},
		{"internal", "/workspace/internal"},
		{"internal/handler", "/workspace/internal/handler"},
	}

	for _, tc := range tests {
		t.Run(tc.lcaResult, func(t *testing.T) {
			workDir := "/workspace"
			if tc.lcaResult != "." {
				workDir = "/workspace/" + tc.lcaResult
			}
			if workDir != tc.expectedDir {
				t.Errorf("workDir = %q, want %q", workDir, tc.expectedDir)
			}
		})
	}
}

// --- Mock types for handler dependency interfaces ---

type mockSpawner struct {
	lastRequest agent.SpawnRequest
	spawnErr    error
}

func (m *mockSpawner) Spawn(_ context.Context, req agent.SpawnRequest) (*agent.Session, error) {
	m.lastRequest = req
	if m.spawnErr != nil {
		return nil, m.spawnErr
	}
	return &agent.Session{ID: req.ID, Status: "running"}, nil
}

type mockRepoCache struct {
	ensureErr   error
	worktreeErr error
}

func (m *mockRepoCache) EnsureRepo(_ context.Context, _, _, _ string) (string, error) {
	if m.ensureErr != nil {
		return "", m.ensureErr
	}
	return "/cache/owner/repo.git", nil
}

func (m *mockRepoCache) CreateWorktree(_ context.Context, _, _, _, _ string) (string, error) {
	if m.worktreeErr != nil {
		return "", m.worktreeErr
	}
	return "/cache/owner/repo.git/worktrees-data/wt-1", nil
}

func (m *mockRepoCache) RemoveWorktree(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockRepoCache) HostPath(containerPath string) string {
	return containerPath
}

type mockProvider struct {
	name     string
	agentEnv map[string]string
	authURL  string
	files    []provider.ChangedFile
	filesErr error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) GetRepository(_ context.Context, _, _ string) (*provider.Repository, error) {
	return nil, nil
}

func (m *mockProvider) GetMergeRequest(_ context.Context, _, _ string, _ int) (*provider.MergeRequest, error) {
	return nil, nil
}

func (m *mockProvider) GetChangedFiles(_ context.Context, _, _ string, _ int) ([]provider.ChangedFile, error) {
	return m.files, m.filesErr
}

func (m *mockProvider) PostComment(_ context.Context, _, _ string, _ int, _ string) error {
	return nil
}

func (m *mockProvider) GetComments(_ context.Context, _, _ string, _ int) ([]provider.Comment, error) {
	return nil, nil
}

func (m *mockProvider) AgentEnv() map[string]string {
	return m.agentEnv
}

func (m *mockProvider) AuthenticatedCloneURL(rawURL string) (string, error) {
	if m.authURL != "" {
		return m.authURL, nil
	}
	return rawURL, nil
}

type mockRegistry struct {
	providers map[string]provider.Provider
}

func (m *mockRegistry) Get(name string) provider.Provider {
	return m.providers[name]
}

// --- Tests for provider env wiring ---

func TestHandle_PassesProviderEnvToSpawner(t *testing.T) {
	spawner := &mockSpawner{}
	cache := &mockRepoCache{}
	prov := &mockProvider{
		name: "gitlab",
		agentEnv: map[string]string{
			"GITLAB_TOKEN": "glpat-test-token",
			"GITLAB_HOST":  "gitlab.example.com",
		},
	}
	reg := &mockRegistry{
		providers: map[string]provider.Provider{
			"gitlab": prov,
		},
	}

	h := NewAgentHandler(spawner, cache, reg, "", "")

	evt := &event.Event{
		Type:         event.TypeMRComment,
		Provider:     "gitlab",
		RepoOwner:    "owner",
		RepoName:     "repo",
		RepoURL:      "https://gitlab.example.com/owner/repo.git",
		MRNumber:     1,
		SourceBranch: "feature",
		TargetBranch: "main",
		Timestamp:    time.Now(),
	}

	cfg := &config.MergedConfig{}
	parsedIntent := &intent.ParsedIntent{Instructions: "do something"}

	err := h.Handle(context.Background(), evt, cfg, parsedIntent)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	if spawner.lastRequest.Env == nil {
		t.Fatal("expected SpawnRequest.Env to be set, got nil")
	}

	if spawner.lastRequest.Env["GITLAB_TOKEN"] != "glpat-test-token" {
		t.Errorf("GITLAB_TOKEN = %q, want %q", spawner.lastRequest.Env["GITLAB_TOKEN"], "glpat-test-token")
	}

	if spawner.lastRequest.Env["GITLAB_HOST"] != "gitlab.example.com" {
		t.Errorf("GITLAB_HOST = %q, want %q", spawner.lastRequest.Env["GITLAB_HOST"], "gitlab.example.com")
	}
}

func TestHandle_NilProviderSkipsEnv(t *testing.T) {
	spawner := &mockSpawner{}
	cache := &mockRepoCache{}
	reg := &mockRegistry{
		providers: map[string]provider.Provider{},
	}

	h := NewAgentHandler(spawner, cache, reg, "", "")

	evt := &event.Event{
		Type:         event.TypeMRComment,
		Provider:     "unknown",
		RepoOwner:    "owner",
		RepoName:     "repo",
		RepoURL:      "https://example.com/owner/repo.git",
		MRNumber:     1,
		SourceBranch: "feature",
		TargetBranch: "main",
		Timestamp:    time.Now(),
	}

	cfg := &config.MergedConfig{}
	parsedIntent := &intent.ParsedIntent{Instructions: "do something"}

	err := h.Handle(context.Background(), evt, cfg, parsedIntent)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	if spawner.lastRequest.Env != nil {
		t.Errorf("expected SpawnRequest.Env to be nil, got %v", spawner.lastRequest.Env)
	}
}
