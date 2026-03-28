---
name: yak-mapping
description: Surveying the herd. Use when planning work by approaching goals and discovering blockers — emergent planning through action, not top-down decomposition
---

# Yak Mapping

## Overview

**Yak mapping is emergent planning through action.** You discover work structure
by approaching goals and finding what blocks you, not by decomposing from the
top down.

**Core principle:** "It is in the doing of the work that we discover the work
that we must do." — Woody Zuill

## Announcement

**Always start yak mapping by saying:**

"I'm using yak-mapping to discover the work structure by approaching the goal.
I'll add yaks one at a time and show the map after each addition."

This sets expectations that we're doing emergent discovery, not top-down
planning.

## When to Use

Use when:
- User asks you to "plan" or "map out" or "break down" work
- Starting to structure a new feature or goal
- Need to break down complex work into spawnable tasks

Don't use when:
- Just executing already-mapped work
- Single straightforward task
- User provides a detailed step-by-step plan

## ⚠️ CRITICAL: yx Syntax

**Run `yx add --help` before your first add** to confirm current syntax.

Key rules:
- **Names are space-separated words**: `yx add my task` (NOT `yx add my/task`)
- **Nesting uses --under**: `yx add child --under parent`
- **Create parent first**: `yx add parent`, then `yx add child --under parent`
- **Reference tasks by leaf name**: If you created `yx add worker --under extract`,
  reference it as `worker`, not `extract worker`
- **Pipe context via stdin**: `echo "description" | yx context my task`

## ⚠️ THE IRON LAW

**After EVERY `yx add`, immediately run `yx ls` to show what changed.**

No exceptions:
- Not "I'll show it at the end"
- Not "just adding a quick one"
- Not "the structure is obvious"

`yx add` → `yx ls` is non-negotiable. This keeps the human in sync with your
thinking.

## The Approach Pattern

### Core Loop (ONE yak at a time)

```
1. Add ONE yak
2. Show map with `yx ls`
3. Add context to that yak (pipe via stdin)
4. Pick ONE child to explore next
5. Repeat
```

### Step-by-Step Process

**1. Start with the Goal**
```bash
yx add deploy to production
yx ls              # Always show after adding
```

**2. Approach It (Don't Decompose)**

Ask yourself: "If we tried to implement this RIGHT NOW, what would we try
first?"

Don't ask: "What are all the components?"

**3. Discover ONE Blocker**

When approaching reveals "we need X first", add X as a child:
```bash
yx add deployment script --under deploy to production
yx ls              # Show the updated map
```

**The nesting means:** "deploy to production is BLOCKED BY deployment script"

**4. Add Context Before Going Deeper**
```bash
echo "Create a bash script that deploys the app to production.
- Entry point: scripts/deploy.sh
- Must handle rollback on failure
- Uses existing Docker images" | yx context deployment script
```

**5. Approach This Blocker**

Now explore this one level deeper:
```bash
# Approaching "deployment script" reveals we need Docker config first
yx add docker config --under deployment script
yx ls              # Always show after adding
```

**6. Continue ONE Level at a Time**

Keep approaching and discovering until you hit a leaf node (can implement
without discovering new blockers).

### When to Stop Exploring a Branch

Stop when:
- You've reached a leaf (no new blockers discovered)
- You've identified enough structure to start work
- Going deeper requires actually doing the work (not just thinking)

Then explore other branches or spawn workers for ready leaves.

## Dependency Structure

**Parent = blocked goal. Children = blockers. Work deepest-first.**

When you approach goal B and discover you need A first, B becomes the
**parent** of A. This is an artifact of discovery, not a planning decision.

yx enforces: **parent cannot be marked done until children complete.**

### Multiple Blockers

If approaching reveals 3 things blocking a goal, they ALL become children:

```bash
yx add deploy to production
yx ls
yx add deployment script --under deploy to production
yx ls
yx add configure secrets --under deploy to production
yx ls
yx add update docs --under deploy to production
yx ls
```

### Work Order

Work **deepest-first** (leaves before parents):

```
○ deploy to production
├─ ○ deployment script
│  ╰─ ○ docker config     ← Start here (leaf)
├─ ○ configure secrets     ← Or here (leaf)
╰─ ○ update docs           ← Or here (leaf)
```

## Yak Granularity

**Leaf yaks should be completable by a single worker in one session.**

**Right-sized yaks:**
- ✅ Approaching reveals 2-4 blockers → probably good size
- ✅ Approaching reveals 0 blockers and feels ready to implement → perfect leaf
- ❌ Approaching reveals 0 blockers and feels tiny → too granular
- ❌ Approaching reveals 6+ blockers → too large, needs intermediate level

## Context Pattern

**Write contexts assuming a worker will implement — zero shared context.**

Workers are spawned in isolated tabs. They only know what's in the yak context.

Include:
- **Goal**: What this accomplishes (1 sentence)
- **Definition of Done**: Specific, testable criteria (3-5 bullets)
- **Known Knowns**: File paths, patterns, dependencies, constraints
- **Known Unknowns**: Open questions (answered during implementation)

### Be SPECIFIC

- ✅ "Update go.mod module path from github.com/old to github.com/new"
- ✅ "Source: repos/yakthang/yak-box/cmd/spawn.go around line 270"
- ✅ "Run 'just yakbox-build' to verify the fix"
- ❌ "fix the module" (too vague)
- ❌ "update the code" (what code? where?)

### Example Context

```bash
echo 'Fix assigned-to path resolution in yak-box spawn.

Source: repos/yakthang/yak-box/cmd/spawn.go around line 270.
The bug: filepath.Join(absYakPath, taskSlug, "assigned-to") uses
the flat task name, but tasks can be nested (e.g.
.yaks/release-yakthang/yak-box/native-spawn-bug/).

Fix: search the .yaks/ tree to find the actual directory matching
the task name, then write assigned-to there.

Definition of Done:
- assigned-to files are written to the correct nested path
- go test ./... passes
- manual test: spawn a worker with --yaks for a nested task,
  verify .yaks/<full-path>/assigned-to exists' | yx context fix assigned-to path
```

## After Mapping is Complete

Once you've discovered blockers and identified leaf nodes:

**Present the map and ask:**

```
Mapping complete! Ready-to-implement leaf yaks:
- [list leaf nodes with their parent context]

Next steps:
1. Spawn workers for independent leaves (parallel)
2. You choose which yaks to start with
3. Review the map first

Which would you like?
```

**When spawning workers for leaves**, remember:
- Independent leaves can be worked in parallel
- Each worker gets `--cwd` scoped to the right repo
- One worker per directory to avoid conflicts
- Use `--runtime sandbox` for interactive Claude Code sessions

## Common Mistakes

### ❌ Top-Down Decomposition
```bash
# WRONG: Planning all components upfront
yx add event logging --under sync
yx add git storage --under sync
yx add replay algorithm --under sync
# You haven't approached anything yet!
```

### ✅ Discovery Through Approach
```bash
# RIGHT: What would we try first?
yx add sync
yx ls
# "If we approached sync, we'd need to write events"
yx add write events --under sync
yx ls
# "If we approached that, we'd need a log command"
yx add implement log --under write events
yx ls
```

### ❌ Batch Creation
Adding 5 yaks without showing structure between each.

### ✅ Incremental Updates
Add one, show map, add context, add next, show map.

### ❌ Over-Planning Context
Writing detailed implementation plans before discovering blockers.

### ✅ Lightweight Context
Goal + done criteria + key decisions. Details emerge when you work on it.

### ❌ Nesting by "feels like subtask"
```bash
# WRONG: CI is not blocked by local setup
yx add local lint --under ci workflow
```

### ✅ Nesting by "blocks the parent"
```bash
# RIGHT: CI workflow is blocked BY local lint setup
yx add local lint --under ci workflow
# (same structure, but the REASONING matters — ask "what blocks this?")
```

## Quick Reference

| Action | Command |
|--------|---------|
| Add goal | `yx add my goal` |
| Add blocker | `yx add blocker --under my goal` |
| Show map | `yx ls` |
| Add context | `echo "..." \| yx context my task` |
| Read context | `yx context --show my task` |
| Mark done | `yx done my task` |
| Check syntax | `yx add --help` |

## Red Flags

- **Adding multiple yaks without `yx ls` between them**
- Creating all yaks before showing any structure
- Planning "components" instead of discovering blockers
- Writing implementation details in context before approaching work
- Nesting by "feels like subtask" instead of "blocks the parent"
- **Exploring 3+ levels deep before adding context to parents**
- Definition of done too vague ("make it work", "add tests")
- Known knowns without specifics (no file paths, patterns, or examples)

**If you catch yourself doing these, stop and restart with approach-first.**
