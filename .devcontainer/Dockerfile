# Yak development container
# Includes all toolchains needed for yak-box, yak-map, and OpenCode development

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
ENV CI=true

# Install essential build tools first
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    git \
    bash \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.26
RUN wget -q https://go.dev/dl/go1.26.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz && \
    rm go1.26.0.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:/go/bin:${PATH}"
ENV GOPATH="/go"
ENV GO111MODULE=on

# Install gopls (Go language server for LSP)
RUN go install golang.org/x/tools/gopls@latest

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
    RUSTUP_HOME=/opt/rustup \
    CARGO_HOME=/opt/cargo \
    sh -s -- -y --default-toolchain stable --no-modify-path
ENV PATH="/opt/cargo/bin:${PATH}"
ENV RUSTUP_HOME=/opt/rustup
ENV CARGO_HOME=/opt/cargo

RUN rustup target add wasm32-wasip1

# Install OpenCode CLI
RUN curl -fsSL https://opencode.ai/install | bash && \
    cp /root/.opencode/bin/opencode /usr/local/bin/opencode && \
    chmod +x /usr/local/bin/opencode

# Install Claude Code CLI
RUN curl -fsSL https://claude.ai/install.sh | bash && \
    cp /root/.local/bin/claude /usr/local/bin/claude && \
    chmod +x /usr/local/bin/claude

# Pre-seed Claude config so first-run onboarding wizard doesn't block sandboxed workers
RUN mkdir -p /root/.claude && echo '{"numStartups": 1}' > /root/.claude/.claude.json

# Copy yx binary if present in build context (wildcard makes it optional).
# NOTE: yx is dynamically linked against libgit2 1.9 which isn't in Ubuntu 24.04.
# Until yx is statically linked (see static-link-yx yak), yx won't work in the container.
COPY yx* /tmp/
RUN if [ -f /tmp/yx ]; then \
        mv /tmp/yx /usr/local/bin/yx && \
        chmod +x /usr/local/bin/yx; \
    fi

# Trust any mounted workspace (container runs as root, repo owned by host user)
RUN git config --global --add safe.directory '*'

# Allow passwordless sudo so host user can install packages when needed
RUN echo 'ALL ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Create workspace
WORKDIR /workspace

# Default shell
SHELL ["/bin/bash", "-lc"]

# Set default command (can be overridden)
CMD ["/bin/bash"]