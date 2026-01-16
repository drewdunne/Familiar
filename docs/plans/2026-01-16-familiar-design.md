# Familiar: Design Document

> A self-hosted service that bridges git provider webhooks to Claude Code agents,
> enabling autonomous code review and MR/PR interactions.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Configuration](#configuration)
4. [Agent Execution](#agent-execution)
5. [Testing Strategy](#testing-strategy)
6. [Continuous Integration](#continuous-integration)
7. [Implementation Phases](#implementation-phases)
8. [Future Considerations](#future-considerations)

---

## Overview

### Problem

Developers want AI assistance on merge requests—reviewing code, answering
questions, making requested changes—without manual intervention. Existing
solutions require API keys and per-token costs. Developers with Claude Code
Max subscriptions want to leverage that existing access.

### Solution

Familiar listens for webhooks from GitLab and GitHub, then spawns isolated
Claude Code agents in Docker containers to handle each event. Agents operate
on git worktrees, inherit project-specific context via claude.md files, and
can push commits, post comments, and (if permitted) merge PRs.

### Key Properties

- **Ephemeral agents**: One agent per event, terminated after completion
- **Isolated execution**: Agents run in Docker containers with mounted worktrees
- **Context-aware**: Agents run from the correct directory to inherit claude.md files
- **Configurable**: Server defaults + per-repo overrides for prompts and permissions
- **Observable**: Agents run in tmux sessions, logs preserved for debugging

---

## Architecture

### System Diagram

```
┌─────────────────┐     webhook      ┌─────────────────────────────────────┐
│  GitLab/GitHub  │ ───────────────► │            Familiar                 │
│                 │ ◄─────────────── │                                     │
└─────────────────┘   API calls      │  ┌─────────────┐  ┌──────────────┐  │
                                     │  │  Webhook    │  │   Config     │  │
                                     │  │  Server     │  │   Manager    │  │
                                     │  └──────┬──────┘  └──────────────┘  │
                                     │         │                           │
                                     │         ▼                           │
                                     │  ┌─────────────┐  ┌──────────────┐  │
                                     │  │   Event     │  │   Intent     │  │
                                     │  │   Router    │──│   Parser     │  │
                                     │  └──────┬──────┘  │   (LLM)      │  │
                                     │         │         └──────────────┘  │
                                     │         ▼                           │
                                     │  ┌─────────────┐  ┌──────────────┐  │
                                     │  │   Agent     │  │   Session    │  │
                                     │  │   Spawner   │──│   Manager    │  │
                                     │  └──────┬──────┘  └──────────────┘  │
                                     │         │                           │
                                     └─────────┼───────────────────────────┘
                                               │
                    ┌──────────────────────────┼──────────────────────────┐
                    │         Host Filesystem  │                          │
                    │                          ▼                          │
                    │  ┌─────────────────────────────────────────────┐    │
                    │  │  /var/cache/familiar/repos/                 │    │
                    │  │    └── repo-name (bare)                     │    │
                    │  │        └── worktrees/                       │    │
                    │  │            ├── agent-abc123/  ◄─── mounted  │    │
                    │  │            └── agent-def456/                │    │
                    │  └─────────────────────────────────────────────┘    │
                    │                          │                          │
                    │                          │ volume mount             │
                    │                          ▼                          │
                    │  ┌─────────────────────────────────────────────┐    │
                    │  │  Docker Container (agent-abc123)            │    │
                    │  │    ┌─────────────────────────────────┐      │    │
                    │  │    │  tmux: claude-code session      │      │    │
                    │  │    │  └── claude (autonomous mode)   │      │    │
                    │  │    └─────────────────────────────────┘      │    │
                    │  │    /workspace ◄── worktree mounted here     │    │
                    │  └─────────────────────────────────────────────┘    │
                    │                                                     │
                    │  ┌─────────────────────────────────────────────┐    │
                    │  │  /var/log/familiar/                         │    │
                    │  │    └── repo-name/mr-123/                    │    │
                    │  │        └── 2026-01-16T10-30-00-agent.log    │    │
                    │  └─────────────────────────────────────────────┘    │
                    └─────────────────────────────────────────────────────┘
```

### Components

**Webhook Server**
- Receives HTTP POST from GitLab/GitHub
- Verifies signatures using configured secrets
- Parses payload into normalized event structure

**Event Router**
- Determines event type (mr_opened, mr_comment, mr_updated, mention)
- Fetches repo-level config, merges with server defaults
- Applies debouncing for rapid events
- Checks if event type is enabled for this repo
- Queues event if at concurrency limit

**Intent Parser**
- Calls Claude API to extract user intent from comments
- Identifies requested actions (merge, approve, etc.)
- Returns structured list of requested actions
- Designed as pluggable interface (API now, CLI strategy possible later)

**Config Manager**
- Loads and watches server config file for hot-reload
- Fetches repo config from .familiar/config.yaml
- Merges configs with proper precedence

**Agent Spawner**
- Ensures bare repo cache exists, fetches latest
- Creates worktree for agent session
- Calculates working directory (LCA algorithm)
- Constructs agent prompt with permissions
- Spawns Docker container with worktree mounted

**Session Manager**
- Tracks active agent containers
- Enforces timeout, kills runaway agents
- Captures logs on completion
- Cleans up worktrees and containers

**Log Manager**
- Writes agent output to filesystem
- Organizes by repo/MR/timestamp
- Runs periodic cleanup based on retention config

---

## Configuration

### Server Config

Location: `/etc/familiar/config.yaml` (or path specified via `--config` flag)

```yaml
server:
  host: "0.0.0.0"
  port: 8080

logging:
  dir: "/var/log/familiar"
  retention_days: 30

concurrency:
  max_agents: 5
  queue_size: 20

agents:
  timeout_minutes: 30
  debounce_seconds: 10

repo_cache:
  dir: "/var/cache/familiar/repos"

providers:
  github:
    auth_method: "pat"  # "pat" now, "app" in future
    token: "${GITHUB_TOKEN}"
    webhook_secret: "${GITHUB_WEBHOOK_SECRET}"
  gitlab:
    auth_method: "pat"
    token: "${GITLAB_TOKEN}"
    webhook_secret: "${GITLAB_WEBHOOK_SECRET}"

llm:
  # Intent parsing strategy: "api" or "cli" (cli not yet implemented)
  strategy: "api"

  # API strategy config
  api:
    provider: "anthropic"
    model: "claude-sonnet-4-20250514"
    api_key: "${ANTHROPIC_API_KEY}"

  # CLI strategy config (future)
  # cli:
  #   timeout_seconds: 30

# Default prompts per event type
prompts:
  mr_opened: |
    Review this merge request for bugs, security issues, and code quality.
    Provide actionable feedback as inline comments.
    If changes are needed, list them clearly.
    If the MR looks good, approve it with a summary of what you reviewed.

  mr_comment: |
    A user has commented on this merge request.
    Address their question or request directly.
    If they asked for code changes, make them and push a commit.

  mr_updated: |
    New commits have been pushed to this merge request.
    Review the changes since your last review.
    Focus on whether previous feedback was addressed.

  mention: |
    You were mentioned in a comment.
    Follow the user's instructions precisely.

# Default permissions
permissions:
  merge: "never"
  approve: "never"
  push_commits: "on_request"
  dismiss_reviews: "never"

# Default enabled events
events:
  mr_opened: true
  mr_comment: true
  mr_updated: true
  mention: true
```

### Repo Config

Location: `.familiar/config.yaml` in the repository root

```yaml
# Override enabled events
events:
  mr_opened: true
  mr_comment: true
  mr_updated: false  # disable automatic re-review
  mention: true

# Override prompts
prompts:
  mr_opened: |
    Review with focus on:
    - API design consistency
    - Database query efficiency (watch for N+1)
    - Error handling patterns
    Refer to docs/CONTRIBUTING.md for our standards.

# Override permissions
permissions:
  merge: "on_request"
  push_commits: "always"

# Optional: custom Docker image for agents
agent_image: "ghcr.io/myorg/familiar-agent-custom:latest"
```

### Config Merging

Precedence (highest to lowest):
1. Repo config values
2. Server config values
3. Built-in defaults

Arrays are replaced, not merged. Maps are deep-merged.

### Environment Variables

Familiar loads environment variables from multiple sources:

1. **System environment** - Always read
2. **Env file** - Loaded if present (development convenience)

#### Env File Locations (checked in order)

- Path specified via `--env-file` flag
- `.env` in current working directory
- `/etc/familiar/familiar.env`

#### Example `.env` file

```bash
# .env (add to .gitignore)

# Git provider credentials
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
GITHUB_WEBHOOK_SECRET=your-webhook-secret
GITLAB_TOKEN=glpat-xxxxxxxxxxxx
GITLAB_WEBHOOK_SECRET=your-webhook-secret

# Intent parsing (API-based for now)
ANTHROPIC_API_KEY=sk-ant-xxxxxxxxxxxx
```

#### Deployment

**Systemd:**
```ini
[Service]
EnvironmentFile=/etc/familiar/familiar.env
ExecStart=/usr/local/bin/familiar serve --config /etc/familiar/config.yaml
```

**Docker:**
```bash
docker run -d \
  --env-file /etc/familiar/familiar.env \
  -v /etc/familiar:/etc/familiar:ro \
  familiar:latest
```

**Docker Compose:**
```yaml
services:
  familiar:
    image: familiar:latest
    env_file:
      - ./familiar.env
```

The `.env` file should never be committed to git. The repo includes a
`.env.example` template with placeholder values.

### Environment Variable Substitution

Config values starting with `${` and ending with `}` are substituted
from environment variables. Example: `${GITHUB_TOKEN}` reads from
the `GITHUB_TOKEN` environment variable.

### Authentication Architecture

Currently supports PAT (Personal Access Token) authentication for both providers.
The provider interface is designed to support additional auth methods in the future:

```go
type AuthMethod interface {
    GetToken(ctx context.Context) (string, error)
}

// Current
type PATAuth struct { ... }

// Future
type GitHubAppAuth struct { ... }
type GitLabOAuthAuth struct { ... }
```

This allows adding GitHub App or GitLab OAuth authentication without
changing the rest of the system.

---

## Agent Execution

### Working Directory Selection (LCA Algorithm)

Agents must run from the correct directory to inherit the appropriate
`claude.md` files. Familiar calculates the Least Common Ancestor (LCA)
of all files modified in the MR.

**Algorithm:**

1. Fetch list of modified files from MR/PR
2. Extract directory paths for each file
3. Find deepest common ancestor directory
4. Agent runs from this directory

**Example:**

Repository structure:
```
/repo
├── claude.md                    # Project-wide context
├── services/
│   ├── claude.md                # Services-specific context
│   ├── auth/
│   │   ├── claude.md            # Auth service context
│   │   └── handler.go
│   └── billing/
│       ├── claude.md            # Billing service context
│       └── handler.go
└── lib/
    └── utils.go
```

| Modified Files | LCA Directory | claude.md files inherited |
|----------------|---------------|---------------------------|
| `services/auth/handler.go` | `/repo/services/auth` | root, services, auth |
| `services/auth/handler.go`, `services/billing/handler.go` | `/repo/services` | root, services |
| `services/auth/handler.go`, `lib/utils.go` | `/repo` | root only |

**Edge cases:**

- Deleted files: Use parent directory of deleted file
- Files in repo root: LCA is repo root
- Empty MR (no files): LCA is repo root

### Prompt Construction

The agent prompt is assembled from multiple sources:

```
┌─────────────────────────────────────────────────────────────────┐
│ SYSTEM CONTEXT                                                  │
│ - You are an AI assistant working on merge request !123         │
│ - Repository: myorg/myrepo                                      │
│ - Branch: feature/add-login                                     │
│ - Target: main                                                  │
├─────────────────────────────────────────────────────────────────┤
│ BASE PROMPT (from config, per event type)                       │
│ - "Review this merge request for bugs, security issues..."      │
├─────────────────────────────────────────────────────────────────┤
│ USER INSTRUCTIONS (extracted from comment/description)          │
│ - "Also check for SQL injection vulnerabilities"                │
├─────────────────────────────────────────────────────────────────┤
│ PERMISSIONS                                                     │
│ - You MAY push commits to the MR branch                         │
│ - You MAY merge this MR (user requested)                        │
│ - You must NOT approve this MR                                  │
│ - You must NOT force push or push to protected branches         │
├─────────────────────────────────────────────────────────────────┤
│ SAFETY REMINDERS                                                │
│ - Branch protection is enabled; destructive actions will fail   │
│ - If uncertain, ask via comment rather than taking action       │
└─────────────────────────────────────────────────────────────────┘
```

### Permission Enforcement

Permissions are enforced via prompt instructions. The three levels:

| Level | Prompt instruction |
|-------|-------------------|
| `always` | "You SHOULD [action] when appropriate" |
| `on_request` | "You MAY [action]" (only if user requested) |
| `never` | "You must NOT [action] under any circumstances" |

**Branch protection as backstop:**

Even if an agent attempts a forbidden action, GitLab/GitHub branch
protection rules provide a hard backstop:

- No force push to protected branches
- No direct push to main (require MR)
- Require reviews before merge

Users MUST configure branch protection. This is documented as a
setup requirement.

### Intent Parser Interface

The intent parser is designed as a pluggable interface to support
multiple strategies:

```go
type IntentParser interface {
    Parse(ctx context.Context, comment string) (*ParsedIntent, error)
}

// Current implementation
type APIIntentParser struct { ... }

// Future implementation
type CLIIntentParser struct { ... }
```

**Why support multiple strategies?**

Some users prefer consolidating all LLM functionality under their
Max subscription rather than managing a separate API key—simplicity
of billing and access at the expense of slightly more service
complexity. The CLI strategy (future) spawns a lightweight agent
for intent parsing, keeping everything within the subscription.

### Claude Code Authentication

Agents use the host's Claude Code authentication (Max subscription).

**Setup (one-time):**
```bash
# On the host machine, authenticate Claude Code
claude login
```

**Container mounting:**
Familiar automatically mounts the Claude auth directory:
```
-v ${HOME}/.claude:/home/agent/.claude:ro
```

This allows all agents to use your authenticated session without
storing credentials in config files or environment variables.

### Docker Container

**Base image:** Alpine + Go 1.25 + git + tmux + Claude Code CLI

**Dockerfile:**
```dockerfile
FROM golang:1.25-alpine

RUN apk add --no-cache \
    git \
    tmux \
    openssh-client \
    ca-certificates

# Install Claude Code CLI
RUN go install github.com/anthropics/claude-code@latest

# Create non-root user
RUN adduser -D -h /home/agent agent
USER agent
WORKDIR /workspace

ENTRYPOINT ["/bin/sh"]
```

**Container lifecycle:**

1. Familiar creates worktree on host
2. Spawns container with worktree mounted at `/workspace`
3. Mounts Claude auth from host (`~/.claude`)
4. Starts tmux session inside container
5. Runs Claude Code with constructed prompt (autonomous mode)
6. On completion or timeout, captures logs
7. Stops and removes container
8. Cleans up worktree

**Debugging:**
```bash
# List active agents
docker ps --filter "label=familiar.agent"

# Attach to agent's tmux session
docker exec -it familiar-agent-abc123 tmux attach -t claude
```

---

## Testing Strategy

### Unit Tests

Isolated logic with no external dependencies:

| Component | Tests |
|-----------|-------|
| LCA algorithm | Various file path combinations, edge cases (deleted files, root-level files, empty MR) |
| Config parsing | YAML parsing, env var substitution, default values |
| Config merging | Server + repo config precedence, deep merge behavior |
| Webhook signature verification | Valid signatures, invalid signatures, missing headers |
| Webhook payload parsing | GitLab and GitHub payload formats, malformed payloads |
| Permission checking | always/on_request/never logic, user-requested action matching |
| Prompt construction | Template assembly, variable substitution, permission injection |
| Debounce logic | Rapid events, timeout behavior, queue ordering |
| Log retention | Age calculation, cleanup selection |

### Integration Tests

Component interaction, mocked external services:

| Flow | What's tested |
|------|---------------|
| Webhook → Event Router | Signature verified, event type detected, config fetched |
| Event Router → Agent Spawner | Debouncing, concurrency limits, queue behavior |
| Config Manager | Hot-reload on file change, repo config fetching |
| Git operations | Bare clone, worktree creation/cleanup, LCA directory selection |
| Intent Parser (API) | Request formatting, response parsing (mocked API) |
| Session Manager | Container tracking, timeout enforcement, cleanup |

**Test infrastructure:**
- `testcontainers-go` for real Docker container tests
- Mock HTTP servers for GitLab/GitHub API responses
- Recorded webhook payloads from real events

### End-to-End Tests

Full flows against real services. **Requires user-provided secrets and configuration.**

| Test | Requirements |
|------|--------------|
| Webhook receipt | Test repo with webhook configured, ngrok or similar for local dev |
| Full agent cycle | GitLab/GitHub tokens, test MR, Docker socket access |
| Comment posting | API tokens with write access |
| Commit pushing | API tokens, test branch |

**E2E test setup:**

When ready to run E2E tests, you'll need to:

1. Create a test repository on GitLab/GitHub
2. Configure webhook pointing to Familiar instance
3. Provide tokens in `.env.test`:
   ```bash
   # .env.test (never committed)
   TEST_GITHUB_TOKEN=ghp_xxxxxxxxxxxx
   TEST_GITHUB_REPO=youruser/familiar-test
   TEST_GITLAB_TOKEN=glpat-xxxxxxxxxxxx
   TEST_GITLAB_REPO=youruser/familiar-test
   ```
4. Run: `go test -tags=e2e ./...`

E2E tests are skipped by default (require `-tags=e2e` flag).

### Contract Tests

Validate assumptions about external API formats:

- GitLab webhook payload structure (recorded from real webhooks)
- GitHub webhook payload structure
- GitLab API response formats
- GitHub API response formats

Contract tests use recorded fixtures and alert when API formats change.

### Test Organization

```
familiar/
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go        # Unit tests
│   ├── webhook/
│   │   ├── handler.go
│   │   ├── handler_test.go       # Unit tests
│   │   └── testdata/
│   │       ├── github_pr_opened.json
│   │       └── gitlab_mr_opened.json
│   └── ...
├── test/
│   ├── integration/
│   │   ├── agent_spawn_test.go   # Integration tests
│   │   └── config_reload_test.go
│   ├── e2e/
│   │   ├── full_cycle_test.go    # E2E tests (build tag: e2e)
│   │   └── setup_test.go
│   └── fixtures/
│       └── ...                   # Recorded API responses
└── ...
```

---

## Continuous Integration

**Platform:** GitHub Actions

**Workflow (`.github/workflows/test.yml`):**
```yaml
name: Test

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...
      - name: Check coverage
        run: |
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$coverage < 80" | bc -l) )); then
            echo "Coverage ${coverage}% is below 80%"
            exit 1
          fi
```

**Branch protection on `main`:**
- Require PR (no direct push)
- Require status checks to pass
- Require 80% coverage (enforced in workflow)

---

## Implementation Phases

Development follows TDD and incremental PRs. Each phase is a reviewable chunk.

### Documentation Requirement

The README must include **comprehensive setup instructions** for developers
using each supported git provider (GitHub and GitLab). This includes:

- Account/token setup
- Webhook configuration (URL, events, secrets)
- Branch protection configuration (required)
- Repository config file setup (`.familiar/config.yaml`)
- Troubleshooting common issues

**At the end of each phase:** Review and update the README to ensure setup
instructions reflect any changes introduced in that phase.

---

### Phase 1: Project Skeleton + Webhook Server + Repo Init

**Goal:** Establish foundation, receive webhooks

- Create `~/repos/Familiar` directory
- Initialize Go module with Go 1.25
- Initialize git repo, push to GitHub
- Set up GitHub Actions CI (`.github/workflows/test.yml`)
- Configure branch protection on `main`:
  - Require PR (no direct push)
  - Require CI to pass
  - Require 80% coverage
- Project structure:
  ```
  familiar/
  ├── cmd/
  │   └── familiar/
  │       └── main.go
  ├── internal/
  │   ├── config/
  │   ├── webhook/
  │   └── server/
  ├── docs/
  │   └── plans/
  ├── .github/
  │   └── workflows/
  │       └── test.yml
  ├── .env.example
  ├── .gitignore
  ├── go.mod
  └── README.md
  ```
- HTTP server with `/webhook/github` and `/webhook/gitlab` endpoints
- Signature verification for both providers
- Config file loading (server config only)
- Health check endpoint (`/health`)
- **Tests:** Webhook parsing, signature verification (unit tests)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~10-15 files

---

### Phase 2: Git Provider Abstraction + Repo Caching

**Goal:** Interact with GitLab/GitHub APIs, manage local repo cache

- Provider interface:
  ```go
  type Provider interface {
      GetMergeRequest(ctx, repo, mrID) (*MergeRequest, error)
      GetChangedFiles(ctx, repo, mrID) ([]string, error)
      PostComment(ctx, repo, mrID, body) error
      // ... etc
  }
  ```
- GitLab provider implementation (PAT auth)
- GitHub provider implementation (PAT auth)
- Bare repo cloning and caching
- Worktree creation and cleanup
- **Tests:** Provider API mocking, worktree lifecycle (unit + integration)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~10-12 files
- **Note:** Integration tests against real APIs available if you provide tokens

---

### Phase 3: Event Routing + Config Merging

**Goal:** Route events, merge server + repo config

- Event type detection (mr_opened, mr_comment, mr_updated, mention)
- Normalized event structure across providers
- Fetch repo config from `.familiar/config.yaml`
- Config merging with proper precedence
- Debouncing logic for rapid events
- Event enabled/disabled checking
- **Tests:** Config merging, debounce behavior, event normalization (unit)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~8-10 files

---

### Phase 4: Intent Parsing

**Goal:** Extract user intent from comments

- Intent parser interface (for future CLI strategy)
- API-based implementation using Claude API
- Extract requested actions (merge, approve, etc.)
- Map parsed intent to permission checks
- **Tests:** Mocked API responses, intent extraction (unit + integration)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~5-8 files

---

### Phase 5: Docker Agent Spawning

**Goal:** Spawn isolated agent containers

- Agent container Dockerfile:
  - Alpine + Go 1.25 + git + tmux + Claude Code
- Agent spawner:
  - Create container with worktree mounted
  - Mount Claude auth from host
  - Start tmux session
  - Run Claude Code in autonomous mode
- Session manager:
  - Track active containers
  - Enforce concurrency limits
  - Queue overflow events
- **Tests:** testcontainers for spawn/cleanup, concurrency limits (integration)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~12-15 files
- **Note:** Requires Docker socket access for tests

---

### Phase 6: LCA Algorithm + Prompt Construction

**Goal:** Run agents from correct directory with proper prompts

- LCA algorithm for working directory selection
- Prompt construction:
  - System context (repo, MR, branch info)
  - Base prompt (from config)
  - User instructions (from parsed intent)
  - Permission statements
  - Safety reminders
- Pass constructed prompt to agent
- **Tests:** LCA edge cases, prompt construction (unit)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~6-8 files

---

### Phase 7: Logging + Cleanup

**Goal:** Capture logs, manage retention

- Capture agent output from containers
- Write to filesystem:
  ```
  /var/log/familiar/
  └── {repo}/
      └── {mr-id}/
          └── {timestamp}-{event-type}.log
  ```
- Configurable retention period
- Background cleanup routine (runs daily)
- **Tests:** Log writing, retention calculation, cleanup (unit + integration)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~6-8 files

---

### Phase 8: Deployment Packaging

**Goal:** Make it easy to deploy

- Dockerfile for Familiar service itself
- Systemd unit file
- Docker Compose example
- README with:
  - Quick start guide
  - Configuration reference
  - Branch protection setup (required)
  - Webhook configuration for GitLab/GitHub
- Example configs
- **Tests:** Container builds successfully
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~8-10 files

---

### Phase 9: Hardening

**Goal:** Production readiness

- Agent timeout enforcement
- Graceful shutdown (drain queue, wait for agents)
- Error recovery:
  - Container dies unexpectedly
  - Git operations fail
  - API rate limiting
- Metrics endpoint (optional, for observability)
- **Tests:** Timeout scenarios, failure modes (integration)
- **Docs check:** Review README setup instructions; update if this phase affects them
- **PR size:** ~6-8 files

---

### Phase Summary

| Phase | Focus | PR Size | Secrets Needed |
|-------|-------|---------|----------------|
| 1 | Skeleton + webhooks | ~10-15 | None |
| 2 | Providers + repo cache | ~10-12 | Optional (for integration tests) |
| 3 | Event routing + config | ~8-10 | None |
| 4 | Intent parsing | ~5-8 | ANTHROPIC_API_KEY (for integration tests) |
| 5 | Docker spawning | ~12-15 | Docker socket |
| 6 | LCA + prompts | ~6-8 | None |
| 7 | Logging | ~6-8 | None |
| 8 | Deployment | ~8-10 | None |
| 9 | Hardening | ~6-8 | Full setup (for E2E) |

**Total:** ~9 PRs, ~70-90 files

---

## Future Considerations

Items explicitly deferred from v1, documented for future development.

### Authentication

**GitHub App authentication:**
- Register Familiar as a GitHub App
- Users install app on repos (scoped permissions)
- JWT + installation token flow
- Better security model than PAT

**GitLab OAuth Application:**
- Register as OAuth application
- Group/project-level tokens
- More granular than personal PAT

*Design note:* Provider interface already accommodates multiple auth strategies.

### Intent Parsing

**CLI-based parsing:**
- Spawn lightweight Claude Code agent for intent extraction
- Consolidates all LLM usage under Max subscription
- No separate API key needed

*Design note:* IntentParser interface supports swappable strategies.

### Additional Git Providers

**Gitea / Forgejo:**
- Self-hosted Git with webhook support
- API similar to GitHub
- Straightforward to add via provider interface

**Bitbucket:**
- Different webhook format
- Different API structure
- Lower priority unless requested

### Agent Capabilities

**Multi-step workflows:**
- Agent chains (review → fix → re-review)
- Conditional actions based on outcomes

**Custom tools:**
- Allow repos to define MCP tools agents can use
- Project-specific integrations

### Observability

**Metrics endpoint:**
- Prometheus-compatible `/metrics`
- Agent spawn counts, durations, success rates
- Queue depth, timeout counts

**Webhook dashboard:**
- Simple web UI showing recent events
- Agent status, logs viewer
- Low priority (logs sufficient for v1)
