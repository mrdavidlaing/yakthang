# Yak Orchestrator Deployment Guide

## Overview

This guide covers deploying the Yak Orchestrator to a GCP Compute Engine VM running Ubuntu 24.04.

## Prerequisites

- GCP project with Compute Engine API enabled
- gcloud CLI installed and authenticated
- SSH key configured for GCP

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

### 3. Copy Provisioning Script

```bash
gcloud compute scp setup-vm.sh yak-orchestrator:~ --zone=us-central1-a
```

### 4. Run Provisioning Script

```bash
# Set git config environment variables (optional, will prompt if not set)
export YAKOB_GIT_NAME="Your Name"
export YAKOB_GIT_EMAIL="your.email@example.com"

# Run as root
gcloud compute ssh yak-orchestrator --zone=us-central1-a -- \
  "sudo YAKOB_GIT_NAME='$YAKOB_GIT_NAME' YAKOB_GIT_EMAIL='$YAKOB_GIT_EMAIL' bash setup-vm.sh"
```

The script will:
- Install Docker Engine, git, zellij, watch, jq
- Install OpenCode CLI and yx (currently mock - see Known Issues)
- Create yakob user with docker group
- Configure git identity
- Build yak-worker:latest container image
- Create systemd service file
- Apply security hardening (UFW, SSH, fail2ban, Docker daemon)

### 5. Copy Workspace Files

```bash
# Copy orchestrator files to VM
gcloud compute scp --recurse \
  orchestrator.kdl spawn-worker.sh check-workers.sh kill-worker.sh cleanup-workers.sh \
  worker.Dockerfile .yaks/ \
  yak-orchestrator:/home/yakob/workspace/ \
  --zone=us-central1-a
```

### 6. Set API Key

```bash
gcloud compute ssh yak-orchestrator --zone=us-central1-a

# Edit service to add API key
sudo systemctl edit yak-orchestrator
# Add this line:
# Environment="ANTHROPIC_API_KEY=sk-ant-..."

# Save and exit
```

### 7. Start Orchestrator

```bash
sudo systemctl start yak-orchestrator
sudo systemctl status yak-orchestrator

# Enable auto-start on boot (optional)
sudo systemctl enable yak-orchestrator
```

### 8. Attach to Session

```bash
zellij attach yak-orchestrator
```

## Verification Checklist

- [ ] VM created and accessible via SSH
- [ ] setup-vm.sh completed without errors
- [ ] yakob user exists: `id yakob`
- [ ] Docker works: `sudo -u yakob docker ps`
- [ ] Worker image exists: `docker images | grep yak-worker`
- [ ] Systemd service file exists: `ls /etc/systemd/system/yak-orchestrator.service`
- [ ] API key set: `sudo systemctl cat yak-orchestrator | grep ANTHROPIC_API_KEY`
- [ ] Orchestrator running: `sudo systemctl status yak-orchestrator`
- [ ] Zellij session active: `zellij list-sessions`
- [ ] Can attach: `zellij attach yak-orchestrator`

## Known Issues

### Mock OpenCode/yx Installations

**Issue**: setup-vm.sh uses mock scripts for opencode and yx (echo version numbers only).

**Impact**: Workers will spawn but opencode won't actually execute tasks.

**Workaround**: Replace mock installations with real binaries before production use.

**Tracking**: See `.sisyphus/notepads/persistent-vm-plan/issues.md`

## Next Steps

- See [OPERATIONS.md](./OPERATIONS.md) for day-to-day operations
- See [SECURITY.md](./SECURITY.md) for security architecture
- See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for common issues
