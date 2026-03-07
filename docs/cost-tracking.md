# Cost Tracking System Specification

## Overview

The Yakob orchestrator platform tracks costs from worker sessions (Claude Code agents running as Docker containers or native processes). This system extracts, persists, and reports that data.

## Problem Statement

### The Ephemeral Worker Problem

Workers run in Docker containers with their data stored on tmpfs:

```
--tmpfs /home/worker:rw,exec,size=1g
```

When a container stops, all cost data is lost forever. This was the critical gap this system addresses.

## Architecture

```
          │ spawns via yak-box spawn
          ▼
┌─────────────────────────────────┐
│  Claude Code Workers (Docker)   │  Ephemeral containers
│  ├─ cost data (tmpfs!)          │  ⚠️ LOST when container stops
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

### 1. yak-box spawn (Exit Hook)

Modified the inner script template to capture cost data before container exit.

**Key changes:**
- Removed `exec` so cleanup runs after the agent exits
- Write to `.worker-costs/` (bind-mounted, survives container stop)

**Output files:**
- `{Worker}-{timestamp}.json` — Full session with per-message costs
- `{Worker}-{timestamp}.stats.txt` — Human-readable summary

### 2. cost-summary.sh

Worker cost reporting.

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

Workers:
  Yakriel:             $2.03
  Yakov:               $4.12

                          Total: $6.15
```

### 3. yak-box check (Live Cost)

Added live cost display for running Docker workers.

**Output:**
```
Live Cost:
  yak-worker-network-1     $0.45
  yak-worker-cost-track    $1.23
```

### 4. CSV History

Daily totals appended to `.worker-costs/daily-totals.csv`:

```csv
date,worker_cost,total_cost,workers
2026-02-15,6.20,6.20,2
```

## Design Decisions

### Why Capture-on-Exit?

| Option | Approach | Pros | Cons |
|--------|----------|------|------|
| A: Capture-on-exit | Run before container stops | No mount changes, uses built-in tools | Misses if crash |
| B: Bind-mount data | Mount full data directory | Full data preservation | Needs path changes |
| C: Periodic extraction | Cron `docker exec` into workers | Works while alive | Misses final costs |

Option A chosen for simplicity — uses tools already verified working.

### Historical Data

All data kept indefinitely. No pruning — storage is cheap, data is valuable.

## Files Reference

| File | Purpose |
|------|---------|
| `bin/yak-box` | Worker spawner with exit hook |
| `cost-summary.sh` | Worker cost reporter |
| `bin/yak-box check` | Worker status + live cost |
| `.worker-costs/` | Persistent cost data storage |
| `.worker-costs/daily-totals.csv` | Historical totals |

## Integration Points

### Daily Summary

The daily summary runs:
1. `yx ls` — Task status
2. `yak-box check` — Worker status + live costs
3. `./cost-summary.sh --today` — Cost summary

### Future Enhancements

- **Budget alerts**: Notify when daily/weekly cost exceeds threshold
- **Per-task yx fields**: Write cost to task metadata
  ```bash
  echo "$4.12" | yx field network-filtering task-cost
  ```
- **Cost attribution**: Map worker costs to specific tasks via `.yaks/*/assigned-to`
