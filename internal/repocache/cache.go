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

// CreateWorktree creates a git worktree for the given ref.
// Returns the path to the worktree.
func (c *Cache) CreateWorktree(ctx context.Context, owner, repo, ref, worktreeID string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	repoPath := c.RepoPath(owner, repo)
	worktreePath := filepath.Join(repoPath, "worktrees-data", worktreeID)

	// Create worktree directory
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", fmt.Errorf("creating worktree directory: %w", err)
	}

	// Create worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", worktreePath, ref)
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("creating worktree: %w: %s", err, output)
	}

	return worktreePath, nil
}

// RemoveWorktree removes a git worktree.
func (c *Cache) RemoveWorktree(ctx context.Context, owner, repo, worktreeID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	repoPath := c.RepoPath(owner, repo)
	worktreePath := filepath.Join(repoPath, "worktrees-data", worktreeID)

	// Remove worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing worktree: %w: %s", err, output)
	}

	return nil
}

// WorktreePath returns the path where a worktree would be created.
func (c *Cache) WorktreePath(owner, repo, worktreeID string) string {
	return filepath.Join(c.RepoPath(owner, repo), "worktrees-data", worktreeID)
}
