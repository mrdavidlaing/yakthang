# Security Policy: Two-Phase Network Isolation

## Overview

The worker spawning system implements a **two-phase network isolation strategy** to balance security with operational needs:

- **Phase 1 (Setup)**: Temporary network access for dependency installation
- **Phase 2 (Work)**: Complete network isolation during task execution

This design prevents workers from making unauthorized external connections while allowing legitimate dependency management.

## Rationale

### Why Network Isolation?

Workers execute untrusted code in isolated containers. Without network isolation, a compromised or malicious worker could:
- Exfiltrate sensitive data
- Download malware or backdoors
- Participate in botnet attacks
- Access internal services

### Why Two Phases?

A single "always isolated" approach would prevent legitimate operations:
- `npm install` requires downloading packages from registries
- `pip install` requires PyPI access
- `apt-get install` requires package repository access
- Build tools may need to fetch dependencies

The two-phase approach solves this by:
1. **Allowing network access only when explicitly requested** (Phase 1)
2. **Defaulting to complete isolation** (Phase 2)
3. **Making the security boundary explicit** in the spawn command

## Implementation

### Phase 1: Setup with Network Access

Use the `--setup-network` flag to enable bridge networking for dependency installation:

```bash
# Install dependencies with network access
./spawn-worker.sh --setup-network --cwd ./project --name "setup" "npm install && pip install -r requirements.txt"
```

**Network Mode**: `--network bridge`
- Full access to external networks
- Can reach package registries, CDNs, and external APIs
- Use ONLY for dependency installation

### Phase 2: Work without Network (Default)

Omit the `--setup-network` flag for task execution:

```bash
# Run build/test tasks with network isolation
./spawn-worker.sh --cwd ./project --name "build" "npm run build"
./spawn-worker.sh --cwd ./project --name "test" "npm test"
```

**Network Mode**: `--network none` (default)
- No external network access
- Cannot reach package registries, external APIs, or other hosts
- Prevents data exfiltration and unauthorized outbound connections

## Usage Guidelines

### When to Use `--setup-network`

✅ **Use `--setup-network` for:**
- Initial dependency installation (`npm install`, `pip install`, `apt-get install`)
- Downloading build tools or SDKs
- Fetching configuration from remote sources (if necessary)

### When NOT to Use `--setup-network`

❌ **Never use `--setup-network` for:**
- Running tests or builds (use pre-installed dependencies)
- Executing application code
- Long-running worker tasks
- Any task that doesn't explicitly need external network access

## Security Implications

### Default Isolation (Phase 2)

The default `--network none` provides:
- **Data Exfiltration Prevention**: Workers cannot send data to external hosts
- **Malware Prevention**: Cannot download or execute remote code
- **Lateral Movement Prevention**: Cannot access other services on the network
- **Compliance**: Meets security requirements for untrusted code execution

### Setup Phase Risks (Phase 1)

When using `--setup-network`, be aware:
- **Temporary Exposure**: Network access is only during setup, not during work
- **Package Integrity**: Verify package sources and checksums when possible
- **Dependency Auditing**: Review dependencies before installation
- **Isolation Boundary**: Clearly separate setup and work phases

## Best Practices

1. **Separate Setup and Work**
   ```bash
   # Phase 1: Setup with network
   ./spawn-worker.sh --setup-network --cwd ./project --name "setup" "npm install"
   
   # Phase 2: Work without network
   ./spawn-worker.sh --cwd ./project --name "build" "npm run build"
   ```

2. **Pre-cache Dependencies**
   - Install dependencies once in Phase 1
   - Reuse cached dependencies in Phase 2 workers
   - Reduces setup overhead and improves security

3. **Audit Dependencies**
   - Review `package.json`, `requirements.txt`, etc. before Phase 1
   - Use lock files (`package-lock.json`, `Pipfile.lock`) for reproducibility
   - Consider using private registries for sensitive dependencies

4. **Monitor Setup Phase**
   - Log all network activity during Phase 1
   - Verify expected packages are downloaded
   - Alert on unexpected external connections

## Implementation Details

### Docker Network Modes

The implementation uses Docker's native network modes:

- **`--network none`**: No network interfaces except loopback
  - Prevents all external communication
  - Minimal performance overhead
  - Ideal for isolated task execution

- **`--network bridge`**: Default Docker bridge network
  - Full access to external networks
  - Allows DNS resolution and outbound connections
  - Use only for setup/installation phases

### Related Security Flags

The worker container also applies:
- `--security-opt no-new-privileges`: Prevents privilege escalation
- `--tmpfs /home/worker/.cache`: Ephemeral cache (no persistence)
- `--cpus 2 --memory 4g --pids-limit 512`: Resource limits
- `--user $(id -u):$(id -g)`: Non-root execution

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

**Credentials**: ANTHROPIC_API_KEY environment variable

**Management**: Set in yakob's shell profile or systemd service

**Purpose**: Orchestrator and workers inherit this key

**Setup**:
- Add to `/etc/systemd/system/yak-orchestrator.service`:
  ```ini
  [Service]
  Environment="ANTHROPIC_API_KEY=sk-ant-..."
  ```
- Or add to yakob's `~/.bashrc`:
  ```bash
  export ANTHROPIC_API_KEY="sk-ant-..."
  ```
- Ensure file permissions: `chmod 600 ~/.bashrc`
- Verify inheritance: `echo $ANTHROPIC_API_KEY` (should show key)

### Layer 3: Ephemeral Workers (Containers)

**Scope**: Individual Docker containers

**Credentials**: Inherited from yakob via `-e` flag

**Management**: Automatic via spawn-worker.sh

**Purpose**: Workers use API key for OpenCode operations

**Implementation**: spawn-worker.sh passes `-e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}"` (line 265)

**Security Properties**:
- API key not baked into Docker image
- Each container gets fresh environment
- Containers destroyed after task completion
- No credential persistence in container filesystem

## API Key Handling

### Storage

**Primary Method**: ANTHROPIC_API_KEY stored in yakob user environment only

**Options**:
1. **Systemd service file** (recommended for production):
   - Path: `/etc/systemd/system/yak-orchestrator.service`
   - Permissions: `chmod 600` (root-owned)
   - Survives reboots, managed by systemd

2. **Shell profile** (development/testing):
   - Path: `~/.bashrc` or `~/.profile`
   - Permissions: `chmod 600` (yakob-owned)
   - Loaded on interactive login

### Transmission

**Container Passthrough**: Docker `-e` flag passes environment variable to container

**Implementation**:
```bash
docker run -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" ...
```

**Security Notes**:
- `${ANTHROPIC_API_KEY:-}` syntax prevents errors if unset
- Environment variables visible in `docker inspect` (restrict access)
- Not logged by Docker daemon (unlike command arguments)

### Rotation

**Procedure**:
1. Generate new API key in Anthropic dashboard
2. Update yakob's environment:
   ```bash
   sudo systemctl edit yak-orchestrator
   # Update Environment="ANTHROPIC_API_KEY=sk-ant-NEW..."
   ```
3. Restart orchestrator service:
   ```bash
   sudo systemctl restart yak-orchestrator
   ```
4. Kill existing workers (they have old key):
   ```bash
   docker ps -q --filter "ancestor=yak-worker:latest" | xargs docker kill
   ```
5. Revoke old key in Anthropic dashboard

**Frequency**: Quarterly or after team changes

### Revocation

**Emergency Procedure**:
1. Revoke key in Anthropic dashboard (immediate effect)
2. Remove from yakob's environment:
   ```bash
   sudo systemctl edit yak-orchestrator
   # Remove Environment="ANTHROPIC_API_KEY=..." line
   ```
3. Restart service and kill workers:
   ```bash
   sudo systemctl restart yak-orchestrator
   docker ps -q --filter "ancestor=yak-worker:latest" | xargs docker kill
   ```

## Credential Directory Structure (Conceptual)

If additional credentials are needed in the future, use this structure:

```
/etc/yak-creds/
  anthropic-api-key    # Alternative to environment variable
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
- Workers read from `/creds/anthropic-api-key`
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
   sudo systemctl edit yak-orchestrator
   ```
   
   Add to the override file:
   ```ini
   [Service]
   Environment="ANTHROPIC_API_KEY=sk-ant-api03-YOUR_KEY_HERE"
   ```

5. **Verify inheritance**:
   ```bash
   sudo systemctl start yak-orchestrator
   
   # Check orchestrator has key
   sudo systemctl show yak-orchestrator | grep ANTHROPIC_API_KEY
   
   # Spawn test worker and check logs
   sudo -u yakob ./spawn-worker.sh --name "test" "echo 'API key present'"
   docker logs <container-id> 2>&1 | grep -i "api"
   ```

6. **Secure the service file**:
   ```bash
   sudo chmod 600 /etc/systemd/system/yak-orchestrator.service.d/override.conf
   ```

### Verification Checklist

- [ ] yakob user exists: `id yakob`
- [ ] API key in systemd service: `sudo systemctl show yak-orchestrator | grep ANTHROPIC`
- [ ] Service starts successfully: `sudo systemctl status yak-orchestrator`
- [ ] Workers inherit key: Check container environment with `docker inspect`
- [ ] Orchestrator can spawn workers: `./spawn-worker.sh --name "test" "echo hello"`
- [ ] Service file permissions: `ls -l /etc/systemd/system/yak-orchestrator.service.d/`

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

- **Monitor usage** via Anthropic dashboard for anomalies
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

**Symptom**: Workers fail with "ANTHROPIC_API_KEY not set" error

**Diagnosis**:
```bash
# Check yakob's environment
sudo -u yakob env | grep ANTHROPIC

# Check systemd service
sudo systemctl show yak-orchestrator | grep ANTHROPIC

# Check container environment
docker inspect <container-id> | grep ANTHROPIC
```

**Solutions**:
1. Verify key is set in systemd service: `sudo systemctl edit yak-orchestrator`
2. Restart service: `sudo systemctl restart yak-orchestrator`
3. Check service status: `sudo systemctl status yak-orchestrator`
4. Verify spawn-worker.sh passes `-e ANTHROPIC_API_KEY` (line 265)

### Permission Denied

**Symptom**: Cannot read credential files or access systemd service

**Diagnosis**:
```bash
# Check file permissions
ls -l /etc/systemd/system/yak-orchestrator.service.d/

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
sudo systemctl status yak-orchestrator
```

**Solutions**:
1. Kill all existing workers: `docker ps -q --filter "ancestor=yak-worker:latest" | xargs docker kill`
2. Restart orchestrator: `sudo systemctl restart yak-orchestrator`
3. Verify new key in service: `sudo systemctl show yak-orchestrator | grep ANTHROPIC`
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

- [ ] **Tmpfs Mounts**: Writable areas use tmpfs (ephemeral)
  - Verify: `docker inspect <container> --format '{{.HostConfig.Tmpfs}}'`
  - Should show: /tmp and /home/worker/.cache

- [ ] **Network Isolation**: Default network mode is none
  - Verify: `docker inspect <container> --format '{{.HostConfig.NetworkMode}}'`
  - Should show: none (unless --setup-network used)

- [ ] **No New Privileges**: Containers cannot gain privileges
  - Verify: `docker inspect <container> --format '{{.HostConfig.SecurityOpt}}'`
  - Should show: [no-new-privileges]

- [ ] **Resource Limits**: CPU, memory, pids limits applied
  - Verify: `docker inspect <container> --format '{{.HostConfig.NanoCpus}} {{.HostConfig.Memory}} {{.HostConfig.PidsLimit}}'`
  - Should show: non-zero values

### Credential Security

- [ ] **API Key**: Stored in systemd service environment only
  - Verify: `sudo systemctl cat yak-orchestrator | grep ANTHROPIC_API_KEY`
  - Should NOT show actual key value in git

- [ ] **Service File Permissions**: Restricted to root
  - Verify: `ls -l /etc/systemd/system/yak-orchestrator.service`
  - Should show: -rw------- root root

### Monitoring

- [ ] **Failed SSH Attempts**: Monitor fail2ban logs
  - Command: `sudo fail2ban-client status sshd`

- [ ] **Container Escapes**: Monitor for privilege escalation attempts
  - Command: `sudo journalctl -u docker -f | grep -i "privilege\|escape"`

- [ ] **Resource Usage**: Monitor container resource consumption
  - Command: `docker stats --no-stream`
