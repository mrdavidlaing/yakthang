---
name: yak-wrap
description: Closing the barn. End-of-session wrap-up for Yakob. Harvests done yaks into a worklog summary, prunes the map, and reorganizes remaining work for the next session. Can be called at any natural break point — not just end of day.
---

# Yak Wrap 🌅

**Session closing. Time to wrap up the shaving.**

Yak-wrap walks the yak map, harvests the stories from this session's completed work,
generates a worklog summary, prunes the done yaks, and tidies the remaining
map for the next session.

## When to Use

Use at the end of a work session when:
- Multiple yaks have been shaved during the day
- You want to capture what was done before pruning
- The yak map needs tidying for the next session

## Announcement

**Always start by saying:**

"Wrapping this session. Let me walk the pasture and see what got shaved."

## Phase 1: Harvest

Walk the done yaks and collect their stories.

### Reconcile stale states first

Before harvesting, scan for yaks where `agent-status` says `done:` but
`yx ls` still shows them as todo/wip. These are yaks that were shaved but
never got their `yx done` — the worker finished but didn't mark the state.

```bash
# For every todo/wip yak, check if agent-status says done:
# If agent-status starts with "done:" but state is not done, mark it:
yx done <name>
```

Fix all of these before proceeding. This ensures the map is accurate
before you harvest and prune.

```bash
# See the full map
yx ls

# For each done yak, read its story:
yx context --show <name>                    # The brief (Yakob → agent)
yx field --show <name> agent-status         # The outcome summary
yx field --show <name> comments.md          # Findings, decisions, surprises
yx field --show <name> supervised-by        # Who was supervising when it was shaved
```

**Collect from every done yak:**

| Field | What it tells you |
|-------|-------------------|
| `context.md` | What was asked for |
| `agent-status` | What was delivered (one-line) |
| `comments.md` | The interesting bits — decisions, surprises, loose ends |
| `supervised-by` | Which human was supervising when it was shaved |

**Also check `yx log`** for timestamps — this tells you what was shaved
*today* vs previously. Focus the worklog on today's work.

### Grouping

Group done yaks by their parent. The parent provides narrative structure:
- "Under **cleanup review**, we shaved 5 yaks..."
- "The **cross-repo workstreams** work continued with..."

Orphan done yaks (no parent) get their own section.

## Phase 2: Report

Generate a worklog summary in markdown. Write it to stdout for the human
to review, then append it to the session yak's `comments.md`.

Find the active session yak (lives under 📋 worklogs):
```bash
session=$(yx ls --format plain | grep "session-" | head -1)
```

Write the report to the session yak's `comments.md` field using a heredoc
piped to `yx field`. **Do not use `yx show --format json`** — that flag
doesn't exist and will silently produce empty output, causing the write to
fail without error.

```bash
# If appending to existing comments, read them first:
existing=$(yx field --show "$session" comments.md 2>/dev/null)

# Write the full content via heredoc pipe — this is the reliable method:
cat <<EOF | yx field "$session" comments.md
${existing:+$existing

---

}## Yak Wrap — $(date '+%Y-%m-%d %H:%M')

[paste report here]
EOF
```

**Verify the write succeeded:**
```bash
yx field --show "$session" comments.md | head -5
```

If the field is empty after writing, the pipe failed — retry with a simpler
heredoc (no variable interpolation in the preamble).

Each yak wrap appends a new timestamped section — the session yak's
`comments.md` accumulates the full narrative across the day.

### Structure

Append the following to `{YYYY-MM-DD}-worklog.md` (and print to stdout):

```markdown
# Yak Wrap — YYYY-MM-DD HH:MM (supervised by @username)

## Highlights

Top-line accomplishments. One bullet per major outcome. Written for someone
who wasn't watching the yak map all day.

- Removed all td task management references; replaced with yx-based workflow
- Added --skill flag to yak-box for portable worker instructions
- Stripped worker agent files — skills now delivered via @home folder

## Shaved Yaks

Grouped by parent. For each yak: what was asked, what was done, anything
notable. Keep it concise — 1-3 lines per yak.

### cleanup review
- **remove td refs** — Cleaned td from flake.nix, .gitignore, skills dir,
  AGENTS.md, and release commands. Created yak-shaving-handbook skill as
  replacement.
- **skill flag for yak-box** — Added --skill flag to spawn; copies skill
  folders recursively into worker @home.

### fixes
- **tab emoji** — Added emoji prefix to Zellij tab names for quick scanning.

## Interesting Findings

Things discovered during the day that are worth knowing. Pulled from
`comments.md` fields and orchestrator observations.

- Worker agent .md files were duplicating instructions already injected by
  prompt.go — the "dedup" turned out to be a deletion.
- yx stores all state in refs/notes/yaks — .yaks/ is just a projection.
  `yx sync` handles bidirectional push/pull.

## Loose Ends

Things noticed but not addressed. Each is a candidate for a future yak.

- AGENTS.md and CLAUDE.md are still identical (item #1 from review)
- orchestrator.kdl hardcodes --dangerously-skip-permissions
- yak-box not yet buildable via nix flake (manual `just build-yakbox` needed)

## Remaining Yaks

Brief summary of what's still on the map after pruning.
(Generated after Phase 3.)
```

### Writing Guidelines

- **Highlights**: Written for a stakeholder. No jargon. What changed and why
  it matters.
- **Shaved Yaks**: Written for the team. Concise but technical. Reference
  repos and files where helpful.
- **Interesting Findings**: The "huh, didn't expect that" moments. Pulled
  directly from `comments.md` fields. If no yak had interesting findings,
  skip this section.
- **Loose Ends**: Not a TODO list — these are *observations* that may or may
  not become yaks. The human decides.

## Phase 3: Prune

After the human has reviewed the worklog:

```bash
# Show what will be pruned
yx ls

# Prune all done yaks
yx prune

# Show the cleaned map
yx ls
```

**Always show the map before and after pruning.** The human should see what's
being removed.

**Important:** `yx prune` removes done yaks whose children are all done.
A done parent with undone children will NOT be pruned — that's correct
behavior.

**Never prune the session yak.** It is a record, not work. Session yaks live
under `📋 worklogs` and persist there as the day's record.

## Phase 4: Reorganize

Review the remaining yak map and tidy it for next time.

### What to look for

1. **Orphaned children** — yaks whose parent was pruned but they're still
   todo. Do they need a new parent, or are they standalone now?
2. **Stale groups** — parent yaks with only 1 remaining child. Collapse?
3. **Logical regrouping** — after pruning, do the remaining yaks cluster
   into different themes than before?
4. **Naming** — any yaks whose names no longer reflect their scope after
   the day's work?

### How to reorganize

```bash
# Move a yak under a new parent
yx move <name> --under <new-parent>

# Move a yak to root (remove from parent)
yx move <name> --root

# Rename a yak
yx rename <old-name> <new-name>

# Show the result
yx ls
```

**Ask before reorganizing.** Show the human what you'd like to change and why.
Don't silently rearrange the map.

### Present the tidied map

```
Here's the yak map for next session:

[yx ls output]

N yaks remaining across M groups. Suggested starting points:
- [leaf yak] — ready to shave, no blockers
- [leaf yak] — independent, can run in parallel with above
```

## Phase 5: Sync

Push everything to remote so the state is preserved:

```bash
yx sync
```

## Quick Reference

| Phase | What | Command |
|-------|------|---------|
| Harvest | Reconcile stale states, read done yaks | `yx done`, `yx context --show`, `yx field <name> <field> --show` |
| Report | Generate worklog, append to session yak | `yx field "$session" comments.md` (pipe content) |
| Prune | Remove done yaks | `yx prune` |
| Reorganize | Tidy remaining | `yx move`, `yx rename` |
| Sync | Push to remote | `yx sync` |

## Red Flags

- **Pruning before reporting** — you lose the comments.md and context.md data
- **Skipping the harvest** — the worklog becomes a guess instead of a record
- **Reorganizing without asking** — the human may have reasons for the current structure
- **Forgetting to sync** — tomorrow's session won't have today's state
- **Writing highlights in jargon** — highlights are for stakeholders, not shavers
- **Skipping reconciliation** — yaks with `done:` in agent-status but todo/wip state will be missed by harvest and survive pruning
- **Using `yx show --format json`** — this flag doesn't exist. Use `yx field --show` to read fields, and always verify writes landed with `yx field --show` after piping
