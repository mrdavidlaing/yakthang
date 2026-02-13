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
		local ZELLIJ_VERSION="0.43.1"
		local ZELLIJ_URL="https://github.com/zellij-org/zellij/releases/download/v${ZELLIJ_VERSION}/zellij-x86_64-unknown-linux-musl.tar.gz"

		curl -fsSL "$ZELLIJ_URL" | tar xz -C /usr/local/bin
		chmod +x /usr/local/bin/zellij
		log "Zellij installed: $(zellij --version)"
	fi

	log "System packages installed"
}

#------------------------------------------------------------------------------
# 3. Install GitHub CLI
#------------------------------------------------------------------------------

install_gh_cli() {
	log "Installing GitHub CLI..."

	if command -v gh &>/dev/null; then
		log "GitHub CLI already installed: $(gh --version)"
		return 0
	fi

	curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg |
		gpg --dearmor -o /usr/share/keyrings/githubcli-archive-keyring.gpg

	echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" |
		tee /etc/apt/sources.list.d/github-cli.list >/dev/null

	apt-get update
	apt-get install -y gh

	log "GitHub CLI installed: $(gh --version)"
}

#------------------------------------------------------------------------------
# 4. Install Node.js 22 (required for OpenClaw Gateway)
#------------------------------------------------------------------------------

install_nodejs() {
	log "Installing Node.js 22..."

	if command -v node &>/dev/null; then
		local node_version=$(node --version | cut -d'v' -f2 | cut -d'.' -f1)
		if [[ "$node_version" -ge 22 ]]; then
			log "Node.js already installed: $(node --version)"
			return 0
		else
			log "Node.js version too old: v$node_version (need v22+), upgrading..."
		fi
	fi

	# Install Node.js 22 from NodeSource
	log "Adding NodeSource repository for Node.js 22..."
	curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
	apt-get install -y nodejs

	log "Node.js installed: $(node --version)"
}

#------------------------------------------------------------------------------
# 5. Install OpenCode CLI
#------------------------------------------------------------------------------

install_opencode() {
	log "Installing OpenCode CLI..."

	if command -v opencode &>/dev/null; then
		log "OpenCode already installed: $(opencode --version)"
		return 0
	fi

	# Install using official install script
	log "Downloading and running official OpenCode installer..."
	curl -fsSL https://opencode.ai/install | bash

	log "OpenCode CLI installed: $(opencode --version)"
}

#------------------------------------------------------------------------------
# 6. Install OpenClaw Gateway
#------------------------------------------------------------------------------

install_openclaw() {
	log "Installing OpenClaw Gateway..."

	if command -v openclaw &>/dev/null; then
		log "OpenClaw already installed: $(openclaw --version)"
		return 0
	fi

	log "Installing OpenClaw via npm..."
	npm install -g openclaw@latest

	log "OpenClaw installed: $(openclaw --version)"
}

#------------------------------------------------------------------------------
# 7. Install yx (Yak task manager) from source
#------------------------------------------------------------------------------

install_yx() {
	log "Installing yx..."

	if [[ -x /usr/local/bin/yx ]]; then
		log "yx already installed: $(/usr/local/bin/yx --version 2>&1)"
		return 0
	fi

	log "Installing Rust toolchain via rustup..."
	if ! command -v rustup &>/dev/null; then
		curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
		source "$HOME/.cargo/env"
	else
		log "rustup already installed"
	fi

	local CLONE_DIR="/home/yakob/yakthang/tmp/mrdavidlaing-yaks"

	log "Cloning mrdavidlaing/yaks repository (ls-format-flag branch)..."
	mkdir -p "$(dirname "$CLONE_DIR")"
	gh repo clone mrdavidlaing/yaks "$CLONE_DIR" -- --branch ls-format-flag

	log "Building yx from source..."
	cd "$CLONE_DIR"
	cargo build --release

	log "Installing yx binary to /usr/local/bin..."
	install -m 0755 target/release/yx /usr/local/bin/yx

	log "yx installed: $(yx --version)"
}

#------------------------------------------------------------------------------
# 8. Security Hardening
#------------------------------------------------------------------------------

configure_security() {
	log "Configuring security hardening..."

	# --- Configure UFW firewall ---
	log "Configuring UFW firewall..."
	apt-get install -y ufw
	ufw default deny incoming
	ufw default allow outgoing
	ufw allow ssh
	ufw --force enable

	# --- Harden SSH configuration ---
	log "Hardening SSH..."
	sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
	sed -i 's/PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
	sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
	sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
	systemctl reload sshd || systemctl reload ssh || true

	# --- Install fail2ban ---
	log "Installing fail2ban..."
	apt-get install -y fail2ban
	systemctl enable fail2ban
	systemctl start fail2ban

	# --- Configure Docker daemon security ---
	log "Configuring Docker daemon..."
	mkdir -p /etc/docker
	cat >/etc/docker/daemon.json <<'DOCKER_EOF'
{
  "live-restore": true,
  "userland-proxy": false,
  "no-new-privileges": true,
  "icc": false,
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
DOCKER_EOF
	systemctl restart docker

	log "Security hardening complete"
}

#------------------------------------------------------------------------------
# 9. Create yakob user (if not exists)
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
# 10. Configure yakob's git identity
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
# 11. Create workspace directory
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
# 12. Setup OpenClaw workspace
#------------------------------------------------------------------------------

setup_openclaw_workspace() {
	log "Setting up OpenClaw workspace..."

	local openclaw_workspace="/home/yakob/yakthang/.openclaw/workspace"
	local yaks_source="/home/yakob/yakthang/.yaks"

	# Create OpenClaw workspace directory
	if [[ -d "$openclaw_workspace" ]]; then
		log "OpenClaw workspace already exists: $openclaw_workspace"
	else
		mkdir -p "$openclaw_workspace"
		log "Created OpenClaw workspace: $openclaw_workspace"
	fi

	# Symlink .yaks directory
	local yaks_link="$openclaw_workspace/.yaks"
	if [[ -L "$yaks_link" ]]; then
		log ".yaks symlink already exists"
	elif [[ -e "$yaks_link" ]]; then
		log "WARNING: $yaks_link exists but is not a symlink, skipping"
	else
		ln -s "$yaks_source" "$yaks_link"
		log "Created symlink: $yaks_link -> $yaks_source"
	fi

	# Create OpenClaw agent directories (required by openclaw doctor)
	local agent_sessions_dir="/home/yakob/.openclaw/agents/main/sessions"
	local credentials_dir="/home/yakob/.openclaw/credentials"

	if [[ ! -d "$agent_sessions_dir" ]]; then
		mkdir -p "$agent_sessions_dir"
		log "Created agent sessions directory: $agent_sessions_dir"
	fi

	if [[ ! -d "$credentials_dir" ]]; then
		mkdir -p "$credentials_dir"
		chmod 700 "$credentials_dir"
		log "Created credentials directory: $credentials_dir"
	fi

	# Ensure correct ownership
	chown -R yakob:yakob /home/yakob/yakthang/.openclaw
	chown -R yakob:yakob /home/yakob/.openclaw

	log "OpenClaw workspace setup complete"
}

#------------------------------------------------------------------------------
# 13. Copy worker.Dockerfile and build image
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
# 14. Create OpenClaw Gateway systemd service
#------------------------------------------------------------------------------

create_systemd_service() {
	log "Creating OpenClaw Gateway systemd service..."

	local service_file="/etc/systemd/system/openclaw-gateway.service"

	cat >"$service_file" <<'EOF'
[Unit]
Description=OpenClaw Gateway (Yakob Orchestrator)
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=yakob
Group=yakob
WorkingDirectory=/home/yakob/yakthang

# Environment variables for credentials (set via systemctl edit)
Environment="ANTHROPIC_API_KEY="
Environment="ZELLIJ_SESSION_NAME=yak-workers"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

# Optional: Uncomment when adding Slack integration
# Environment="SLACK_APP_TOKEN="
# Environment="SLACK_BOT_TOKEN="

ExecStart=/usr/bin/openclaw gateway --port 18789

Restart=on-failure
RestartSec=10
TimeoutStopSec=30

StandardOutput=journal
StandardError=journal
SyslogIdentifier=openclaw-gateway

[Install]
WantedBy=multi-user.target
EOF

	# Reload systemd to recognize the new service
	systemctl daemon-reload

	log "Created systemd service: $service_file"
	log "NOTE: Service not enabled/started. To use:"
	log "  1. Set ANTHROPIC_API_KEY in override file:"
	log "     sudo mkdir -p /etc/systemd/system/openclaw-gateway.service.d"
	log "     sudo tee /etc/systemd/system/openclaw-gateway.service.d/override.conf <<< '[Service]'"
	log "     sudo tee -a /etc/systemd/system/openclaw-gateway.service.d/override.conf <<< 'Environment=\"ANTHROPIC_API_KEY=your-key-here\"'"
	log "  2. Enable: systemctl enable openclaw-gateway"
	log "  3. Start a Zellij session: zellij --session yak-workers"
	log "  4. Start: systemctl start openclaw-gateway"
	log "  5. Check status: systemctl status openclaw-gateway"
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
	install_gh_cli
	install_nodejs
	install_opencode
	install_openclaw
	install_yx
	configure_security
	create_yakob_user
	configure_yakob_git
	setup_workspace
	setup_openclaw_workspace
	build_worker_image
	create_systemd_service

	log "=================================================="
	log "VM provisioning complete!"
	log ""
	log "Next steps:"
	log "  1. Set ANTHROPIC_API_KEY:"
	log "     sudo mkdir -p /etc/systemd/system/openclaw-gateway.service.d"
	log "     echo '[Service]' | sudo tee /etc/systemd/system/openclaw-gateway.service.d/override.conf"
	log "     echo 'Environment=\"ANTHROPIC_API_KEY=sk-ant-your-key\"' | sudo tee -a /etc/systemd/system/openclaw-gateway.service.d/override.conf"
	log "     sudo systemctl daemon-reload"
	log ""
	log "  2. Run OpenClaw onboarding (as yakob):"
	log "     su - yakob"
	log "     cd /home/yakob/yakthang"
	log "     openclaw onboard --workspace /home/yakob/yakthang/.openclaw/workspace"
	log ""
	log "  3. Customize identity files (SOUL.md, AGENTS.md, HEARTBEAT.md)"
	log "     in /home/yakob/yakthang/.openclaw/workspace/"
	log ""
	log "  4. Enable and start OpenClaw Gateway:"
	log "     sudo systemctl enable openclaw-gateway"
	log "     sudo systemctl start openclaw-gateway"
	log "     sudo systemctl status openclaw-gateway"
	log ""
	log "  5. Add cron jobs (as yakob, after gateway is running):"
	log "     # Worker sweep (every 2 hours)"
	log "     openclaw cron add \\"
	log "       --name \"Worker sweep\" \\"
	log "       --cron \"0 */2 * * *\" \\"
	log "       --tz \"UTC\" \\"
	log "       --session main \\"
	log "       --system-event \"Check for blocked workers and stale tasks. Run check-workers.sh.\" \\"
	log "       --wake now"
	log ""
	log "     # Daily summary (5pm UTC)"
	log "     openclaw cron add \\"
	log "       --name \"Daily summary\" \\"
	log "       --cron \"0 17 * * *\" \\"
	log "       --tz \"UTC\" \\"
	log "       --session isolated \\"
	log "       --message \"Summarize today's work: completed tasks, blocked workers, tomorrow's priorities. Run yx ls and check-workers.sh.\" \\"
	log "       --announce"
	log ""
	log "     # Verify cron jobs"
	log "     openclaw cron list"
	log ""
	log "OpenClaw workspace: /home/yakob/yakthang/.openclaw/workspace"
	log "Task state symlink: /home/yakob/yakthang/.openclaw/workspace/.yaks -> /home/yakob/yakthang/.yaks"
	log "Gateway port: 18789 (http://localhost:18789/)"
	log ""
	log "NOTE: yakob must log out and back in for docker group to take effect"
}

main "$@"
