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

# Trust any mounted workspace (container runs as root, repo owned by host user)
RUN git config --global --add safe.directory '*'

# Create non-root worker user (UID/GID set at runtime via --user flag)
RUN useradd -m -s /bin/bash worker

# Set working directory
WORKDIR /workspace

# No entrypoint — spawn-worker.sh specifies the full command
ENTRYPOINT []

# Document the per-project extension pattern
LABEL description="Minimal Yak worker base image. Projects extend with: FROM yak-worker:latest"
LABEL maintainer="Yak Orchestrator"
LABEL version="1.0"
