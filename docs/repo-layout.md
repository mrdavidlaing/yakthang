# Repository Layout Spec

## Overview

yakthang is both a development environment for its own tooling and a reusable orchestration framework. The layout separates specs, source, compiled output, and scratch space.

## Directory Structure

```
yakthang/
├── docs/           # Specs, design docs, intent documentation
├── src/
│   ├── yaks/       # yx CLI - task tracker (Rust)
│   ├── yakmap/     # Zellij plugin (Rust/WASM)
│   └── yakbox/     # Docker orchestration tool (Go)
├── bin/            # Compiled binaries from src/
├── tmp/            # Ephemeral scratch space (gitignored)
├── .yaks/          # Task state (managed by yx)
└── .openclaw/      # OpenClaw workspace config
```

## Principles

1. **`docs/`** contains all specs describing intent and design. Not READMEs scattered across source dirs -- centralized here.

2. **`src/`** groups all tool source code. Each sub-directory is a self-contained project with its own build system.

3. **`bin/`** holds compiled output. When using yakthang for a non-development purpose, you need `bin/` + `docs/` + `.yaks/` -- no source required.

4. **`tmp/`** is ephemeral, gitignored. Workers and builds use it for intermediate artifacts.

## Container Mounts

Docker worker containers mount the entire `yakthang/` directory. Workers see everything -- docs, source, bins, task state. Scoping is done via task context (`yx`), not filesystem isolation. Git is the safety net against workers wandering outside their lane.

## Migration Notes

Current top-level scripts (`spawn-worker.sh`, `check-workers.sh`, `shutdown-worker.sh`, etc.) and config files (`orchestrator.kdl`, `worker.Dockerfile`, etc.) need to find appropriate homes:

- Orchestration scripts → `src/yakbox/` or a dedicated `scripts/` dir
- `worker.Dockerfile` → `src/yakbox/`
- `orchestrator.kdl` → root (Zellij layout) or `docs/`
- `themes/` → TBD

These decisions can be made during implementation.
