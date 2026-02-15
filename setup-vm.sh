#!/usr/bin/env bash
set -euo pipefail

# setup-vm.sh - Provision VM for Yak Orchestration
#
# This script sets up a fresh VM with all required tools for running the
# Yak orchestration system. Supports Ubuntu 24.04 and Arch Linux.
# It creates the yakob user, installs dependencies, builds the worker
# image, and prepares systemd.
#
# Usage: sudo bash setup-vm.sh
#
# Environment Variables (optional — skip interactive prompts):
#   YAKOB_GIT_NAME     - Git user.name for yakob (will prompt if not set)
#   YAKOB_GIT_EMAIL    - Git user.email for yakob (will prompt if not set)
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
#   resources before creating them and uses non-interactive package installs.

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
# OS Detection & Package Management
#------------------------------------------------------------------------------

DISTRO=""

detect_os() {
	if [[ -f /etc/os-release ]]; then
		# shellcheck disable=SC1091
		. /etc/os-release
		case "$ID" in
		ubuntu | debian)
			DISTRO="ubuntu"
			;;
		arch | endeavouros)
			DISTRO="arch"
			;;
		*)
			log "ERROR: Unsupported distribution: $ID"
			exit 1
			;;
		esac
	else
		log "ERROR: Cannot detect OS (missing /etc/os-release)"
		exit 1
	fi
	log "Detected OS: $DISTRO (${PRETTY_NAME:-unknown})"
}

pkg_update() {
	case "$DISTRO" in
	ubuntu) apt-get update ;;
	arch) pacman -Syu --noconfirm ;;
	esac
}

pkg_install() {
	case "$DISTRO" in
	ubuntu) apt-get install -y "$@" ;;
	arch) pacman -S --noconfirm --needed "$@" ;;
	esac
}

#------------------------------------------------------------------------------
# 1. Install Docker Engine
#------------------------------------------------------------------------------

install_docker() {
	log "Installing Docker Engine..."

	if command -v docker &>/dev/null; then
		log "Docker already installed: $(docker --version)"
		return 0
	fi

	case "$DISTRO" in
	ubuntu)
		apt-get remove -y docker.io docker-doc docker-compose podman-docker containerd runc 2>/dev/null || true

		apt-get update
		apt-get install -y ca-certificates curl gnupg

		install -m 0755 -d /etc/apt/keyrings
		if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
			curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
			chmod a+r /etc/apt/keyrings/docker.gpg
		fi

		if [[ ! -f /etc/apt/sources.list.d/docker.list ]]; then
			echo \
				"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
              $(. /etc/os-release && echo "$VERSION_CODENAME") stable" |
				tee /etc/apt/sources.list.d/docker.list >/dev/null
		fi

		apt-get update
		apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
		;;
	arch)
		pkg_install docker docker-buildx docker-compose
		;;
	esac

	systemctl start docker
	systemctl enable docker

	log "Docker installed: $(docker --version)"
}

#------------------------------------------------------------------------------
# 2. Install system packages (git, zellij, watch, jq)
#------------------------------------------------------------------------------

install_system_packages() {
	log "Installing system packages..."

	case "$DISTRO" in
	ubuntu)
		apt-get update
		apt-get install -y git watch jq build-essential pkg-config libssl-dev
		;;
	arch)
		pkg_install git procps-ng jq base-devel pkgconf openssl
		;;
	esac

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

	case "$DISTRO" in
	ubuntu)
		curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg |
			gpg --dearmor -o /usr/share/keyrings/githubcli-archive-keyring.gpg

		echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" |
			tee /etc/apt/sources.list.d/github-cli.list >/dev/null

		apt-get update
		apt-get install -y gh
		;;
	arch)
		pkg_install github-cli
		;;
	esac

	log "GitHub CLI installed: $(gh --version)"
}

#------------------------------------------------------------------------------
# 4. Install Node.js 22 (required for OpenClaw Gateway)
#------------------------------------------------------------------------------

install_nodejs() {
	log "Installing Node.js 22..."

	if command -v node &>/dev/null; then
		local node_version
		node_version=$(node --version | cut -d'v' -f2 | cut -d'.' -f1)
		if [[ "$node_version" -ge 22 ]]; then
			log "Node.js already installed: $(node --version)"
			return 0
		else
			log "Node.js version too old: v$node_version (need v22+), upgrading..."
		fi
	fi

	case "$DISTRO" in
	ubuntu)
		log "Adding NodeSource repository for Node.js 22..."
		curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
		apt-get install -y nodejs
		;;
	arch)
		pkg_install nodejs-lts-jod npm
		;;
	esac

	log "Node.js installed: $(node --version)"
}

#------------------------------------------------------------------------------
# 5. Install OpenCode CLI
#------------------------------------------------------------------------------

install_opencode() {
	log "Installing OpenCode CLI (as yakob user)..."

	if su - yakob -c "command -v opencode" &>/dev/null; then
		log "OpenCode already installed: $(su - yakob -c 'opencode --version')"
		return 0
	fi

	# Install using official install script as yakob user
	log "Downloading and running official OpenCode installer as yakob..."
	su - yakob -c "curl -fsSL https://opencode.ai/install | bash"

	log "OpenCode CLI installed: $(su - yakob -c 'opencode --version')"
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

	log "Installing Rust toolchain via rustup (as yakob user)..."
	if ! su - yakob -c "command -v rustup" &>/dev/null; then
		su - yakob -c "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable"
	else
		log "rustup already installed for yakob"
	fi

	local CLONE_DIR="/home/yakob/yakthang/tmp/mrdavidlaing-yaks"

	log "Cloning mrdavidlaing/yaks repository (ls-format-flag branch)..."
	mkdir -p /home/yakob/yakthang/tmp
	chown -R yakob:yakob /home/yakob
	su - yakob -c "gh repo clone mrdavidlaing/yaks /home/yakob/yakthang/tmp/mrdavidlaing-yaks -- --branch ls-format-flag"

	log "Building yx from source (as yakob user)..."
	su - yakob -c "cd /home/yakob/yakthang/tmp/mrdavidlaing-yaks && source ~/.cargo/env && cargo build --release"

	log "Installing yx binary to /usr/local/bin..."
	install -m 0755 /home/yakob/yakthang/tmp/mrdavidlaing-yaks/target/release/yx /usr/local/bin/yx

	log "yx installed: $(yx --version)"
}

#------------------------------------------------------------------------------
# 8. Security Hardening
#------------------------------------------------------------------------------

configure_security() {
	log "Configuring security hardening..."

	log "Configuring UFW firewall..."
	pkg_install ufw
	ufw default deny incoming
	ufw default allow outgoing
	ufw allow ssh
	ufw --force enable

	log "Hardening SSH..."
	sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
	sed -i 's/PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
	sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
	sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
	systemctl reload sshd 2>/dev/null || systemctl reload ssh 2>/dev/null || true

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
# 10b. Run OpenClaw onboard (handles auth + Slack config interactively)
#------------------------------------------------------------------------------

run_openclaw_onboard() {
	log "Running OpenClaw onboard..."

	local openclaw_workspace="/home/yakob/yakthang/.openclaw/workspace"

	# openclaw onboard handles:
	#   - API key or setup-token authentication
	#   - Slack configuration (app token, bot token, user ID)
	#   - Generates ~/.openclaw/openclaw.json
	log "Starting interactive onboard (authenticate via API key or setup-token)..."
	su - yakob -c "openclaw onboard --workspace ${openclaw_workspace}"

	log "OpenClaw onboard complete"
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

	# Copy yx binary to workspace (required by Dockerfile)
	# Use the installed binary — the temp build dir may not exist on re-runs
	if [[ -x /usr/local/bin/yx ]]; then
		cp /usr/local/bin/yx "$workspace/yx"
		chown yakob:yakob "$workspace/yx"
	else
		log "ERROR: yx binary not found at /usr/local/bin/yx — run install_yx first"
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

Environment="ZELLIJ_SESSION_NAME=yakthang"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

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

	systemctl daemon-reload

	log "Created systemd service: $service_file"
}

#------------------------------------------------------------------------------
# Main
#------------------------------------------------------------------------------

main() {
	log "Starting VM provisioning for Yak Orchestration"
	log "=================================================="

	check_root
	detect_os

	if [[ "$DISTRO" == "arch" ]]; then
		log "Syncing package database and upgrading system..."
		pacman -Syu --noconfirm
	fi

	install_docker
	install_system_packages
	install_gh_cli
	install_nodejs
	create_yakob_user
	install_opencode
	install_openclaw
	install_yx
	configure_security
	configure_yakob_git
	setup_workspace
	setup_openclaw_workspace
	run_openclaw_onboard
	build_worker_image
	create_systemd_service

	log "=================================================="
	log "VM provisioning complete!"
	log ""
	log "Next steps:"
	log "  1. Start a Zellij session for workers:"
	log "     zellij --session yakthang"
	log ""
	log "  2. Enable and start OpenClaw Gateway:"
	log "     sudo systemctl enable --now openclaw-gateway"
	log ""
	log "  3. Add cron jobs (as yakob, after gateway is running):"
	log "     openclaw cron add --name 'Worker sweep' --cron '0 */2 * * *' --tz UTC --session main --system-event 'Check for blocked workers and stale tasks. Run check-workers.sh.' --wake now"
	log "     openclaw cron add --name 'Daily summary' --cron '0 17 * * *' --tz UTC --session isolated --message 'Summarize today. Run yx ls, check-workers.sh, ./cost-summary.sh --today.' --announce"
	log ""
	log "  4. Verify:"
	log "     openclaw doctor"
	log "     openclaw agents list"
}

main "$@"
