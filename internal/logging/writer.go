package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogEntry contains metadata for creating a log file.
type LogEntry struct {
	AgentID   string
	RepoOwner string
	RepoName  string
	MRNumber  int
	EventType string
	Timestamp time.Time
}

// Writer manages log files organized by repository and MR.
type Writer struct {
	baseDir string
}

// NewWriter creates a new Writer with the specified base directory.
func NewWriter(baseDir string) *Writer {
	return &Writer{baseDir: baseDir}
}

// Create creates a new log file for the given entry and returns the path.
// Directory structure: baseDir/owner/repo/mrNumber/timestamp-eventType-agentID.log
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

// Append writes data to the specified log file.
func (w *Writer) Append(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}
