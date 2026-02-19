# Agent Instructions for yak-box

## Building

To build and install the binary:

```bash
cd /home/yakob/yakthang/src/yak-box
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o /home/yakob/yakthang/bin/yak-box .
```

This compiles directly to the bin directory with version information embedded at build time.

For development builds without version embedding:

```bash
go build -o /home/yakob/yakthang/bin/yak-box .
```

## DevContainer Support

yak-box now supports reading `.devcontainer/devcontainer.json` configs when spawning workers.

When spawning a worker, yak-box will automatically:
1. Look for `.devcontainer/devcontainer.json` in the working directory
2. Parse the config and apply supported properties
3. Override the default Docker image if specified
4. Apply environment variables (containerEnv and remoteEnv)
5. Mount additional volumes specified in the mounts array

Supported devcontainer.json properties:
- `image`: Override the default yak-shaver:latest image
- `containerEnv`: Environment variables for the container
- `remoteEnv`: Environment variables with variable substitution support
- `mounts`: Additional Docker volume mounts

Variable substitution patterns supported:
- `${localEnv:VAR}`: Host environment variables
- `${containerEnv:VAR}`: Container environment variables
- `${localWorkspaceFolder}`: Workspace path on host
- `${containerWorkspaceFolder}`: Workspace path in container

Example devcontainer.json:
```json
{
  "image": "mcr.microsoft.com/devcontainers/go:1.21",
  "containerEnv": {
    "PROJECT": "my-project"
  },
  "remoteEnv": {
    "PATH": "${containerEnv:PATH}:/custom/bin"
  },
  "mounts": [
    "source=/tmp,target=/tmp,type=bind"
  ]
}
```