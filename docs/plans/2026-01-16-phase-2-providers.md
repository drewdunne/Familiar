# Phase 2: Git Provider Abstraction + Repo Caching Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phase 1. Review Phase 1 implementation before starting.

**Goal:** Create provider abstraction for GitLab/GitHub APIs, implement PAT authentication, and build repo caching with git worktrees.

**Architecture:** Provider interface abstracts git provider operations. Repo cache maintains bare clones with worktrees created per-agent session. Authentication uses PAT tokens from config.

**Tech Stack:** Go 1.25, go-github/v60, go-gitlab, git CLI

**Prerequisites:** Phase 1 complete (config, server, webhook handlers)

---

## Task 1: Define Provider Interface

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/types.go`

**Step 1: Create types file**

Create `internal/provider/types.go`:
```go
package provider

import "time"

// MergeRequest represents a merge request/pull request.
type MergeRequest struct {
	ID           int
	Number       int       // PR number (GitHub) or MR IID (GitLab)
	Title        string
	Description  string
	SourceBranch string
	TargetBranch string
	State        string    // open, closed, merged
	Author       string
	URL          string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Comment represents a comment on a merge request.
type Comment struct {
	ID        int
	Body      string
	Author    string
	CreatedAt time.Time
}

// ChangedFile represents a file changed in a merge request.
type ChangedFile struct {
	Path      string
	Status    string // added, modified, deleted, renamed
	Additions int
	Deletions int
}

// Repository represents a git repository.
type Repository struct {
	ID        int
	Name      string
	FullName  string // owner/repo
	CloneURL  string
	SSHURL    string
	DefaultBranch string
}
```

**Step 2: Create provider interface**

Create `internal/provider/provider.go`:
```go
package provider

import "context"

// Provider defines the interface for git provider operations.
type Provider interface {
	// Name returns the provider name (github, gitlab).
	Name() string

	// GetRepository fetches repository metadata.
	GetRepository(ctx context.Context, owner, repo string) (*Repository, error)

	// GetMergeRequest fetches a merge request by number.
	GetMergeRequest(ctx context.Context, owner, repo string, number int) (*MergeRequest, error)

	// GetChangedFiles returns files changed in a merge request.
	GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]ChangedFile, error)

	// PostComment posts a comment on a merge request.
	PostComment(ctx context.Context, owner, repo string, number int, body string) error

	// GetComments fetches comments on a merge request.
	GetComments(ctx context.Context, owner, repo string, number int) ([]Comment, error)
}
```

**Step 3: Commit**

```bash
git add internal/provider/
git commit -m "feat(provider): define provider interface and types"
```

---

## Task 2: GitHub Provider - Repository and MR Fetching (TDD)

**Files:**
- Create: `internal/provider/github/github.go`
- Create: `internal/provider/github/github_test.go`

**Step 1: Write failing test**

Create `internal/provider/github/github_test.go`:
```go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubProvider_GetRepository(t *testing.T) {
	// Mock GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or incorrect authorization header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             123,
			"name":           "repo",
			"full_name":      "owner/repo",
			"clone_url":      "https://github.com/owner/repo.git",
			"ssh_url":        "git@github.com:owner/repo.git",
			"default_branch": "main",
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	repo, err := p.GetRepository(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}

	if repo.FullName != "owner/repo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "owner/repo")
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestGitHubProvider_GetMergeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     999,
			"number": 42,
			"title":  "Test PR",
			"body":   "Description",
			"state":  "open",
			"head":   map[string]string{"ref": "feature"},
			"base":   map[string]string{"ref": "main"},
			"user":   map[string]string{"login": "author"},
			"html_url": "https://github.com/owner/repo/pull/42",
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	mr, err := p.GetMergeRequest(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetMergeRequest() error = %v", err)
	}

	if mr.Number != 42 {
		t.Errorf("Number = %d, want %d", mr.Number, 42)
	}
	if mr.Title != "Test PR" {
		t.Errorf("Title = %q, want %q", mr.Title, "Test PR")
	}
	if mr.SourceBranch != "feature" {
		t.Errorf("SourceBranch = %q, want %q", mr.SourceBranch, "feature")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/provider/github/... -v
```

Expected: FAIL - package doesn't exist.

**Step 3: Implement GitHub provider**

Create `internal/provider/github/github.go`:
```go
package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/drewdunne/familiar/internal/provider"
	"github.com/google/go-github/v60/github"
)

// GitHubProvider implements provider.Provider for GitHub.
type GitHubProvider struct {
	client *github.Client
	token  string
}

// Option configures the GitHub provider.
type Option func(*GitHubProvider)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) Option {
	return func(p *GitHubProvider) {
		p.client.BaseURL, _ = p.client.BaseURL.Parse(url + "/")
	}
}

// New creates a new GitHub provider.
func New(token string, opts ...Option) *GitHubProvider {
	httpClient := &http.Client{
		Transport: &tokenTransport{token: token},
	}
	client := github.NewClient(httpClient)

	p := &GitHubProvider{
		client: client,
		token:  token,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// tokenTransport adds authorization header to requests.
type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

// Name returns the provider name.
func (p *GitHubProvider) Name() string {
	return "github"
}

// GetRepository fetches repository metadata.
func (p *GitHubProvider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	r, _, err := p.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("fetching repository: %w", err)
	}

	return &provider.Repository{
		ID:            int(r.GetID()),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		CloneURL:      r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		DefaultBranch: r.GetDefaultBranch(),
	}, nil
}

// GetMergeRequest fetches a pull request by number.
func (p *GitHubProvider) GetMergeRequest(ctx context.Context, owner, repo string, number int) (*provider.MergeRequest, error) {
	pr, _, err := p.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("fetching pull request: %w", err)
	}

	return &provider.MergeRequest{
		ID:           int(pr.GetID()),
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		Description:  pr.GetBody(),
		SourceBranch: pr.GetHead().GetRef(),
		TargetBranch: pr.GetBase().GetRef(),
		State:        pr.GetState(),
		Author:       pr.GetUser().GetLogin(),
		URL:          pr.GetHTMLURL(),
		CreatedAt:    pr.GetCreatedAt().Time,
		UpdatedAt:    pr.GetUpdatedAt().Time,
	}, nil
}

// GetChangedFiles returns files changed in a pull request.
func (p *GitHubProvider) GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]provider.ChangedFile, error) {
	files, _, err := p.client.PullRequests.ListFiles(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing changed files: %w", err)
	}

	result := make([]provider.ChangedFile, len(files))
	for i, f := range files {
		result[i] = provider.ChangedFile{
			Path:      f.GetFilename(),
			Status:    f.GetStatus(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
		}
	}
	return result, nil
}

// PostComment posts a comment on a pull request.
func (p *GitHubProvider) PostComment(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := p.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("posting comment: %w", err)
	}
	return nil
}

// GetComments fetches comments on a pull request.
func (p *GitHubProvider) GetComments(ctx context.Context, owner, repo string, number int) ([]provider.Comment, error) {
	comments, _, err := p.client.Issues.ListComments(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}

	result := make([]provider.Comment, len(comments))
	for i, c := range comments {
		result[i] = provider.Comment{
			ID:        int(c.GetID()),
			Body:      c.GetBody(),
			Author:    c.GetUser().GetLogin(),
			CreatedAt: c.GetCreatedAt().Time,
		}
	}
	return result, nil
}
```

**Step 4: Add dependency**

```bash
go get github.com/google/go-github/v60
go mod tidy
```

**Step 5: Run tests**

```bash
go test ./internal/provider/github/... -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/provider/github/ go.mod go.sum
git commit -m "feat(provider): implement GitHub provider"
```

---

## Task 3: GitLab Provider (TDD)

**Files:**
- Create: `internal/provider/gitlab/gitlab.go`
- Create: `internal/provider/gitlab/gitlab_test.go`

**Step 1: Write failing test**

Create `internal/provider/gitlab/gitlab_test.go`:
```go
package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLabProvider_GetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner%2Frepo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Errorf("missing or incorrect token header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":                123,
			"name":              "repo",
			"path_with_namespace": "owner/repo",
			"http_url_to_repo":  "https://gitlab.com/owner/repo.git",
			"ssh_url_to_repo":   "git@gitlab.com:owner/repo.git",
			"default_branch":    "main",
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	repo, err := p.GetRepository(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("GetRepository() error = %v", err)
	}

	if repo.FullName != "owner/repo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "owner/repo")
	}
}

func TestGitLabProvider_GetMergeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/owner%2Frepo/merge_requests/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            999,
			"iid":           42,
			"title":         "Test MR",
			"description":   "Description",
			"state":         "opened",
			"source_branch": "feature",
			"target_branch": "main",
			"author":        map[string]string{"username": "author"},
			"web_url":       "https://gitlab.com/owner/repo/-/merge_requests/42",
		})
	}))
	defer server.Close()

	p := New("test-token", WithBaseURL(server.URL))
	mr, err := p.GetMergeRequest(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetMergeRequest() error = %v", err)
	}

	if mr.Number != 42 {
		t.Errorf("Number = %d, want %d", mr.Number, 42)
	}
	if mr.Title != "Test MR" {
		t.Errorf("Title = %q, want %q", mr.Title, "Test MR")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/provider/gitlab/... -v
```

Expected: FAIL

**Step 3: Implement GitLab provider**

Create `internal/provider/gitlab/gitlab.go`:
```go
package gitlab

import (
	"context"
	"fmt"
	"net/url"

	"github.com/drewdunne/familiar/internal/provider"
	"github.com/xanzy/go-gitlab"
)

// GitLabProvider implements provider.Provider for GitLab.
type GitLabProvider struct {
	client *gitlab.Client
}

// Option configures the GitLab provider.
type Option func(*GitLabProvider)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(baseURL string) Option {
	return func(p *GitLabProvider) {
		p.client, _ = gitlab.NewClient(p.client.Token(), gitlab.WithBaseURL(baseURL+"/api/v4"))
	}
}

// New creates a new GitLab provider.
func New(token string, opts ...Option) *GitLabProvider {
	client, _ := gitlab.NewClient(token)
	p := &GitLabProvider{client: client}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider name.
func (p *GitLabProvider) Name() string {
	return "gitlab"
}

// projectPath encodes owner/repo for GitLab API.
func projectPath(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

// GetRepository fetches repository metadata.
func (p *GitLabProvider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	project, _, err := p.client.Projects.GetProject(projectPath(owner, repo), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching project: %w", err)
	}

	return &provider.Repository{
		ID:            project.ID,
		Name:          project.Name,
		FullName:      project.PathWithNamespace,
		CloneURL:      project.HTTPURLToRepo,
		SSHURL:        project.SSHURLToRepo,
		DefaultBranch: project.DefaultBranch,
	}, nil
}

// GetMergeRequest fetches a merge request by IID.
func (p *GitLabProvider) GetMergeRequest(ctx context.Context, owner, repo string, number int) (*provider.MergeRequest, error) {
	mr, _, err := p.client.MergeRequests.GetMergeRequest(projectPath(owner, repo), number, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching merge request: %w", err)
	}

	return &provider.MergeRequest{
		ID:           mr.ID,
		Number:       mr.IID,
		Title:        mr.Title,
		Description:  mr.Description,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
		Author:       mr.Author.Username,
		URL:          mr.WebURL,
		CreatedAt:    *mr.CreatedAt,
		UpdatedAt:    *mr.UpdatedAt,
	}, nil
}

// GetChangedFiles returns files changed in a merge request.
func (p *GitLabProvider) GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]provider.ChangedFile, error) {
	changes, _, err := p.client.MergeRequests.GetMergeRequestChanges(projectPath(owner, repo), number, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching merge request changes: %w", err)
	}

	result := make([]provider.ChangedFile, len(changes.Changes))
	for i, c := range changes.Changes {
		status := "modified"
		if c.NewFile {
			status = "added"
		} else if c.DeletedFile {
			status = "deleted"
		} else if c.RenamedFile {
			status = "renamed"
		}
		result[i] = provider.ChangedFile{
			Path:   c.NewPath,
			Status: status,
		}
	}
	return result, nil
}

// PostComment posts a comment on a merge request.
func (p *GitLabProvider) PostComment(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := p.client.Notes.CreateMergeRequestNote(projectPath(owner, repo), number, &gitlab.CreateMergeRequestNoteOptions{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("posting comment: %w", err)
	}
	return nil
}

// GetComments fetches comments on a merge request.
func (p *GitLabProvider) GetComments(ctx context.Context, owner, repo string, number int) ([]provider.Comment, error) {
	notes, _, err := p.client.Notes.ListMergeRequestNotes(projectPath(owner, repo), number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}

	result := make([]provider.Comment, len(notes))
	for i, n := range notes {
		result[i] = provider.Comment{
			ID:        n.ID,
			Body:      n.Body,
			Author:    n.Author.Username,
			CreatedAt: *n.CreatedAt,
		}
	}
	return result, nil
}
```

**Step 4: Add dependency**

```bash
go get github.com/xanzy/go-gitlab
go mod tidy
```

**Step 5: Run tests**

```bash
go test ./internal/provider/gitlab/... -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/provider/gitlab/ go.mod go.sum
git commit -m "feat(provider): implement GitLab provider"
```

---

## Task 4: Provider Registry

**Files:**
- Create: `internal/provider/registry.go`
- Create: `internal/provider/registry_test.go`

**Step 1: Write failing test**

Create `internal/provider/registry_test.go`:
```go
package provider

import (
	"testing"

	"github.com/drewdunne/familiar/internal/config"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
// ... implement other methods as no-ops for test

func TestRegistry_Get(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			GitHub: config.GitHubConfig{Token: "gh-token"},
			GitLab: config.GitLabConfig{Token: "gl-token"},
		},
	}

	reg := NewRegistry(cfg)

	gh := reg.Get("github")
	if gh == nil {
		t.Error("Get(github) returned nil")
	}
	if gh.Name() != "github" {
		t.Errorf("github provider name = %q, want %q", gh.Name(), "github")
	}

	gl := reg.Get("gitlab")
	if gl == nil {
		t.Error("Get(gitlab) returned nil")
	}
	if gl.Name() != "gitlab" {
		t.Errorf("gitlab provider name = %q, want %q", gl.Name(), "gitlab")
	}

	unknown := reg.Get("unknown")
	if unknown != nil {
		t.Error("Get(unknown) should return nil")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/provider/... -v -run TestRegistry
```

**Step 3: Implement registry**

Create `internal/provider/registry.go`:
```go
package provider

import (
	"github.com/drewdunne/familiar/internal/config"
	"github.com/drewdunne/familiar/internal/provider/github"
	"github.com/drewdunne/familiar/internal/provider/gitlab"
)

// Registry manages provider instances.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry from config.
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
	}

	if cfg.Providers.GitHub.Token != "" {
		r.providers["github"] = github.New(cfg.Providers.GitHub.Token)
	}

	if cfg.Providers.GitLab.Token != "" {
		r.providers["gitlab"] = gitlab.New(cfg.Providers.GitLab.Token)
	}

	return r
}

// Get returns the provider for the given name, or nil if not configured.
func (r *Registry) Get(name string) Provider {
	return r.providers[name]
}

// List returns all configured provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
```

**Step 4: Run tests**

```bash
go test ./internal/provider/... -v
```

**Step 5: Commit**

```bash
git add internal/provider/registry.go internal/provider/registry_test.go
git commit -m "feat(provider): add provider registry"
```

---

## Task 5: Repo Cache - Clone and Fetch (TDD)

**Files:**
- Create: `internal/repocache/cache.go`
- Create: `internal/repocache/cache_test.go`

**Step 1: Write failing test**

Create `internal/repocache/cache_test.go`:
```go
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
		{"touch", "README.md"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup command %v failed: %v", args, err)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/repocache/... -v
```

**Step 3: Implement cache**

Create `internal/repocache/cache.go`:
```go
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
```

**Step 4: Run tests**

```bash
go test ./internal/repocache/... -v
```

**Step 5: Commit**

```bash
git add internal/repocache/
git commit -m "feat(repocache): add bare repo cloning and caching"
```

---

## Task 6: Repo Cache - Worktree Management (TDD)

**Files:**
- Modify: `internal/repocache/cache.go`
- Modify: `internal/repocache/cache_test.go`

**Step 1: Write failing test**

Add to `internal/repocache/cache_test.go`:
```go
func TestCache_CreateWorktree(t *testing.T) {
	cacheDir := t.TempDir()
	sourceDir := t.TempDir()
	setupTestRepo(t, sourceDir)

	cache := New(cacheDir)

	// First ensure repo exists
	_, err := cache.EnsureRepo(context.Background(), sourceDir, "test-owner", "test-repo")
	if err != nil {
		t.Fatalf("EnsureRepo() error = %v", err)
	}

	// Create worktree
	worktreePath, err := cache.CreateWorktree(context.Background(), "test-owner", "test-repo", "main", "agent-123")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Verify worktree exists and has files
	if _, err := os.Stat(filepath.Join(worktreePath, "README.md")); os.IsNotExist(err) {
		t.Error("Expected README.md in worktree")
	}

	// Clean up worktree
	if err := cache.RemoveWorktree(context.Background(), "test-owner", "test-repo", "agent-123"); err != nil {
		t.Errorf("RemoveWorktree() error = %v", err)
	}

	// Verify worktree is gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("Worktree should be removed")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/repocache/... -v -run TestCache_CreateWorktree
```

**Step 3: Implement worktree methods**

Add to `internal/repocache/cache.go`:
```go
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
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, ref)
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
```

**Step 4: Run tests**

```bash
go test ./internal/repocache/... -v
```

**Step 5: Commit**

```bash
git add internal/repocache/
git commit -m "feat(repocache): add worktree creation and removal"
```

---

## Task 7: Update README with Phase 2 Setup Notes

**Files:**
- Modify: `README.md`

**Step 1: Add provider setup section**

Add detailed information about required token scopes:
- GitHub: `repo` scope for full repository access
- GitLab: `api` scope for API access

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add provider token scope requirements"
```

---

## Task 8: Run Full Test Suite

**Step 1: Run all tests with coverage**

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Step 2: Verify coverage >= 80%**

If below threshold, add tests for uncovered paths.

**Step 3: Commit if needed**

```bash
git add .
git commit -m "test: ensure 80% coverage for Phase 2"
```

---

## Summary

| Task | Component | Tests Added |
|------|-----------|-------------|
| 1 | Provider interface + types | - |
| 2 | GitHub provider | 2 |
| 3 | GitLab provider | 2 |
| 4 | Provider registry | 1 |
| 5 | Repo cache - clone/fetch | 1 |
| 6 | Repo cache - worktrees | 1 |
| 7 | README updates | - |
| 8 | Coverage verification | - |

**Total: 8 tasks, ~7 tests**
