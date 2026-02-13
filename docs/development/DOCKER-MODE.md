# Local Docker Mode Testing

## Overview

The Yak Orchestrator supports two runtime modes:
- **Zellij mode**: Local development with Zellij tabs (default on macOS/Linux without Docker)
- **Docker mode**: Isolated containers (default when Docker available, required for VM deployment)

This guide covers testing Docker mode locally before deploying to a VM.

## Prerequisites

- Docker Desktop installed (macOS/Windows) or Docker Engine (Linux)
- User in docker group (Linux): `sudo usermod -aG docker $USER`
- Worker image built: `docker build -f worker.Dockerfile -t yak-worker:latest .`

## Runtime Detection

spawn-worker.sh auto-detects the runtime:

```bash
# Check what runtime will be used
RUNTIME="" ./spawn-worker.sh --cwd . --name test "echo test"
# Spawns in Docker if available, else Zellij
```

Override with environment variable:

```bash
# Force Docker mode
RUNTIME=docker ./spawn-worker.sh --cwd . --name test "echo test"

# Force Zellij mode
RUNTIME=zellij ./spawn-worker.sh --cwd . --name test "echo test"
```

## Building Worker Image

```bash
# Build from repository root
docker build -f worker.Dockerfile -t yak-worker:latest .

# Verify image exists
docker images | grep yak-worker
```

## Testing Worker Spawning

### Basic Test

```bash
# Spawn test worker
RUNTIME=docker ./spawn-worker.sh --cwd . --name "test-worker" "echo 'Hello from Docker'"

# Check container running
docker ps --filter "name=yak-worker-test-worker"

# View logs
docker logs yak-worker-test-worker

# Cleanup
docker rm -f yak-worker-test-worker
```

### Volume Mount Test

```bash
# Create test task
mkdir -p .yaks/test-volume
echo "todo" > .yaks/test-volume/state

# Spawn worker that modifies state
RUNTIME=docker ./spawn-worker.sh --cwd . --name "volume-test" \
  "echo 'done' > .yaks/test-volume/state && cat .yaks/test-volume/state"

# Wait and check
docker wait yak-worker-volume-test
cat .yaks/test-volume/state  # Should show "done"

# Cleanup
docker rm yak-worker-volume-test
rm -rf .yaks/test-volume
```

### Network Isolation Test

```bash
# Test default (no network)
RUNTIME=docker ./spawn-worker.sh --cwd . --name "no-net" \
  "curl -s --max-time 5 https://google.com || echo 'Network blocked'"

docker logs yak-worker-no-net
# Should show: "Network blocked"

# Test with setup network
RUNTIME=docker ./spawn-worker.sh --setup-network --cwd . --name "with-net" \
  "curl -s --max-time 5 https://google.com && echo 'Network works'"

docker logs yak-worker-with-net
# Should show: "Network works" or HTML

# Cleanup
docker rm -f yak-worker-no-net yak-worker-with-net
```

### Resource Limits Test

```bash
# Test different profiles
RUNTIME=docker ./spawn-worker.sh --resources light --cwd . --name "light" "echo test"
RUNTIME=docker ./spawn-worker.sh --resources default --cwd . --name "default" "echo test"
RUNTIME=docker ./spawn-worker.sh --resources heavy --cwd . --name "heavy" "echo test"

# Verify limits
docker inspect yak-worker-light --format 'CPU={{.HostConfig.NanoCpus}} Mem={{.HostConfig.Memory}}'
docker inspect yak-worker-default --format 'CPU={{.HostConfig.NanoCpus}} Mem={{.HostConfig.Memory}}'
docker inspect yak-worker-heavy --format 'CPU={{.HostConfig.NanoCpus}} Mem={{.HostConfig.Memory}}'

# Cleanup
docker rm -f yak-worker-light yak-worker-default yak-worker-heavy
```

### Security Flags Test

```bash
# Spawn worker
RUNTIME=docker ./spawn-worker.sh --cwd . --name "security-test" "echo test"

# Verify security flags
docker inspect yak-worker-security-test --format 'CapDrop={{.HostConfig.CapDrop}}'
# Expected: [ALL]

docker inspect yak-worker-security-test --format 'ReadOnly={{.HostConfig.ReadonlyRootfs}}'
# Expected: true

docker inspect yak-worker-security-test --format 'Network={{.HostConfig.NetworkMode}}'
# Expected: none

docker inspect yak-worker-security-test --format 'SecurityOpt={{.HostConfig.SecurityOpt}}'
# Expected: [no-new-privileges]

# Cleanup
docker rm -f yak-worker-security-test
```

## Worker Management Scripts

### kill-worker.sh

```bash
# Spawn long-running worker
RUNTIME=docker ./spawn-worker.sh --cwd . --name "long-task" "sleep 300"

# Kill it
./kill-worker.sh long-task

# Verify stopped
docker ps --filter "name=yak-worker-long-task"
# Should show nothing
```

### cleanup-workers.sh

```bash
# Spawn and stop several workers
RUNTIME=docker ./spawn-worker.sh --cwd . --name "worker-1" "echo done"
RUNTIME=docker ./spawn-worker.sh --cwd . --name "worker-2" "echo done"
RUNTIME=docker ./spawn-worker.sh --cwd . --name "worker-3" "echo done"

# Wait for completion
sleep 5

# Cleanup stopped containers
./cleanup-workers.sh

# Verify cleaned
docker ps -a --filter "name=yak-worker-"
# Should show nothing or only running workers
```

### check-workers.sh

```bash
# Spawn mix of workers
RUNTIME=docker ./spawn-worker.sh --cwd . --name "running" "sleep 60" &
RUNTIME=docker ./spawn-worker.sh --cwd . --name "quick" "echo done"

sleep 2

# Check status
./check-workers.sh
# Should show:
# - Running Workers (Docker): yak-worker-running
# - Stopped Workers (Docker): yak-worker-quick

# Cleanup
./kill-worker.sh running
./cleanup-workers.sh
```

## Known Limitations

### Mock OpenCode/yx

The worker image currently uses mock opencode/yx installations (echo version only).

**Impact**: Workers spawn correctly but don't execute actual opencode tasks.

**Workaround**: For full testing, replace mock installations with real binaries in worker.Dockerfile.

**Tracking**: See `.sisyphus/notepads/persistent-vm-plan/issues.md`

### macOS File Permissions

Docker Desktop on macOS may have permission issues with bind mounts.

**Solution**: Ensure workspace is in a Docker-accessible location (e.g., /Users/).

## Switching Between Modes

### Local Development (Zellij)

```bash
# Use Zellij for interactive development
RUNTIME=zellij ./spawn-worker.sh --cwd ./project --name "dev-worker" "Work on feature X"
```

### Pre-Deployment Testing (Docker)

```bash
# Test Docker mode before VM deployment
RUNTIME=docker ./spawn-worker.sh --cwd ./project --name "test-worker" "Work on feature X"
```

### VM Deployment (Docker Auto)

On the VM, Docker is auto-detected (no RUNTIME override needed):

```bash
# Automatically uses Docker mode
./spawn-worker.sh --cwd ./project --name "prod-worker" "Work on feature X"
```

## Troubleshooting

See [TROUBLESHOOTING.md](../deployment/TROUBLESHOOTING.md) for common issues.

## Next Steps

- Deploy to VM: See [DEPLOYMENT.md](../deployment/DEPLOYMENT.md)
- Operations guide: See [OPERATIONS.md](../deployment/OPERATIONS.md)
- Security architecture: See [SECURITY.md](../deployment/SECURITY.md)
