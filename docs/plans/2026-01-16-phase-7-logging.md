# Phase 7: Logging + Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-6.

**Goal:** Capture agent output to filesystem, organize logs by repo/MR/timestamp, implement configurable retention with automatic cleanup.

**Tech Stack:** Go 1.25, existing packages

**Prerequisites:** Phases 1-6 complete

---

## Task 1: Log Writer (TDD)

**Files:**
- Create: `internal/logging/writer.go`
- Create: `internal/logging/writer_test.go`

**Step 1: Write failing tests**

```go
package logging

import (
	"os"
	"path/filepath"
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
```

**Step 2: Implement writer**

```go
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LogEntry struct {
	AgentID   string
	RepoOwner string
	RepoName  string
	MRNumber  int
	EventType string
	Timestamp time.Time
}

type Writer struct {
	baseDir string
}

func NewWriter(baseDir string) *Writer {
	return &Writer{baseDir: baseDir}
}

func (w *Writer) Create(entry LogEntry) (string, error) {
	dir := filepath.Join(
		w.baseDir,
		entry.RepoOwner,
		entry.RepoName,
		fmt.Sprint(entry.MRNumber),
	)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating log directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s-%s.log",
		entry.Timestamp.Format("2006-01-02T15-04-05"),
		entry.EventType,
		entry.AgentID,
	)

	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("creating log file: %w", err)
	}
	f.Close()

	return path, nil
}

func (w *Writer) Append(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}
```

---

## Task 2: Log Capture from Container

**Files:**
- Modify: `internal/agent/spawner.go`
- Create: `internal/logging/capture.go`

Add method to capture container logs and write to log file when agent completes.

```go
func (s *Spawner) CaptureAndStop(ctx context.Context, sessionID string, logPath string) error {
	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}

	// Get container logs
	logs, err := s.client.GetContainerLogs(ctx, session.ContainerID)
	if err != nil {
		return err
	}
	defer logs.Close()

	// Write to log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	io.Copy(f, logs)

	return s.Stop(ctx, sessionID)
}
```

---

## Task 3: Cleanup Routine (TDD)

**Files:**
- Create: `internal/logging/cleanup.go`
- Create: `internal/logging/cleanup_test.go`

**Step 1: Write failing test**

```go
func TestCleanup_OldLogs(t *testing.T) {
	baseDir := t.TempDir()

	// Create old log
	oldDir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(oldDir, 0755)
	oldFile := filepath.Join(oldDir, "2020-01-01T00-00-00-mr_opened-agent.log")
	os.WriteFile(oldFile, []byte("old"), 0644)
	os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))

	// Create recent log
	recentDir := filepath.Join(baseDir, "owner", "repo", "2")
	os.MkdirAll(recentDir, 0755)
	recentFile := filepath.Join(recentDir, "recent.log")
	os.WriteFile(recentFile, []byte("recent"), 0644)

	cleaner := NewCleaner(baseDir, 30) // 30 days retention
	deleted, err := cleaner.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be deleted")
	}
	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("Recent file should still exist")
	}
}
```

**Step 2: Implement cleaner**

```go
package logging

import (
	"os"
	"path/filepath"
	"time"
)

type Cleaner struct {
	baseDir       string
	retentionDays int
}

func NewCleaner(baseDir string, retentionDays int) *Cleaner {
	return &Cleaner{baseDir: baseDir, retentionDays: retentionDays}
}

func (c *Cleaner) Cleanup() (int, error) {
	threshold := time.Now().AddDate(0, 0, -c.retentionDays)
	var deleted int

	err := filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Before(threshold) {
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
		return nil
	})

	// Clean up empty directories
	c.cleanEmptyDirs()

	return deleted, err
}

func (c *Cleaner) cleanEmptyDirs() {
	filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == c.baseDir {
			return nil
		}
		entries, _ := os.ReadDir(path)
		if len(entries) == 0 {
			os.Remove(path)
		}
		return nil
	})
}
```

---

## Task 4: Background Cleanup Scheduler

**Files:**
- Create: `internal/logging/scheduler.go`

```go
package logging

import (
	"log"
	"time"
)

type CleanupScheduler struct {
	cleaner *Cleaner
	ticker  *time.Ticker
	stop    chan struct{}
}

func NewCleanupScheduler(cleaner *Cleaner, interval time.Duration) *CleanupScheduler {
	return &CleanupScheduler{
		cleaner: cleaner,
		ticker:  time.NewTicker(interval),
		stop:    make(chan struct{}),
	}
}

func (s *CleanupScheduler) Start() {
	go func() {
		for {
			select {
			case <-s.ticker.C:
				deleted, err := s.cleaner.Cleanup()
				if err != nil {
					log.Printf("Cleanup error: %v", err)
				} else if deleted > 0 {
					log.Printf("Cleaned up %d old log files", deleted)
				}
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *CleanupScheduler) Stop() {
	s.ticker.Stop()
	close(s.stop)
}
```

---

## Task 5: Run Full Test Suite

Verify coverage >= 80%

---

## Summary

| Task | Component | Tests |
|------|-----------|-------|
| 1 | Log writer | 1 |
| 2 | Log capture | - |
| 3 | Cleanup routine | 1 |
| 4 | Scheduler | - |
| 5 | Coverage | - |

**Total: 5 tasks, ~2 tests**
