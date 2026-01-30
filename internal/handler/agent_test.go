package handler

import (
	"os"
	"strings"
	"testing"

	"github.com/drewdunne/familiar/internal/lca"
	"github.com/drewdunne/familiar/internal/logging"
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
