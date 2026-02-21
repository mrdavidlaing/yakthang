# Docker Mode

## Overview

The Yak Orchestrator supports two runtime modes:
- **Zellij mode**: Workers run directly in Zellij tabs as native processes
- **Docker mode**: Workers run inside Docker containers, launched in Zellij tabs

Both modes present the same UX — the worker gets a Zellij tab with the
opencode TUI in the top pane and a shell in the bottom pane. Docker mode
adds container isolation around the opencode process.

## Prerequisites

- Docker Engine installed, user in docker group
- Zellij running (Docker workers still use Zellij tabs for the TUI)
- `OPENCODE_API_KEY` exported in your environment (e.g. `~/.profile`)

Worker images are built automatically from `.devcontainer/devcontainer.json`
(or a default base image when no devcontainer config is present). The old
`worker.Dockerfile` approach has been replaced.

## How Docker Mode Works

Docker mode is **not** headless. The flow is:

1. `yak-box spawn` writes the prompt to a temp file
2. It generates a wrapper script that runs `docker run -it` with the prompt
3. It creates a Zellij tab layout that runs the wrapper as a `command` pane
4. The container runs opencode interactively — the TUI renders in the pane

The container gets a PTY via `docker run -it`, which opencode needs for its
TUI. The Zellij command pane provides the terminal that Docker bridges into.

## Runtime Detection

yak-box spawn auto-detects the runtime (Docker first, then Zellij):

```bash
# Force Docker mode
./bin/yak-box spawn --runtime docker --cwd . --name test --yaks test/foo "Do the thing"

# Force Zellij mode  
./bin/yak-box spawn --runtime zellij --cwd . --name test --yaks test/foo "Do the thing"
```

## Building the Worker Image

Worker images are now configured via `.devcontainer/devcontainer.json`:

```json
{
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "features": { ... },
  "postCreateCommand": "apt-get update && ...",
  "containerEnv": { "MY_VAR": "value" },
  "mounts": [ ... ]
}
```

If no devcontainer config is found, yak-box uses a default base image.
The image is built/pulled automatically on first spawn — no manual build
step required.

For the repository-root devcontainer at `.devcontainer/`:
```bash
# Verify the config
cat .devcontainer/devcontainer.json
```

## Container Architecture

### What gets mounted

| Mount | Target | Mode | Purpose |
|-------|--------|------|---------|
| Workspace root | Same path | rw | Code access (git repo) |
| .yaks directory | Same path | rw | Task state (yx) |
| Prompt file | /opt/worker/prompt.txt | ro | Worker instructions |
| Inner script | /opt/worker/start.sh | ro | Startup script |

### What is ephemeral (tmpfs)

| Mount | Size | Purpose |
|-------|------|---------|
| /tmp | 2g | Bun runtime, native binaries (needs `exec`) |

### Persistent worker homes

Each worker persona gets a persistent home directory at
`.yak-boxes/@home/{Persona}/`. This directory is bind-mounted into the
container and survives restarts, crashes, and kills.

| Path | Purpose |
|------|---------|
| `.yak-boxes/@home/{Persona}/` | Worker home directory |
| `.yak-boxes/@home/{Persona}/.local/share/opencode/opencode.db` | OpenCode SQLite DB |
| `.yak-boxes/@home/{Persona}/.bash_history` | Shell history |

This replaced the previous ephemeral tmpfs approach where worker state
was lost on container shutdown.

### Security hardening

All of these are applied and have been tested with opencode:

| Flag | Effect |
|------|--------|
| `--read-only` | Root filesystem is read-only |
| `--cap-drop ALL` | No Linux capabilities |
| `--security-opt no-new-privileges` | No privilege escalation |
| `--user $(id -u):$(id -g)` | Runs as host user (non-root) |
| `--network bridge` | Network access for LLM API |

### Critical: tmpfs needs `exec`

The `--tmpfs /tmp` mount **must** include `exec`:

```
--tmpfs /tmp:rw,exec,size=2g
```

OpenCode uses Bun, which extracts native `.node` binaries to `/tmp` and
executes them. Without `exec`, Bun fails silently and the TUI never renders.
This was the root cause of the "blank pane" bug during Docker worker development.

## Authentication

Workers receive `OPENCODE_API_KEY` as an environment variable. The key must
be set in the spawning user's environment before running yak-box spawn.

```bash
# In ~/.profile or ~/.bashrc (before the interactive guard)
export OPENCODE_API_KEY="sk-open-..."
```

yak-box spawn will refuse to start a Docker worker if the key is not set.

The container does NOT mount the host's `$HOME` or opencode auth.json.
Each worker persona has a persistent home directory at `.yak-boxes/@home/{Persona}/`
(see [Persistent worker homes](#persistent-worker-homes) above).

## Testing

### E2E smoke test

```bash
./bin/yak-box spawn --runtime docker --cwd . --name test-docker \
  --yaks test/docker-yak "Say hello and report done via yx"

# Check the worker tab — TUI should render with the opencode interface
# Check task status
yx field --show test/docker-yak agent-status

# Cleanup
docker kill yak-worker-test-docker
```

### Verify security flags

```bash
docker inspect yak-worker-test-docker --format '
  CapDrop={{.HostConfig.CapDrop}}
  ReadOnly={{.HostConfig.ReadonlyRootfs}}
  SecurityOpt={{.HostConfig.SecurityOpt}}
  User={{.Config.User}}'
```

## Troubleshooting

### Blank pane — TUI doesn't render

**Cause**: `--tmpfs /tmp` without `exec` flag. Bun can't load native binaries.

**Fix**: Ensure tmpfs has `exec`: `--tmpfs /tmp:rw,exec,size=2g`

### "OPENCODE_API_KEY not set" error

**Cause**: Key not in environment. yak-box spawn checks before launching.

**Fix**: Export the key in your shell profile and source it.

### "invalid x-api-key" in opencode TUI

**Cause**: Key is truncated or incorrect.

**Fix**: Verify with `echo $OPENCODE_API_KEY | wc -c` (should be ~108 chars).

### Permission denied errors inside container

**Cause**: `--user` flag without matching tmpfs uid/gid.

**Fix**: tmpfs mounts need `uid=` and `gid=` to match the `--user` value.

### Container exits immediately

**Cause**: opencode crash, usually from missing write permissions.

**Fix**: Check `docker logs <container-name>` for the Bun/EACCES error message.

## Related Docs

- [Worker Spawning](../worker-spawning.md) — yak-box spawn design
- [Security](../deployment/SECURITY.md) — full security model
- [Troubleshooting](../deployment/TROUBLESHOOTING.md) — general issues
