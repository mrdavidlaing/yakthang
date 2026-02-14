# Cost Tracking System Specification

## Overview

The Yakob orchestrator platform has two cost sources: OpenClaw Gateway (orchestrator sessions) and OpenCode workers (Docker containers). Both tools track costs internally, but the data is scattered and often lost. This system extracts, persists, correlates, and reports that data.

## Problem Statement

### The Ephemeral Worker Problem

OpenCode workers run in Docker containers with their data stored on tmpfs:

```
--tmpfs /home/worker:rw,exec,size=1g
```

When a container stops, all cost data is lost forever. This was the critical gap this system addresses.

### Data Sources Already Exist

Neither OpenClaw nor OpenCode need modification — both already track costs:

- **OpenCode**: `opencode stats`, `opencode export <session>`
- **OpenClaw**: JSONL transcripts with per-message usage/cost

The job is extraction, not implementation.

## Architecture

```
┌─────────────────────────────────┐
│  OpenClaw Gateway (systemd)     │  Port 18789
│  ├─ sessions.json               │  Session metadata + token counts
│  ├─ *.jsonl                     │  Transcripts with per-message cost
│  └─ Control UI (:18789)         │  Dashboard
└─────────────────────────────────┘
          │
          │ spawns via spawn-worker.sh
          ▼
┌─────────────────────────────────┐
│  OpenCode Workers (Docker)      │  Ephemeral containers
│  ├─ opencode.db (tmpfs!)        │  ⚠️ LOST when container stops
│  ├─ opencode stats              │  CLI cost reporting
│  └─ opencode export <session>   │  JSON export with cost
└─────────────────────────────────┘
          │
          │ exit hook → writes to bind-mounted dir
          ▼
┌─────────────────────────────────┐
│  .worker-costs/                 │  Persistent storage
│  ├─ {Worker}-{timestamp}.json   │  Full session exports
│  ├─ {Worker}-{timestamp}.stats  │  Human-readable stats
│  └─ daily-totals.csv            │  Historical data
└─────────────────────────────────┘
```

## Components

### 1. spawn-worker.sh (Exit Hook)

Modified the inner script template to capture cost data before container exit.

**Key changes:**
- Removed `exec` so cleanup runs after opencode exits
- Capture full session export: `opencode export <session>`
- Capture stats: `opencode stats --models`
- Write to `.worker-costs/` (bind-mounted, survives container stop)

**Output files:**
- `{Worker}-{timestamp}.json` — Full session with per-message costs
- `{Worker}-{timestamp}.stats.txt` — Human-readable summary

### 2. cost-openclaw.sh

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

### 3. cost-summary.sh

Unified reporting that combines both sources.

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
  claude-sonnet-4-5:   $16.96 (100.0%)
```

### 4. check-workers.sh (Live Cost)

Added live cost display for running Docker workers.

**Output:**
```
Live Cost:
  yak-worker-network-1     $0.45
  yak-worker-cost-track    $1.23
```

### 5. CSV History

Daily totals appended to `.worker-costs/daily-totals.csv`:

```csv
date,openclaw_cost,opencode_cost,total_cost,sessions,workers
2026-02-14,10.81,0,10.81,2,0
2026-02-15,8.50,6.20,14.70,3,2
```

### 6. Daily Summary Cron

Updated the daily summary cron job to include cost data:

```
openclaw cron edit <JOB_ID> --message "... Run yx ls, check-workers.sh, and ./cost-summary.sh --today first."
```

## Design Decisions

### Why Capture-on-Exit (Option A)?

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| A: Capture-on-exit | Run before container stops | No mount changes, uses built-in tools | Misses if crash |
| B: Bind-mount data | Mount `~/.local/share/opencode/` | Full data preservation | Needs path changes |
| C: Periodic extraction | Cron `docker exec` into workers | Works while alive | Misses final costs |

Option A chosen for simplicity — uses tools already verified working.

### Why JSONL over WebSocket RPC?

OpenClaw transcripts are plain JSONL files. No auth required, no WebSocket client needed. The RPC endpoints return richer data but require more complexity.

Future enhancement: Add WebSocket RPC for daily breakdown, latency stats.

### Historical Data

All data kept indefinitely. No pruning — storage is cheap, data is valuable.

## Files Reference

| File | Purpose |
|------|---------|
| `spawn-worker.sh` | Worker spawner with exit hook |
| `cost-openclaw.sh` | OpenClaw cost extractor |
| `cost-summary.sh` | Unified cost reporter |
| `check-workers.sh` | Worker status + live cost |
| `.worker-costs/` | Persistent cost data storage |
| `.worker-costs/daily-totals.csv` | Historical totals |

## Integration Points

### Daily Summary Cron

The 17:00 UTC cron job now runs:
1. `yx ls` — Task status
2. `check-workers.sh` — Worker status + live costs
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

The system uses approximate pricing from OpenClaw transcripts when cost isn't explicitly recorded:

| Model | Input ($/M) | Output ($/M) | Cache Read | Cache Write |
|-------|-------------|--------------|------------|-------------|
| claude-opus-4-5 | $15.00 | $75.00 | $0.60 | $0.30 |
| claude-sonnet-4-5 | $3.00 | $15.00 | $0.30 | $0.15 |
| claude-haiku-4-5 | $0.80 | $4.00 | $0.10 | $0.05 |

Note: These are estimates. Actual costs may vary based on provider pricing.
