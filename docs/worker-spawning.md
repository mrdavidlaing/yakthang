# Worker Spawning

## Overview

Workers are disposable opencode instances that execute tasks in isolated
Zellij tabs. The orchestrator spawns them via `spawn-worker.sh`, which
handles tab creation, prompt injection, and identity assignment.

## Usage

```bash
./spawn-worker.sh --cwd <dir> --name <tab-name> [--mode plan|build] "<prompt>"
```

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `--cwd` | Yes | — | Working directory for the worker |
| `--name` | Yes | — | Logical name (used in logs, not tab title) |
| `--mode` | No | `build` | Agent mode: `plan` or `build` |
| `--yak-path` | No | `$PWD/.yaks` | Path to shared task state |

## What Happens When You Spawn

1. **Identity assignment** — A random yak-shaver personality is picked from
   the roster (Yakriel, Yakueline, Yakov, Yakira). Each has a distinct emoji
   and personality trait. The Zellij tab title shows the identity.

2. **Prompt assembly** — The worker's prompt is built from three parts:
   - Personality preamble (loaded from `.opencode/personalities/<name>-worker.md`)
   - Role description (plan vs build — see below)
   - User-provided prompt (the actual task instructions)
   - yx task tracker instructions (how to use `yx ls`, report status, etc.)

3. **Temp file setup** — The assembled prompt is written to a temp file.
   A wrapper script (`run.sh`) is generated that reads the prompt and
   execs opencode.

4. **Zellij tab creation** — A temporary KDL layout is generated defining
   the worker tab (opencode pane + shell pane). The tab is created via
   `zellij action new-tab --layout`.

5. **Focus restoration** — After a 0.3s pause (to let the tab initialize),
   focus returns to the previous tab via `zellij action go-to-previous-tab`.

## Runtime Modes

### Zellij Runtime

The wrapper script execs `opencode` directly. The opencode process runs
natively on the host in the Zellij command pane.

### Docker Runtime

The wrapper script execs `docker run -it` which launches a container with
the opencode TUI. The container is hardened with a read-only filesystem,
dropped capabilities, non-root user, and resource limits. The Zellij command
pane provides the terminal that Docker bridges into the container.

Key differences from Zellij mode:
- opencode runs inside a container, not directly on the host
- Auth is via `ANTHROPIC_API_KEY` env var (not host's auth.json)
- Container has its own ephemeral HOME directory (tmpfs)
- Workspace and .yaks are bind-mounted into the container

See [Docker Mode](development/DOCKER-MODE.md) for architecture details.

## Yak-Shaver Personalities

| Name | Emoji | Trait |
|------|-------|-------|
| Yakriel | 🦬🪒 | Precise and methodical. Measures twice, shaves once. |
| Yakueline | 🦬💈 | Fast and fearless. Ships first, asks forgiveness later. |
| Yakov | 🦬🔔 | Cautious and thorough. Better safe than shorn. |
| Yakira | 🦬🧶 | Cheerful and communicative. Leaves detailed status updates. |

Personalities are randomly assigned. They give each worker tab a distinct
identity and influence the agent's working style through the system prompt.

## Prompt Injection Design

The critical design choice: **sub-repos have no knowledge of the orchestration
layer**. Workers receive everything they need to know about `yx` inline in
their system prompt. This means:

- No CLAUDE.md or `.opencode/` config needed in sub-repos (the workspace root
  has `.opencode/` for the orchestrator and personality templates)
- Sub-repos stay completely clean
- Workers are fully disposable — spawn a fresh one anytime
- The orchestrator controls exactly what instructions each worker gets

## Worker Scoping Rules

- **One worker per sub-repo** — Avoid two workers editing the same codebase
- **Scope to a task subtree** — Each worker's prompt should specify which
  `yx` tasks they own (e.g. "Work on tasks under auth/api/*")
- **Use `--cwd` to isolate** — Workers operate in their assigned directory

## Examples

```bash
# Simple build worker in a sub-repo
./spawn-worker.sh --cwd ./api --name "api-auth" \
  "Work on auth/api/* tasks."

# Plan worker for complex task
./spawn-worker.sh --mode plan --cwd ./api --name "api-planner" \
  "Plan the auth refactor. Analyze the codebase and write a plan."

# Worker with custom yak-path (if .yaks is elsewhere)
./spawn-worker.sh --cwd ./api --name "api-auth" --yak-path /path/to/.yaks \
  "Work on auth/api/* tasks."
```
