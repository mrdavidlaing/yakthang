# Agent Instructions for yak-box

## Building

```bash
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o yak-box .
```

For development builds without version embedding:

```bash
go build -o yak-box .
```

## Testing

```bash
cd src/yak-box && shellspec
```

## DevContainer Support

yak-box uses a unified `.devcontainer/Dockerfile` pattern for all worker images.

**How it works:**
1. The project's `.devcontainer/Dockerfile` is the default worker image (`yak-worker:latest`)
2. External projects can provide their own `.devcontainer/Dockerfile` to customize the image
3. Projects without `.devcontainer/` fall back to the default image
4. The `devcontainer.json` config can override the image, env vars, and mounts

When spawning a worker, yak-box will automatically:
1. Build the `yak-worker:latest` image from the project's `.devcontainer/Dockerfile` if needed
2. Look for `.devcontainer/devcontainer.json` in the working directory
3. Parse the config and apply supported properties
4. Override the default Docker image if specified
5. Apply environment variables (containerEnv and remoteEnv)
6. Mount additional volumes specified in the mounts array

Supported devcontainer.json properties:
- `image`: Override the default yak-worker:latest image
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
