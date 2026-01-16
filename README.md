# Familiar

A self-hosted service that bridges git provider webhooks to Claude Code agents, enabling autonomous code review and MR/PR interactions.

## Overview

Familiar listens for webhooks from GitLab and GitHub, then spawns isolated Claude Code agents in Docker containers to handle each event. Agents operate on git worktrees, inherit project-specific context via `claude.md` files, and can push commits, post comments, and (if permitted) merge PRs.

### Key Features

- **Ephemeral agents**: One agent per event, terminated after completion
- **Isolated execution**: Agents run in Docker containers with mounted worktrees
- **Context-aware**: Agents run from the correct directory to inherit `claude.md` files
- **Configurable**: Server defaults + per-repo overrides for prompts and permissions
- **Observable**: Agents run in tmux sessions, logs preserved for debugging

## Requirements

- Go 1.25+
- Docker
- Claude Code CLI (authenticated with Max subscription)
- GitHub and/or GitLab account with API tokens

## Quick Start

> **Note:** Comprehensive setup instructions will be added as the project develops.

```bash
# Clone the repository
git clone https://github.com/yourusername/familiar.git
cd familiar

# Copy example env file
cp .env.example .env
# Edit .env with your tokens

# Build
go build -o familiar ./cmd/familiar

# Run
./familiar serve --config config.yaml
```

## Setup Instructions

### Prerequisites

1. **Claude Code CLI**: Install and authenticate
   ```bash
   # Install Claude Code
   # (installation instructions vary by platform)

   # Authenticate (one-time)
   claude login
   ```

2. **Docker**: Ensure Docker is installed and running

### GitHub Setup

> Detailed instructions coming in Phase 1

1. Create a Personal Access Token (PAT)
2. Configure webhook on your repository
3. Set up branch protection rules

### GitLab Setup

> Detailed instructions coming in Phase 1

1. Create a Personal Access Token (PAT)
2. Configure webhook on your project
3. Set up branch protection rules

### Branch Protection (Required)

Familiar relies on branch protection as a safety backstop. You **must** configure:

- No direct pushes to `main` (require merge requests)
- No force pushes to protected branches
- (Recommended) Require reviews before merge

## Configuration

### Server Configuration

Create a `config.yaml` file:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

logging:
  dir: "/var/log/familiar"
  retention_days: 30

providers:
  github:
    auth_method: "pat"
    token: "${GITHUB_TOKEN}"
    webhook_secret: "${GITHUB_WEBHOOK_SECRET}"
  gitlab:
    auth_method: "pat"
    token: "${GITLAB_TOKEN}"
    webhook_secret: "${GITLAB_WEBHOOK_SECRET}"

# See docs/plans/2026-01-16-familiar-design.md for full configuration options
```

### Repository Configuration

Add `.familiar/config.yaml` to your repository to customize behavior:

```yaml
events:
  mr_opened: true
  mr_comment: true
  mr_updated: false
  mention: true

permissions:
  merge: "on_request"
  push_commits: "always"
```

## Development

### Running Tests

```bash
# Unit tests
go test ./...

# With coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# E2E tests (requires configuration)
go test -tags=e2e ./...
```

### Project Structure

```
familiar/
├── cmd/
│   └── familiar/
│       └── main.go
├── internal/
│   ├── config/
│   ├── webhook/
│   ├── provider/
│   ├── agent/
│   └── server/
├── docs/
│   └── plans/
├── test/
│   ├── integration/
│   ├── e2e/
│   └── fixtures/
├── .github/
│   └── workflows/
├── .env.example
├── .gitignore
├── go.mod
└── README.md
```

## Documentation

- [Design Document](docs/plans/2026-01-16-familiar-design.md) - Full architecture and implementation plan

## Contributing

Contributions are welcome! Please read the design document first to understand the architecture.

1. Fork the repository
2. Create a feature branch
3. Ensure tests pass with 80%+ coverage
4. Submit a pull request

## License

MIT License - see LICENSE file for details.
