---
name: yak-triage
description: Sorting the herd at the gate. Session start ritual for Yakob. Establishes the time window and WIP limit, surveys the yak map, and helps the operator decide what to tackle this session — before acceleration flow begins.
---

# Yak Triage 🔍

**New session. Before the first shaver spawns, let's figure out what we're working with.**

Yak-triage runs at the start of every session. It asks the pre-commitment
questions while reflective control is high, surveys the map, and produces a
session plan that constrains the rest of the session.

**Why upfront:** Once shavers are running and flow begins, the dopamine loop
makes stopping cues invisible. Pre-commitment is the only reliable mechanism.
This step is non-negotiable.

## Announcement

**Always start by saying:**

"New session. Let's triage before we shave anything."

---

## Phase 1: Pre-Commitment Questions

Ask these questions **before** looking at the yak map. Get the constraints
first — then assess what fits within them.

### 1a. Time window

> "What's your hard stop for this session?"

Accept any format: "12:30", "in 2 hours", "after lunch". Convert to a
concrete time and confirm it back.

If the operator doesn't know or says "as long as it takes" — push back once:
> "A rough estimate is fine. Having no hard stop is what we're trying to fix."

Record as: **hard stop = HH:MM**

### 1b. WIP limit

> "How many shavers do you want running in parallel? (Default: 3)"

If the operator accepts the default, proceed. If they say "as many as needed" — note
the default of 3 and move on. Don't debate it.

Record as: **WIP limit = N**

### 1c. Any must-dos?

> "Anything that must happen this session regardless of what the map shows?"

These get priority placement in the session plan. If none, proceed.

---

## Phase 2: Survey the Map

Now read the yak map:

```bash
yx ls
```

Categorise what you see:

| Category | Description |
|----------|-------------|
| **In flight** | WIP yaks — already running, count against WIP limit |
| **Ready** | Leaf yaks with no blockers — candidates for this session |
| **Blocked** | Yaks waiting on something — skip unless the blocker can be resolved |
| **Done** | Not yet pruned — note these for yak wrap, don't shave again |

Also check the previous session's worklog for context on what was in flight:

```bash
ls {YYYY-MM-DD}-worklog.md 2>/dev/null && tail -50 {YYYY-MM-DD}-worklog.md
```

If today's worklog exists, surface any loose ends or parked items from the
last wrap — these are warm candidates.

---

## Phase 3: Session Plan

With the time window and map in hand, propose a session plan.

### Sizing guidance

Rough heuristics (adjust based on yak complexity):

| Window | Realistic throughput |
|--------|----------------------|
| 1 hour | 2–4 small yaks, or 1 substantial yak |
| 2 hours | 4–8 yaks with good parallelism |
| 3+ hours | Full sprint with multiple waves |

**Don't over-fill.** It's better to finish everything and pull more than to
have 8 yaks in flight when yak wrap triggers.

### Present the plan

```
Session: HH:MM → HH:MM (N hours)
WIP limit: N shavers

Suggested focus:
  - [yak-name] — [one line: why this one fits]
  - [yak-name] — [one line: why this one fits]
  - [yak-name] — [one line: why this one fits, or "warm from last session"]

Holding back:
  - [yak-name] — [why skipping: blocked / too big / lower priority]
```

If the operator has must-dos from Phase 1, list those first.

### Confirm

> "Does this look right, or do you want to swap anything?"

Wait for the operator's response. Adjust if needed. Once confirmed, proceed.

---

## Phase 4: Commit

Create a session yak to hold the session state and running worklog.

```bash
# Create the session yak under 📋 worklogs
yx add session-YYYY-MM-DD-HHMM \
  --under 📋 worklogs \
  --field hard-stop=HH:MM \
  --field wip-limit=N \
  --field started="YYYY-MM-DD HH:MM"

# Mark the session as active so it shows wip in the yak map
yx start session-YYYY-MM-DD-HHMM

# Store the session plan in context.md (what we committed to tackle)
# context.md is the Yakob→shaver brief field — use yx context for it
yx context session-YYYY-MM-DD-HHMM
# Paste/type: the confirmed focus list from Phase 3
```

The session yak is Yakob's source of truth for the session:
- Fields are read for WIP enforcement and hard stop triggering
- `comments.md` accumulates the running worklog (yak wrap appends here)
- The yak stays visible in `yx ls` throughout the day — don't prune it

**Do not mark the session yak as done** — it is not work to be completed,
it is a record. Yak wrap will append to it; it persists until manually pruned.

### Announce ready

```
Session committed. Hard stop: HH:MM. WIP limit: N.
Starting with: [yak-name-1], [yak-name-2].
Say "yak wrap" at any point to close out early.
```

**Start the heartbeat** before spawning the first shaver:

```
/loop 5m yx ls
```

This runs `yx ls` every 5 minutes so Yakob sees shaver progress between turns.

Then begin shaving.

---

## Yakob: Enforcing the Session During Shaving

Once yak triage completes, Yakob reads the session yak and enforces:

```bash
# Find the active session yak (name starts with "session-")
session=$(yx ls --format plain | grep "^session-" | head -1)

# Read session parameters as JSON
session_json=$(yx show "$session" --format json)
# Parse with jq (or read fields individually):
hard_stop=$(echo "$session_json" | jq -r '.fields["hard-stop"]')
wip_limit=$(echo "$session_json" | jq -r '.fields["wip-limit"]')
```

**WIP ceiling** — before spawning a new shaver, count wip yaks in the current
map (excluding the session yak itself). Yakob can read `yx ls` output and count
the wip state indicators directly. If the count meets the limit:

> "WIP limit (N) reached. Finish or park something first."
> Do not spawn. Surface the drain queue instead.

**Hard stop** — when a shaver finishes and current time ≥ hard stop:

> "Hard stop reached (HH:MM). Triggering yak wrap — no new shavers."
> Call `/yak-wrap` automatically.

**90-minute break prompt** — if session duration > 90 minutes and no break taken:
> "You've been shaving for 90 minutes. Take a break before the next shaver?"
> (Consequential prompt — wait for response before spawning.)

---

## Quick Reference

| Phase | What | Command |
|-------|------|---------|
| Pre-commit | Get hard stop + WIP limit | (ask the operator) |
| Survey | Read the map | `yx ls` |
| Plan | Propose session focus | (present + confirm) |
| Commit | Create session yak | `yx add session-YYYY-MM-DD-HHMM` + field sets |

## Red Flags

- **Skipping pre-commitment** — surveying the map before setting constraints lets the map set the agenda. Constraints first, always.
- **Over-filling the plan** — more yaks than the window can hold creates the problem we're solving. Size conservatively.
- **No hard stop** — if the operator won't name one, default to 2 hours from now and state it explicitly.
- **Ignoring the WIP count of in-flight yaks** — yaks already running count against the limit from the start.
