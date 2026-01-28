package logging

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestCleanupScheduler_StartStop(t *testing.T) {
	baseDir := t.TempDir()
	cleaner := NewCleaner(baseDir, 30)
	scheduler := NewCleanupScheduler(cleaner, 100*time.Millisecond)

	// Start should not block
	scheduler.Start()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	scheduler.Stop()
}

func TestCleanupScheduler_CleanupCalled(t *testing.T) {
	baseDir := t.TempDir()

	// Create an old file that will be cleaned up
	dir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(dir, 0755)
	oldFile := filepath.Join(dir, "old.log")
	os.WriteFile(oldFile, []byte("old"), 0644)
	os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))

	cleaner := NewCleaner(baseDir, 30)
	scheduler := NewCleanupScheduler(cleaner, 50*time.Millisecond)

	scheduler.Start()

	// Wait for at least one cleanup cycle
	time.Sleep(100 * time.Millisecond)

	scheduler.Stop()

	// Verify the old file was cleaned up
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should have been deleted by scheduled cleanup")
	}
}

func TestCleanupScheduler_MultipleIntervals(t *testing.T) {
	baseDir := t.TempDir()

	var cleanupCount int32

	// We'll create files in a way that we can count cleanups
	// Create the first old file
	dir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(dir, 0755)

	cleaner := NewCleaner(baseDir, 30)
	scheduler := NewCleanupScheduler(cleaner, 30*time.Millisecond)

	// Create a file before each expected cleanup
	go func() {
		for i := 0; i < 3; i++ {
			oldFile := filepath.Join(dir, "old"+string(rune('0'+i))+".log")
			os.WriteFile(oldFile, []byte("old"), 0644)
			os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))
			time.Sleep(30 * time.Millisecond)
			// After each interval, check if file was cleaned
			if _, err := os.Stat(oldFile); os.IsNotExist(err) {
				atomic.AddInt32(&cleanupCount, 1)
			}
		}
	}()

	scheduler.Start()

	// Wait for multiple cycles
	time.Sleep(150 * time.Millisecond)

	scheduler.Stop()

	count := atomic.LoadInt32(&cleanupCount)
	if count < 2 {
		t.Errorf("Expected at least 2 cleanup cycles, got %d", count)
	}
}

func TestCleanupScheduler_StopPreventsCleanup(t *testing.T) {
	baseDir := t.TempDir()

	// Create an old file
	dir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(dir, 0755)
	oldFile := filepath.Join(dir, "old.log")
	os.WriteFile(oldFile, []byte("old"), 0644)
	os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))

	cleaner := NewCleaner(baseDir, 30)
	scheduler := NewCleanupScheduler(cleaner, 100*time.Millisecond)

	scheduler.Start()

	// Stop immediately before cleanup runs
	scheduler.Stop()

	// Wait to ensure no cleanup runs after stop
	time.Sleep(150 * time.Millisecond)

	// File should still exist since we stopped before cleanup could run
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Error("File should still exist since scheduler was stopped before cleanup")
	}
}

func TestNewCleanupScheduler(t *testing.T) {
	baseDir := t.TempDir()
	cleaner := NewCleaner(baseDir, 30)
	interval := 1 * time.Hour

	scheduler := NewCleanupScheduler(cleaner, interval)

	if scheduler.cleaner != cleaner {
		t.Error("Scheduler should have the provided cleaner")
	}
	if scheduler.ticker == nil {
		t.Error("Scheduler should have a ticker")
	}
	if scheduler.stop == nil {
		t.Error("Scheduler should have a stop channel")
	}

	// Clean up the ticker
	scheduler.ticker.Stop()
}
