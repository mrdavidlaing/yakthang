# Cost Tracking System Specification

## Overview

The Yakob orchestrator platform has two cost sources: OpenClaw Gateway (orchestrator sessions) and OpenCode workers (Docker containers). Both tools track costs internally, but the data is scattered and often lost. This system extracts, persists, correlates, and reports that data.

## Problem Statement

### The Original Ephemeral Worker Problem

OpenCode workers initially ran in Docker containers with tmpfs storage. When a container stopped, all cost data was lost. An exit hook in `inner.sh` attempted to export costs before container shutdown, but this failed whenever containers crashed or were killed.

### The Solution: Persistent Worker Homes

Workers now have persistent home directories at `.yak-boxes/@home/{Persona}/`. The OpenCode SQLite database survives at `.yak-boxes/@home/{Persona}/.local/share/opencode/opencode.db` regardless of how the container shuts down. Cost data is queried directly from these databases on demand — no export step required.

### Data Sources

- **OpenCode**: SQLite DB at each worker's persistent home (`opencode db` CLI for queries)
- **OpenClaw**: JSONL transcripts with per-message usage/cost

## Architecture

```
┌─────────────────────────────────┐
│  OpenClaw Gateway (systemd)     │  Port 18789
│  ├─ sessions.json               │  Session metadata + token counts
│  ├─ *.jsonl                     │  Transcripts with per-message cost
│  └─ Control UI (:18789)         │  Dashboard
└─────────────────────────────────┘
          │
          │ spawns via yak-box spawn
          ▼
┌─────────────────────────────────┐
│  OpenCode Workers (Docker)      │  Containers with persistent homes
│  └─ opencode.db                 │  SQLite DB (persists via bind mount)
└─────────────────────────────────┘
          │
          │ bind-mounted persistent home
          ▼
┌─────────────────────────────────┐
│  .yak-boxes/@home/{Persona}/    │  Persistent storage per worker
│  └─ .local/share/opencode/      │
│      └─ opencode.db             │  ✅ Survives crashes, kills, restarts
└─────────────────────────────────┘
          │
          │ queried on demand by cost scripts
          ▼
┌─────────────────────────────────┐
│  Cost Reporting Scripts         │
│  ├─ cost-workers.sh             │  Direct DB queries for worker costs
│  ├─ cost-recover.sh             │  Session export recovery (idempotent)
│  ├─ cost-openclaw.sh            │  OpenClaw JSONL cost extraction
│  ├─ cost-summary.sh             │  Unified report (workers + openclaw)
│  └─ .worker-costs/              │  Exported sessions + CSV history
└─────────────────────────────────┘
```

## Components

### 1. cost-workers.sh (Primary Worker Cost Source)

Queries worker costs directly from persistent home SQLite databases. Computes token usage and estimated costs using a built-in pricing table. Works regardless of container state.

**How it works:**
- Iterates over `.yak-boxes/@home/*/`
- Runs `opencode db` queries against each worker's `opencode.db`
- Extracts `tokens.input`, `tokens.output`, `tokens.cache.read`, `tokens.cache.write`, and `modelID` from assistant messages
- Computes costs using per-model pricing rates

**Usage:**
```bash
./cost-workers.sh                  # TSV output (all time)
./cost-workers.sh --summary        # Formatted per-worker summary
./cost-workers.sh --days 1         # Last 24 hours only
./cost-workers.sh --days 7 --summary  # Last week, formatted
```

**TSV output columns:** `persona  model  input_tokens  output_tokens  cache_read  cache_write  cost`

### 2. cost-recover.sh (Session Export Recovery)

Scans all persistent worker homes and exports sessions to `.worker-costs/`. Uses `opencode export` first, falls back to DB queries for token summaries. Idempotent — tracks exported sessions in `.worker-costs/.exported-sessions`.

**Usage:**
```bash
./cost-recover.sh             # Export all unprocessed sessions
./cost-recover.sh --dry-run   # Preview without exporting
```

### 3. cost-openclaw.sh

Parses OpenClaw JSONL transcripts to extract costs by session type.

**Session classification:**
- `agent:main:cron:*` → Cron jobs
- `agent:main:slack:*` → Slack threads
- `agent:main:heartbeat:*` → Heartbeat checks
- Other → Interactive

**Usage:**
```bash
./cost-openclaw.sh --today    # Default
./cost-openclaw.sh --week
./cost-openclaw.sh --month
./cost-openclaw.sh --all
```

### 4. cost-summary.sh

Unified reporting that combines OpenClaw and worker costs. Worker costs are sourced from `cost-workers.sh` (DB-based), not from `.worker-costs/*.json` exports.

**Usage:**
```bash
./cost-summary.sh --today        # Today's costs
./cost-summary.sh --week         # Last 7 days
./cost-summary.sh --month        # Last 30 days
./cost-summary.sh --all          # All time
./cost-summary.sh --today --append-csv  # Add to history
```

**Output format:**
```
═══ Cost Report: 2026-02-14 ═══

OpenClaw (Orchestrator):
  Total:                   $10.81
  Interactive:         $7.69
  Slack:               $3.12

OpenCode (Workers):
  Yakriel:             $2.03
  Yakov:               $4.12

                          Total: $16.96

Models:
  claude-sonnet-4-6:   $16.96 (100.0%)
```

### 5. yak-box check (Live Cost)

Added live cost display for running Docker workers.

**Output:**
```
Live Cost:
  yak-worker-network-1     $0.45
  yak-worker-cost-track    $1.23
```

### 6. CSV History

Daily totals appended to `.worker-costs/daily-totals.csv`:

```csv
date,openclaw_cost,opencode_cost,total_cost,sessions,workers
2026-02-14,10.81,0,10.81,2,0
2026-02-15,8.50,6.20,14.70,3,2
```

### 7. Daily Summary Cron

Updated the daily summary cron job to include cost data:

```
openclaw cron edit <JOB_ID> --message "... Run yx ls, yak-box check, and ./cost-summary.sh --today first."
```

## Design Decisions

### Why Persistent Homes + DB Queries?

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| A: Capture-on-exit | Export before container stops | Simple | Misses crashes/kills |
| B: Persistent homes + DB queries | Query SQLite directly from host | **Crash-proof**, no export needed | Requires persistent home dirs |
| C: Periodic extraction | Cron `docker exec` into workers | Works while alive | Misses final costs, extra overhead |

Option B chosen — persistent worker homes (`.yak-boxes/@home/`) were already implemented for other reasons, making this the natural and most reliable approach. The exit hook (Option A) remains as a secondary path but is no longer the primary cost capture mechanism.

### Why JSONL over WebSocket RPC?

OpenClaw transcripts are plain JSONL files. No auth required, no WebSocket client needed. The RPC endpoints return richer data but require more complexity.

Future enhancement: Add WebSocket RPC for daily breakdown, latency stats.

### Historical Data

All data kept indefinitely. No pruning — storage is cheap, data is valuable.

## Files Reference

| File | Purpose |
|------|---------|
| `cost-workers.sh` | Direct DB queries for worker costs (primary) |
| `cost-recover.sh` | Session export recovery (idempotent) |
| `cost-openclaw.sh` | OpenClaw cost extractor |
| `cost-summary.sh` | Unified cost reporter (workers + openclaw) |
| `bin/yak-box` | Worker spawner (includes exit hook as fallback) |
| `bin/yak-box check` | Worker status + live cost |
| `.yak-boxes/@home/` | Persistent worker homes with SQLite DBs |
| `.worker-costs/` | Exported sessions + CSV history |
| `.worker-costs/daily-totals.csv` | Historical totals |

## Integration Points

### Daily Summary Cron

The 17:00 UTC cron job now runs:
1. `yx ls` — Task status
2. `yak-box check` — Worker status + live costs
3. `./cost-summary.sh --today` — Cost summary
4. Posts combined summary to Slack

### Future Enhancements

- **Budget alerts**: Notify when daily/weekly cost exceeds threshold
- **WebSocket RPC**: Richer OpenClaw data (latency, daily breakdown)
- **Per-task yx fields**: Write cost to task metadata
  ```bash
  echo "$4.12" | yx field network-filtering task-cost
  ```
- **Cost attribution**: Map worker costs to specific tasks via `.yaks/*/assigned-to`

## Pricing Model

Workers use the `github-copilot` provider which bills via **premium requests** (flat subscription + overage at $0.04/request × model multiplier), not per-token. The `cost` field in the OpenCode DB is always `0` for this provider.

`cost-workers.sh` computes a **token-equivalent cost** using direct-API rates from each provider. This gives a comparable spend figure for budgeting. It is not the literal Copilot invoice amount but is directionally accurate.

| Model | Input ($/M) | Output ($/M) | Cache Read | Cache Write |
|-------|-------------|--------------|------------|-------------|
| claude-opus-4-5/4-6 | $15.00 | $75.00 | $1.50 | $18.75 |
| claude-sonnet-4-4/4-5/4-6 | $3.00 | $15.00 | $0.30 | $3.75 |
| claude-haiku-4-4/4-5 | $0.80 | $4.00 | $0.08 | $1.00 |
| gemini-2.5-pro | $1.25 | $10.00 | $0.32 | $0.32 |
| gemini-3-pro/3-pro-preview | $2.00 | $12.00 | $0.20 | $0.20 |
| gemini-3-flash-preview | $0.15 | $0.60 | $0.04 | $0.04 |
| grok-code-fast-1 | $0.20 | $1.50 | $0.02 | $0.02 |
| gpt-4o | $2.50 | $10.00 | $1.25 | $1.25 |
| gpt-5 | $10.00 | $40.00 | $2.50 | $2.50 |
| gpt-5.2 | $5.00 | $20.00 | $1.25 | $1.25 |

Internal models (`providerID: opencode`, e.g. `big-pickle`) are not priced — they report `$0` in cost output.

Model names in the DB use dots (e.g., `claude-sonnet-4.6`); the pricing lookup normalizes dots to dashes for table key matching.
