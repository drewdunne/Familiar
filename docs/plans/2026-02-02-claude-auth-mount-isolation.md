# Feature Note: Isolate Agent Claude Auth State

**Date:** 2026-02-02
**Status:** Future improvement
**Relates to:** `internal/agent/spawner.go` — auth dir mount

## Context

The agent container mounts the host's `~/.claude` directory so that Claude Code can authenticate. Currently this is mounted read-write so Claude can write settings and session state.

## Concerns

1. **Shared mutable state:** Multiple concurrent agents write to the same `~/.claude` directory, risking race conditions on session files, history, and todos.
2. **Host state mutation:** Agents can modify the host user's Claude Code settings (`settings.json`, `CLAUDE.md`, history, etc.).

## Proposed Solution

Mount only the credentials file read-only, and give each agent its own ephemeral Claude home:

```yaml
# Instead of mounting the entire ~/.claude directory:
mounts:
  - source: ~/.claude/.credentials.json
    target: /home/agent/.claude/.credentials.json
    readonly: true
  - source: ~/.claude/settings.json
    target: /home/agent/.claude/settings.json
    readonly: true
```

This way:
- Authentication works (credentials readable)
- Settings are preserved (theme, preferences)
- Each agent gets its own session state (no cross-contamination)
- Host `~/.claude` is never modified by agents

## Considerations

- Claude Code may require other files from `~/.claude` at startup (e.g. `settings.local.json`)
- Need to verify the minimal set of files Claude needs to skip first-run setup
- May need to pre-bake a default theme/settings into the agent image
