# Agent Provider Credentials Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give agent containers authenticated access to the GitLab/GitHub API so they can read MR comments, post replies, and push commits.

**Architecture:** Add `AgentEnv()` to the Provider interface, refactor the handler to depend on interfaces (not concrete types) for testability, wire provider env vars into SpawnRequest.Env, and install `glab`/`gh` CLIs in the agent image.

**Tech Stack:** Go 1.24, Docker (Alpine), glab CLI, gh CLI

---

### Task 1: Add `AgentEnv()` to Provider Interface (DONE)

Already implemented. `AgentEnv() map[string]string` added to `provider.Provider` interface, implemented in both GitLab and GitHub providers, with passing tests.

**Files changed:**
- `internal/provider/provider.go`
- `internal/provider/gitlab/gitlab.go` (added `baseURL` field, `AgentEnv()` method)
- `internal/provider/github/github.go` (added `AgentEnv()` method)
- `internal/provider/gitlab/gitlab_test.go` (added `TestGitLabProvider_AgentEnv`)
- `internal/provider/github/github_test.go` (added `TestGitHubProvider_AgentEnv`)

---

### Task 2: Define Handler Dependency Interfaces

The handler currently depends on concrete types `*agent.Spawner`, `*repocache.Cache`, and `*registry.Registry`. Define interfaces in the handler package (Go idiom: accept interfaces, return structs) so the handler can be unit tested with mocks.

**Files:**
- Modify: `internal/handler/agent.go`

**Step 1: Write the failing test**

Add a test to `internal/handler/agent_test.go` that constructs an `AgentHandler` with mock implementations and calls `Handle()`. The test verifies that `SpawnRequest.Env` contains the provider's `AgentEnv()` values.

```go
// Mock types in agent_test.go
type mockSpawner struct {
	lastRequest agent.SpawnRequest
	spawnErr    error
}

func (m *mockSpawner) Spawn(ctx context.Context, req agent.SpawnRequest) (*agent.Session, error) {
	m.lastRequest = req
	if m.spawnErr != nil {
		return nil, m.spawnErr
	}
	return &agent.Session{ID: req.ID, ContainerID: "mock-container"}, nil
}

type mockRepoCache struct {
	ensureErr   error
	worktreeErr error
}

func (m *mockRepoCache) EnsureRepo(ctx context.Context, cloneURL, owner, repo string) (string, error) {
	if m.ensureErr != nil {
		return "", m.ensureErr
	}
	return "/cache/" + owner + "/" + repo + ".git", nil
}

func (m *mockRepoCache) CreateWorktree(ctx context.Context, owner, repo, ref, worktreeID string) (string, error) {
	if m.worktreeErr != nil {
		return "", m.worktreeErr
	}
	return "/cache/" + owner + "/" + repo + ".git/worktrees-data/" + worktreeID, nil
}

func (m *mockRepoCache) RemoveWorktree(ctx context.Context, owner, repo, worktreeID string) error {
	return nil
}

func (m *mockRepoCache) HostPath(containerPath string) string {
	return containerPath // No translation in tests
}

type mockProvider struct {
	name      string
	agentEnv  map[string]string
	authURL   string
	files     []provider.ChangedFile
	filesErr  error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) AgentEnv() map[string]string { return m.agentEnv }
func (m *mockProvider) AuthenticatedCloneURL(rawURL string) (string, error) {
	if m.authURL != "" {
		return m.authURL, nil
	}
	return rawURL, nil
}
func (m *mockProvider) GetChangedFiles(ctx context.Context, owner, repo string, number int) ([]provider.ChangedFile, error) {
	return m.files, m.filesErr
}
func (m *mockProvider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	return nil, nil
}
func (m *mockProvider) GetMergeRequest(ctx context.Context, owner, repo string, number int) (*provider.MergeRequest, error) {
	return nil, nil
}
func (m *mockProvider) PostComment(ctx context.Context, owner, repo string, number int, body string) error {
	return nil
}
func (m *mockProvider) GetComments(ctx context.Context, owner, repo string, number int) ([]provider.Comment, error) {
	return nil, nil
}

type mockRegistry struct {
	providers map[string]provider.Provider
}

func (m *mockRegistry) Get(name string) provider.Provider {
	return m.providers[name]
}
```

Then the actual test:

```go
func TestHandle_PassesProviderEnvToSpawner(t *testing.T) {
	ms := &mockSpawner{}
	mc := &mockRepoCache{}
	mp := &mockProvider{
		name: "gitlab",
		agentEnv: map[string]string{
			"GITLAB_TOKEN": "glpat-secret",
			"GITLAB_HOST":  "https://gitlab.example.com",
		},
	}
	mr := &mockRegistry{providers: map[string]provider.Provider{"gitlab": mp}}

	h := NewAgentHandler(ms, mc, mr, "", "")

	evt := &event.Event{
		Provider:     "gitlab",
		RepoOwner:    "owner",
		RepoName:     "repo",
		RepoURL:      "https://gitlab.example.com/owner/repo.git",
		MRNumber:     1,
		SourceBranch: "feature",
		Type:         event.TypeMROpened,
		Timestamp:    time.Now(),
	}

	err := h.Handle(context.Background(), evt, &config.MergedConfig{}, nil)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify provider env vars were passed to spawner
	if ms.lastRequest.Env == nil {
		t.Fatal("SpawnRequest.Env should not be nil")
	}
	if ms.lastRequest.Env["GITLAB_TOKEN"] != "glpat-secret" {
		t.Errorf("Env[GITLAB_TOKEN] = %q, want %q", ms.lastRequest.Env["GITLAB_TOKEN"], "glpat-secret")
	}
	if ms.lastRequest.Env["GITLAB_HOST"] != "https://gitlab.example.com" {
		t.Errorf("Env[GITLAB_HOST] = %q, want %q", ms.lastRequest.Env["GITLAB_HOST"], "https://gitlab.example.com")
	}
}

func TestHandle_NilProviderSkipsEnv(t *testing.T) {
	ms := &mockSpawner{}
	mc := &mockRepoCache{}
	mr := &mockRegistry{providers: map[string]provider.Provider{}}

	h := NewAgentHandler(ms, mc, mr, "", "")

	evt := &event.Event{
		Provider:     "unknown",
		RepoOwner:    "owner",
		RepoName:     "repo",
		RepoURL:      "https://example.com/owner/repo.git",
		MRNumber:     1,
		SourceBranch: "feature",
		Type:         event.TypeMROpened,
		Timestamp:    time.Now(),
	}

	err := h.Handle(context.Background(), evt, &config.MergedConfig{}, nil)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Env should be nil when no provider matched
	if ms.lastRequest.Env != nil {
		t.Errorf("SpawnRequest.Env = %v, want nil", ms.lastRequest.Env)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestHandle_PassesProviderEnvToSpawner -v`
Expected: FAIL — `NewAgentHandler` takes concrete types, not interfaces.

**Step 3: Define interfaces and refactor handler struct**

In `internal/handler/agent.go`, add these interfaces and update the struct:

```go
// AgentSpawner spawns agent containers.
type AgentSpawner interface {
	Spawn(ctx context.Context, req agent.SpawnRequest) (*agent.Session, error)
}

// RepoCache manages repository clones and worktrees.
type RepoCache interface {
	EnsureRepo(ctx context.Context, cloneURL, owner, repo string) (string, error)
	CreateWorktree(ctx context.Context, owner, repo, ref, worktreeID string) (string, error)
	RemoveWorktree(ctx context.Context, owner, repo, worktreeID string) error
	HostPath(containerPath string) string
}

// ProviderRegistry looks up configured providers by name.
type ProviderRegistry interface {
	Get(name string) provider.Provider
}
```

Update `AgentHandler` struct fields from concrete to interface types:
- `spawner *agent.Spawner` -> `spawner AgentSpawner`
- `repoCache *repocache.Cache` -> `repoCache RepoCache`
- `registry *registry.Registry` -> `registry ProviderRegistry`

Update `NewAgentHandler` signature to accept interfaces.

**Step 4: Wire `AgentEnv()` into Handle**

In the `Handle` method, after the provider lookup, add:

```go
var spawnEnv map[string]string
if provider != nil {
    spawnEnv = provider.AgentEnv()
}
```

And add `Env: spawnEnv` to the `SpawnRequest`.

**Step 5: Update main.go**

`cmd/familiar/main.go` line 97 passes concrete types — these already satisfy the new interfaces. No changes needed to the call site, but verify it compiles.

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/handler/ -v`
Expected: PASS — all existing tests + new tests pass.

**Step 7: Commit**

```bash
git add internal/handler/ internal/provider/
git commit -m "feat: pass provider credentials to agent containers

Add AgentEnv() to Provider interface. Refactor handler to use
interfaces for testability. Wire provider env vars into SpawnRequest."
```

---

### Task 3: Update Dockerfile with glab and gh CLIs

**Files:**
- Modify: `docker/agent/Dockerfile`

**Step 1: Add glab and gh to the Dockerfile**

Alpine packages: `github-cli` (for `gh`) and `glab` (for `glab`). Both are in the Alpine community repo. If the packages aren't available on the Alpine version used by node:22-alpine, fall back to binary downloads.

```dockerfile
RUN apk add --no-cache \
    git \
    tmux \
    openssh-client \
    ca-certificates \
    curl \
    bash \
    github-cli \
    glab
```

**Step 2: Verify packages are available**

Run: `docker build -t familiar-agent:test -f docker/agent/Dockerfile docker/agent/`
Expected: Build succeeds. If either package is unavailable, switch to binary install.

**Fallback (binary install):**

```dockerfile
# Install gh CLI
RUN curl -fsSL https://github.com/cli/cli/releases/latest/download/gh_*_linux_amd64.tar.gz | tar xz -C /usr/local/bin --strip-components=2 '*/bin/gh'

# Install glab CLI
RUN curl -fsSL https://gitlab.com/gitlab-org/cli/-/releases/permalink/latest/downloads/glab_*_linux_amd64.tar.gz | tar xz -C /usr/local/bin --strip-components=1 'bin/glab'
```

**Step 3: Commit**

```bash
git add docker/agent/Dockerfile
git commit -m "feat: add glab and gh CLIs to agent container"
```

---

### Task 4: Full Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass.

**Step 2: Run linter**

Run: `go vet ./...`
Expected: Clean.

**Step 3: Build the binary**

Run: `go build -o familiar ./cmd/familiar`
Expected: Compiles.

**Step 4: Build the agent image**

Run: `docker build -t familiar-agent:test -f docker/agent/Dockerfile docker/agent/`
Expected: Builds successfully with glab and gh available.
