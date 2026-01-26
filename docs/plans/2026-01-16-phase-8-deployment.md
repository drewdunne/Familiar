# Phase 8: Deployment Packaging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

> **Note:** This plan may need adjustment based on patterns established in Phases 1-7.

**Goal:** Package Familiar for deployment via Docker and systemd, comprehensive README with setup instructions, example configs.

**Tech Stack:** Docker, systemd, shell scripts

**Prerequisites:** Phases 1-7 complete

---

## Task 1: Familiar Service Dockerfile

**Files:**
- Create: `Dockerfile`

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o familiar ./cmd/familiar

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates git docker-cli

COPY --from=builder /build/familiar /usr/local/bin/familiar

# Create directories
RUN mkdir -p /etc/familiar /var/log/familiar /var/cache/familiar

EXPOSE 8080

ENTRYPOINT ["familiar"]
CMD ["serve", "--config", "/etc/familiar/config.yaml"]
```

**Step 2: Create .dockerignore**

```
.git
.worktrees
*.md
!README.md
.env*
```

**Step 3: Verify build**

```bash
docker build -t familiar:local .
```

**Step 4: Commit**

---

## Task 2: Docker Compose

**Files:**
- Create: `docker-compose.yml`

```yaml
version: '3.8'

services:
  familiar:
    build: .
    image: familiar:latest
    container_name: familiar
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/etc/familiar/config.yaml:ro
      - familiar-logs:/var/log/familiar
      - familiar-cache:/var/cache/familiar
      - /var/run/docker.sock:/var/run/docker.sock
      - ${HOME}/.claude:/root/.claude:ro
    env_file:
      - .env

volumes:
  familiar-logs:
  familiar-cache:
```

**Commit**

---

## Task 3: Systemd Unit File

**Files:**
- Create: `deploy/familiar.service`

```ini
[Unit]
Description=Familiar - Git webhook to Claude Code bridge
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=familiar
Group=familiar
EnvironmentFile=/etc/familiar/familiar.env
ExecStart=/usr/local/bin/familiar serve --config /etc/familiar/config.yaml
Restart=on-failure
RestartSec=5

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/log/familiar /var/cache/familiar

[Install]
WantedBy=multi-user.target
```

**Commit**

---

## Task 4: Comprehensive README

**Files:**
- Modify: `README.md`

Update with complete sections:

### Quick Start

```bash
# Clone
git clone https://github.com/drewdunne/familiar.git
cd familiar

# Configure
cp config.example.yaml config.yaml
cp .env.example .env
# Edit .env with your tokens

# Run with Docker
docker-compose up -d

# Or build and run locally
go build -o familiar ./cmd/familiar
./familiar serve --config config.yaml
```

### GitHub Setup

1. **Create Personal Access Token**
   - Go to Settings > Developer settings > Personal access tokens
   - Create token with `repo` scope
   - Copy token to `GITHUB_TOKEN` in `.env`

2. **Configure Webhook**
   - Go to repo Settings > Webhooks > Add webhook
   - Payload URL: `https://your-server:8080/webhook/github`
   - Content type: `application/json`
   - Secret: Same as `GITHUB_WEBHOOK_SECRET` in `.env`
   - Events: Pull requests, Issue comments, Push

3. **Configure Branch Protection** (Required)
   - Go to repo Settings > Branches > Add rule
   - Branch name pattern: `main`
   - Enable: Require pull request reviews
   - Enable: Require status checks
   - Disable: Allow force pushes

### GitLab Setup

1. **Create Personal Access Token**
   - Go to User Settings > Access Tokens
   - Create token with `api` scope
   - Copy token to `GITLAB_TOKEN` in `.env`

2. **Configure Webhook**
   - Go to project Settings > Webhooks
   - URL: `https://your-server:8080/webhook/gitlab`
   - Secret token: Same as `GITLAB_WEBHOOK_SECRET` in `.env`
   - Triggers: Merge request events, Comments, Push events

3. **Configure Branch Protection** (Required)
   - Go to project Settings > Repository > Protected branches
   - Protect `main`
   - Allowed to merge: Maintainers
   - Allowed to push: No one

### Claude Code Authentication

```bash
# On the host machine, authenticate once
claude login

# The ~/.claude directory will be mounted into agent containers
```

### Troubleshooting

- **Webhook not received**: Check firewall, verify URL is accessible
- **Signature verification failed**: Ensure webhook secret matches config
- **Agent fails to start**: Check Docker is running, verify image exists
- **Permission denied**: Verify PAT has correct scopes

**Commit**

---

## Task 5: Example Configs

**Files:**
- Already created: `config.example.yaml`
- Create: `.env.example` (already exists)
- Create: `deploy/nginx.conf` (optional reverse proxy)

**Commit**

---

## Task 6: Verify Docker Build

```bash
# Build service image
docker build -t familiar:latest .

# Build agent image
docker build -t familiar-agent:latest -f docker/agent/Dockerfile .

# Test run
docker run --rm familiar:latest version
```

**Commit if fixes needed**

---

## Summary

| Task | Component |
|------|-----------|
| 1 | Service Dockerfile |
| 2 | Docker Compose |
| 3 | Systemd unit |
| 4 | README documentation |
| 5 | Example configs |
| 6 | Build verification |

**Total: 6 tasks**
