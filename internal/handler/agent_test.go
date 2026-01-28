package handler

import (
	"testing"

	"github.com/drewdunne/familiar/internal/lca"
)

func TestNewAgentHandler(t *testing.T) {
	// Test that NewAgentHandler returns a properly initialized handler
	handler := NewAgentHandler(nil, nil, nil)

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
