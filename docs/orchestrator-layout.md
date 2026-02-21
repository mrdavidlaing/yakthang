# Orchestrator Layout

## Overview

Yakthang uses Zellij as its terminal multiplexer. A single KDL layout file
(`orchestrator.kdl`) defines the orchestrator tab, and workers are spawned
dynamically as additional tabs.

## Orchestrator Tab

The orchestrator tab ("Yakob's 🛖") is a vertical split with three panes:

```
┌──────────────┬──────────────────────────────┐
│              │                              │
│   yak-map    │     orchestrator (openclaw)   │
│   (33%)      │          (80% of 67%)        │
│              │                              │
│  live task   │  Yakob plans, spawns, and    │
│  tree via    │  monitors from here          │
│  `yx ls`     │                              │
│              ├──────────────────────────────┤
│              │  shell (orchestrator) (20%)  │
│              │  manual commands             │
└──────────────┴──────────────────────────────┘
```

### Yak-map pane (left, 33%)

Runs the **YakMap Zellij WASM plugin** (`bin/yak-map.wasm`), providing a
live view of all tasks and their states. The plugin reads `.yaks/` directly
(no dependency on the `yx` binary) and auto-refreshes every 2 seconds. It
supports keyboard navigation (↑/↓) and color-coded task status. This is
the orchestrator's primary situational awareness tool.

### Orchestrator pane (right top, ~80% of 67%)

Runs an interactive `openclaw` instance. This is where Yakob (the orchestrator
persona) operates. The orchestrator reads task state, writes task context,
and spawns workers — but never edits application code directly.

### Shell pane (right bottom, ~20% of 67%)

A plain shell for manual commands: running `yak-box check`, reading fields,
git operations, or anything the orchestrator needs outside of openclaw.

## Worker Tabs

Worker tabs are created dynamically by `yak-box spawn` (see
[worker-spawning.md](worker-spawning.md)). Each worker tab has:

```
┌──────────────────────────────────────────────┐
│  compact-bar                                 │
├──────────────────────────────────────────────┤
│                                              │
│  opencode (plan|build)  (67%)                │
│  worker agent instance                       │
│                                              │
├──────────────────────────────────────────────┤
│  shell: /path/to/cwd    (33%)                │
│  worker's local shell                        │
├──────────────────────────────────────────────┤
│  status-bar                                  │
└──────────────────────────────────────────────┘
```

The tab name shows the worker's randomly assigned identity (e.g.
"Yakueline 🦬💈").

## Launching

```bash
./launch.sh
```

This runs `zellij --layout orchestrator.kdl`, which creates the orchestrator
tab. Everything else is spawned from there.

## Key Design Decisions

- **Zellij over tmux**: Zellij's KDL layouts and `new-tab --layout` command
  make it possible to define worker tab structure declaratively.
- **YakMap WASM plugin**: The left pane runs a compiled Rust plugin that reads
  `.yaks/` directly, providing richer visualization (tree rendering, color
  coding, keyboard navigation) than the previous `watch` + `yx ls` approach.
- **Opencode for both orchestrator and workers**: The orchestrator runs
  interactive openclaw; workers run opencode via `yak-box spawn`.
- **Focus returns to orchestrator**: After spawning a worker tab,
  `yak-box spawn` calls `zellij action go-to-previous-tab` so the
  orchestrator doesn't lose its place.
