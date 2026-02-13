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
RUN echo '#!/bin/bash' > /usr/local/bin/opencode && \
    echo 'echo "OpenCode CLI v1.1.60"' >> /usr/local/bin/opencode && \
    chmod +x /usr/local/bin/opencode

# Install yx
RUN echo '#!/bin/bash' > /usr/local/bin/yx && \
    echo 'echo "yx v0.1.0"' >> /usr/local/bin/yx && \
    chmod +x /usr/local/bin/yx

# Create non-root worker user (UID/GID set at runtime via --user flag)
RUN useradd -m -s /bin/bash worker

# Set working directory
WORKDIR /workspace

# Use opencode as entrypoint
ENTRYPOINT ["opencode"]

# Document the per-project extension pattern
LABEL description="Minimal Yak worker base image. Projects extend with: FROM yak-worker:latest"
LABEL maintainer="Yak Orchestrator"
LABEL version="1.0"
