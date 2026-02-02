# Agent Provider Credentials

## Problem

Agent containers cannot interact with the GitLab/GitHub API. The primary container has provider tokens in its config, but they are not forwarded to agent containers. Agents also lack the `glab` and `gh` CLI tools.

This causes 403 errors when agents attempt to read MR comments, post replies, or interact with the provider API.

## Design

Three changes, all minimal and building on existing plumbing.

### 1. Provider Interface: `AgentEnv()`

Add `AgentEnv() map[string]string` to the `Provider` interface. Each implementation returns the env vars its CLI tool needs for authentication.

**GitLab** returns:
- `GITLAB_TOKEN` — the PAT (always)
- `GITLAB_HOST` — the base URL (only if configured, for self-hosted instances)

**GitHub** returns:
- `GITHUB_TOKEN` — the PAT

Both `glab` and `gh` CLIs read these env vars natively, so no `auth login` step is needed inside the container.

### 2. Handler Wiring

`AgentHandler.Handle` already looks up the event's provider via `h.registry.Get(evt.Provider)`. After that lookup, call `provider.AgentEnv()` and pass the result into `SpawnRequest.Env`.

`SpawnRequest.Env` already exists and the spawner already iterates over it to inject env vars into containers. No structural changes needed.

Only the event's provider credentials are injected — not all configured providers.

### 3. Dockerfile: Install CLI Tools

Add `glab` and `gh` to the agent image (`docker/agent/Dockerfile`). Both are available as Alpine packages. If not available in the base repos for node:22-alpine, fall back to downloading release binaries.

## Scope

- Git push already works (worktree inherits authenticated remote URL)
- No merge capability — agents can read, comment, and push only
- No changes to config schema, SpawnRequest struct, or spawner logic

## Files Changed

- `internal/provider/provider.go` — add `AgentEnv()` to interface
- `internal/provider/gitlab/gitlab.go` — implement `AgentEnv()`
- `internal/provider/github/github.go` — implement `AgentEnv()`
- `internal/handler/agent.go` — populate `SpawnRequest.Env`
- `docker/agent/Dockerfile` — install `glab` and `gh`
- Tests for each provider's `AgentEnv()` implementation
