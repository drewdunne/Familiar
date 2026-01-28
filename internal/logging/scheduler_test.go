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

func TestCleanupScheduler_DoubleStop(t *testing.T) {
	baseDir := t.TempDir()
	cleaner := NewCleaner(baseDir, 30)
	scheduler := NewCleanupScheduler(cleaner, 100*time.Millisecond)

	scheduler.Start()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Double stop should not panic (idempotent)
	scheduler.Stop()
	scheduler.Stop()
}

func TestCleanupScheduler_InitialCleanup(t *testing.T) {
	baseDir := t.TempDir()

	// Create an old file that will be cleaned up
	dir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(dir, 0755)
	oldFile := filepath.Join(dir, "old.log")
	os.WriteFile(oldFile, []byte("old"), 0644)
	os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))

	cleaner := NewCleaner(baseDir, 30)
	// Use a long interval to ensure cleanup happens from initial run, not scheduled
	scheduler := NewCleanupScheduler(cleaner, 10*time.Second)

	scheduler.Start()

	// Wait for initial cleanup to run (should be immediate)
	time.Sleep(50 * time.Millisecond)

	scheduler.Stop()

	// Verify the old file was cleaned up by initial cleanup
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should have been deleted by initial cleanup")
	}
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

func TestCleanupScheduler_StopPreventsScheduledCleanup(t *testing.T) {
	baseDir := t.TempDir()

	// Create the directory structure but no old files yet
	dir := filepath.Join(baseDir, "owner", "repo", "1")
	os.MkdirAll(dir, 0755)

	cleaner := NewCleaner(baseDir, 30)
	// Use a long interval so scheduled cleanup won't run during our test
	scheduler := NewCleanupScheduler(cleaner, 500*time.Millisecond)

	scheduler.Start()

	// Wait for initial cleanup to complete (it runs in a goroutine)
	// Note: cleanEmptyDirs() runs multiple passes, so we need a generous wait
	time.Sleep(200 * time.Millisecond)

	// Stop before any scheduled cleanup runs
	scheduler.Stop()

	// Recreate the directory since initial cleanup may have removed it (it was empty)
	os.MkdirAll(dir, 0755)

	// Now create an old file after stop - this file should NOT be cleaned up
	// because the scheduler is stopped and initial cleanup already finished
	oldFile := filepath.Join(dir, "old.log")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	os.Chtimes(oldFile, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -60))

	// Wait to verify no cleanup runs
	time.Sleep(100 * time.Millisecond)

	// File should still exist since scheduler was stopped before this file was created
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Error("File should still exist since scheduler was stopped")
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
