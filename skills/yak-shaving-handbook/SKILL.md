---
name: yak-shaving-handbook
description: The shaver's field guide. The complete operating guide for shavers in the yakthang environment. Covers task lifecycle (yx show, start, shaver-message, done), message checking (yakob-message), heartbeat (via /loop), and notes for Yakob.
---

# Yak Shaving Handbook

The complete operating guide for shavers. Covers task lifecycle, message checking, and heartbeat.

---

## 1. Task lifecycle

## Overview

`yx` is a DAG-based task tracker. Tasks (yaks) are organized in a tree where
**parents are blocked by children** — you work deepest-first, and a parent
cannot be marked done until all its children are.

Three fields carry the conversation between Yakob and agents:

- `context.md` — **Yakob → agent**: the brief. What to do, definition of done, known constraints.
- `comments.md` — **agent → Yakob**: the response. What was done, decisions made, surprises found.
- `shaver-message` — **agent → Yakob**: live progress signal. What you're doing right now.

## Yak IDs

Every yak has a unique **ID** — a hyphenated slug with a short random suffix
(e.g., `yak-map-show-on-enter-improvements-al09`). Use the ID to reference
yaks in all yx commands.

Discover IDs with `--format json`:

```bash
yx show <name> --format json | jq -r .id          # Get a single yak's ID
yx show <name> --format json | jq -r '.children[].id'  # Get child IDs
```

All commands below use `<id>` to mean the yak ID.

## Session Start (Every Time)

Before doing any work, orient yourself:

```bash
yx ls                              # See all tasks and their states
yx show <id>                       # Full detail view: context, children, fields, metadata
yx context <id>                    # Read your task's brief from Yakob (defaults to show)
yx field <id> comments.md          # Check if a previous agent left notes (defaults to show)
```

`yx show` gives you the complete picture in one command — context, children,
custom fields, creation date, and author. Use it as your first stop. Fall back
to `yx context` or `yx field` when you need just one piece.

`yx ls` shows the full DAG. Identify:
- Your assigned task
- Its children (you must complete these before the parent)
- Its state (todo / wip / done)

Then claim it, stamp the supervisor, and announce you've started:

```bash
yx start <id>
echo "$supervisor" | yx field <id> supervised-by
echo "starting work" | yx field <id> shaver-message
```

`$supervisor` is the human username passed in your spawn prompt (e.g.
"zgagnon"). This associates the yak with the human who was supervising when
it was shaved — distinct from `created_by` (who planned it) and
`assigned-to` (which shaver worked on it).

## State Transitions

Shavers **do not control state**. Yakob owns all `wip-state` transitions.

```
todo ──(human)──→ wip ──(human)──→ done
                   │
          Yakob manages wip-state:
          🪒 shaving → 🚫 blocked → 💤 sleeping
          → 👀🙏 ready-for-sniff-test → 👀 under-review
          → 👀❌ failed-sniff-test → 👀🧑 ready-for-human
```

- **todo → wip**: Human decides (via Yakob)
- **wip-state transitions**: Yakob decides based on your `shaver-message`
- **wip → done**: Human decides after reviewing

Use `yx start <id>` to claim work (sets state to wip). Do **not** use
`yx done <id>` — tell Yakob you're finished via `shaver-message` and Yakob
will handle the review and done transition.

## shaver-message: Live Signal

`shaver-message` is your communication channel to Yakob. Write free-text
messages describing what you're doing, what you've found, or what's blocking
you. No prefix convention — just say what you mean.

```bash
# Starting
echo "starting work, reading the codebase" | yx field <id> shaver-message

# Progress milestone
echo "updated flake.nix, removing gitignore entry next" | yx field <id> shaver-message

# Blocked
echo "stuck: flake.nix has merge conflict, cannot proceed without resolution" | yx field <id> shaver-message

# Finished
echo "done — removed td from flake.nix, .gitignore, AGENTS.md, and skills dir" | yx field <id> shaver-message
```

Yakob reads your messages and decides state transitions:
- You say "stuck" or describe a blocker → Yakob sets wip-state to `blocked`
- You say "done" or "finished" → Yakob sets wip-state to `ready-for-sniff-test`
- You describe progress → Yakob keeps wip-state as `shaving`

`shaver-message` is not the place for reasoning or detail — that goes in `comments.md`.

## spend: Cost Tracking

If you're running in a Claude Code worker with `goccc` available, track your session cost alongside messages:

```bash
# Update spend field with current session cost
goccc -days 0 -json 2>/dev/null | jq -r '.summary.total_cost // "0"' | yx field <id> spend

# Combined with message
echo "implementing feature X" | yx field <id> shaver-message
goccc -days 0 -json 2>/dev/null | jq -r '.summary.total_cost // "0"' | yx field <id> spend
```

The `spend` field accumulates session cost. Update it at meaningful milestones (not every tiny change):
- When starting work
- At progress checkpoints (when updating shaver-message)
- When blocked or finished

If `goccc` or `jq` aren't available, skip the spend update silently (the `2>/dev/null` suppresses errors).

## comments.md: Notes for Yakob

Write `comments.md` when you have something Yakob needs to know beyond the
bare message. It's the agent's response to `context.md` — written for
Yakob to read when reviewing completed work.

Good candidates:

- **Decisions made** — and why, especially where you deviated from the brief
- **Surprises** — things discovered that weren't in the context
- **Loose ends** — things noticed but out of scope, worth a future yak
- **Caveats** — anything Yakob should know before signing off

```bash
echo "Replaced the entire start.md rather than patching individual td commands — the workflow steps were too intertwined to patch cleanly. Release matrix logic preserved intact. Note: allowed-tools frontmatter also had Bash(td:*) which I removed." | yx field <id> comments.md
```

Write it once, near the end of your work. It doesn't need to be long — a
few sentences is enough. Even for straightforward work, write at least one
line noting what was changed and where. Reviewers use this to know where to
look.

## Handling Blockers

If you're blocked, don't spin — report and yield:

```bash
echo "stuck: <clear reason>" | yx field <id> shaver-message
```

Be specific about what's needed to unblock:

```bash
# Too vague
echo "stuck: something's wrong with the build" | yx field <id> shaver-message

# Good
echo "stuck: nix flake check fails with 'attribute yx missing' — yx buildRustPackage may need cargoHash update" | yx field <id> shaver-message
```

Then stop. Don't retry the same failing approach. Yakob will read your message,
set wip-state to `blocked`, and intervene or reassign.

## Completing Work

When you believe the work is done:

```bash
echo "done — <one sentence summary>" | yx field <id> shaver-message
```

If there's anything worth telling Yakob, write `comments.md` before reporting done.

**Do not run `yx done`** — Yakob will move your yak through the review gate
(sniff test → human review) and the human decides when to mark it done.

## Working with the DAG

yx enforces: **a parent cannot be done until all children are done.**

Work **deepest-first** (leaves before parents):

```
● my feature          ← cannot mark done until children done
├─ ● sub-task-a       ← do this first
╰─ ● sub-task-b       ← then this
```

### Adding sub-tasks

```bash
yx add child task --under parent task    # Nest under a parent
yx add child task --in parent task       # Same (--in, --into, --blocks are aliases)
```

### Removing tasks

```bash
yx remove <id>                # Remove a leaf task
yx remove --recursive <id>      # Remove a task and all its children
```

---

## 2. Message checking

Yakob can send instructions to a shaver during the session by writing to the yak's `yakob-message` field. **After each major step**, poll for messages and follow any instructions before continuing.

```bash
yx field --show <id> yakob-message
```

- If the output is **non-empty**, treat it as instructions from Yakob. Follow them (e.g. adjust approach, switch task, add notes), then continue or stop as directed.
- If **empty**, proceed with your next step.

No external nudge is required — checking `yakob-message` is part of the shaver workflow.

---

## 3. Heartbeat

Heartbeat is **Yakob's responsibility**, not the shaver's. Shavers don't need to do anything for heartbeat.

Yakob uses `/loop` to schedule recurring status checks:

```
/loop 5m yx ls; yx field --show <id> shaver-message
```

- `/loop` uses `CronCreate` under the hood — session-only, auto-expires after 3 days.
- No external scripts, no fswatch dependency, no manual relaunching required.
- Yakob relaunches the loop as needed; shavers just keep their `shaver-message` updated.

---

## Quick Reference

| Action | Command |
|--------|---------|
| See all tasks | `yx ls` |
| Full task detail | `yx show <id>` |
| Get yak ID | `yx show <name> --format json \| jq -r .id` |
| Read task brief | `yx context <id>` |
| Read previous agent's notes | `yx field <id> comments.md` |
| Claim a task | `yx start <id>` |
| Stamp supervisor | `echo "$supervisor" \| yx field <id> supervised-by` |
| Report progress | `echo "..." \| yx field <id> shaver-message` |
| Check for Yakob message | `yx field --show <id> yakob-message` |
| Heartbeat (Yakob's responsibility) | `/loop 5m yx ls; yx field --show <id> shaver-message` |
| Update cost (if goccc available) | `goccc -days 0 -json 2>/dev/null \| jq -r '.summary.total_cost // "0"' \| yx field <id> spend` |
| Leave notes for Yakob | `echo "..." \| yx field <id> comments.md` |
| Add sub-task | `yx add child --under parent` |
| Remove task tree | `yx remove --recursive <id>` |
| Report finished | `echo "done — <summary>" \| yx field <id> shaver-message` |

## Red Flags

- **Starting without reading context** — always `yx show` or `yx context` first
- **Starting without checking comments.md** — a previous agent may have left important notes
- **No shaver-message updates** — Yakob is flying blind
- **Vague messages** — "doing stuff" tells nobody anything
- **Decisions buried in shaver-message** — put reasoning in `comments.md`, not the live signal
- **Marking parent done before children** — yx will reject this anyway
- **Retrying a blocked operation** — report it and stop
- **Running `yx done` yourself** — Yakob handles the done transition after review
- **Trying to set wip-state** — Yakob owns all state transitions; you just communicate via shaver-message
