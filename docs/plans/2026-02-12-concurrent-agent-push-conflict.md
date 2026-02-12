# Concurrent Agent Push Conflict on Same MR Branch

**Date:** 2026-02-12
**Status:** Open
**Severity:** High — agents silently fail to push completed work

## Problem

When multiple webhook events fire for the same MR (comments, mentions, review notes), Familiar spawns a new agent for each event. Every agent gets its own git worktree of the same source branch, makes changes, and tries to push. Only the first agent's push succeeds — subsequent agents fail because the remote branch has diverged.

## Observed Behavior

On MR #2 (`feature/config-module` in `drewdunne/tinyhost`), the reflog shows:

1. Agent `1770920891` committed and pushed `chore(api): add ellipsis to server started message`
2. Agent `1770926000` committed `chore(api): add exclamation mark to server started message` — pushed successfully
3. Agent `1770927984` committed, then had to `pull --rebase` before pushing
4. Agent `1770928771` committed, had to `reset` and retry
5. Agent `1770934569` committed `refactor(discord-bot): clarify deploy-commands test descriptions`, attempted rebase twice, failed 3 push attempts, and gave up

The agent reported: "All 101 tests pass locally, but I'm unable to push due to concurrent activity on this branch (3 attempts failed)."

## Root Cause

`AgentHandler.Handle()` spawns agents fire-and-forget with no coordination:

- No check for existing running agents on the same MR/branch
- No queuing — every webhook event immediately spawns a new agent
- Each agent's worktree is created from the current branch tip at spawn time
- By the time a later agent pushes, the remote has moved forward

## Impact

- Agent work is lost (committed locally in a worktree but never pushed)
- Wasted compute — agent runs for minutes, produces valid changes, then can't deliver them
- Confusing MR comments — agent reports it made changes but couldn't push

## Potential Solutions

### Option A: Single-agent-per-MR lock

Before spawning, check if an agent is already running for the same `owner/repo/MR`. If so, either:
- Queue the new event and process it after the current agent finishes
- Drop the event and let the running agent handle it (if it's already working on the same feedback)

### Option B: Pull-rebase before push (agent-side)

Teach the agent prompt to always `git pull --rebase` before pushing. This is fragile — merge conflicts can occur, and it doesn't solve the fundamental concurrency problem.

### Option C: Event coalescing

Batch rapid-fire events for the same MR into a single agent invocation with a combined prompt. Use a short debounce window (e.g., 30 seconds) to collect events before spawning.

### Recommended

Option A (lock) + Option C (coalescing) together. The lock prevents races, and coalescing reduces unnecessary agent spawns for rapid comment threads.

## Related: Agent Lifecycle Cleanup

`Handle()` is fire-and-forget — it spawns the agent but never waits for completion. This causes three types of resource leak:

1. **Exited containers accumulate** — 38 dead containers observed on 2026-02-12 (small ~5MB, but unbounded growth)
2. **Orphaned worktrees accumulate** — 66 worktrees totaling 1.3GB observed on 2026-02-12 (this is the real disk eater)
3. **Log files are never populated** — `CaptureAndStop()` exists but is never called (fixed separately: tmux `tee` + `cat` for Docker log capture)

### Proposed Fix: Agent Completion Watcher

Add a background goroutine that monitors running agent containers for exit, then:

1. Calls `CaptureAndStop()` to write logs and remove the container
2. Calls `RepoCache.RemoveWorktree()` to clean up the worktree
3. Removes the session from the spawner's tracking map

This watcher would complement the existing `startTimeoutWatcher()` and could share the same polling loop. The `OnTimeout` callback already exists but isn't wired to cleanup — this fix would close that gap too.

### Related Issues

- Agent log files are 0 bytes (fixed: tmux now tees output to stdout via `/tmp/claude-output.log`)
- Concurrent agents on same MR branch (main issue in this doc)
