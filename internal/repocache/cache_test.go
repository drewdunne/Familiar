package repocache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCache_EnsureRepo(t *testing.T) {
	// Create a temp directory for the cache
	cacheDir := t.TempDir()

	// Create a temp git repo to clone from
	sourceDir := t.TempDir()
	setupTestRepo(t, sourceDir)

	cache := New(cacheDir)

	// First call should clone
	repoPath, err := cache.EnsureRepo(context.Background(), sourceDir, "test-owner", "test-repo")
	if err != nil {
		t.Fatalf("EnsureRepo() error = %v", err)
	}

	// Verify it's a bare repo
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); os.IsNotExist(err) {
		t.Error("Expected bare repo (HEAD file at root)")
	}

	// Second call should fetch, not clone
	repoPath2, err := cache.EnsureRepo(context.Background(), sourceDir, "test-owner", "test-repo")
	if err != nil {
		t.Fatalf("EnsureRepo() second call error = %v", err)
	}

	if repoPath != repoPath2 {
		t.Errorf("Second call returned different path: %q vs %q", repoPath, repoPath2)
	}
}

func setupTestRepo(t *testing.T, dir string) {
	t.Helper()
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup command %v failed: %v", args, err)
		}
	}

	// Create a file and commit
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
}
