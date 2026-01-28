# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Familiar - A self-hosted service that bridges git provider webhooks to Claude Code agents.
Go 1.24+ with standard library patterns.

## Development Philosophy

### TDD is mandatory

- Write the test FIRST
- Verify it FAILS before implementing
- Write minimal code to pass
- Verify it PASSES
- No production code without a failing test first

### Testing rules

- Mock external services only (GitHub, GitLab, Claude CLI)
- Use table-driven tests for Go
- 80% coverage floor enforced by CI

### Git workflow

- Commit when a work item is complete (tests pass, lint clean)
- Leave pushing to the user - never push automatically
- Use conventional commit messages (fix:, feat:, chore:, etc.)
- MR titles must start with a ticket number (e.g., `DEV-155: Add user authentication`)
  - If no ticket number is provided, prompt the user for one before creating the MR

## Feature Development Workflow

This workflow ensures human oversight while leveraging AI for speed. AI is treated as a capable junior developer whose output must be reviewed.

### Phase 1: Brainstorm & Plan

Before writing code, clarify what you're building:

1. Use `superpowers:brainstorming` to explore requirements
2. Break ambiguous features into clear, testable pieces
3. Create a worktree for isolation: `superpowers:using-git-worktrees`
4. Use `superpowers:writing-plans` to create implementation plan

**Outputs** (saved to `docs/plans/`):
- Design doc: `YYYY-MM-DD-<topic>-design.md`
- Implementation plan: `YYYY-MM-DD-<feature-name>.md`

### Phase 2: TDD Loop (repeat for each piece)

```
┌─────────────────────────────────────────────┐
│  1. Human writes/approves test first        │
│     - AI can draft, but human must review   │
│     - Test should express INTENT, not impl  │
├─────────────────────────────────────────────┤
│  2. Verify test FAILS                       │
│     - If it passes, test is wrong           │
│     - This proves the test is meaningful    │
├─────────────────────────────────────────────┤
│  3. AI implements minimal code to pass      │
│     - Use superpowers:test-driven-development│
│     - Keep changes small and focused        │
├─────────────────────────────────────────────┤
│  4. Verify test PASSES                      │
│     - Run go test ./...                     │
├─────────────────────────────────────────────┤
│  5. Human reviews the diff                  │
│     - Understand what was written           │
│     - Check for architectural fit           │
│     - No rubber-stamping                    │
└─────────────────────────────────────────────┘
```

**Two-prompt rule**: If AI can't get it right in 2 attempts, break the task smaller.

### Phase 3: Review & Merge

1. Run full test suite: `go test ./...`
2. Run linter: `go vet ./...`
3. Use `superpowers:verification-before-completion`
4. Use `superpowers:requesting-code-review`
5. Human reviews and approves the PR
6. Merge only after CI passes AND human approval

### Key Principles

| Principle | Why |
|-----------|-----|
| No code to main without human review | AI produces functional but architecturally weak code |
| Tests express intent, not implementation | Keeps AI from "gaming" tests |
| Small, focused changes | Easier to review, less drift |
| Human writes/approves tests first | Tests are the spec - human controls WHAT, AI handles HOW |
| Understand before merging | You own this code now |

### Anti-patterns to Avoid

- One-shot "perfect spec" attempts for exploratory features
- Merging without reading the diff
- Letting AI write tests AND implementation without reviewing tests first
- Batching too many changes in one PR
- Trusting coverage numbers over test quality

## Commands

### Core commands

```bash
go build -o familiar ./cmd/familiar  # Build the binary
go test ./...                         # Run all tests
go test -race -coverprofile=coverage.out ./...  # Test with coverage
go tool cover -func=coverage.out      # View coverage report
go vet ./...                          # Run linter
go fmt ./...                          # Format code
```

### Running the server

```bash
./familiar serve --config config.yaml  # Run the server
```

### Logs and debugging

**Log locations:**
- **Local development:** `./logs/` (configured in `config.yaml` → `logging.dir`)
- **Docker:** `docker logs familiar-familiar-1` (stdout/stderr)
- **Default path:** `/var/log/familiar` (if not overridden)

**Log directory structure:**
```
logs/
  <owner>/
    <repo>/
      <mr_number>/
        <timestamp>-<event_type>-<agent_id>.log
```

**Viewing Docker container logs:**
```bash
docker logs familiar-familiar-1 2>&1 | tail -100   # Recent logs
docker logs -f familiar-familiar-1                 # Follow logs
```

## Project Structure

```
cmd/
  familiar/           # Main entry point

internal/
  config/             # Configuration loading
  webhook/            # Webhook handling
  provider/           # Git provider interfaces (GitHub, GitLab)
  agent/              # Claude agent spawning
  server/             # HTTP server

docs/
  plans/              # Design docs and implementation plans
```

## Superpowers skills to use

- `test-driven-development`: Always for new code
- `requesting-code-review`: Before merging
- `receiving-code-review`: When processing MR feedback
- `git-worktrees`: For feature branches
- `subagent-driven-development`: For parallel implementation tasks
- `systematic-debugging`: When tests fail unexpectedly
- `verification-before-completion`: Before claiming work is done

## Receiving MR Feedback via GitLab

When asked to review and respond to MR comments, use the `superpowers:receiving-code-review` skill and follow these steps:

### 1. Fetch MR comments

```bash
# View MR with comments
glab mr view --comments

# Get detailed notes via API (includes note IDs)
glab api projects/:id/merge_requests/<MR_NUMBER>/notes
```

### 2. Get discussion thread IDs for replying

```bash
# List discussions with their IDs
glab api "projects/:id/merge_requests/<MR_NUMBER>/discussions?per_page=100" | \
  jq '.[] | select(.notes[0].type == "DiffNote") | {id: .id, note_ids: [.notes[].id], file: .notes[0].position.new_path}'
```

### 3. Reply to specific discussion threads

```bash
# Reply to a discussion thread (not a top-level comment)
glab api --method POST "projects/:id/merge_requests/<MR_NUMBER>/discussions/<DISCUSSION_ID>/notes" \
  -f body="Your response here"
```

### Key principles when responding

- **Verify before implementing** - Check the codebase before agreeing
- **No performative agreement** - Skip "Great point!" or "You're right!"
- **Technical acknowledgment only** - State what you'll do or push back with reasoning
- **Ask if unclear** - Don't implement partial understanding
- **Reply in threads** - Use discussion API, not top-level MR comments
