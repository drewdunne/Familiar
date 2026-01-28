package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogWriter_Write(t *testing.T) {
	baseDir := t.TempDir()
	writer := NewWriter(baseDir)

	entry := LogEntry{
		AgentID:   "agent-123",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  42,
		EventType: "mr_opened",
		Timestamp: time.Now(),
	}

	logPath, err := writer.Create(entry)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Write some content
	if err := writer.Append(logPath, []byte("test log line\n")); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}

	// Verify directory structure
	expectedDir := filepath.Join(baseDir, "owner", "repo", "42")
	if !strings.HasPrefix(logPath, expectedDir) {
		t.Errorf("Log path %q should be under %q", logPath, expectedDir)
	}
}

func TestLogWriter_Create_DirectoryStructure(t *testing.T) {
	baseDir := t.TempDir()
	writer := NewWriter(baseDir)

	entry := LogEntry{
		AgentID:   "test-agent",
		RepoOwner: "myorg",
		RepoName:  "myrepo",
		MRNumber:  123,
		EventType: "mr_comment",
		Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	logPath, err := writer.Create(entry)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Check directory exists
	dir := filepath.Dir(logPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Log directory should exist")
	}

	// Check filename format
	filename := filepath.Base(logPath)
	if !strings.Contains(filename, "mr_comment") {
		t.Errorf("Filename %q should contain event type", filename)
	}
	if !strings.Contains(filename, "test-agent") {
		t.Errorf("Filename %q should contain agent ID", filename)
	}
}

func TestLogWriter_Append_MultipleWrites(t *testing.T) {
	baseDir := t.TempDir()
	writer := NewWriter(baseDir)

	entry := LogEntry{
		AgentID:   "agent-456",
		RepoOwner: "owner",
		RepoName:  "repo",
		MRNumber:  1,
		EventType: "mention",
		Timestamp: time.Now(),
	}

	logPath, err := writer.Create(entry)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Multiple appends
	writer.Append(logPath, []byte("line 1\n"))
	writer.Append(logPath, []byte("line 2\n"))
	writer.Append(logPath, []byte("line 3\n"))

	// Read and verify
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	expected := "line 1\nline 2\nline 3\n"
	if string(content) != expected {
		t.Errorf("Content = %q, want %q", string(content), expected)
	}
}

func TestLogWriter_Append_NonexistentFile(t *testing.T) {
	baseDir := t.TempDir()
	writer := NewWriter(baseDir)

	err := writer.Append("/nonexistent/path/file.log", []byte("data"))
	if err == nil {
		t.Error("Append() should error for nonexistent file")
	}
}
