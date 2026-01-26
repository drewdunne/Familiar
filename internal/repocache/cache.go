package repocache

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Cache manages bare git repo clones.
type Cache struct {
	baseDir string
	mu      sync.Mutex
}

// New creates a new repo cache at the given directory.
func New(baseDir string) *Cache {
	return &Cache{baseDir: baseDir}
}

// EnsureRepo ensures a bare clone of the repo exists and is up to date.
// Returns the path to the bare repo.
func (c *Cache) EnsureRepo(ctx context.Context, cloneURL, owner, repo string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	repoPath := filepath.Join(c.baseDir, owner, repo+".git")

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Clone bare repo
		if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
			return "", fmt.Errorf("creating cache directory: %w", err)
		}

		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", cloneURL, repoPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("cloning repo: %w: %s", err, output)
		}
	} else {
		// Fetch updates
		cmd := exec.CommandContext(ctx, "git", "fetch", "--all")
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("fetching repo: %w: %s", err, output)
		}
	}

	return repoPath, nil
}

// RepoPath returns the path where a repo would be cached.
func (c *Cache) RepoPath(owner, repo string) string {
	return filepath.Join(c.baseDir, owner, repo+".git")
}
