FROM node:22-alpine

# Install dependencies
RUN apk add --no-cache \
    git \
    tmux \
    openssh-client \
    ca-certificates \
    curl \
    bash \
    github-cli \
    glab

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user for agent
RUN adduser -D -h /home/agent -s /bin/bash agent

# Create directories
RUN mkdir -p /home/agent/.claude /workspace && \
    chown -R agent:agent /home/agent /workspace

USER agent
WORKDIR /workspace

# Default entrypoint - will be overridden
ENTRYPOINT ["/bin/bash"]
