package logging

import (
	"os"
	"path/filepath"
	"time"
)

// Cleaner handles cleanup of old log files based on a retention policy.
type Cleaner struct {
	baseDir       string
	retentionDays int
}

// NewCleaner creates a new Cleaner with the specified base directory and retention period.
func NewCleaner(baseDir string, retentionDays int) *Cleaner {
	return &Cleaner{baseDir: baseDir, retentionDays: retentionDays}
}

// Cleanup removes log files older than the retention period and cleans up empty directories.
// Returns the number of files deleted and any error encountered.
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

// cleanEmptyDirs removes empty directories within the base directory.
func (c *Cleaner) cleanEmptyDirs() {
	// Walk in reverse depth order to clean nested empty dirs first
	// We need multiple passes since removing a dir may make its parent empty
	for {
		removedAny := false
		filepath.Walk(c.baseDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() || path == c.baseDir {
				return nil
			}
			entries, _ := os.ReadDir(path)
			if len(entries) == 0 {
				if os.Remove(path) == nil {
					removedAny = true
				}
			}
			return nil
		})
		if !removedAny {
			break
		}
	}
}
