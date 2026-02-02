# Fix Claude Auth Dir Mount

**Date:** 2026-02-02
**Status:** Draft

## Problem

Agent containers don't receive the `.claude` directory, causing Claude CLI to
hit first-run interactive prompts (theme selection, etc.) and hang.

**Root cause:** `spawner.go` uses `os.Stat` to check if `ClaudeAuthDir` exists
before adding the Docker mount. This check runs inside the Familiar container,
where the host path doesn't exist. The Docker API resolves mount sources from
the host filesystem, so the check is wrong and the mount is silently skipped.

## Solution

1. **Remove the `os.Stat` guard** in `Spawn()`. If `ClaudeAuthDir` is
   configured (non-empty), add the mount unconditionally. Docker will return an
   error at container creation time if the path doesn't exist on the host.

2. **Add a startup warning** in `NewSpawner()` when `ClaudeAuthDir` is empty,
   so operators know agents will hit first-run prompts.

## Changes

**File:** `internal/agent/spawner.go`

### NewSpawner (~line 57)

```go
if cfg.ClaudeAuthDir == "" {
    log.Println("WARNING: claude_auth_dir not configured; agents will hit first-run prompts")
}
```

### Spawn (lines 92-101)

Before:
```go
if s.cfg.ClaudeAuthDir != "" {
    if _, err := os.Stat(s.cfg.ClaudeAuthDir); err == nil {
        mounts = append(mounts, docker.Mount{
            Source:   s.cfg.ClaudeAuthDir,
            Target:   "/home/agent/.claude",
            ReadOnly: true,
        })
    }
}
```

After:
```go
if s.cfg.ClaudeAuthDir != "" {
    mounts = append(mounts, docker.Mount{
        Source:   s.cfg.ClaudeAuthDir,
        Target:   "/home/agent/.claude",
        ReadOnly: true,
    })
}
```

## Testing

- Verify mount is added when `ClaudeAuthDir` is non-empty
- Verify mount is not added when `ClaudeAuthDir` is empty
- Verify warning is logged when `ClaudeAuthDir` is empty

## Files touched

- `internal/agent/spawner.go`
- `internal/agent/spawner_test.go`
