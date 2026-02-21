# Minimal Yak worker base image
# Projects extend this with their own runtimes via .devcontainer/Dockerfile
# Example extension:
#   FROM yak-worker:latest
#   RUN apt-get update && apt-get install -y nodejs npm

FROM ubuntu:24.04

# Install essential packages only
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Install OpenCode CLI
RUN curl -fsSL https://opencode.ai/install | bash \
    && cp /root/.opencode/bin/opencode /usr/local/bin/opencode \
    && chmod +x /usr/local/bin/opencode

# Install yx (pre-built binary copied from host)
COPY yx /usr/local/bin/yx
RUN chmod +x /usr/local/bin/yx

# Copy dynamic user creation entrypoint
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Install showboat (executable demo document system for workers)
# https://github.com/simonw/showboat — v0.6.0, Apache-2.0
RUN curl -fsSL https://github.com/simonw/showboat/releases/download/v0.6.0/showboat-linux-amd64 \
    -o /usr/local/bin/showboat && chmod +x /usr/local/bin/showboat

# Install RTK (context compression for AI agent tool output)
# https://github.com/rtk-ai/rtk — reduces token consumption by 70-99%
RUN curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh \
    && mv /root/.local/bin/rtk /usr/local/bin/rtk \
    && chmod +x /usr/local/bin/rtk

# Trust any mounted workspace (container runs as root, repo owned by host user)
RUN git config --global --add safe.directory '*'

# Create non-root worker user (UID/GID set at runtime via --user flag)
RUN useradd -m -s /bin/bash worker

# Set working directory
WORKDIR /workspace

# Use entrypoint to dynamically create user if needed
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

# Document the per-project extension pattern
LABEL description="Minimal Yak worker base image. Projects extend with: FROM yak-worker:latest"
LABEL maintainer="Yak Orchestrator"
LABEL version="1.0"
