#!/usr/bin/env bash
set -euo pipefail

# setup-vm.sh - Provision GCP VM for Yak Orchestration
#
# This script sets up a fresh Ubuntu 24.04 GCP VM with all required tools
# for running the Yak orchestration system. It creates the yakob user,
# installs dependencies, builds the worker image, and prepares systemd.
#
# Usage: sudo bash setup-vm.sh
#
# Environment Variables (optional):
#   YAKOB_GIT_NAME   - Git user.name for yakob (will prompt if not set)
#   YAKOB_GIT_EMAIL  - Git user.email for yakob (will prompt if not set)
#
# GCP Deployment Example:
#   # Create the VM
#   gcloud compute instances create yak-orchestrator \
#     --zone=us-central1-a \
#     --machine-type=e2-standard-2 \
#     --image-family=ubuntu-2404-lts-amd64 \
#     --image-project=ubuntu-os-cloud \
#     --boot-disk-size=50GB
#
#   # Copy this script to the VM
#   gcloud compute scp setup-vm.sh yak-orchestrator:~ --zone=us-central1-a
#
#   # Run the script (with optional git config)
#   gcloud compute ssh yak-orchestrator --zone=us-central1-a -- \
#     sudo YAKOB_GIT_NAME="Yakob" YAKOB_GIT_EMAIL="yakob@example.com" bash setup-vm.sh
#
# Idempotency:
#   This script can be run multiple times safely. It checks for existing
#   resources before creating them and uses non-interactive apt installs.

#------------------------------------------------------------------------------
# Helper Functions
#------------------------------------------------------------------------------

log() {
	echo "[setup-vm] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

check_root() {
	if [[ $EUID -ne 0 ]]; then
		echo "ERROR: This script must be run as root (use sudo)" >&2
		exit 1
	fi
}

#------------------------------------------------------------------------------
# 1. Install Docker Engine (Official Ubuntu 24.04 method)
#------------------------------------------------------------------------------

install_docker() {
	log "Installing Docker Engine..."

	# Check if Docker is already installed
	if command -v docker &>/dev/null; then
		log "Docker already installed: $(docker --version)"
		return 0
	fi

	# Remove any old versions
	apt-get remove -y docker.io docker-doc docker-compose podman-docker containerd runc 2>/dev/null || true

	# Install prerequisites
	apt-get update
	apt-get install -y ca-certificates curl gnupg

	# Add Docker's official GPG key
	install -m 0755 -d /etc/apt/keyrings
	if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
		curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
		chmod a+r /etc/apt/keyrings/docker.gpg
	fi

	# Set up the repository
	if [[ ! -f /etc/apt/sources.list.d/docker.list ]]; then
		echo \
			"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
          $(. /etc/os-release && echo "$VERSION_CODENAME") stable" |
			tee /etc/apt/sources.list.d/docker.list >/dev/null
	fi

	# Install Docker Engine
	apt-get update
	apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

	# Start and enable Docker service
	systemctl start docker
	systemctl enable docker

	log "Docker installed: $(docker --version)"
}

#------------------------------------------------------------------------------
# 2. Install system packages (git, zellij, watch, jq)
#------------------------------------------------------------------------------

install_system_packages() {
	log "Installing system packages..."

	apt-get update
	apt-get install -y git watch jq

	# Zellij - install from GitHub releases (not in apt)
	if command -v zellij &>/dev/null; then
		log "Zellij already installed: $(zellij --version)"
	else
		log "Installing Zellij from GitHub releases..."
		local ZELLIJ_VERSION="0.40.1"
		local ZELLIJ_URL="https://github.com/zellij-org/zellij/releases/download/v${ZELLIJ_VERSION}/zellij-x86_64-unknown-linux-musl.tar.gz"

		curl -fsSL "$ZELLIJ_URL" | tar xz -C /usr/local/bin
		chmod +x /usr/local/bin/zellij
		log "Zellij installed: $(zellij --version)"
	fi

	log "System packages installed"
}

#------------------------------------------------------------------------------
# 3. Install OpenCode CLI
# TODO: Replace with real installation when available
# See: .sisyphus/notepads/persistent-vm-plan/issues.md
#------------------------------------------------------------------------------

install_opencode() {
	log "Installing OpenCode CLI..."

	if [[ -x /usr/local/bin/opencode ]]; then
		log "OpenCode already installed: $(/usr/local/bin/opencode --version 2>&1 || echo 'mock')"
		return 0
	fi

	# TODO: Replace this mock with real installation
	# The official install script (https://opencode.ai/install.sh) returns 404
	# Options when available:
	#   1. curl -fsSL https://opencode.ai/install.sh | bash
	#   2. Download pre-built Linux binary
	#   3. Install from package manager

	log "WARNING: Installing mock OpenCode (real install script not available)"
	cat >/usr/local/bin/opencode <<'EOF'
#!/bin/bash
# Mock OpenCode CLI - replace with real installation
# See: .sisyphus/notepads/persistent-vm-plan/issues.md
echo "OpenCode CLI v1.1.60 (mock)"
EOF
	chmod +x /usr/local/bin/opencode

	log "OpenCode CLI installed (mock): $(/usr/local/bin/opencode)"
}

#------------------------------------------------------------------------------
# 4. Install yx (Yak task manager)
# TODO: Replace with real installation when available
# See: .sisyphus/notepads/persistent-vm-plan/issues.md
#------------------------------------------------------------------------------

install_yx() {
	log "Installing yx..."

	if [[ -x /usr/local/bin/yx ]]; then
		log "yx already installed: $(/usr/local/bin/yx --version 2>&1 || echo 'mock')"
		return 0
	fi

	# TODO: Replace this mock with real installation
	# The install script (https://raw.githubusercontent.com/yakthang/yx/main/install.sh) returns 404
	# Options when available:
	#   1. curl -fsSL https://raw.githubusercontent.com/yakthang/yx/main/install.sh | bash
	#   2. Download pre-built Linux binary
	#   3. Install from cargo (if Rust-based)

	log "WARNING: Installing mock yx (real install script not available)"
	cat >/usr/local/bin/yx <<'EOF'
#!/bin/bash
# Mock yx CLI - replace with real installation
# See: .sisyphus/notepads/persistent-vm-plan/issues.md
echo "yx v0.1.0 (mock)"
EOF
	chmod +x /usr/local/bin/yx

	log "yx installed (mock): $(/usr/local/bin/yx)"
}

#------------------------------------------------------------------------------
# 5. Create yakob user (if not exists)
#------------------------------------------------------------------------------

create_yakob_user() {
	log "Setting up yakob user..."

	# Create user if doesn't exist
	if id yakob &>/dev/null; then
		log "User yakob already exists"
	else
		useradd -m -s /bin/bash yakob
		log "Created user yakob"
	fi

	# Ensure docker group exists
	if ! getent group docker &>/dev/null; then
		groupadd docker
		log "Created docker group"
	fi

	# Add yakob to docker group (idempotent)
	if groups yakob | grep -q docker; then
		log "yakob already in docker group"
	else
		usermod -aG docker yakob
		log "Added yakob to docker group"
	fi
}

#------------------------------------------------------------------------------
# 6. Configure yakob's git identity
#------------------------------------------------------------------------------

configure_yakob_git() {
	log "Configuring yakob's git identity..."

	local git_name="${YAKOB_GIT_NAME:-}"
	local git_email="${YAKOB_GIT_EMAIL:-}"

	# Prompt for git config if not provided via environment
	if [[ -z "$git_name" ]]; then
		if [[ -t 0 ]]; then
			read -rp "Enter git user.name for yakob: " git_name
		else
			log "WARNING: YAKOB_GIT_NAME not set and no TTY available, using default"
			git_name="Yakob Orchestrator"
		fi
	fi

	if [[ -z "$git_email" ]]; then
		if [[ -t 0 ]]; then
			read -rp "Enter git user.email for yakob: " git_email
		else
			log "WARNING: YAKOB_GIT_EMAIL not set and no TTY available, using default"
			git_email="yakob@localhost"
		fi
	fi

	# Set git config as yakob user
	su - yakob -c "git config --global user.name '$git_name'"
	su - yakob -c "git config --global user.email '$git_email'"

	log "Git configured for yakob: $git_name <$git_email>"
}

#------------------------------------------------------------------------------
# 7. Create workspace directory
#------------------------------------------------------------------------------

setup_workspace() {
	log "Setting up workspace directory..."

	local workspace="/home/yakob/workspace"

	if [[ -d "$workspace" ]]; then
		log "Workspace already exists: $workspace"
	else
		mkdir -p "$workspace"
		log "Created workspace: $workspace"
	fi

	# Ensure correct ownership
	chown -R yakob:yakob "$workspace"
}

#------------------------------------------------------------------------------
# 8. Copy worker.Dockerfile and build image
#------------------------------------------------------------------------------

build_worker_image() {
	log "Building yak-worker image..."

	local workspace="/home/yakob/workspace"
	local dockerfile_src="./worker.Dockerfile"
	local dockerfile_dst="$workspace/worker.Dockerfile"

	# Copy Dockerfile if source exists
	if [[ -f "$dockerfile_src" ]]; then
		cp "$dockerfile_src" "$dockerfile_dst"
		chown yakob:yakob "$dockerfile_dst"
		log "Copied worker.Dockerfile to workspace"
	elif [[ ! -f "$dockerfile_dst" ]]; then
		log "ERROR: worker.Dockerfile not found at $dockerfile_src or $dockerfile_dst"
		log "Please copy worker.Dockerfile to /home/yakob/workspace manually"
		return 1
	fi

	# Check if image already exists
	if docker image inspect yak-worker:latest &>/dev/null; then
		log "yak-worker:latest image already exists"
		log "To rebuild, run: docker build -t yak-worker:latest -f $dockerfile_dst $workspace"
		return 0
	fi

	# Build the image as yakob (needs docker group access)
	# Note: newgrp doesn't work in scripts, so we use docker directly
	# yakob's docker group membership will be active on next login
	docker build -t yak-worker:latest -f "$dockerfile_dst" "$workspace"

	log "Built yak-worker:latest image"
}

#------------------------------------------------------------------------------
# 9. Create systemd service file (but don't enable/start)
#------------------------------------------------------------------------------

create_systemd_service() {
	log "Creating systemd service file..."

	local service_file="/etc/systemd/system/yak-orchestrator.service"

	cat >"$service_file" <<'EOF'
[Unit]
Description=Yak Orchestrator
Documentation=https://github.com/yakthang/yak-orchestrator
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=yakob
Group=yakob
WorkingDirectory=/home/yakob/workspace

# API key must be set before starting
# Edit this file or create override: systemctl edit yak-orchestrator
Environment="ANTHROPIC_API_KEY="

# Run orchestrator in zellij with the defined layout
ExecStart=/usr/local/bin/zellij --layout /home/yakob/workspace/orchestrator.kdl

# Restart policy
Restart=on-failure
RestartSec=10
TimeoutStopSec=30

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=yak-orchestrator

[Install]
WantedBy=multi-user.target
EOF

	# Reload systemd to recognize the new service
	systemctl daemon-reload

	log "Created systemd service: $service_file"
	log "NOTE: Service not enabled/started. To use:"
	log "  1. Set ANTHROPIC_API_KEY: systemctl edit yak-orchestrator"
	log "  2. Copy orchestrator.kdl to /home/yakob/workspace/"
	log "  3. Enable: systemctl enable yak-orchestrator"
	log "  4. Start:  systemctl start yak-orchestrator"
}

#------------------------------------------------------------------------------
# Main
#------------------------------------------------------------------------------

main() {
	log "Starting GCP VM provisioning for Yak Orchestration"
	log "=================================================="

	check_root

	install_docker
	install_system_packages
	install_opencode
	install_yx
	create_yakob_user
	configure_yakob_git
	setup_workspace
	build_worker_image
	create_systemd_service

	log "=================================================="
	log "VM provisioning complete!"
	log ""
	log "Next steps:"
	log "  1. Log in as yakob: su - yakob"
	log "  2. Copy project files to /home/yakob/workspace/"
	log "  3. Set ANTHROPIC_API_KEY in service config"
	log "  4. Enable and start the service"
	log ""
	log "NOTE: yakob must log out and back in for docker group to take effect"
	log ""
	log "WARNING: OpenCode and yx are currently mock installations."
	log "         Replace with real binaries before production use."
	log "         See: .sisyphus/notepads/persistent-vm-plan/issues.md"
}

main "$@"
