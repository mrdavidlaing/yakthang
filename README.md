# Yakthang — Yak Orchestration System

Yakthang is an autonomous software orchestration system. Think of it as a digital yak ranch — each "yak" is a task that needs shaving (completion), and "yak shavers" are autonomous agents that do the work.

## The Philosophy of Yak Shaving

The term "yak shaving" describes a familiar situation: you have a goal, but along the way you discover a cascade of smaller tasks you must complete first. Want to deploy that feature? First you need tests. Want tests? First you need CI setup. Want CI? First you need infrastructure. Each "yak" that appears on the path to your goal wasn't planned — it just appeared.

Yakthang is designed to manage exactly this kind of work. When you're trying to accomplish something and find yourself shaving yaks to shave yaks, Yakthang helps you track all of them, organize them, and parallelize the work.

### What is a Yakthang?

A play on the Tibetan: ཡིག་ཐང་, yik tang - the high-altitude plateau in Tibet where yaks roam free. In the context of this project its the place where yaks get discovered... organised and shaved.

## The Big Picture

The primary interface is **Zellij** — a terminal multiplexer that provides the orchestration environment. When you run `./launch.sh`, you open **Yakob's Yurt**, which looks like this:

![Yakob's Yurt screenshot](https://private-user-images.githubusercontent.com/227505/550423003-bbefbbea-6b0c-40a9-9f5b-74d7e0b95ff9.png)

### YakMap — Visual Task Map

The left pane runs a **YakMap Zellij plugin** that visualizes your yak map in real-time. It reads from `.yaks/` (maintained by the `yx` CLI) and displays:
- All tasks and their states
- Dependencies between tasks
- Worker assignments

### Yakob — The Orchestrator

The right pane runs **Yakob**, a long-running OpenClaw instance that:
- Plans and organizes work into the yak map
- Spawns yak shavers to work on tasks
- Monitors progress and handles blocked tasks
- Coordinates parallel work

You interact with Yakob directly in this pane — map new yaks, spawn yak shavers, check status, or give guidance.

### Yak Shavers — The Workers

Yak shavers spawn as **additional Zellij tabs**, each running an OpenCode instance with the context of a specific yak. They operate **semi-autonomously**:
- Given a yak's context, they work independently
- For straightforward tasks, they proceed without intervention
- For complex issues, you can focus their tab and provide additional guidance

```
Yakob's 🛖 (main tab)
│
├── YakMap (left) — visual task map from .yaks/
├── Yakob (right top) — orchestration agent  
└── Shell (right bottom) — manual commands

[Tab: api-shasher] — shaver working on auth-api
[Tab: frontend-shasher] — shaver working on frontend-refactor
```

### Optional: Messaging Integration

You can optionally configure Slack/Telegram integration for:
- Receiving notifications when yaks complete
- Remote status queries
- Triggering new yaks from chat

But most interaction happens in Zellij — messaging is optional.

## Key Components

### yx — Task Tracker

A DAG-based TODO list for tracking work:

```
$ yx ls
NAME                      STATUS       PRIORITY
yakthang-v2/add-overview  wip          high
auth-api                  pending      medium
frontend-refactor         blocked      low
```

Each task has:
- **Context** — Detailed requirements and notes
- **Status** — pending, wip, done, blocked
- **Custom fields** — agent-status, priority, depends-on, etc.

### yak-box — Worker Manager

CLI tool for spawning yak shavers:

```bash
# Spawn a sandboxed shaver for API tasks
yak-box spawn --cwd ./api --name api-shasher --yaks auth/api

# Spawn a native shaver with heavy resources
yak-box spawn --cwd ./backend --name backend-shasher --runtime native --resources heavy

# Check status of all shavers
yak-box check
```

Shavers run in two modes:
- **Sandboxed** — Isolated Docker container with resource limits
- **Native** — Direct execution on the host with full system access

### .yaks/ — Task Directory

Stores task state as a directory tree:

```
.yaks/
├── yakthang-v2/
│   ├── add-overview-docs/
│   │   ├── context       # Task requirements
│   │   └── agent-status  # Current worker status
│   └── compile-yx/
├── auth/
│   └── api/
└── frontend/
```

### .yak-boxes/ — Worker Metadata

Runtime directory for shaver instances:

```
.yak-boxes/
├── add-overview-docs.meta   # Shaver configuration
├── api-shasher.meta
└── api-shasher.log          # Execution log
```

## How It Works

### 1. You Work with Yakob in the Main Pane

In the Yakob pane, type your request:

```
Add user authentication to the API
```

### 2. Yakob Creates the Yak

Yakob creates a task in `.yaks/` with context from your request.

### 3. Yakob Spawns Yak Shavers

Yakob calls `yak-box spawn` to create new Zellij tabs with shavers.

### 4. Shavers Work (Semi-Autonomously)

Each shaver runs in its own tab with the yak's context. For simple tasks, they proceed independently. For complex work, you can focus their tab and provide guidance.

### 5. Progress Visible in YakMap

The YakMap pane updates in real-time as shavers update task status.

## Quick Start

### Launch the Orchestrator

```bash
./launch.sh
```

This opens Zellij with YakobsYurt:
- **Left pane**: YakMap — live task visualization
- **Right top**: Yakob (OpenClaw 2E) — orchestration
- **Right bottom**: Shell — manual commands

### Work with Yakob

In the main Yakob pane, ask for work:

```
Yakob: add a login endpoint to the API
Yakob: creating yak: auth-api-login
Yakob: spawning api-shasher...
```

### Monitor

```bash
# From shell pane - check all shavers and tasks
yak-box check

# Or just watch YakMap in the left pane
```

### Interact with a Shaver

When a shaver needs guidance, focus their tab:

```
Can you also add logout? And handle token expiry properly.
```

### Handle Blocked Shavers

```bash
yx state my-feature blocked
yx field my-feature agent-status "blocked: waiting for API spec"
```

## Workflow Examples

### Basic Workflow

```
1. In Yakob pane: "implement user auth"
2. Yakob creates yak, spawns shaver
3. Watch progress in YakMap
4. Shaver completes, YakMap shows done
5. Optionally: focus shaver tab to review work
```

### Parallel Workflow

```
1. "add user, product, and order APIs"
2. Yakob creates three yaks
3. Yakob spawns three shavers in parallel
4. YakMap shows all working simultaneously
5. Each completes independently
```

### Interactive Workflow

```
1. Yakob spawns shaver for complex feature
2. Shaver gets stuck on an edge case
3. You focus their tab
4. Give additional context: "check how other endpoints do validation"
5. Shaver continues with new guidance
```

## Design Philosophy

- **Zellij-first**: Primary interface is the terminal multiplexer
- **Semi-autonomous**: Shavers work independently, but you can intervene
- **Visual**: YakMap provides real-time task visualization
- **Isolated**: Shavers run in containers or separate contexts
- **Observable**: Task state always visible in YakMap

## Directory Structure

```
yakthang/
├── bin/
│   ├── yak-box           # Worker manager CLI
│   └── yak-map.wasm      # YakMap Zellij plugin
├── docs/                 # Documentation
├── orchestrator.kdl      # Zellij layout definition
├── launch.sh             # Entry point
├── .yaks/                # Task state directory
└── .yak-boxes/           # Worker metadata directory
```

## See Also

- [docs/worker-spawning.md](docs/worker-spawning.md) — Worker spawning details
- [docs/orchestrator-layout.md](docs/orchestrator-layout.md) — Terminal layout
