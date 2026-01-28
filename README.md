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

- Go 1.24+
- Docker
- Claude Code CLI (authenticated with Max subscription)
- GitHub and/or GitLab account with API tokens

## Network Setup

Familiar needs to receive webhooks from your Git provider. How you expose it depends on your setup:

### Option 1: Tailscale (Self-Hosted Git Provider)

If your GitLab/GitHub instance is on your Tailscale network, Familiar can be reached directly via its Tailscale hostname or IP. No public exposure needed.

```bash
# Find your Tailscale hostname
tailscale status

# Webhook URL format
http://<tailscale-hostname>:7000/webhook/gitlab
http://<tailscale-hostname>:7000/webhook/github

# Example
http://dev:7000/webhook/gitlab
http://100.79.249.93:7000/webhook/gitlab
```

### Option 2: Tailscale Funnel (GitLab.com / GitHub.com)

If using GitLab.com or GitHub.com (SaaS), you need to expose Familiar to the public internet. Tailscale Funnel provides a secure way to do this.

```bash
# Enable Funnel (one-time setup via Tailscale admin console)
# Then expose port 7000
tailscale funnel 7000

# This gives you a public URL like:
# https://<machine>.<tailnet>.ts.net

# Webhook URL format
https://<machine>.<tailnet>.ts.net/webhook/gitlab
https://<machine>.<tailnet>.ts.net/webhook/github
```

### Option 3: ngrok or Similar

Alternatively, use ngrok or another tunneling service:

```bash
ngrok http 7000

# Use the generated URL for webhooks
https://<random>.ngrok.io/webhook/gitlab
```

## Quick Start

```bash
# Clone the repository
git clone https://github.com/drewdunne/familiar.git
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

#### 1. Create a Personal Access Token (PAT)

1. Go to GitHub and click your profile picture in the top-right corner
2. Select **Settings** from the dropdown menu
3. Scroll down and click **Developer settings** in the left sidebar
4. Click **Personal access tokens** > **Tokens (classic)**
5. Click **Generate new token** > **Generate new token (classic)**
6. Give your token a descriptive name (e.g., "Familiar Service")
7. Set an expiration date (recommended: 90 days or custom)
8. Select the following scopes:
   - **`repo`** - Full control of private repositories (required for reading code, posting comments, and pushing commits)
9. Click **Generate token**
10. **Important**: Copy the token immediately and store it securely. You won't be able to see it again.

Set the token in your environment:
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

#### 2. Configure Webhook on Your Repository

1. Navigate to your repository on GitHub
2. Click **Settings** > **Webhooks** > **Add webhook**
3. Configure the webhook:
   - **Payload URL**: Your Familiar URL + `/webhook/github` (see [Network Setup](#network-setup))
   - **Content type**: `application/json`
   - **Secret**: Generate a secure secret and save it (you'll need this for `GITHUB_WEBHOOK_SECRET`)
   - **SSL verification**: Enable (recommended for production)
4. Select **Let me select individual events** and choose:
   - **Pull requests** - For PR opened, updated, and closed events
   - **Pull request reviews** - For review submitted events
   - **Pull request review comments** - For inline code comments
   - **Issue comments** - For comments on PRs (mentions)
5. Ensure **Active** is checked
6. Click **Add webhook**

Set the webhook secret in your environment:
```bash
export GITHUB_WEBHOOK_SECRET="your_webhook_secret_here"
```

#### 3. Set Up Branch Protection Rules

1. Navigate to your repository on GitHub
2. Click **Settings** > **Branches**
3. Under **Branch protection rules**, click **Add rule**
4. Configure the rule:
   - **Branch name pattern**: `main` (or your default branch)
   - Check **Require a pull request before merging**
   - Check **Require approvals** (recommended: at least 1)
   - Check **Dismiss stale pull request approvals when new commits are pushed**
   - Check **Do not allow bypassing the above settings**
   - Optionally check **Require status checks to pass before merging**
5. Click **Create** or **Save changes**

### GitLab Setup

#### 1. Create a Personal Access Token (PAT)

1. Log in to GitLab and click your avatar in the top-right corner
2. Select **Edit profile** (or **Preferences** depending on GitLab version)
3. In the left sidebar, click **Access Tokens**
4. Click **Add new token**
5. Configure the token:
   - **Token name**: Give it a descriptive name (e.g., "Familiar Service")
   - **Expiration date**: Set an appropriate expiration (recommended: 90 days)
   - **Select scopes**:
     - **`api`** - Full API access (required for reading code, posting comments, and managing merge requests)
6. Click **Create personal access token**
7. **Important**: Copy the token immediately and store it securely. You won't be able to see it again.

Set the token in your environment:
```bash
export GITLAB_TOKEN="glpat-your_token_here"
```

#### 2. Configure Webhook on Your Project

1. Navigate to your project on GitLab
2. Click **Settings** > **Webhooks**
3. Configure the webhook:
   - **URL**: Your Familiar URL + `/webhook/gitlab` (see [Network Setup](#network-setup))
   - **Secret token**: Generate a secure secret and save it (you'll need this for `GITLAB_WEBHOOK_SECRET`)
   - **Trigger events**: Select the following:
     - **Merge request events** - For MR opened, updated, and merged events
     - **Comments** - For comments on merge requests (including mentions)
     - **Push events** (optional) - If you want to track pushes to MR branches
4. **SSL verification**: Enable (recommended for production)
5. Click **Add webhook**

Set the webhook secret in your environment:
```bash
export GITLAB_WEBHOOK_SECRET="your_webhook_secret_here"
```

#### 3. Set Up Branch Protection Rules

1. Navigate to your project on GitLab
2. Click **Settings** > **Repository**
3. Expand the **Protected branches** section
4. Add a protected branch rule:
   - **Branch**: `main` (or your default branch)
   - **Allowed to merge**: Select appropriate roles (e.g., Maintainers)
   - **Allowed to push and merge**: Select **No one** (forces use of merge requests)
   - **Allowed to force push**: Ensure this is **disabled**
5. Click **Protect**

For additional security, consider enabling:
- **Settings** > **Merge requests** > **Merge checks**:
  - **Pipelines must succeed**
  - **All discussions must be resolved**

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

## Deployment

### Docker Deployment (Recommended)

The easiest way to deploy Familiar is using Docker Compose:

1. Copy the example configuration:
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your settings
   ```

2. Build the agent container image:
   ```bash
   docker build -t familiar-agent:latest -f docker/agent/Dockerfile .
   ```

   This builds a container with Claude Code CLI installed. Agents run in these
   containers with your Claude authentication mounted from the host.

3. Set environment variables:
   ```bash
   export GITHUB_TOKEN="your-github-token"
   export GITHUB_WEBHOOK_SECRET="your-webhook-secret"
   export GITLAB_TOKEN="your-gitlab-token"
   export GITLAB_WEBHOOK_SECRET="your-webhook-secret"

   # REQUIRED: Absolute path on the HOST for repo cache
   # Agent containers mount this directory, so it must be a host path
   export REPO_CACHE_DIR="/home/youruser/familiar-cache"

   # Log directory (use /var/log/familiar for Docker, matches named volume)
   export LOG_DIR="/var/log/familiar"

   # Claude CLI auth directory (host path, mounted into agent containers)
   export CLAUDE_AUTH_DIR="/home/youruser/.claude"

   # Agent container image (use the image built in step 2)
   export AGENT_IMAGE="familiar-agent:latest"
   ```

   **Important:** `REPO_CACHE_DIR` must be an absolute path on your host machine.
   Familiar spawns agent containers that mount this directory directly from the
   host filesystem (via the Docker socket), so relative paths won't work.

4. Start the service:
   ```bash
   docker compose up -d
   ```

5. View logs:
   ```bash
   docker compose logs -f
   ```

### Systemd Deployment

For native deployment without Docker:

1. Build the binary:
   ```bash
   go build -o familiar ./cmd/familiar
   sudo mv familiar /usr/local/bin/
   ```

2. Create system user:
   ```bash
   sudo useradd -r -s /bin/false familiar
   ```

3. Set up directories:
   ```bash
   sudo mkdir -p /etc/familiar /var/log/familiar /var/cache/familiar
   sudo cp config.example.yaml /etc/familiar/config.yaml
   sudo chown -R familiar:familiar /var/log/familiar /var/cache/familiar
   ```

4. Create environment file:
   ```bash
   sudo tee /etc/familiar/env << EOF
   GITHUB_TOKEN=your-github-token
   GITHUB_WEBHOOK_SECRET=your-webhook-secret
   GITLAB_TOKEN=your-gitlab-token
   GITLAB_WEBHOOK_SECRET=your-webhook-secret
   EOF
   sudo chmod 600 /etc/familiar/env
   ```

5. Install and start the service:
   ```bash
   sudo cp deploy/familiar.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now familiar
   ```

6. Check status:
   ```bash
   sudo systemctl status familiar
   sudo journalctl -u familiar -f
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
