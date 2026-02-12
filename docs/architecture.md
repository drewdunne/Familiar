# Familiar Architecture

## High-Level Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Git Provider                                  │
│                         (GitHub / GitLab)                                   │
│                                                                             │
│   MR opened ──┐   Comment posted ──┐   Commits pushed ──┐   @mention ──┐  │
└───────────────┼─────────────────────┼─────────────────────┼──────────────┼──┘
                │                     │                     │              │
                ▼                     ▼                     ▼              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Familiar Server                                     │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                     HTTP Server (port 7000)                          │   │
│  │                                                                      │   │
│  │   /webhook/github ──┐         /health ──→ Docker ping               │   │
│  │   /webhook/gitlab ──┤         /metrics ──→ JSON counters            │   │
│  └─────────────────────┼────────────────────────────────────────────────┘   │
│                        ▼                                                    │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    Webhook Handler                                   │   │
│  │                                                                      │   │
│  │   • Verify signature (HMAC-SHA256 / token)                          │   │
│  │   • Parse raw JSON payload                                          │   │
│  │   • Return GitHubEvent or GitLabEvent                               │   │
│  └─────────────────────┬────────────────────────────────────────────────┘   │
│                        ▼                                                    │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                   Event Normalizer                                   │   │
│  │                                                                      │   │
│  │   Provider-specific payload ──→ Normalized Event                     │   │
│  │                                                                      │   │
│  │   Fields: Type, Provider, Repo, MR#, Branches,                      │   │
│  │           CommentBody, CommentFilePath, CommentLine,                 │   │
│  │           CommentDiscussionID, Actor                                 │   │
│  └─────────────────────┬────────────────────────────────────────────────┘   │
│                        ▼                                                    │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                     Event Router                                     │   │
│  │                                                                      │   │
│  │   ┌─────────────┐  ┌──────────────┐  ┌───────────────┐             │   │
│  │   │ Bot filter   │→│ Event enabled │→│  Debouncer    │             │   │
│  │   │ (skip bots)  │  │ check        │  │  (10s window) │             │   │
│  │   └─────────────┘  └──────────────┘  └───────┬───────┘             │   │
│  └──────────────────────────────────────────────┼───────────────────────┘   │
│                        ┌────────────────────────┘                           │
│                        ▼                                                    │
│        ┌───────────────────────────┐                                        │
│        │     Config Merge          │                                        │
│        │                           │                                        │
│        │  Server config (YAML)     │                                        │
│        │  + Repo config (.familiar)│                                        │
│        │  = MergedConfig           │                                        │
│        └─────────────┬─────────────┘                                        │
│                      ▼                                                      │
│        ┌───────────────────────────┐                                        │
│        │    Intent Parser (LLM)    │   (comments/mentions only)             │
│        │                           │                                        │
│        │  Comment text ──→ Actions │                                        │
│        │  (merge, push, approve)   │                                        │
│        └─────────────┬─────────────┘                                        │
│                      ▼                                                      │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    Agent Handler                                     │   │
│  │                                                                      │   │
│  │   1. Get authenticated clone URL from provider                      │   │
│  │   2. Ensure bare repo in cache (clone or fetch)                     │   │
│  │   3. Create git worktree for source branch                          │   │
│  │   4. Get changed files → calculate LCA → set workdir               │   │
│  │   5. Build prompt (context + base prompt + permissions + safety)    │   │
│  │   6. Spawn agent container                                          │   │
│  │   7. Create log file                                                │   │
│  └─────────────────────┬────────────────────────────────────────────────┘   │
│                        ▼                                                    │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    Agent Spawner                                     │   │
│  │                                                                      │   │
│  │   • Enforce max agents limit                                        │   │
│  │   • Resolve container UID from process                              │   │
│  │   • Configure mounts, env, command                                  │   │
│  │   • Create + start Docker container                                 │   │
│  │   • Track session, handle timeouts                                  │   │
│  └─────────────────────┬────────────────────────────────────────────────┘   │
└────────────────────────┼────────────────────────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Agent Container                                       │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Mounts                                                             │   │
│   │                                                                     │   │
│   │   /workspace ←── git worktree (source branch, read-write)          │   │
│   │   /cache     ←── bare repo cache (read-write)                      │   │
│   │   /home/agent ←── tmpfs (ephemeral home, 0777)                     │   │
│   │   /claude-auth-src ←── credentials (read-only bind)                │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Container Startup Script                                           │   │
│   │                                                                     │   │
│   │   1. Copy credentials → /home/agent/.claude/                       │   │
│   │   2. Write CLAUDE.md  → /home/agent/.claude/CLAUDE.md              │   │
│   │   3. Configure git safe.directory + credential helper              │   │
│   │   4. Configure glab (if GitLab)                                    │   │
│   │   5. tmux new-session → claude -p "$FAMILIAR_PROMPT"               │   │
│   │   6. Wait for tmux session to finish                               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Claude Code Agent                                                  │   │
│   │                                                                     │   │
│   │   Loads:                                                            │   │
│   │     ~/.claude/CLAUDE.md    (Familiar agent identity + workflow)     │   │
│   │     /workspace/CLAUDE.md   (project-specific rules, if exists)     │   │
│   │     $FAMILIAR_PROMPT       (task-specific prompt from webhook)      │   │
│   │                                                                     │   │
│   │   Actions:                                                          │   │
│   │     • Read/write code in /workspace                                │   │
│   │     • Run tests                                                    │   │
│   │     • Commit, rebase, push                                         │   │
│   │     • Post comments via glab/gh CLI                                │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   Container exits → session cleaned up → worktree removed                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Docker Deployment

### Required Environment Variables

```bash
# UID/GID — Familiar runs as your host user so worktrees have correct ownership
export UID              # bash exposes this automatically
export GID=$(id -g)
export DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)

# Paths
export REPO_CACHE_DIR="/home/youruser/familiar-cache"   # absolute host path
export CLAUDE_AUTH_DIR="/home/youruser/.claude"           # Claude CLI credentials
export AGENT_IMAGE="familiar-agent:latest"                # agent container image
export LOG_DIR="/var/log/familiar"
export LOG_HOST_DIR="./logs"

# Provider tokens
export GITHUB_TOKEN="ghp_..."
export GITHUB_WEBHOOK_SECRET="..."
export GITLAB_TOKEN="glpat-..."
export GITLAB_WEBHOOK_SECRET="..."
export GITLAB_BASE_URL="https://gitlab.example.com"      # if self-hosted

# Intent parsing (Anthropic API, separate from Claude CLI auth)
export ANTHROPIC_API_KEY="sk-ant-..."
```

### Starting the Service

```bash
# Build and start (UID/GID/DOCKER_GID must be set)
export UID GID=$(id -g) DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
docker compose up -d --build

# View logs
docker logs -f familiar-familiar-1

# Restart after code changes
docker compose up -d --build
```

### docker-compose.yml Explained

```yaml
services:
  familiar:
    build: .
    network_mode: host              # shares host network (for Tailscale, etc.)
    user: "${UID}:${GID}"           # run as host user for file ownership
    group_add:
      - "${DOCKER_GID}"            # access to /var/run/docker.sock
    volumes:
      - ./config.yaml:/etc/familiar/config.yaml:ro
      - /var/run/docker.sock:/var/run/docker.sock    # spawn agent containers
      - ./logs:/var/log/familiar                      # agent session logs
      - ${REPO_CACHE_DIR}:/cache                      # bare repo cache
      - ${CLAUDE_AUTH_DIR}:/claude-auth:ro             # Claude CLI credentials
```

Key points:
- **`user: "${UID}:${GID}"`** — Familiar creates worktrees that agent containers mount. If UIDs don't match, agents can't write to the worktree.
- **`group_add: ["${DOCKER_GID}"]`** — The Docker socket has a specific GID. Without this, Familiar can't spawn agent containers.
- **`network_mode: host`** — Required if your git provider is on a private network (e.g., Tailscale). Agents also use host networking so they can reach the same endpoints.

## Agent Instructions (CLAUDE.md Injection)

Every agent container receives a `CLAUDE.md` file at `/home/agent/.claude/CLAUDE.md` during startup. This gives the agent:

- **Identity** — knows it's ephemeral, that unpushed work is lost
- **Environment** — understands `/workspace`, `/cache`, `/home/agent` layout
- **Push workflow** — fetch, change, test, rebase, push
- **Concurrency handling** — rebase-then-push with 3 retries
- **Safety constraints** — never force-push, never modify `/cache`, always push
- **Comment etiquette** — reply in discussion threads, reference the exact line

The instructions are embedded in the Go binary via `go:embed` (`internal/agent/instructions/familiar.md`) and passed to the container through the `FAMILIAR_CLAUDE_MD` environment variable. This avoids shell quoting issues with markdown content.

```
Build time:   familiar.md ──[go:embed]──→ instructions.Content()
Spawn time:   instructions.Content() ──→ env: FAMILIAR_CLAUDE_MD=<content>
Container:    printf "$FAMILIAR_CLAUDE_MD" > /home/agent/.claude/CLAUDE.md
Claude Code:  auto-loads ~/.claude/CLAUDE.md as global instructions
```

## Prompt Assembly

```
┌──────────────────────────────────┐
│          Final Prompt            │
│                                  │
│  ┌────────────────────────────┐  │
│  │ ## Context                 │  │  ← Repo, MR#, branches, provider
│  │ - Repository: owner/repo  │  │
│  │ - MR #42: feat → main     │  │
│  │ - Title: Add feature      │  │
│  ├────────────────────────────┤  │
│  │ ## Comment                 │  │  ← If comment event
│  │ @reviewer: Fix this line   │  │
│  │ File: src/index.ts (ln 8) │  │  ← If line-level (diff note)
│  │ Discussion ID: abc123     │  │  ← For thread replies
│  ├────────────────────────────┤  │
│  │ ## Base Prompt             │  │  ← From config (per event type)
│  │ "Address their request..." │  │
│  ├────────────────────────────┤  │
│  │ ## User Instructions       │  │  ← From LLM intent parser
│  │ "Fix the typo"            │  │
│  ├────────────────────────────┤  │
│  │ ## Permissions             │  │  ← Computed from config + event
│  │ - You MAY push commits    │  │
│  │ - You must NOT merge      │  │
│  ├────────────────────────────┤  │
│  │ ## Safety                  │  │  ← Always included
│  │ - Never force push        │  │
│  └────────────────────────────┘  │
└──────────────────────────────────┘
```

### Comment Context

For line-level comments (GitLab diff notes, GitHub PR review comments), the prompt includes:

- **File path** and **line number** from the webhook's `position` data
- **Discussion ID** so the agent replies in-thread instead of posting a top-level comment
- A note that the comment references that exact code location

General MR comments (not on a specific line) include just the comment body and author.

## Push Permission Evaluation

```
push_commits config value
        │
        ├── "always"  ──→ GRANTED
        ├── "never"   ──→ DENIED
        └── "on_request"
                │
                ├── Intent has ActionMerge or ActionPush? ──→ GRANTED
                │
                ├── Event is MR opened or updated? ──→ GRANTED
                │
                ├── Event is MR comment or mention? ──→ GRANTED
                │
                └── Otherwise ──→ DENIED
```

Under `on_request`, push is effectively granted for all current MR-related event types. The distinction from `always` is that future non-MR event types would not automatically get push permission.

## Repo Cache & Worktree Layout

```
Host filesystem:
  $REPO_CACHE_DIR/repos/
    └── owner/
        └── repo.git/                          (bare clone)
            ├── HEAD, refs/, objects/           (git internals)
            └── worktrees-data/
                ├── gitlab-repo-42-1707753600/  (agent 1 worktree)
                └── gitlab-repo-42-1707753610/  (agent 2 worktree)

Inside Familiar container:
  /cache/repos/...                             (same tree, bind-mounted)

Inside agent container:
  /workspace/  ←── worktree checked out to source branch
  /cache/      ←── bare repo (for git operations, do not modify)
```

Familiar uses **dual-path resolution**: it operates on `/cache` inside its own container, but passes the `$REPO_CACHE_DIR` host path when creating agent container bind mounts (since agent containers are siblings, not children).

## Component Dependencies

```
cmd/familiar/main.go
  ├── config.Load()
  ├── registry.New()              ←── provider/github, provider/gitlab
  ├── repocache.New()
  ├── agent.NewSpawner()          ←── docker.NewClient()
  │                               ←── instructions.Content()
  ├── handler.NewAgentHandler()   ←── spawner, repoCache, registry
  │                               ←── prompt.NewBuilder()
  │                               ←── logging.NewWriter()
  ├── event.NewRouter()           ←── handler, intent.NewParser()
  └── server.New()                ←── router, webhook handlers
```

## Project Structure

```
familiar/
├── cmd/familiar/                  # CLI entry point (serve command)
├── internal/
│   ├── agent/
│   │   ├── instructions/          # go:embed'd CLAUDE.md for agents
│   │   ├── spawner.go             # Container lifecycle management
│   │   └── spawner_test.go
│   ├── config/                    # YAML config loading + merging
│   ├── docker/                    # Docker SDK wrapper
│   ├── event/                     # Event normalization + routing
│   ├── handler/                   # Agent handler orchestration
│   ├── intent/                    # Intent parsing (LLM-based)
│   │   └── api/                   # Anthropic API parser
│   ├── lca/                       # Least common ancestor (workdir)
│   ├── logging/                   # Agent session log management
│   ├── metrics/                   # Atomic counters (/metrics endpoint)
│   ├── prompt/                    # Prompt construction
│   ├── provider/
│   │   ├── github/                # GitHub API integration
│   │   └── gitlab/                # GitLab API integration
│   ├── registry/                  # Provider registry
│   ├── repocache/                 # Bare repo cache + worktrees
│   ├── server/                    # HTTP server + routes
│   └── webhook/                   # Webhook signature verification
├── docs/
│   ├── architecture.md            # This file
│   └── plans/                     # Design docs and implementation plans
├── config.yaml                    # Server configuration
├── config.example.yaml            # Example config (safe to commit)
├── docker-compose.yml
├── Dockerfile
├── .env.example                   # Example environment variables
└── CLAUDE.md                      # Project dev instructions
```

## Debugging

### Live Agent Session

Attach to a running agent's Claude Code session:

```bash
docker exec -it familiar-agent-<ID> tmux attach-session -t claude
```

The agent ID is logged by Familiar when the container is spawned.

### Agent Logs

```bash
# Familiar server logs
docker logs -f familiar-familiar-1

# Agent session logs (organized by repo/MR)
ls logs/<owner>/<repo>/<mr_number>/
```

### Common Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| `Unable to find group` on startup | `DOCKER_GID` not set | `export DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)` |
| Agent can't write to `/workspace` | UID mismatch | Ensure `UID=$(id -u) GID=$(id -g)` are exported |
| Agent can't reach git provider | Network isolation | Use `network_mode: host` in both compose and agent config |
| Agent reports no push permission | Config + event type | Check `push_commits` in config; see push permission evaluation above |
| Agent comments on wrong file | Missing position data | Ensure webhook sends note events with position (GitLab diff notes) |
