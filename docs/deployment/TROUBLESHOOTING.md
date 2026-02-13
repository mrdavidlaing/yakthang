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

### Workers spawn but TUI is blank (no output in pane)

**Symptom**: Zellij tab opens but the opencode pane shows nothing.

**Cause**: `--tmpfs /tmp` mounted without `exec` flag. Opencode uses Bun,
which extracts native `.node` binaries to `/tmp` and needs to execute them.
Without `exec`, the file watcher binding fails silently and the TUI never
renders.

**Solution**: Ensure the tmpfs mount includes `exec`:
```bash
--tmpfs /tmp:rw,exec,size=2g
```

**History**: This was the root cause of the Docker worker "blank pane" bug.
The default tmpfs mount options include `noexec`, which prevents Bun from
loading its native addon via `dlopen()`.

### Workers spawn but show "invalid x-api-key"

**Symptom**: TUI renders but the LLM response shows an auth error.

**Cause**: `ANTHROPIC_API_KEY` is truncated or incorrect.

**Solution**: Verify the key length and contents:
```bash
echo $ANTHROPIC_API_KEY | wc -c  # Should be ~108 chars
```

### Workers spawn but exit immediately

**Symptom**: Containers appear in `docker ps -a` with Exited status.

**Cause**: Worker command failed — usually a permission error from opencode
trying to write to a read-only filesystem without proper tmpfs mounts.

**Solution**: Check container logs for the error:
```bash
docker logs yak-worker-<name>
```

Common errors:
- `EACCES: permission denied, mkdir '/.local'` — HOME not set or not writable
- `EROFS: read-only file system` — missing tmpfs mount for a writable path

### Workers can't write to .yaks/

**Symptom**: Permission denied errors when yx tries to update task state.

**Cause**: UID mismatch between host and container.

**Solution**: Verify spawn-worker.sh uses `--user $(id -u):$(id -g)` and
that the .yaks directory is writable by that user on the host.

## Network Issues

### Workers can't reach the LLM API

**Symptom**: opencode TUI shows connection errors or timeouts.

**Cause**: Network mode or DNS issue.

**Solution**:
```bash
# Verify network mode is bridge
docker inspect yak-worker-<name> --format '{{.HostConfig.NetworkMode}}'
# Should show: bridge

# Test connectivity from inside container
docker exec yak-worker-<name> curl -s -o /dev/null -w "%{http_code}" https://api.anthropic.com
# Should show: 404 (endpoint exists but no auth)
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
