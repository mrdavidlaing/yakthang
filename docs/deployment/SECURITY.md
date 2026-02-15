# Security Policy: Container Hardening

## Overview

Docker workers run with a hardened container configuration. The container
filesystem is read-only, all Linux capabilities are dropped, privilege
escalation is blocked, and the process runs as a non-root user. Writable
areas are limited to explicit tmpfs mounts and bind-mounted volumes.

## Container Security Flags

All of these are applied by spawn-worker.sh and verified working with opencode:

| Flag | Purpose |
|------|---------|
| `--read-only` | Root filesystem is read-only |
| `--cap-drop ALL` | Drop all Linux capabilities |
| `--security-opt no-new-privileges` | Prevent privilege escalation via setuid/setgid |
| `--user $(id -u):$(id -g)` | Run as host user, not root |
| `--pids-limit` | Limit process count (DoS prevention) |
| `--cpus` / `--memory` | Resource limits per profile |

## Filesystem Layout

### Read-only base image

The container's root filesystem is read-only. The image contains only:
- Ubuntu 24.04 base packages
- git, bash, curl, ca-certificates
- opencode CLI (`/usr/local/bin/opencode`)
- yx binary (`/usr/local/bin/yx`)

### Writable tmpfs mounts

| Mount | Options | Size | Purpose |
|-------|---------|------|---------|
| `/tmp` | rw,exec | 2g | Bun runtime extracts and executes native `.node` binaries here |
| `/home/worker` | rw,exec | 1g | Opencode data: logs, storage, plugins, config |
| `/home/worker/.cache` | rw,exec | 1g | Bun/npm package cache |

**Critical**: `/tmp` must have `exec`. Opencode uses Bun, which extracts
native addon binaries to `/tmp` and dlopen()s them. Without `exec`, the file
watcher binding fails and the TUI never renders (blank pane).

### Bind-mounted volumes

| Host path | Container path | Mode | Purpose |
|-----------|---------------|------|---------|
| Workspace root | Same path | rw | Git repo access |
| .yaks directory | Same path | rw | Task state |
| Prompt file | /opt/worker/prompt.txt | ro | Worker instructions |
| Inner script | /opt/worker/start.sh | ro | Startup script |

The workspace is mounted at the same absolute path so that git operations
and file references work identically inside and outside the container.

## Network Policy

Workers currently use `--network bridge` because they need HTTPS access to
the LLM API (api.anthropic.com). This is the minimum required for opencode
to function.

**TODO**: Implement network filtering (see `docker-workers/network-filtering`)
to restrict outbound connections to only the LLM API endpoints.

## Authentication

Workers receive `OPENCODE_API_KEY` as an environment variable passed via
`docker run -e`. The key is:

- NOT baked into the Docker image
- NOT stored in any file inside the container
- NOT persisted (container is ephemeral, tmpfs is volatile)
- Sourced from the spawning user's environment

spawn-worker.sh will exit with an error if the key is not set.

## Non-root Execution

Containers run as the host user via `--user $(id -u):$(id -g)`. This means:

- The process cannot modify the container's root filesystem (also read-only)
- Files created in the workspace have correct host ownership
- The HOME directory is set to `/home/worker` (a tmpfs mount)
- tmpfs mounts use `uid=` and `gid=` flags to match the user

## Troubleshooting

### "Network is unreachable" during setup

**Problem**: `--setup-network` flag not working or network still blocked

**Solution**:
1. Verify flag is passed: `./spawn-worker.sh --setup-network ...`
2. Check Docker daemon is running: `docker ps`
3. Verify container network mode: `docker inspect <container> | grep NetworkMode`
4. Check Docker network configuration: `docker network ls`

### "Connection refused" during work phase

**Problem**: Worker trying to access external service during Phase 2

**Solution**:
1. This is expected behavior - network is isolated
2. Move the operation to Phase 1 (setup) if needed
3. Or pre-cache/pre-download required resources in Phase 1

### Dependency installation fails in Phase 1

**Problem**: Package registry unreachable even with `--setup-network`

**Solution**:
1. Check host network connectivity: `curl https://registry.npmjs.org`
2. Verify Docker network configuration
3. Check for firewall/proxy rules blocking container traffic
4. Consider using a private registry or mirror

## References

- [Docker Network Documentation](https://docs.docker.com/network/)
- [Container Security Best Practices](https://docs.docker.com/engine/security/)
- [NIST Container Security Guidelines](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-190.pdf)

---

# Credential Management

## Three-Layer Security Model

The Yak orchestration system uses a three-layer security model to manage credentials and access control:

### Layer 1: VM Boundary (GCP)

**Scope**: GCP Compute Engine VM access

**Credentials**: SSH keys, GCP IAM permissions

**Management**: GCP Console, gcloud CLI

**Purpose**: Controls who can access the VM itself

**Setup**:
- Use GCP OS Login for SSH key management
- Restrict firewall rules (no inbound except SSH)
- Use service accounts with minimal permissions
- Enable VPC Service Controls for additional isolation

### Layer 2: yakob User (Host)

**Scope**: yakob user on the VM

**Credentials**: OPENCODE_API_KEY environment variable

**Management**: Set in yakob's shell profile or systemd service

**Purpose**: Orchestrator and workers inherit this key

**Setup**:
- Add to `/etc/systemd/system/openclaw-gateway.service`:
  ```ini
  [Service]
  Environment="OPENCODE_API_KEY=sk-open-..."
  ```
- Or add to yakob's `~/.bashrc`:
  ```bash
  export OPENCODE_API_KEY="sk-open-..."
  ```
- Ensure file permissions: `chmod 600 ~/.bashrc`
- Verify inheritance: `echo $OPENCODE_API_KEY` (should show key)

### Layer 3: Ephemeral Workers (Containers)

**Scope**: Individual Docker containers

**Credentials**: Inherited from yakob via `-e` flag

**Management**: Automatic via spawn-worker.sh

**Purpose**: Workers use API key for OpenCode operations

**Implementation**: spawn-worker.sh passes `-e OPENCODE_API_KEY="${OPENCODE_API_KEY:-}"` (line 265)

**Security Properties**:
- API key not baked into Docker image
- Each container gets fresh environment
- Containers destroyed after task completion
- No credential persistence in container filesystem

## API Key Handling

### Storage

**Primary Method**: OPENCODE_API_KEY stored in yakob user environment only

**Options**:
1. **Systemd service file** (recommended for production):
   - Path: `/etc/systemd/system/openclaw-gateway.service`
   - Permissions: `chmod 600` (root-owned)
   - Survives reboots, managed by systemd

2. **Shell profile** (development/testing):
   - Path: `~/.bashrc` or `~/.profile`
   - Permissions: `chmod 600` (yakob-loaded)
   - Loaded on interactive login

### Transmission

**Container Passthrough**: Docker `-e` flag passes environment variable to container

**Implementation**:
```bash
docker run -e OPENCODE_API_KEY="${OPENCODE_API_KEY:-}" ...
```

**Security Notes**:
- `${OPENCODE_API_KEY:-}` syntax prevents errors if unset
- Environment variables visible in `docker inspect` (restrict access)
- Not logged by Docker daemon (unlike command arguments)

### Rotation

**Procedure**:
1. Generate new API key in OpenCode dashboard
2. Update yakob's environment:
   ```bash
   sudo systemctl edit openclaw-gateway
   # Update Environment="OPENCODE_API_KEY=sk-open-NEW..."
   ```
3. Restart orchestrator service:
   ```bash
   sudo systemctl restart openclaw-gateway
   ```
4. Kill existing workers (they have old key):
   ```bash
   docker ps -q --filter "ancestor=yak-worker:latest" | xargs docker kill
   ```
5. Revoke old key in OpenCode dashboard

**Frequency**: Quarterly or after team changes

### Revocation

**Emergency Procedure**:
1. Revoke key in OpenCode dashboard (immediate effect)
2. Remove from yakob's environment:
   ```bash
   sudo systemctl edit openclaw-gateway
   # Remove Environment="OPENCODE_API_KEY=..." line
   ```
3. Restart service and kill workers:
   ```bash
   sudo systemctl restart openclaw-gateway
   docker ps -q --filter "ancestor=yak-worker:latest" | xargs docker kill
   ```

## Credential Directory Structure (Conceptual)

If additional credentials are needed in the future, use this structure:

```
/etc/yak-creds/
  opencode-api-key     # Alternative to environment variable
  github-token         # For private repo access (if needed)
  docker-registry      # For private Docker registries
  ...
```

**Permissions**:
```bash
sudo mkdir -p /etc/yak-creds
sudo chmod 700 /etc/yak-creds
sudo chown yakob:yakob /etc/yak-creds
sudo chmod 600 /etc/yak-creds/*
```

**Usage**:
- Mount as read-only volume: `-v /etc/yak-creds:/creds:ro`
- Workers read from `/creds/opencode-api-key`
- Requires code changes to spawn-worker.sh

**Current Status**: Not implemented (environment variable approach sufficient)

## Deployment Credential Setup

### Initial VM Provisioning

1. **Provision VM** (see `setup-vm.sh`):
   ```bash
   gcloud compute instances create yak-vm \
     --machine-type=e2-standard-4 \
     --image-family=ubuntu-2204-lts \
     --image-project=ubuntu-os-cloud \
     --boot-disk-size=50GB
   ```

2. **SSH into VM**:
   ```bash
   gcloud compute ssh yak-vm
   ```

3. **Run setup script** (creates yakob user, installs Docker):
   ```bash
   sudo bash setup-vm.sh
   ```

### API Key Configuration

4. **Set API key in systemd service**:
   ```bash
   sudo systemctl edit openclaw-gateway
   ```
   
   Add to the override file:
   ```ini
   [Service]
   Environment="OPENCODE_API_KEY=sk-open-api03-YOUR_KEY_HERE"
   ```

5. **Verify inheritance**:
   ```bash
   sudo systemctl start openclaw-gateway
   
   # Check orchestrator has key
   sudo systemctl show openclaw-gateway | grep OPENCODE_API_KEY
   
   # Spawn test worker and check logs
   sudo -u yakob ./spawn-worker.sh --name "test" "echo 'API key present'"
   docker logs <container-id> 2>&1 | grep -i "api"
   ```

6. **Secure the service file**:
   ```bash
   sudo chmod 600 /etc/systemd/system/openclaw-gateway.service.d/override.conf
   ```

### Verification Checklist

- [ ] yakob user exists: `id yakob`
- [ ] API key in systemd service: `sudo systemctl show openclaw-gateway | grep OPENCODE`
- [ ] Service starts successfully: `sudo systemctl status openclaw-gateway`
- [ ] Workers inherit key: Check container environment with `docker inspect`
- [ ] Orchestrator can spawn workers: `./spawn-worker.sh --name "test" "echo hello"`
- [ ] Service file permissions: `ls -l /etc/systemd/system/openclaw-gateway.service.d/`

## Security Best Practices

### Credential Hygiene

- **Never commit credentials** to git repositories
  - Use `.gitignore` for `.env` files
  - Scan commits with `git-secrets` or `truffleHog`
  - Revoke immediately if accidentally committed

- **Use environment variables** for API keys (not files when possible)
  - Reduces attack surface (no file to steal)
  - Easier to rotate (update service, restart)
  - Standard practice for 12-factor apps

- **Restrict file permissions** (600 for files, 700 for directories)
  - Prevents other users from reading credentials
  - Use `chmod` and `chown` consistently
  - Audit with `find /etc -type f -perm /o+r` (find world-readable files)

### Operational Security

- **Rotate keys regularly** (quarterly or after team changes)
  - Set calendar reminders
  - Document rotation procedure
  - Test rotation in staging first

- **Monitor usage** via OpenCode dashboard for anomalies
   - Check for unexpected usage spikes
   - Review API call patterns
   - Set up billing alerts

- **Revoke immediately** if compromise suspected
  - Follow emergency revocation procedure
  - Investigate root cause
  - Rotate all related credentials

### Access Control

- **Limit VM access** to essential personnel only
  - Use GCP IAM roles (Compute Instance Admin)
  - Enable OS Login for centralized SSH key management
  - Audit access logs regularly

- **Restrict sudo access** on the VM
  - yakob user should NOT have sudo by default
  - Use separate admin account for system changes
  - Log all sudo commands (`/var/log/auth.log`)

- **Isolate orchestrator** from other services
  - Dedicated VM for Yak orchestration
  - No other applications on same VM
  - Reduces blast radius of compromise

### Container Security

- **Workers run as non-root** (enforced by spawn-worker.sh)
  - `--user $(id -u):$(id -g)` flag
  - Prevents privilege escalation
  - Limits damage from container breakout

- **Network isolation** (see Two-Phase Network Isolation section)
  - Default `--network none` prevents exfiltration
  - Use `--setup-network` only for dependency installation
  - Monitor network activity during setup phase

- **Resource limits** prevent DoS
  - `--cpus 2 --memory 4g --pids-limit 512`
  - Prevents single worker from consuming all resources
  - Protects orchestrator and other workers

- **Ephemeral containers** (no persistence)
  - Containers destroyed after task completion
  - No credential persistence in filesystem
  - Fresh environment for each task

## Troubleshooting

### API Key Not Found

**Symptom**: Workers fail with "OPENCODE_API_KEY not set" error

**Diagnosis**:
```bash
# Check yakob's environment
sudo -u yakob env | grep OPENCODE

# Check systemd service
sudo systemctl show openclaw-gateway | grep OPENCODE

# Check container environment
docker inspect <container-id> | grep OPENCODE
```

**Solutions**:
1. Verify key is set in systemd service: `sudo systemctl edit openclaw-gateway`
2. Restart service: `sudo systemctl restart openclaw-gateway`
3. Check service status: `sudo systemctl status openclaw-gateway`
4. Verify spawn-worker.sh passes `-e OPENCODE_API_KEY` (line 265)

### Permission Denied

**Symptom**: Cannot read credential files or access systemd service

**Diagnosis**:
```bash
# Check file permissions
ls -l /etc/systemd/system/openclaw-gateway.service.d/

# Check current user
whoami

# Check yakob user permissions
id yakob
```

**Solutions**:
1. Ensure files are owned by yakob: `sudo chown yakob:yakob <file>`
2. Set correct permissions: `sudo chmod 600 <file>`
3. Use `sudo -u yakob` to run commands as yakob user

### Key Rotation Failed

**Symptom**: Workers still using old API key after rotation

**Diagnosis**:
```bash
# Check if old workers still running
docker ps --filter "ancestor=yak-worker:latest"

# Check orchestrator service status
sudo systemctl status openclaw-gateway
```

**Solutions**:
1. Kill all existing workers: `docker ps -q --filter "ancestor=yak-worker:latest" | xargs docker kill`
2. Restart orchestrator: `sudo systemctl restart openclaw-gateway`
3. Verify new key in service: `sudo systemctl show openclaw-gateway | grep OPENCODE`
4. Spawn test worker to confirm new key works

---

## Security Hardening Checklist

### VM-Level Security

- [ ] **Firewall (UFW)**: Configured to allow SSH only
  - Verify: `sudo ufw status`
  - Should show: Status: active, 22/tcp ALLOW
  
- [ ] **SSH Hardening**: Password authentication disabled, root login disabled
  - Verify: `grep PasswordAuthentication /etc/ssh/sshd_config`
  - Should show: PasswordAuthentication no
  - Verify: `grep PermitRootLogin /etc/ssh/sshd_config`
  - Should show: PermitRootLogin no

- [ ] **Fail2ban**: Installed and running
  - Verify: `sudo systemctl status fail2ban`
  - Should show: active (running)

- [ ] **Docker Daemon**: Security settings applied
  - Verify: `cat /etc/docker/daemon.json`
  - Should contain: no-new-privileges, icc: false, log limits

### Container-Level Security

- [ ] **Capabilities Dropped**: Containers run with --cap-drop ALL
  - Verify: `docker inspect <container> --format '{{.HostConfig.CapDrop}}'`
  - Should show: [ALL]

- [ ] **Read-Only Root**: Containers have read-only rootfs
  - Verify: `docker inspect <container> --format '{{.HostConfig.ReadonlyRootfs}}'`
  - Should show: true

- [ ] **Tmpfs Mounts**: Writable areas use tmpfs (ephemeral, with exec)
  - Verify: `docker inspect <container> --format '{{.HostConfig.Tmpfs}}'`
  - Should show: /tmp, /home/worker, /home/worker/.cache
  - /tmp MUST have exec (Bun native binaries)

- [ ] **Non-root User**: Containers run as host user
  - Verify: `docker inspect <container> --format '{{.Config.User}}'`
  - Should show: host uid:gid (e.g. 1003:1004)

- [ ] **Network**: Bridge mode (required for LLM API access)
  - Verify: `docker inspect <container> --format '{{.HostConfig.NetworkMode}}'`
  - Should show: bridge
  - TODO: implement network filtering to restrict to API endpoints only

- [ ] **No New Privileges**: Containers cannot gain privileges
  - Verify: `docker inspect <container> --format '{{.HostConfig.SecurityOpt}}'`
  - Should show: [no-new-privileges]

- [ ] **Resource Limits**: CPU, memory, pids limits applied
  - Verify: `docker inspect <container> --format '{{.HostConfig.NanoCpus}} {{.HostConfig.Memory}} {{.HostConfig.PidsLimit}}'`
  - Should show: non-zero values

### Credential Security

- [ ] **API Key**: Stored in systemd service environment only
  - Verify: `sudo systemctl cat openclaw-gateway | grep OPENCODE_API_KEY`
  - Should NOT show actual key value in git

- [ ] **Service File Permissions**: Restricted to root
  - Verify: `ls -l /etc/systemd/system/openclaw-gateway.service`
  - Should show: -rw------- root root

### Monitoring

- [ ] **Failed SSH Attempts**: Monitor fail2ban logs
  - Command: `sudo fail2ban-client status sshd`

- [ ] **Container Escapes**: Monitor for privilege escalation attempts
  - Command: `sudo journalctl -u docker -f | grep -i "privilege\|escape"`

- [ ] **Resource Usage**: Monitor container resource consumption
  - Command: `docker stats --no-stream`
