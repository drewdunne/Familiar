FROM golang:1.25-alpine

# Install dependencies
RUN apk add --no-cache \
    git \
    tmux \
    openssh-client \
    ca-certificates \
    curl \
    bash

# Install Claude Code CLI
# Note: Update installation method based on actual Claude Code distribution
RUN curl -fsSL https://claude.ai/install.sh | sh || \
    echo "Claude Code installation placeholder - update when available"

# Create non-root user for agent
RUN adduser -D -h /home/agent -s /bin/bash agent

# Create directories
RUN mkdir -p /home/agent/.claude /workspace && \
    chown -R agent:agent /home/agent /workspace

USER agent
WORKDIR /workspace

# Default entrypoint - will be overridden
ENTRYPOINT ["/bin/bash"]
