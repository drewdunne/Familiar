# Run All Containers as Host User — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate the UID mismatch between the Familiar server container (root) and agent containers (UID 1000) so that worktrees created by the server are writable by the agent.

**Architecture:** Pass the host user's UID/GID into the Familiar server container via docker-compose so it runs as the same user. The agent container already resolves to the same UID from the claude auth dir. This aligns all file ownership: host user creates worktrees, agent runs as same UID, no permission issues.

**Tech Stack:** Docker, docker-compose, Go, shell

---

## Root Cause

| Component | Current UID | Source |
|-----------|-------------|--------|
| Host user | 1000 | System user `drewdunne` |
| Familiar server (Docker) | 0 (root) | No `USER` in Dockerfile |
| Repo cache / worktrees | 0 (root) | Created by Familiar server |
| Claude auth dir | 1000 | Host bind mount |
| Agent container | 1000 | Resolved from claude auth dir |

Agent (UID 1000) can't write to worktrees (owned by UID 0). Fix: run Familiar as UID 1000 so worktrees are created as UID 1000.

## Prerequisites

- Host user must be in the `docker` group (verified: `drewdunne` is in `docker` group)
- Docker socket must be group-accessible (default on most distros)

---

### Task 1: Update docker-compose.yml to run as host user

**Files:**
- Modify: `docker-compose.yml`

**Step 1: Add user directive to docker-compose.yml**

Add `user: "${UID}:${GID}"` to the familiar service so it runs as the host user instead of root.

```yaml
services:
  familiar:
    build: .
    network_mode: host
    user: "${UID}:${GID}"
    volumes:
      # ... existing volumes
```

**Step 2: Verify docker-compose config parses**

Run: `UID=$(id -u) GID=$(id -g) docker compose -f docker-compose.yml config`
Expected: Valid YAML output with `user: "1000:1000"` (or whatever the host UID/GID is)

**Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "fix: run familiar server as host user to fix worktree permissions"
```

---

### Task 2: Update Dockerfile for non-root operation

**Files:**
- Modify: `Dockerfile`

**Step 1: Ensure directories are world-writable or created at runtime**

The current Dockerfile creates `/etc/familiar`, `/var/log/familiar`, `/var/cache/familiar` as root. Since volumes override these at runtime, we just need to ensure the binary is accessible. However, if no volume is mounted, the dirs need to be writable.

Change the `mkdir` to create dirs with open permissions:

```dockerfile
# Create directories with open permissions (actual dirs come from volume mounts)
RUN mkdir -p /etc/familiar /var/log/familiar /var/cache/familiar && \
    chmod 777 /var/log/familiar /var/cache/familiar
```

No `USER` directive — the UID is set at runtime via docker-compose `user:`.

**Step 2: Rebuild the image**

Run: `docker compose build familiar`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "fix: make server dirs writable for non-root operation"
```

---

### Task 3: Update config.example.yaml with documentation

**Files:**
- Modify: `config.example.yaml`

**Step 1: Add comment about UID requirement**

Add a comment near the top of the file explaining the docker-compose `user:` requirement:

```yaml
# IMPORTANT: The docker-compose.yml runs Familiar as your host user (UID/GID).
# Start with: UID=$(id -u) GID=$(id -g) docker compose up
# This ensures worktrees created by the server are writable by agent containers.
```

**Step 2: Commit**

```bash
git add config.example.yaml
git commit -m "docs: add UID/GID startup instructions to config example"
```

---

### Task 4: Simplify resolveContainerUser to use current process UID

**Files:**
- Modify: `internal/agent/spawner.go`
- Test: `internal/agent/spawner_test.go`

**Context:** Currently `resolveContainerUser` stats the claude auth dir to determine the UID. Since the Familiar server now runs as the host user, the agent should run as the same UID. We can simplify: use `os.Getuid()` directly instead of stat-ing the auth dir. This removes the `ClaudeAuthMountDir` complexity entirely.

**Step 1: Write failing test**

Add a test that verifies the spawner uses the current process UID:

```go
func TestResolveContainerUser_UsesProcessUID(t *testing.T) {
    expected := fmt.Sprintf("%d", os.Getuid())
    got := resolveContainerUser()
    if got != expected {
        t.Errorf("resolveContainerUser() = %q, want %q", got, expected)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestResolveContainerUser_UsesProcessUID -v`
Expected: FAIL (wrong signature — current function takes two string args)

**Step 3: Simplify resolveContainerUser**

```go
// resolveContainerUser returns the UID of the current process as a string.
// Since Familiar runs as the host user (via docker-compose user:), agent
// containers should run as the same UID for consistent file ownership.
func resolveContainerUser() string {
    return fmt.Sprintf("%d", os.Getuid())
}
```

Update the call site in `Spawn()` (line 92):
```go
containerUser := resolveContainerUser()
```

Remove `resolveUID` helper (no longer needed).

Update `SpawnerConfig` to remove `ClaudeAuthMountDir` field.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run TestResolveContainerUser_UsesProcessUID -v`
Expected: PASS

**Step 5: Fix any other tests broken by signature change**

Run: `go test ./internal/agent/ -v`
Expected: All pass

**Step 6: Commit**

```bash
git add internal/agent/spawner.go internal/agent/spawner_test.go
git commit -m "refactor: simplify container user resolution to use process UID"
```

---

### Task 5: Remove ClaudeAuthMountDir from config and main.go

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/familiar/main.go`

**Step 1: Remove ClaudeAuthMountDir from AgentsConfig**

In `config.go`, remove:
```go
ClaudeAuthMountDir string `yaml:"claude_auth_mount_dir"` // Container-local path for UID resolution
```

**Step 2: Remove ClaudeAuthMountDir from main.go SpawnerConfig**

In `main.go`, remove the `ClaudeAuthMountDir` field from the `agent.SpawnerConfig{}` struct literal.

**Step 3: Remove from SpawnerConfig struct**

In `spawner.go`, remove:
```go
ClaudeAuthMountDir string // Container-local path — used for UID resolution when running in Docker
```

**Step 4: Remove from config.example.yaml**

Remove the `claude_auth_mount_dir` comment/line.

**Step 5: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 6: Run vet**

Run: `go vet ./...`
Expected: No issues

**Step 7: Commit**

```bash
git add internal/config/config.go cmd/familiar/main.go internal/agent/spawner.go config.example.yaml
git commit -m "chore: remove ClaudeAuthMountDir config (no longer needed)"
```

---

### Task 6: Integration verification

**Step 1: Rebuild and restart**

```bash
UID=$(id -u) GID=$(id -g) docker compose up --build -d
```

**Step 2: Verify Familiar runs as host user**

Run: `docker exec familiar-familiar-1 id`
Expected: `uid=1000 gid=1000`

**Step 3: Verify worktree ownership**

Trigger a test webhook or inspect existing cache:
Run: `docker exec familiar-familiar-1 ls -ln /cache/`
Expected: Files owned by `1000:1000`, not `0:0`

**Step 4: Verify agent can write to workspace**

Trigger an agent spawn and check logs for the "read-only" error.
Expected: Agent successfully writes to `/workspace`

---

## Out of Scope

- **Push permission prompt issue**: The "no permission to push" message is prompt-driven (`push_commits: "on_request"` without explicit push request in comment). This is separate from the filesystem bug and works as designed.
- **Worktree cleanup**: Existing root-owned worktrees in the cache will need manual cleanup (`sudo chown -R 1000:1000 $REPO_CACHE_DIR`) or a fresh cache.
