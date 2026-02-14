# Yak Orchestrator Deployment Guide

## Overview

This guide covers deploying the Yak Orchestrator to a GCP Compute Engine VM running Ubuntu 24.04.

## Prerequisites

- GCP project with Compute Engine API enabled
- gcloud CLI installed and authenticated
- SSH key configured for GCP
- Secrets ready in 1Password: ANTHROPIC_API_KEY, SLACK_APP_TOKEN, SLACK_BOT_TOKEN

## VM Provisioning

### 1. Create VM Instance

```bash
gcloud compute instances create yak-orchestrator \
  --zone=us-central1-a \
  --machine-type=e2-standard-2 \
  --image-family=ubuntu-2404-lts-amd64 \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=50GB \
  --tags=yak-orchestrator
```

### 2. Configure Firewall

```bash
# Allow SSH only (UFW will be configured by setup-vm.sh)
gcloud compute firewall-rules create allow-ssh-yak \
  --allow tcp:22 \
  --target-tags yak-orchestrator \
  --source-ranges 0.0.0.0/0
```

### 3. Clone Repo and Run Setup

```bash
gcloud compute ssh yak-orchestrator --zone=us-central1-a

# As root on the VM:
sudo -i
cd /home/yakob
git clone <repo-url> yakthang
cd yakthang
bash setup-vm.sh
# Script will prompt for: git identity, ANTHROPIC_API_KEY, Slack tokens
```

The script will:
- Install Docker Engine, git, zellij, watch, jq, Node.js 22
- Install OpenCode CLI, OpenClaw Gateway, and yx
- Create yakob user with docker group
- Configure git identity
- **Prompt for secrets** (ANTHROPIC_API_KEY, Slack tokens from 1Password)
- Generate `~/.openclaw/openclaw.json` with workspace config
- Build yak-worker:latest container image
- Create systemd service with secret override
- Apply security hardening (UFW, SSH, Docker daemon)

Workspace identity files (SOUL.md, AGENTS.md, etc.) come from the git repo —
no manual copying or import scripts needed.

### 4. Start Orchestrator

```bash
# Start a Zellij session for workers
su - yakob -c "zellij --session yak-workers"

# Enable and start the gateway
sudo systemctl enable --now openclaw-gateway
```

### 5. Add Cron Jobs (as yakob)

```bash
su - yakob
openclaw cron add --name 'Worker sweep' --cron '0 */2 * * *' --tz UTC \
  --session main --system-event 'Check for blocked workers and stale tasks. Run check-workers.sh.' --wake now
openclaw cron add --name 'Daily summary' --cron '0 17 * * *' --tz UTC \
  --session isolated --message 'Summarize today. Run yx ls, check-workers.sh, ./cost-summary.sh --today.' --announce
```

## Verification Checklist

- [ ] VM created and accessible via SSH
- [ ] setup-vm.sh completed without errors
- [ ] yakob user exists: `id yakob`
- [ ] Docker works: `sudo -u yakob docker ps`
- [ ] Worker image exists: `docker images | grep yak-worker`
- [ ] OpenClaw config exists: `ls ~/.openclaw/openclaw.json`
- [ ] Systemd service running: `systemctl status openclaw-gateway`
- [ ] Systemd override has secrets: `sudo cat /etc/systemd/system/openclaw-gateway.service.d/override.conf`
- [ ] Workspace files present: `ls /home/yakob/yakthang/.openclaw/workspace/SOUL.md`
- [ ] OpenClaw healthy: `openclaw doctor`
- [ ] Agent configured: `openclaw agents list`
- [ ] Cron jobs active: `openclaw cron list`

## Next Steps

- See [OPERATIONS.md](./OPERATIONS.md) for day-to-day operations
- See [SECURITY.md](./SECURITY.md) for security architecture
- See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for common issues
