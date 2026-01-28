package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestCleanup_EmptyDirectories(t *testing.T) {
	baseDir := t.TempDir()

	// Create a directory with only an old file
	oldDir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(oldDir, 0755)
	oldFile := filepath.Join(oldDir, "old.log")
	os.WriteFile(oldFile, []byte("old"), 0644)
	os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))

	cleaner := NewCleaner(baseDir, 30)
	_, err := cleaner.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// The empty directory should be cleaned up
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("Empty directory should be deleted")
	}
}

func TestCleanup_NoOldFiles(t *testing.T) {
	baseDir := t.TempDir()

	// Create only recent logs
	recentDir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(recentDir, 0755)
	recentFile := filepath.Join(recentDir, "recent.log")
	os.WriteFile(recentFile, []byte("recent"), 0644)

	cleaner := NewCleaner(baseDir, 30)
	deleted, err := cleaner.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}

	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("Recent file should still exist")
	}
}

func TestCleanup_EmptyBaseDir(t *testing.T) {
	baseDir := t.TempDir()

	cleaner := NewCleaner(baseDir, 30)
	deleted, err := cleaner.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestCleanup_NonexistentBaseDir(t *testing.T) {
	cleaner := NewCleaner("/nonexistent/path", 30)
	deleted, err := cleaner.Cleanup()

	// Should handle gracefully without error
	if err != nil {
		t.Fatalf("Cleanup() error = %v, want nil", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestCleanup_MultipleOldFiles(t *testing.T) {
	baseDir := t.TempDir()

	// Create multiple old files across different directories
	for i := 1; i <= 3; i++ {
		dir := filepath.Join(baseDir, "owner", "repo", string(rune('0'+i)))
		os.MkdirAll(dir, 0755)
		oldFile := filepath.Join(dir, "old.log")
		os.WriteFile(oldFile, []byte("old"), 0644)
		os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))
	}

	cleaner := NewCleaner(baseDir, 30)
	deleted, err := cleaner.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}
}

func TestCleaner_RetentionDays(t *testing.T) {
	baseDir := t.TempDir()

	// Create file that's 10 days old
	dir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(dir, 0755)
	file := filepath.Join(dir, "test.log")
	os.WriteFile(file, []byte("data"), 0644)
	os.Chtimes(file, time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, -10))

	// With 7 days retention, file should be deleted
	cleaner7 := NewCleaner(baseDir, 7)
	deleted, err := cleaner7.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1 (7-day retention)", deleted)
	}

	// Recreate file
	os.MkdirAll(dir, 0755)
	os.WriteFile(file, []byte("data"), 0644)
	os.Chtimes(file, time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, -10))

	// With 30 days retention, file should NOT be deleted
	cleaner30 := NewCleaner(baseDir, 30)
	deleted, err = cleaner30.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0 (30-day retention)", deleted)
	}
}
