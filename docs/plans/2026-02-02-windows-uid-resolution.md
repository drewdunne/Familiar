# Feature Note: Windows Compatibility for UID Resolution

**Date:** 2026-02-02
**Status:** Future consideration
**Relates to:** `internal/agent/spawner.go` — `resolveUID()`

## Context

The agent spawner resolves the UID of the `claude_auth_dir` owner at runtime using `syscall.Stat_t` so that agent containers run as the same user who owns the credentials. This ensures the container can read files like `.credentials.json` regardless of host UID.

## Platform Support

- **Linux:** Fully supported.
- **macOS:** Fully supported.
- **Windows:** `syscall.Stat_t` is not available. The code will not compile on Windows.

## Why This Isn't a Problem Today

Docker bind-mount UID mapping doesn't apply on Windows — file ownership is handled differently by Docker Desktop's Linux VM layer. The UID mismatch problem that `resolveUID` solves doesn't exist in that environment.

## If Windows Build Support Is Needed

Add a build-tagged stub:

```
internal/agent/uid_unix.go    // +build !windows
internal/agent/uid_windows.go // +build windows (returns "", nil or a sensible default)
```

This is low priority since Familiar's primary deployment target is Linux servers.
