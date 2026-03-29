# Yakthang — Yak Orchestration System

> **If your name isn't David Laing, you probably don't want to use this repository directly.** This is a personal workspace for exploring multi-agent orchestration patterns. Rather than cloning it, I recommend pointing your agent at this repo and exploring it for ideas that might work for your own agentic workflows.

## What is this?

Yakthang is an experiment in **supervised multi-agent software development**. It explores the question: *what happens when you give a human supervisor a team of AI agents, a shared task board, and a terminal multiplexer?*

The name is a play on the Tibetan ཡིག་ཐང་ (*yik tang*) — the high-altitude plateau where yaks roam free. Here, "yaks" are tasks that need shaving, and "shavers" are autonomous agents that do the work.

## Ideas that might be worth stealing

### 1. The Orchestrator pattern

A long-running AI agent (**Yakob**) acts as a supervisor — planning work, spawning workers, monitoring progress, and reacting to blockers. The human (mostly) talks to the orchestrator, not the workers directly.

This creates a natural division of labour: the human decides *what* and *when*, the orchestrator decides *how* and *who*, and the workers just execute.

### 2. Shared task state as coordination mechanism

All agents (orchestrator and workers) share a DAG-based task board (`.yaks/`, managed by the `yx` CLI). Workers read their briefs from it, write progress back to it, and the orchestrator watches it for state changes.

This means agents don't need to talk to each other directly — the task board is the communication channel. Workers are stateless and disposable; the task board is the source of truth.

### 3. Session discipline

Every work session starts with **triage** (hard stop time, WIP limit, scope) and ends with **wrap** (harvest done work, write a worklog, prune the map). This pre-commitment ritual prevents the common failure mode of agentic work: unbounded sessions with no stopping cues.

It's proving an effective pattern for the human in this loop to stay in control of the number of open loops, and thus their ability to stop working.

### 4. Defence in depth for agent isolation (partially implemented)

Workers run inside layered isolation:
- **Filesystem sandboxing** via [sandbox-runtime](https://github.com/anthropic-experimental/sandbox-runtime) (srt) — OS-level write restrictions and network allowlists
- **Devcontainer isolation** via Docker — full container boundaries for heavier isolation
- **Native mode** — no isolation, for research tasks that need unrestricted access

The orchestrator chooses the isolation level per-task based on risk.

### 5. Adversarial review as a quality gate

When a worker reports done, the orchestrator spawns a **fresh, independent reviewer** with no knowledge of the worker's reasoning. The reviewer only sees the original brief, the done summary, and the actual code. This prevents anchoring bias and catches gaps that the implementer (or the orchestrator watching them) would miss.

### 6. Tools with personality, tailored for an audience of one

David likes whimsy, so this workflow unashamedly leans into the yak shaving metaphor. The result is a system where every component has a name that makes you smile — and that matters, because you spend all day looking at it.

- **Yakob** — the orchestrator. A calm, methodical supervisor of yak shavers. He makes dry yak-related puns, sparingly.
- **Shavers** — the workers. Each gets a yak-themed name from a pool: Yakira, Yakoff, Yakriel, Yakueline, Yaklyn, Yakon, Yakitty, and Bob (every group has a Bob).
- **yak-map** — the visual task tree. Yaks graze in the pasture; shavers shave them.
- **yak-box** — the worker launcher. It puts shavers in boxes (sandboxed or not).
- **yak-branding** — the commit convention. Every commit is stamped with the yak-brand skill.
- **sniff test** — the adversarial review. Does this yak smell right?
- **yak triage** — sorting the herd at the gate. Session start ritual.
- **yak wrap** — closing the barn. Session end ritual.

The point isn't the puns. It's that when you build tools for yourself, you can make them *yours*. A tool you enjoy using is a tool you actually use.

## Architecture (for the curious)

```
Zellij (terminal multiplexer)
├── Orchestrator tab
│   ├── yak-map plugin (left) — real-time task visualization
│   ├── Yakob / Claude (right) — orchestrator agent
│   └── Shell (bottom) — manual commands
│
├── Worker tab 1 — Claude in sandbox, shaving a yak
├── Worker tab 2 — Claude in sandbox, shaving another yak
└── ...
```

**Key tools:**
- `yx` — DAG-based task CLI (Rust). Stores state in git notes.
- `yak-box` — Worker lifecycle manager (Go). Spawns/stops workers with isolation.
- `yak-map` — Zellij WASM plugin (Rust). Reads `.yaks/` and renders the tree.
- `srt` — Sandbox runtime (TypeScript/Bun). OS-level filesystem and network sandboxing.

## Exploring this repo

Point your agent here and ask it questions like:
- "How does the orchestrator decide when to spawn workers?"
- "How does the adversarial review process work?"
- "How are workers isolated from each other?"
- "How does session discipline (triage/wrap) work?"
- "How does the task board coordinate agents without direct communication?"

Good starting points:
- `agents/yakob.md` — The orchestrator's full instruction set
- `skills/` — Skill definitions (triage, wrap, review, etc.)
- `src/yak-box/` — Worker spawning and isolation
- `src/yak-map/` — Real-time task visualization plugin
