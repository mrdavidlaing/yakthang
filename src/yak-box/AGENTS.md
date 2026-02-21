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

## Demonstrating Your Work (Optional)

Workers have access to `showboat` — a tool for creating executable demo documents that prove what was built. Output is a plain markdown file (`demo.md`) with embedded commands and their captured outputs.

At the end of your task, create a `demo.md` in your workspace:

```bash
# 1. Initialize a demo document
showboat init demo.md "Task: <task-name>"

# 2. Add narrative context
showboat note demo.md "Built a CLI tool that processes X and outputs Y."

# 3. Execute commands and capture output
showboat exec demo.md bash "your-tool --help"
showboat exec demo.md bash "ls -la src/"
showboat exec demo.md bash "cat output.txt"

# 4. Embed a screenshot (if applicable — e.g. after shot-scraper or playwright)
showboat image demo.md screenshot.png

# 5. Undo a bad entry before it's embedded
showboat pop demo.md
```

**Full command reference:**

| Command | Purpose |
|---------|---------|
| `showboat init <file> <title>` | Create a new demo document with a UUID |
| `showboat note <file> [text]` | Append commentary / narrative |
| `showboat exec <file> <lang> [code]` | Run code, capture stdout+stderr, embed both |
| `showboat image <file> <path>` | Copy image and embed as markdown |
| `showboat pop <file>` | Remove the most recent entry (undo) |
| `showboat verify <file>` | Re-run all code blocks, diff against recorded outputs |
| `showboat extract <file>` | Emit the commands that would recreate the doc |

**Guidelines:**
- Place `demo.md` in your task workspace — the orchestrator reads it from the `.yaks/` workspace mount
- Use `showboat note` for narrative context before each `showboat exec` — explain *why*, not just *what*
- Keep `exec` commands short and read-only where possible (`verify` re-runs them)
- Avoid long-running processes — `exec` blocks until exit (no timeout)
- Use `showboat pop` to discard a failed exec before it's captured
- Skip the demo if your task produces no visible artifact (pure research, blocked task, etc.)

The orchestrator can verify your demo is reproducible with `showboat verify demo.md`.

## RTK — Output Compression

Workers have `rtk` installed — a CLI proxy that compresses verbose command output before it enters your context window, saving 70-99% of tokens on noisy commands.

The `.opencode/plugins/rtk-prefix.ts` plugin auto-prefixes most commands, but as a reinforcement layer, prefer `rtk` for verbose commands:

```
rtk git status      # instead of git status
rtk git log         # instead of git log
rtk git diff        # instead of git diff
rtk ls .            # instead of ls
rtk cargo test      # instead of cargo test
rtk npm test        # instead of npm test
rtk go test ./...   # instead of go test ./...
rtk grep "pattern" .  # instead of grep
```

Check your session's token savings with `rtk gain`.