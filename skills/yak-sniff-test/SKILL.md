---
name: yak-sniff-test
description: Does this yak smell right? Use when a shaver reports done and Yakob needs to verify the work matches the brief using a fresh, independent reviewer agent before accepting or pruning the yak.
---

# Adversarial Review

## Overview

The implementer is never the reviewer. When a shaver's `shaver-message` says
they're done, spawn a fresh agent with no knowledge of the shaver's reasoning
— only the brief, the done summary, the notes, and the git evidence. The
reviewer either confirms delivery or surfaces the gap.

## When to Use

Invoke after a shaver messages that they're finished and Yakob sets `wip-state`
to `ready-for-sniff-test`. Use `/adversarial-review <yak-id>`.

**Don't skip because:**
- "It's a small change" — small changes are exactly the ones that slip through
- "I read the comments.md myself" — you have anchoring bias from the shaver's narrative
- "We're in a hurry" — that's when errors get accepted silently

## Yakob's Steps

### 1. Read the yak

```bash
yx show <yak-id> --format json
```

Collect:
- `context.md` — the original brief (what was asked)
- `shaver-message` — the shaver's done summary
- `comments.md` — the shaver's notes (what changed and where)

### 2. Set wip-state to under-review

```bash
echo "under-review" | yx field <yak-id> wip-state
```

This shows 👀 in the yak-map. Then build the reviewer's prompt using the
template below, substituting the data collected in step 1:

```bash
cat <<'EOF' | yx context "review <yak-name>"
# Adversarial Review

You are an independent reviewer. You did not do this work. You have no knowledge
of why the implementer made their choices. Your only job is to verify that the
actual state of the codebase matches what was asked for.

## Original Brief (what was asked)

<paste context.md here>

## Shaver's Done Summary (shaver-message)

<paste shaver-message here>

## Shaver's Notes (comments.md)

<paste comments.md here — this tells you where to look>

## Your Task

1. Extract the key deliverables from the original brief. What was explicitly
   promised? What are the acceptance criteria?

2. Read comments.md carefully. The shaver should have noted which files,
   repos, or directories were changed. Navigate to those locations.

3. For each deliverable, independently verify it exists in the actual state of
   the codebase. Check git log, file contents, test output — whatever applies.
   Do not trust the summary. Look at the evidence.

4. Produce a binary verdict:
   - `pass: <one-line summary of what you confirmed>`
   - `fail: <one-line summary of the gap>`
   - `needs-info: <what's missing that prevents verification>`

5. Report your verdict in your response (Yakob will handle the state transition).

## Anti-Patterns

- Do NOT ask the shaver to clarify — verify independently or report `needs-info`
- Do NOT use the shaver's reasoning to justify findings — find your own evidence
- Do NOT accept "it should work" — verify it does work
EOF
```

### 3. Launch the reviewer as a subagent

Use a `general-purpose` subagent, not `yak-box spawn`. Reviewers
are read-only agents that don't need workspace isolation, and subagents avoid
keychain/auth issues, stale sessions, and Zellij tab clutter.

```
Agent tool call:
  subagent_type: "general-purpose"
  description: "Review <yak-name>"
  run_in_background: true
  prompt: |
    You are an adversarial reviewer for the "<yak-name>" feature.

    ## Original Brief
    <paste context.md>

    ## Shaver's Done Summary
    <paste shaver-message>

    ## Shaver's Notes
    <paste comments.md or "No comments were left.">

    ## Your Task
    1. Check git log in <relevant-dir> for recent commits
    2. Verify each acceptance criterion against actual code
    3. Note which test commands should be run (e.g., go test ./..., cargo test)
       but do NOT run them yourself — Yakob will run tests separately
    4. Report verdict: pass, fail, or needs-info with file/line evidence
```

### 4. When the subagent returns

**Run the tests yourself.** The subagent cannot run bash commands, so Yakob
must independently verify that tests pass before recording the verdict:

```bash
# Run whatever tests apply to the changed code
cd <relevant-dir> && go test ./...    # or cargo test, npm test, etc.
```

Then parse the verdict from the subagent's response and drive the wip-state transition.

**Yakob sets wip-state based on the verdict:**

```bash
# On pass:
echo "ready-for-human" | yx field <yak-id> wip-state

# On fail:
echo "shaving" | yx field <yak-id> wip-state
echo "<detailed findings with file/line evidence>" | yx field <yak-id> review-notes
echo "Review failed: <summary>. Check review-notes for details." | yx field <yak-id> yakob-message

# On needs-info:
echo "shaving" | yx field <yak-id> wip-state
echo "<what's missing>" | yx field <yak-id> review-notes
echo "Review needs more info: <summary>. Check review-notes." | yx field <yak-id> yakob-message
```

On **pass**: wip-state moves to `ready-for-human` (👀🧑) — the human reviews and decides done.

On **fail**: wip-state resets to `shaving` (🪒) and Yakob messages the shaver
via `yakob-message` with what needs fixing. The shaver picks up from there —
no sub-yak needed.

On **needs-info**: same as fail — reset to shaving and message the shaver
with what's missing.

## Reading Results

The yak-map shows wip-state emoji. For failure details:

```bash
yx field --show <yak-id> review-notes
```

| Verdict | wip-state | Emoji | What happens |
|---------|-----------|-------|-------------|
| pass | ready-for-human | 👀🧑 | Human reviews and decides done |
| fail | shaving | 🪒 | Yakob messages shaver, work continues |
| needs-info | shaving | 🪒 | Yakob messages shaver with what's missing |

## Anti-Patterns

- **Shaver reviews their own work** — no anchoring allowed; fresh agent only
- **Passing reviewer findings to a re-review** — starts the next review clean
- **Skipping because "it's a small change"** — that's exactly the judgment this gate validates
- **Reading comments.md and deciding yourself** — you have Yakob bias; spawn the reviewer

## Why Subagents, Not yak-box spawn

Reviewers are read-only by design. They don't edit files, don't need workspace
isolation, and don't need their own Zellij tab. Subagents:

- Inherit Yakob's auth (no keychain issues)
- Leave no stale sessions or `assigned-to` files
- Don't count against the WIP limit (they're Yakob's work, not independent shavers)
- Run in background and notify on completion

**Do not use `yak-box spawn` for reviews.** That path has known issues with
keychain access, tab cleanup, and assignment paperwork — all overhead with
zero benefit for a read-only task. When spawning shavers for implementation
work, use `--skill .claude/skills/yak-shaving-handbook` (see yakob.md).

## Quick Reference

| Step | Command |
|------|---------|
| Read yak | `yx show <id> --format json` |
| Mark under review | `echo "under-review" \| yx field <id> wip-state` |
| Launch reviewer | Agent tool: `general-purpose`, `run_in_background: true` |
| On pass | `echo "ready-for-human" \| yx field <id> wip-state` |
| On fail | `echo "shaving" \| yx field <id> wip-state` + `yakob-message` to shaver |
| Read failure detail | `yx field --show <id> review-notes` |
