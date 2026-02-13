# Yak Orchestrator Troubleshooting Guide

## VM Provisioning Issues

### setup-vm.sh fails with "command not found"

**Symptom**: Script exits with errors about missing commands.

**Cause**: Prerequisites not installed or PATH issues.

**Solution**:
```bash
# Ensure running as root
sudo bash setup-vm.sh

# Check Ubuntu version
lsb_release -a  # Should be 24.04
```

### Docker installation fails

**Symptom**: `docker ps` returns "command not found" after setup.

**Cause**: Docker installation failed or not in PATH.

**Solution**:
```bash
# Reinstall Docker manually
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker yakob
```

## Orchestrator Service Issues

### Service won't start

**Symptom**: `systemctl start yak-orchestrator` fails.

**Cause**: Missing API key, zellij not found, or orchestrator.kdl missing.

**Solution**:
```bash
# Check service logs
sudo journalctl -u yak-orchestrator -n 50

# Common fixes:
# 1. Set API key
sudo systemctl edit yak-orchestrator
# Add: Environment="ANTHROPIC_API_KEY=sk-ant-..."

# 2. Verify zellij installed
which zellij  # Should show /usr/local/bin/zellij

# 3. Verify orchestrator.kdl exists
ls /home/yakob/workspace/orchestrator.kdl
```

### Can't attach to Zellij session

**Symptom**: `zellij attach yak-orchestrator` says "session not found".

**Cause**: Service not running or session name mismatch.

**Solution**:
```bash
# Check service status
sudo systemctl status yak-orchestrator

# List all sessions
zellij list-sessions

# If service running but no session, restart
sudo systemctl restart yak-orchestrator
```

## Worker Issues

### Workers won't spawn

**Symptom**: `spawn-worker.sh` exits with errors.

**Cause**: Docker not running, image missing, or permission issues.

**Solution**:
```bash
# Check Docker running
docker ps

# Check worker image exists
docker images | grep yak-worker

# Rebuild if missing
cd /home/yakob/workspace
docker build -f worker.Dockerfile -t yak-worker:latest .

# Check spawn-worker.sh executable
chmod +x spawn-worker.sh
```

### Workers spawn but exit immediately

**Symptom**: Containers appear in `docker ps -a` with Exited status.

**Cause**: Mock opencode exits immediately (known issue).

**Solution**: This is expected with mock installations. Check logs:
```bash
docker logs yak-worker-<name>
```

### Workers can't write to .yaks/

**Symptom**: Permission denied errors in worker logs.

**Cause**: UID mismatch between host and container.

**Solution**: Verify spawn-worker.sh uses `--user $(id -u):$(id -g)`:
```bash
grep "user.*id -u" spawn-worker.sh
```

## Network Issues

### Workers have network access (security issue)

**Symptom**: Workers can curl external sites.

**Cause**: --setup-network flag used or network mode incorrect.

**Solution**:
```bash
# Verify default network mode is none
docker inspect yak-worker-<name> --format '{{.HostConfig.NetworkMode}}'
# Should show: none

# Check spawn-worker.sh
grep "network.*none" spawn-worker.sh
```

## Security Issues

### SSH password authentication still enabled

**Symptom**: Can SSH with password.

**Cause**: SSH hardening not applied.

**Solution**:
```bash
# Check SSH config
grep PasswordAuthentication /etc/ssh/sshd_config
# Should show: PasswordAuthentication no

# If not, fix and reload
sudo sed -i 's/PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo systemctl reload sshd
```

### Containers have capabilities

**Symptom**: `docker inspect` shows capabilities not dropped.

**Cause**: spawn-worker.sh missing --cap-drop ALL.

**Solution**:
```bash
# Verify flag present
grep "cap-drop" spawn-worker.sh

# Test with new worker
RUNTIME=docker ./spawn-worker.sh --cwd . --name test "echo test"
docker inspect yak-worker-test --format '{{.HostConfig.CapDrop}}'
# Should show: [ALL]
```

## Performance Issues

### Workers running out of memory

**Symptom**: Workers killed with OOM errors.

**Cause**: Memory limit too low for task.

**Solution**: Use heavy resource profile:
```bash
./spawn-worker.sh --resources heavy --cwd . --name heavy-task "..."
```

### Too many workers running

**Symptom**: VM performance degraded.

**Solution**: Check and cleanup:
```bash
# Check running workers
./check-workers.sh

# Kill specific worker
./kill-worker.sh <worker-name>

# Cleanup stopped containers
./cleanup-workers.sh
```

## Getting Help

1. Check service logs: `sudo journalctl -u yak-orchestrator -f`
2. Check Docker logs: `docker logs yak-worker-<name>`
3. Check yx task status: `yx field --show <task> agent-status`
4. Review security checklist: See [SECURITY.md](./SECURITY.md)
5. Review operations guide: See [OPERATIONS.md](./OPERATIONS.md)
