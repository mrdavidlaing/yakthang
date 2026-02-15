# Yak Orchestrator Operations Guide

This guide covers running and managing the Yak Orchestrator on a GCP VM, including service management, team access, and multi-user collaboration.

## Service Management

### Starting the Orchestrator

The orchestrator runs as a systemd service named `openclaw-gateway`.

```bash
# 1. Set the OPENCODE_API_KEY (one-time setup)
sudo systemctl edit openclaw-gateway
# Add this line in the [Service] section:
# Environment="OPENCODE_API_KEY=sk-open-..."

# 2. Verify orchestrator.kdl is in place
ls -l /home/yakob/workspace/orchestrator.kdl

# 3. Start the service
sudo systemctl start openclaw-gateway

# 4. Check status
sudo systemctl status openclaw-gateway

# 5. Enable auto-start on boot (optional)
sudo systemctl enable openclaw-gateway
```

### Stopping the Orchestrator

```bash
# Graceful stop
sudo systemctl stop openclaw-gateway

# Force stop if needed
sudo systemctl kill openclaw-gateway

# Verify it stopped
sudo systemctl status openclaw-gateway
```

### Viewing Logs

```bash
# Follow logs in real-time
sudo journalctl -u openclaw-gateway -f

# View recent logs (last 100 lines)
sudo journalctl -u openclaw-gateway -n 100

# View logs since last boot
sudo journalctl -u openclaw-gateway -b

# View logs from specific time
sudo journalctl -u openclaw-gateway --since "2 hours ago"
```

### Service Configuration

The service file is located at `/etc/systemd/system/openclaw-gateway.service`.

**Key configuration:**
- **User**: yakob (non-root)
- **WorkingDirectory**: /home/yakob/workspace
- **ExecStart**: Launches zellij with orchestrator.kdl layout
- **Session Name**: yakthang (for team attachment)
- **Restart Policy**: on-failure with 10-second delay

**To modify the service:**

```bash
# Edit the service file (creates override)
sudo systemctl edit openclaw-gateway

# Reload systemd after changes
sudo systemctl daemon-reload

# Restart the service
sudo systemctl restart openclaw-gateway
```

## Team Access

### SSH to VM

**Using gcloud (GCP):**
```bash
gcloud compute ssh openclaw-gateway --zone=us-central1-a
```

**Using direct SSH (if configured):**
```bash
ssh yakob@<vm-ip>
```

### Attaching to Orchestrator Session

The orchestrator runs in a Zellij session named `yakthang`. Multiple team members can attach simultaneously.

```bash
# Attach to the orchestrator session
zellij attach yakthang

# If session doesn't exist, check service status
sudo systemctl status openclaw-gateway

# If service is running but session missing, restart it
sudo systemctl restart openclaw-gateway
```

### Zellij Navigation

Once attached to the orchestrator session:

| Key Combination | Action |
|-----------------|--------|
| **Ctrl+o, d** | Detach (session continues running) |
| **Ctrl+o, w** | Switch between tabs |
| **Ctrl+o, n** | New tab |
| **Ctrl+o, x** | Close current tab |
| **Ctrl+o, q** | Quit (closes entire session - **DON'T DO THIS**) |
| **Ctrl+o, ?** | Show help |

### Multi-User Collaboration

Multiple team members can attach to the same orchestrator session simultaneously:

```bash
# Team member 1
ssh yakob@vm
zellij attach yakthang

# Team member 2 (simultaneously)
ssh yakob@vm
zellij attach yakthang

# Team member 3 (simultaneously)
ssh yakob@vm
zellij attach yakthang
```

**Key points:**
- All team members see the same view
- Changes are synchronized in real-time
- Each member can interact independently
- Detaching doesn't affect others (use Ctrl+o, d)
- Only one member should use Ctrl+o, q (closes session for everyone)

### Session Persistence

The Zellij session persists across SSH disconnects. This is critical for long-running orchestration tasks.

**Workflow:**
1. SSH to VM and attach to orchestrator
2. Orchestrator spawns workers (visible in tabs)
3. Close terminal or disconnect SSH
4. Re-SSH and re-attach
5. Session state is preserved (workers still running)

**Example:**
```bash
# Session 1: Start orchestrator and spawn workers
ssh yakob@vm
zellij attach yakthang
# (orchestrator running, workers spawning)
# Ctrl+o, d to detach

# Session 2: Reconnect later
ssh yakob@vm
zellij attach yakthang
# (same session, workers still running)
```

## Monitoring Workers

### Check Worker Status

```bash
# View all tasks and their state
yx ls

# Check specific task status
yx field --show <task-path> agent-status

# View task context
yx context --show <task-path>
```

### Common Worker Commands

```bash
# List all Zellij sessions
zellij list-sessions

# List all Docker containers
docker ps -a

# View worker logs
docker logs <container-id>

# Stop a specific worker
docker stop <container-id>

# Kill a worker
docker kill <container-id>
```

## Common Operations

### Spawning a Worker

From within the orchestrator session:

```bash
./spawn-worker.sh --cwd ./project --name "my-worker" "Work on task X"
```

### Checking Task Status

```bash
# View all tasks
yx ls

# View task context
yx context --show task/path

# View task agent status
yx field --show task/path agent-status
```

### Viewing Worker Output

```bash
# In orchestrator session, switch to worker tab
Ctrl+o, w  # Switch tabs

# Or from outside, view Docker logs
docker logs <worker-container-name>
```

## Troubleshooting

### Orchestrator Won't Start

```bash
# Check service logs
sudo journalctl -u openclaw-gateway -n 50

# Common issues:
# 1. API key not set
#    Fix: sudo systemctl edit openclaw-gateway
#         Add: Environment="OPENCODE_API_KEY=sk-open-..."

# 2. orchestrator.kdl not found
#    Fix: cp orchestrator.kdl /home/yakob/workspace/

# 3. Zellij not installed
#    Fix: Check /usr/local/bin/zellij exists
#         ls -l /usr/local/bin/zellij

# 4. Docker not running
#    Fix: sudo systemctl start docker
```

### Can't Attach to Session

```bash
# List all Zellij sessions
zellij list-sessions

# If openclaw-gateway not listed:
sudo systemctl status openclaw-gateway

# If service is running but session missing:
sudo systemctl restart openclaw-gateway

# If service won't start, check logs:
sudo journalctl -u openclaw-gateway -n 50
```

### Workers Not Spawning

```bash
# Check Docker is running
docker ps

# Check worker image exists
docker images | grep yak-worker

# Check spawn-worker.sh permissions
ls -l spawn-worker.sh
chmod +x spawn-worker.sh

# Check Docker daemon logs
sudo journalctl -u docker -n 50
```

### SSH Disconnects Frequently

```bash
# Add to ~/.ssh/config on your local machine:
Host openclaw-gateway
  HostName <vm-ip>
  User yakob
  ServerAliveInterval 60
  ServerAliveCountMax 10
  TCPKeepAlive yes

# Then connect with:
ssh openclaw-gateway
```

### Permission Denied Errors

```bash
# Verify yakob user exists
id yakob

# Verify yakob is in docker group
groups yakob

# If not in docker group:
sudo usermod -aG docker yakob

# yakob must log out and back in for group to take effect
su - yakob
```

## Maintenance

### Updating the Orchestrator Code

```bash
# SSH to VM
ssh yakob@vm

# Update code in workspace
cd /home/yakob/workspace
git pull origin main

# Restart orchestrator to pick up changes
sudo systemctl restart openclaw-gateway
```

### Rotating API Keys

```bash
# 1. Generate new API key in OpenCode dashboard
# 2. Update systemd service
sudo systemctl edit openclaw-gateway
# Update: Environment="OPENCODE_API_KEY=sk-open-<new-key>"

# 3. Restart orchestrator
sudo systemctl restart openclaw-gateway

# 4. Kill old workers (they'll use old key)
docker kill $(docker ps -q)

# 5. Verify new workers use new key
docker logs <new-worker-container>
```

### Cleaning Up Old Workers

```bash
# Stop all containers
docker stop $(docker ps -q)

# Remove stopped containers
docker container prune -f

# Remove unused images
docker image prune -f

# View disk usage
docker system df
```

## Reference

### Service File Location
- `/etc/systemd/system/openclaw-gateway.service`

### Orchestrator Layout
- `/home/yakob/workspace/orchestrator.kdl`

### Workspace Directory
- `/home/yakob/workspace/`

### Logs
- `sudo journalctl -u openclaw-gateway`

### Related Documentation
- [SECURITY.md](./SECURITY.md) - Credential management and security policies
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Common issues and solutions
- [DEPLOYMENT.md](./DEPLOYMENT.md) - VM provisioning guide
- [../task-management.md](../task-management.md) - Task management with yx
- [../worker-spawning.md](../worker-spawning.md) - Worker spawning details
- [../development/DOCKER-MODE.md](../development/DOCKER-MODE.md) - Local Docker testing
