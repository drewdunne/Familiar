# Debug Report: Agent Containers Not Responding to Tags

**Date:** 2026-02-02
**Status:** In progress — UID fix complete, secondary issue remains
**Original symptom:** Worker agents "not doing anything" when tagged via `@familiar` in MR comments

## What Was Fixed

### Root Cause #1: UID Mismatch (RESOLVED)

The agent container ran as `agent` (uid=1001), but the Claude auth directory mounted from the host is owned by `drewdunne` (uid=1000). The `.credentials.json` file has `600` permissions, so the agent couldn't read it. Claude Code started, failed to authenticate, and exited after ~20 seconds.

**Fix applied in these files:**

- `internal/agent/spawner.go` — Added `resolveUID()` and `resolveContainerUser()` functions that stat the `ClaudeAuthDir` to determine the owner's UID. Added `ClaudeAuthMountDir` fallback for when familiar itself runs in Docker (host path not accessible, but container-local mount is). The resolved UID is passed as the Docker container `User`.
- `internal/docker/client.go` — Added `User` field to `ContainerConfig`, passed through to `container.Config.User`.
- `docker-compose.yml` — Added `${CLAUDE_AUTH_DIR}:/claude-auth:ro` volume mount so the familiar server container can stat the auth dir for UID resolution.
- `config.yaml` — Added `claude_auth_mount_dir: "/claude-auth"` to agents config.
- `cmd/familiar/main.go` — Wired `ClaudeAuthMountDir` from config to spawner.

**Tests added:**
- `TestResolveUID` — table-driven test for the UID resolution function
- `TestResolveContainerUser` — tests the fallback logic (auth dir → mount dir → empty)
- `TestSpawner_Spawn_SetsContainerUser` — integration test verifying container User is set

**Verification:** After fix, `docker inspect` shows container `User: "1000"` and credentials are readable inside the container.

### Root Cause #2: HOME Directory Mismatch (RESOLVED)

The container runs as UID 1000 which maps to user `node` (home: `/home/node`) in the node-based agent image. But the Claude auth dir is mounted at `/home/agent/.claude`. Claude Code looks at `$HOME/.claude`, so it was finding an empty directory at `/home/node/.claude/` instead of the mounted credentials at `/home/agent/.claude/`.

**Fix:** Set `HOME=/home/agent` in the container environment when `ClaudeAuthDir` is configured (`internal/agent/spawner.go`).

### Root Cause #3: Read-Only Auth Mount (RESOLVED)

The auth dir was mounted `:ro`. Claude Code needs to write session state, settings, etc. Changed to read-write in `internal/agent/spawner.go`. See `docs/plans/2026-02-02-claude-auth-mount-isolation.md` for future improvement notes.

## Current Blocker: Claude Code Silent Exit

### Symptom

After all three fixes above, the agent container starts, Claude Code launches, authenticates successfully (confirmed via NODE_DEBUG), but **exits with code 0 and produces zero output** within 2-3 seconds. No stdout, no stderr, empty tmux pane.

### Evidence Gathered

1. **Claude authenticates successfully.** NODE_DEBUG output showed a STREAM receiving `{"account":{"uuid":"a41b5ff9-...` — account data from the API.

2. **Exit code is always 0.** No error indication.

3. **Zero output in all modes:**
   - `claude -p "hello"` — no stdout
   - `claude -p "hello" --output-format text` — no stdout
   - `claude -p "hello" --output-format json` — no stdout
   - `claude --dangerously-skip-permissions "hello"` in tmux — empty pane
   - `claude --dangerously-skip-permissions` (no prompt) in tmux — empty pane
   - `claude` (no flags at all) in tmux — empty pane

4. **Only works when credentials are ABSENT.** When `$HOME/.claude/.credentials.json` doesn't exist, Claude shows the first-run setup screen (theme selection TUI). With credentials present, it exits silently.

5. **Credentials are valid.** OAuth token `expiresAt: 1770084132102` (Feb 3, 2026 ~23:22 UTC) — not expired.

6. **Container environment is correct:**
   - User: 1000 (matching credentials file owner)
   - HOME: /home/agent
   - Credentials readable: confirmed via `cat`
   - tmux: works correctly
   - node: v22.22.0
   - claude: v2.1.29

### Hypotheses NOT YET TESTED

1. **Read-only credentials file blocks token refresh.** The credentials file was mounted `:ro` in the debug tests. Claude may try to refresh/write the OAuth token on startup. If it fails silently, it exits. **Next step:** Test with a fully writable copy of `.credentials.json` (not a read-only mount).

2. **OAuth token is machine-bound.** The token may be tied to the host machine and rejected when used from a different network context (container). **Next step:** Check Claude Code logs or add more NODE_DEBUG analysis.

3. **Claude Code detects non-primary session.** The `session-env/` directory has 142 entries. Claude might detect concurrent usage and exit. **Next step:** Test with a clean `.claude` dir containing ONLY `.credentials.json`.

4. **`/home/agent` directory permissions.** The `/home/agent` directory is owned by uid=1001 (`agent`). Even though `HOME=/home/agent` and `.claude/` is writable, Claude might try to write to `$HOME` directly (e.g., `.config/`, `.local/`) and fail. **Next step:** Test with HOME set to a fully writable tmpdir (was about to test when context ran out).

### Recommended Next Steps

The most promising hypothesis is #1 or #4. The next agent should:

1. **Test with fully writable home + credentials copy:**
   ```bash
   TMPHOME=$(mktemp -d)
   mkdir -p "$TMPHOME/.claude"
   cp ~/.claude/.credentials.json "$TMPHOME/.claude/"
   cp ~/.claude/settings.json "$TMPHOME/.claude/"

   docker run --rm --entrypoint="/bin/sh" \
     --user 1000 -e "HOME=$TMPHOME" -v "$TMPHOME:$TMPHOME" \
     familiar-agent:latest \
     -c 'timeout 20 claude -p hello 2>/tmp/err 1>/tmp/out; echo "exit=$?"; cat /tmp/out; cat /tmp/err'
   ```

2. **If that works:** The issue is write permissions. Adjust the spawner to either:
   - Copy credentials to a writable tmpdir before launching claude
   - Or ensure the entire `$HOME` is writable

3. **If that doesn't work:** The issue is deeper. Try:
   - Capture full NODE_DEBUG output during a `-p hello` run and analyze the API interaction
   - Check if `CLAUDE_CODE_SKIP_ONBOARDING=1` or similar env vars exist
   - Try `claude api ...` subcommand to test raw API access
   - Check if the agent image needs rebuilding (maybe claude binary is stale)

## Files Changed (Summary)

| File | Change |
|------|--------|
| `internal/agent/spawner.go` | Added `resolveUID`, `resolveContainerUser`, `ClaudeAuthMountDir` config, `HOME` env var, `User` on container config, read-write mount |
| `internal/agent/spawner_test.go` | Added `TestResolveUID`, `TestResolveContainerUser`, `TestSpawner_Spawn_SetsContainerUser` |
| `internal/docker/client.go` | Added `User` field to `ContainerConfig`, passed to Docker API |
| `internal/config/config.go` | Added `ClaudeAuthMountDir` to `AgentsConfig` |
| `cmd/familiar/main.go` | Wired `ClaudeAuthMountDir` config to spawner |
| `docker-compose.yml` | Added `${CLAUDE_AUTH_DIR}:/claude-auth:ro` volume mount |
| `config.yaml` | Added `claude_auth_mount_dir: "/claude-auth"` |
| `config.example.yaml` | Added commented `claude_auth_mount_dir` |
| `internal/prompt/builder.go` | Pre-existing: added MR title, description, comment body to prompts |
| `internal/prompt/builder_test.go` | Pre-existing: tests for prompt builder changes |

## Feature Notes Created

- `docs/plans/2026-02-02-windows-uid-resolution.md` — Windows `syscall.Stat_t` limitation
- `docs/plans/2026-02-02-claude-auth-mount-isolation.md` — Future: isolate agent auth state per container

## Test Status

All tests pass. `go test ./...` and `go vet ./...` clean.

## Stale Containers

There are old agent containers still running from previous test iterations. Clean up with:
```bash
docker ps -a --filter "label=familiar.agent=true" -q | xargs docker rm -f
```
