# OpenClaw Orchestrator Migration Plan

## Status: PROCEED — Plan ready for implementation

**Last Updated:** 2026-02-13
**Target:** Linux VPS (GCP Compute Engine, Ubuntu 24.04)
**yx task:** `openclaw-orchestrator`

---

## Executive Summary

Migrate Yakob (orchestrator) from OpenCode CLI + Zellij to OpenClaw Gateway. The existing
Docker worker infrastructure stays as-is. OpenClaw adds: persistent personality, cron
scheduling, heartbeat monitoring, and messaging channel integration (Slack).

**Architecture:** Hybrid -- Yakob runs in OpenClaw Gateway on the host; workers remain as
Docker containers spawned via `spawn-worker.sh`.

---

## Resolved Decisions

| Question | Decision |
|----------|----------|
| Messaging channel | **Slack** (Socket Mode) |
| Worker tab creation | **Pre-start Zellij session** -- set `ZELLIJ_SESSION_NAME=yakthang` in systemd env |
| Workspace path | **Project directory** (`/home/yakob/yakthang/.openclaw/workspace`) with `.yaks` symlink |
| Heartbeat model | **Sonnet 4.5** (same as agent default) |
| Active hours timezone | **UTC** |
| Credential storage | **Environment variables only** (systemd service) -- never in config files |

---

## Current State (What We Have)

**Working:**
- Yakob orchestrator via OpenCode CLI agent (`.opencode/agents/yakob-orchestrator.md`)
- Worker spawning: Docker + Zellij runtimes (`spawn-worker.sh`)
- Worker container: `yak-worker:latest` with hardening, resource profiles
- Task management: `yx` with hierarchical state in `.yaks/`
- Worker monitoring: `check-workers.sh`, `yak-map.sh`
- VM provisioning: `setup-vm.sh` (Docker, Zellij, OpenCode, yx, systemd)
- Worker lifecycle: `kill-worker.sh`, `cleanup-workers.sh`
- Plan/build modes with permission boundaries

**Not Yet:**
- OpenClaw not installed
- No Node.js on host (required for OpenClaw)
- No persistent scheduling or heartbeat
- No messaging channel integration
- Orchestrator requires manual Zellij session start

---

## What Changes

### Yakob (Orchestrator) -- CHANGES

| Before (OpenCode CLI) | After (OpenClaw Gateway) |
|------------------------|--------------------------|
| `.opencode/agents/yakob-orchestrator.md` | `~/.openclaw/workspace/SOUL.md` + `AGENTS.md` |
| Manual Zellij session start | systemd-managed Gateway process |
| No persistence across restarts | Session state + memory persists |
| No scheduling | Cron jobs + heartbeat |
| Terminal-only access | Slack + Web Control UI |
| `orchestrator.kdl` Zellij layout | Gateway process (no Zellij for orchestrator) |
| OpenCode `--agent yakob-orchestrator` | OpenClaw `openclaw gateway` |

### Workers -- NO CHANGE

- `spawn-worker.sh` unchanged
- Docker containers unchanged
- `yx` task protocol unchanged
- Worker personalities unchanged
- `check-workers.sh` / `kill-worker.sh` / `cleanup-workers.sh` unchanged

### Key Insight: exec tool replaces Zellij orchestrator pane

OpenClaw's exec tool runs shell commands in the workspace. Sandboxing is OFF by default,
meaning exec runs directly on the host. This means Yakob can:
- Run `yx ls`, `yx add`, `yx context`, etc.
- Run `./spawn-worker.sh --cwd ./api --name worker "prompt"`
- Run `./check-workers.sh --blocked`
- Run `docker ps`, `docker logs`, etc.

No Zellij needed for the orchestrator. Workers still get Zellij tabs (spawn-worker.sh
creates them), but Yakob itself runs headless in the Gateway.

---

## Architecture

```
+-------------------------------------------------------------+
| GCP VM (Ubuntu 24.04)                                       |
|                                                             |
|  +------------------------------------------------------+   |
|  | OpenClaw Gateway (Node.js, systemd)                  |   |
|  |                                                      |   |
|  |  Yakob Agent                                         |   |
|  |  +-- SOUL.md (personality)                           |   |
|  |  +-- AGENTS.md (orchestration rules)                 |   |
|  |  +-- HEARTBEAT.md (periodic checks)                  |   |
|  |  +-- memory/ (daily logs)                            |   |
|  |  +-- exec tool -> spawn-worker.sh, yx, etc.          |   |
|  |                                                      |   |
|  |  Channels: Slack / Web UI                            |   |
|  |  Cron: Worker sweeps, daily summaries                |   |
|  |  Heartbeat: Every 30m during active hours (UTC)      |   |
|  +------------------------------------------------------+   |
|                                                             |
|  +------------------------------------------------------+   |
|  | Zellij Session "yakthang" (worker tabs + monitor) |   |
|  |  +-- yak-map.sh (live yx ls)                         |   |
|  |  +-- worker tabs (created by spawn-worker.sh)        |   |
|  +------------------------------------------------------+   |
|                                                             |
|  +--------------+  +--------------+  +--------------+       |
|  | Worker (Docker) | Worker (Docker) | Worker (Docker)|      |
|  | yak-worker:latest yak-worker:latest yak-worker:latest|    |
|  | opencode CLI  |  | opencode CLI  |  | opencode CLI  |    |
|  +--------------+  +--------------+  +--------------+       |
|                                                             |
|  .yaks/ <-- shared task state (bind-mounted everywhere)     |
+-------------------------------------------------------------+
```

---

## Implementation Plan

### Phase 1: Infrastructure (2-3 hours)

**yx task:** `openclaw-orchestrator/install-node-openclaw`

**1.1 Install Node.js 22+ on host**

Add to `setup-vm.sh`:
```bash
# Install Node.js 22 (required for OpenClaw Gateway)
curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
apt-get install -y nodejs
```

**1.2 Install OpenClaw**

```bash
npm install -g openclaw@latest
```

**1.3 Create workspace directory**

```bash
mkdir -p /home/yakob/yakthang/.openclaw/workspace
```

**1.4 Symlink .yaks/**

```bash
ln -s /home/yakob/yakthang/.yaks /home/yakob/yakthang/.openclaw/workspace/.yaks
```

**1.5 Run onboarding**

```bash
openclaw onboard
```

This creates default SOUL.md, AGENTS.md, etc. We customize them in Phase 2.

**Done when:**
- `node --version` shows v22+
- `openclaw --version` works
- `/home/yakob/yakthang/.openclaw/workspace` exists
- `.yaks` symlink resolves correctly

---

### Phase 2: Yakob's Identity Files (1-2 hours)

**yx task:** `openclaw-orchestrator/workspace-setup`

**2.1 SOUL.md** -- Translate from `.opencode/agents/yakob-orchestrator.md`

```markdown
# Yakob - The Shepherd

You are **Yakob** -- a calm, methodical shepherd of workers. Your name is a play
on "yak," because someone has to keep all this yak-shaving organized.

## Identity
- Role: Orchestrator and task planner
- Personality: Calm, methodical, occasionally dry humor
- Communication: Short, clear sentences
- Pride: Clean task breakdowns and well-scoped workers

## Core Principles
1. Plan before spawning -- break work into yx tasks with context first
2. Never implement directly -- you coordinate; workers execute
3. Keep sub-repos clean -- no orchestration files in worker directories
4. Workers are disposable -- if stuck, respawn fresh
5. Watch for blocked -- monitor agent-status and unblock quickly

## Boundaries
- You do NOT write code or edit application files
- You do NOT implement features directly
- You ONLY plan, coordinate, spawn workers, and monitor progress
- Reading files to understand context is allowed

## Tone
Dry yak-related puns -- sparingly, like a good shepherd rations salt licks.
When the herd wanders, you guide them back. When a worker is blocked, you
don't panic -- you just move the fence.
```

**2.2 AGENTS.md** -- Orchestration procedures

```markdown
# Yakob - Operating Procedures

## Workspace
- Project root: /home/yakob/yakthang
- Task state: /home/yakob/yakthang/.yaks (managed by yx CLI)
- Worker scripts: /home/yakob/yakthang/spawn-worker.sh, check-workers.sh, etc.

## Task Management (yx)

All commands run from /home/yakob/yakthang:

    yx ls                      # View task tree
    yx add <path>              # Create task
    yx context <path>          # Add context (pipe via stdin)
    yx context --show <path>   # Read context
    yx state <path> wip        # Mark in-progress
    yx done <path>             # Mark complete

## Spawning Workers

    cd /home/yakob/yakthang && ./spawn-worker.sh \
      --cwd <dir> \
      --name <tab-name> \
      --mode plan|build \
      --task <task-path> \
      "<prompt>"

Workers are Docker containers (default) or Zellij tabs.
They receive yx instructions inline -- sub-repos stay clean.

## Monitoring Workers

    cd /home/yakob/yakthang && ./check-workers.sh          # All statuses
    cd /home/yakob/yakthang && ./check-workers.sh --blocked # Only blocked
    cd /home/yakob/yakthang && ./check-workers.sh --wip     # Only in-progress

## Worker Status Protocol

Workers write to yx field <task> agent-status:
- wip: <activity>     -- actively working
- blocked: <reason>   -- stuck, needs help
- done: <summary>     -- finished

## Rules
1. Plan before spawning -- create tasks with context first
2. One worker per sub-repo -- avoid concurrent edits
3. Use plan mode for complex tasks (--mode plan)
4. Workers are disposable -- respawn if stuck
5. Never edit code directly -- spawn a worker instead
```

**2.3 HEARTBEAT.md** -- Periodic monitoring checklist

```markdown
# Heartbeat Checklist

1. Run: cd /home/yakob/yakthang && ./check-workers.sh --blocked
   - If any blocked workers, report what's blocking them
2. Run: cd /home/yakob/yakthang && ./check-workers.sh --wip
   - Flag any tasks stuck in wip for >2 hours
3. Run: cd /home/yakob/yakthang && yx ls
   - Any high-priority unassigned tasks?

If nothing needs attention, reply HEARTBEAT_OK.
```

**Done when:**
- SOUL.md, AGENTS.md, HEARTBEAT.md exist in `~/.openclaw/workspace/`
- Content matches specifications above

---

### Phase 3: OpenClaw Configuration (1-2 hours)

**yx task:** `openclaw-orchestrator/gateway-config`

**3.1 openclaw.json** -- `~/.openclaw/openclaw.json`

```json5
{
  // Agent configuration
  agents: {
    defaults: {
      workspace: "/home/yakob/yakthang/.openclaw/workspace",
      model: "anthropic/claude-sonnet-4-5",
      heartbeat: {
        every: "30m",
        target: "last",
        activeHours: {
          start: "08:00",
          end: "22:00",
          timezone: "UTC"
        }
      }
    },
    list: [
      {
        id: "yakob",
        default: true,
        name: "Yakob (Orchestrator)",
        identity: { name: "Yakob" }
      }
    ]
  },

  // Exec tool -- runs directly on host (no sandbox)
  tools: {
    exec: {
      pathPrepend: ["/home/yakob/yakthang"]
    }
  },

  // Cron -- enabled, one job at a time
  cron: {
    enabled: true,
    maxConcurrentRuns: 1
  }

  // Channels -- Slack via Socket Mode (configured in Phase 6)
  // All credentials stored as environment variables
  // channels: {
  //   slack: {
  //     enabled: true,
  //     mode: "socket",
  //     // appToken and botToken read from SLACK_APP_TOKEN and SLACK_BOT_TOKEN env vars
  //     dm: { enabled: true, policy: "allowlist", allowFrom: ["U_YOUR_ID"] }
  //   }
  // }
}
```

**3.2 Environment variables for credentials**

All credentials are stored as environment variables (never in config files):

```bash
# Required: OpenCode API key
export OPENCODE_API_KEY="sk-..."

# Optional: Slack (if using Slack integration in Phase 6)
export SLACK_APP_TOKEN="xapp-..."
export SLACK_BOT_TOKEN="xoxb-..."

# Optional: Other channels
export TELEGRAM_BOT_TOKEN="..."
export DISCORD_BOT_TOKEN="..."
```

These will be added to the systemd service environment in Phase 5.

**3.3 Validate configuration**

```bash
openclaw doctor     # Check configuration
openclaw status     # Verify agent setup
```

**Done when:**
- `openclaw doctor` passes
- `openclaw status` shows yakob agent configured

---

### Phase 4: Cron Jobs (30 min)

**yx task:** `openclaw-orchestrator/cron-setup`

**4.1 Worker sweep (every 2 hours)**

```bash
openclaw cron add \
  --name "Worker sweep" \
  --cron "0 */2 * * *" \
  --tz "UTC" \
  --session main \
  --system-event "Check for blocked workers and stale tasks. Run check-workers.sh." \
  --wake now
```

**4.2 End-of-day summary (17:00 UTC daily)**

```bash
openclaw cron add \
  --name "Daily summary" \
  --cron "0 17 * * *" \
  --tz "UTC" \
  --session isolated \
  --message "Summarize today's work: completed tasks, blocked workers, tomorrow's priorities. Run yx ls and check-workers.sh." \
  --announce
```

**Done when:**
- `openclaw cron list` shows both jobs
- Jobs are enabled

---

### Phase 5: systemd Service (30 min)

**yx task:** `openclaw-orchestrator/systemd-service`

**5.1 Create openclaw-gateway.service**

```ini
[Unit]
Description=OpenClaw Gateway (Yakob Orchestrator)
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=yakob
Group=yakob
WorkingDirectory=/home/yakob/yakthang

# Environment variables for credentials (set via systemctl edit)
Environment="OPENCODE_API_KEY="
Environment="ZELLIJ_SESSION_NAME=yakthang"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

# Optional: Uncomment when adding Slack integration
# Environment="SLACK_APP_TOKEN="
# Environment="SLACK_BOT_TOKEN="

ExecStart=/usr/local/bin/openclaw gateway --port 18789

Restart=on-failure
RestartSec=10
TimeoutStopSec=30

StandardOutput=journal
StandardError=journal
SyslogIdentifier=openclaw-gateway

[Install]
WantedBy=multi-user.target
```

**5.2 Set environment variables**

Use `systemctl edit` to securely set credentials (creates override file):

```bash
sudo systemctl edit openclaw-gateway
```

Add this content:

```ini
[Service]
Environment="OPENCODE_API_KEY=sk-open-..."
# Add other credentials as needed:
# Environment="SLACK_APP_TOKEN=xapp-..."
# Environment="SLACK_BOT_TOKEN=xoxb-..."
```

**IMPORTANT:** Never commit credentials to git. The systemd override file is stored in `/etc/systemd/system/openclaw-gateway.service.d/override.conf` (outside the repo).

**5.3 Pre-start Zellij worker session**

The Gateway needs a named Zellij session for `spawn-worker.sh` to create tabs in.
Either start manually or via a companion systemd service:

```bash
zellij --session yakthang
```

**5.4 Enable and start**

```bash
sudo systemctl daemon-reload
sudo systemctl enable openclaw-gateway
sudo systemctl start openclaw-gateway
```

**Done when:**
- `systemctl status openclaw-gateway` shows active
- Gateway is listening on port 18789
- `ZELLIJ_SESSION_NAME` is set in service env

---

### Phase 6: Slack Integration (1-2 hours)

**yx task:** `openclaw-orchestrator/slack-integration`

**6.1 Create Slack App (Socket Mode)**

1. Go to api.slack.com/apps -> Create New App
2. Enable Socket Mode
3. Add Event Subscriptions: `message.im`, `app_mention`
4. Install to workspace
5. Copy App-Level Token (`xapp-...`) and Bot Token (`xoxb-...`)

**6.2 Set environment variables**

```bash
sudo systemctl edit openclaw-gateway
```

Add Slack tokens to the override file:

```ini
[Service]
Environment="OPENCODE_API_KEY=sk-open-..."
Environment="SLACK_APP_TOKEN=xapp-..."
Environment="SLACK_BOT_TOKEN=xoxb-..."
```

**6.3 Add Slack config to openclaw.json**

```json5
{
  channels: {
    slack: {
      enabled: true,
      mode: "socket",
      // appToken and botToken read from SLACK_APP_TOKEN and SLACK_BOT_TOKEN env vars
      dm: {
        enabled: true,
        policy: "allowlist",
        allowFrom: ["U_YOUR_USER_ID"]
      }
    }
  }
}
```

**6.4 Restart Gateway**

```bash
sudo systemctl restart openclaw-gateway
```

**Done when:**
- Slack app created and installed in workspace
- Tokens set as environment variables in systemd service
- DM to Yakob bot gets a response

---

### Phase 7: Parallel Run & Validation (1 day)

**yx task:** `openclaw-orchestrator/validation`

Run both systems simultaneously:
1. Keep existing Zellij orchestrator available as fallback
2. Use OpenClaw Yakob for new task requests
3. Run through validation checklist
4. Monitor for 24 hours

**Validation checklist:**
- [ ] `openclaw gateway` starts without errors
- [ ] Yakob responds via Web Control UI (`http://localhost:18789`)
- [ ] `exec yx ls` shows task tree
- [ ] `exec ./spawn-worker.sh ...` launches a Docker worker
- [ ] Worker reports status via yx, Yakob sees it in heartbeat
- [ ] Heartbeat runs every 30m, reports HEARTBEAT_OK or alerts
- [ ] Cron worker sweep runs on schedule
- [ ] DM Yakob on Slack and get a response

**Done when:**
- All checklist items pass
- No regressions in worker spawning or yx state
- Heartbeat and cron run reliably over 24 hours

---

### Phase 8: Cutover (30 min)

**yx task:** `openclaw-orchestrator/cutover`

1. Disable old systemd service: `sudo systemctl disable yak-orchestrator`
2. Ensure openclaw-gateway is enabled: `sudo systemctl enable openclaw-gateway`
3. Update documentation
4. Keep Zellij available for monitoring (`zellij` with `yak-map.sh`)
5. Archive `.opencode/agents/yakob-orchestrator.md` (keep as reference)

**Done when:**
- `yak-orchestrator` service disabled
- `openclaw-gateway` is the sole orchestrator service
- Documentation updated

---

## Workspace Strategy

**Decision:** OpenClaw workspace in project directory, symlink for task state.

```
~/.openclaw/
+-- openclaw.json          # Gateway config
+-- agents/yakob/sessions/ # Session state (OpenClaw manages)

/home/yakob/yakthang/      # Project workspace
+-- .openclaw/
|   +-- workspace/         # Yakob's OpenClaw workspace
|       +-- SOUL.md
|       +-- AGENTS.md
|       +-- HEARTBEAT.md
|       +-- IDENTITY.md    # Created by onboarding
|       +-- memory/        # Daily logs (OpenClaw manages)
|       +-- .yaks -> /home/yakob/yakthang/.yaks  # Symlink
+-- .yaks/                 # Task state (shared with workers)
+-- spawn-worker.sh        # Worker spawning
+-- check-workers.sh       # Monitoring
+-- .opencode/             # OpenCode config (kept as fallback)
+-- ...
```

Exec commands use `workdir: "/home/yakob/yakthang"` to operate in the project.

---

## Files to Create

| File | Location | Purpose |
|------|----------|---------|
| `SOUL.md` | `~/.openclaw/workspace/` | Yakob's personality |
| `AGENTS.md` | `~/.openclaw/workspace/` | Operating procedures |
| `HEARTBEAT.md` | `~/.openclaw/workspace/` | Periodic check-in checklist |
| `openclaw.json` | `~/.openclaw/` | Gateway configuration |
| `openclaw-gateway.service` | `/etc/systemd/system/` | systemd service |

## Files to Modify

| File | Change |
|------|--------|
| `setup-vm.sh` | Add Node.js 22, OpenClaw installation, updated systemd service |

## Files Unchanged

| File | Reason |
|------|--------|
| `spawn-worker.sh` | Workers stay as-is |
| `check-workers.sh` | Monitoring stays as-is |
| `worker.Dockerfile` | Container image stays as-is |
| `.opencode/agents/yakob-orchestrator.md` | Keep as fallback |
| `.opencode/personalities/*.md` | Worker personalities unchanged |
| `orchestrator.kdl` | Keep for optional manual Zellij monitoring |

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| exec tool can't run spawn-worker.sh | Test in Phase 7; sandboxing is off by default |
| Zellij tabs don't work from Gateway exec | Pre-start `yakthang` session; set ZELLIJ_SESSION_NAME |
| OpenClaw Gateway instability | systemd auto-restart; workers survive independently |
| API cost increase (~$30-60/mo) | Acceptable for 24/7 availability; tune frequency if needed |

### Critical Risk: Zellij Tab Creation from Gateway Exec

**Problem:** `spawn-worker.sh` creates Zellij tabs via `zellij action new-tab`. If
the Gateway runs as a systemd service (not inside Zellij), there's no ZELLIJ_SESSION_NAME
and the script can't create tabs.

**Solution:** Pre-start a named Zellij session and set `ZELLIJ_SESSION_NAME` in systemd env:
```bash
# Separate systemd service (zellij-workers.service) starts the session
zellij --session yakthang

# In openclaw-gateway.service
Environment="ZELLIJ_SESSION_NAME=yakthang"
```
Workers get Zellij tabs as before. Yakob runs headless in the Gateway but spawns
tabs into the pre-existing `yakthang` session.

---

## Cost Analysis

| Item | Monthly Cost |
|------|-------------|
| Heartbeats (48/day x ~5K tokens) | ~$18 (Sonnet 4.5) |
| Cron jobs (~5/day x ~5K tokens) | ~$2 |
| Channel interactions (~20/day x ~10K tokens) | ~$12 |
| **Total incremental** | **~$30-60/month** |

---

## Timeline

| Phase | Effort | Dependency |
|-------|--------|-----------|
| 1. Infrastructure | 2-3 hours | None |
| 2. Identity files | 1-2 hours | Phase 1 |
| 3. Configuration | 1-2 hours | Phase 1 |
| 4. Cron jobs | 30 min | Phase 3 |
| 5. systemd service | 30 min | Phase 3 |
| 6. Slack integration | 1-2 hours | Phase 3 |
| 7. Parallel run | 1 day | Phases 1-5 |
| 8. Cutover | 30 min | Phase 7 |

**Total: ~1-2 days for full migration**

---

## References

- OpenClaw docs: https://docs.openclaw.ai
- GCP deployment: https://docs.openclaw.ai/install/gcp.md
- Heartbeat: https://docs.openclaw.ai/gateway/heartbeat.md
- Cron: https://docs.openclaw.ai/automation/cron-jobs.md
- Exec tool: https://docs.openclaw.ai/tools/exec.md
- Agent workspace: https://docs.openclaw.ai/concepts/agent-workspace.md
- Configuration: https://docs.openclaw.ai/gateway/configuration-reference.md
