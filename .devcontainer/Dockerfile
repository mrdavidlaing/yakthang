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

COPY yx /usr/local/bin/yx 2>/dev/null || echo "Warning: yx not found in build context - it will not be available in the container"
RUN if [ -f /usr/local/bin/yx ]; then chmod +x /usr/local/bin/yx; fi

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