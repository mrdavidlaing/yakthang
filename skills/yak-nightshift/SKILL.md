---
name: yak-nightshift
description: The night crew. Unattended overnight session where Yakob works through a queue of well-defined yaks serially — shave, sniff-test, remediate if needed, push a PR — so the operator wakes up to reviewable PRs.
---

# Yak Nightshift 🌙

**Lights out. The night crew takes over.**

Yak-nightshift is an alternative session mode for Yakob. Instead of interactive
parallel shaving, Yakob works through a pre-approved queue of well-defined yaks
one at a time overnight. Each yak gets shaved, sniff-tested, and pushed as a PR.
The operator wakes up to a set of reviewable PRs.

## When to Use

Invoke before going to bed when the yak map has well-defined, low-strategy work:
- CVE triage and patching
- Dependency updates
- Refactoring for code quality
- Linting cleanups
- Documentation updates
- Test coverage improvements

**Not for:** strategic decisions, architecture changes, or yaks with vague context.

## Announcement

**Always start by saying:**

"Night shift. Let me survey the herd and figure out what the night crew can handle."

---

## Phase 1: Queue

Survey the yak map and build the overnight queue.

```bash
yx ls
```

### Candidate criteria

Good nightshift yaks are:
- **Leaf yaks** — no children or blockers
- **Context-rich** — `context.md` is substantive and actionable
- **Low-strategy** — patches, refactors, cleanups, triage
- **Independent** — don't share files/directories with other queue yaks

For each candidate, verify context exists:
```bash
yx context --show <yak-name>
```

Skip yaks with thin or missing context — they'll block the shaver overnight
with no one to clarify.

### Propose the queue

Present an ordered list:

```
Nightshift queue (proposed):
  1. [yak-name] — [one line: what and where]
  2. [yak-name] — [one line: what and where]
  3. [yak-name] — [one line: what and where]

Holding back:
  - [yak-name] — [why: thin context / strategic / blocked]

Estimated: N yaks over ~8 hours.
```

### Confirm

> "Does this queue look right? I'll work through them in this order overnight."

Wait for the operator's confirmation. Adjust order or membership if requested.
Once confirmed, the queue is locked — no interactive changes overnight.

---

## Phase 2: Commit

Create the nightshift session yak.

```bash
# Calculate hard stop (8 hours from now, or operator-specified)
# Store both epoch (for comparison) and HH:MM (for display)
hard_stop_epoch=$(date -d "+8 hours" +%s 2>/dev/null || date -v+8H +%s)
hard_stop=$(date -d "+8 hours" +"%H:%M" 2>/dev/null || date -v+8H +"%H:%M")

yx add session-nightshift-YYYY-MM-DD-HHMM \
  --under "📋 worklogs" \
  --field hard-stop="$hard_stop" \
  --field hard-stop-epoch="$hard_stop_epoch" \
  --field wip-limit=1 \
  --field type=nightshift \
  --field started="$(date '+%Y-%m-%d %H:%M')" \
  --field queue="yak-1, yak-2, yak-3"

yx start session-nightshift-YYYY-MM-DD-HHMM
```

Store the confirmed queue in context:
```bash
cat <<EOF | yx context session-nightshift-YYYY-MM-DD-HHMM
Nightshift queue (confirmed):
1. yak-1
2. yak-2
3. yak-3

Hard stop: $hard_stop
EOF
```

### Start the heartbeat

```
/loop 5m [heartbeat] yx ls
```

The `[heartbeat]` prefix lets Yakob distinguish automated pulses from operator
messages (see "Heartbeat Recognition" in yak-triage).

**Capture the heartbeat job ID** so it can be cancelled when the queue is
exhausted. After `/loop` creates the cron, run `CronList` and note the job ID
(the most recently created cron entry). Store it for use in Phase 3 teardown.

### Announce

```
Nightshift committed. Hard stop: HH:MM. Queue: N yaks.
Working serially — shave, sniff, PR, next.
Going dark. See you in the morning.
```

---

## Phase 3: Run — The Nightshift Loop

For each yak in the queue, execute the following cycle. **Always check the
hard stop before starting a new yak.**

### Pre-flight check

```bash
# Check hard stop before each yak (epoch comparison handles midnight crossover)
current_epoch=$(date +%s)
hard_stop_epoch=$(yx field --show "$session" hard-stop-epoch)
if [ "$current_epoch" -ge "$hard_stop_epoch" ]; then
  # Hard stop reached — skip to Phase 4 (Wrap)
fi
```

If hard stop is reached, do NOT start the next yak. Proceed to Phase 4.

### Step 1: SHAVE

Prepare skills and pick a shaver name:

```bash
skill_flags=$(ls -d .claude/skills/*/ 2>/dev/null | sed 's|/$||' | xargs -I{} echo "--skill {}" | tr '\n' ' ')

in_use=$(yx ls --format json 2>/dev/null \
  | jq -r '.. | objects | select(.fields) | .fields | to_entries[]
           | select(.key == "Assigned-to") | .value' 2>/dev/null \
  | sort -u)
shaver_name=$(printf '%s\n' Yakira Yakoff Yakriel Yakueline Yaklyn Yakon Yakitty Bob \
  | grep -vxF "$in_use" | shuf | head -1)
```

Spawn a shaver with a worktree. Use `--runtime native` — overnight work
requires git push access, and sandboxed containers may not have credentials
mounted. If your environment mounts SSH keys or git credentials into the
sandbox, you can use `--runtime sandboxed` instead.

```bash
yak-box spawn \
  --cwd <yak-working-directory> \
  --yak-name "<yak-name>" \
  --shaver-name "$shaver_name" \
  --tool claude \
  --runtime native \
  --auto-worktree \
  --yaks <yak-name> \
  $(echo $skill_flags) \
  "Your supervisor is $supervisor. When you start each yak, run:
   echo '$supervisor' | yx field <id> supervised-by

   Work on <yak-name>. Read its context with 'yx context --show <name>',
   do the work, then 'yx done <name>'.

   IMPORTANT: Push your branch when done:
   git push -u origin HEAD"
```

### Step 2: Wait for completion

Poll until the shaver finishes. Check every 2 minutes with a 30-minute
staleness timeout (15 consecutive polls with no status change):

```bash
# Read worktree path created by --auto-worktree
worktree_path=$(yx field --show <yak-name> worktree-path 2>/dev/null)

# Poll every 2 minutes, timeout after 30 minutes of no status change
last_status=""
stale_count=0
while true; do
  status=$(yx field --show <yak-name> agent-status 2>/dev/null)
  state=$(yx show <yak-name> --format json 2>/dev/null | jq -r '.state')

  # Check for completion
  if [[ "$status" == done:* ]] || [[ "$state" == "done" ]]; then break; fi
  if [[ "$status" == blocked:* ]]; then break; fi

  # Check for stale (no change in 30 min = 15 polls)
  if [ "$status" = "$last_status" ]; then
    stale_count=$((stale_count + 1))
    if [ "$stale_count" -ge 15 ]; then
      # Timeout — park the yak
      echo "blocked: shaver timed out (no status change in 30 min)" | yx field <yak-name> agent-status
      break
    fi
  else
    stale_count=0
    last_status="$status"
  fi

  sleep 120
done
```

A shaver is done when:
- `agent-status` starts with `done:` — shaver completed
- `agent-status` starts with `blocked:` — shaver got stuck
- The yak state shows `done` in `yx ls`

**If the shaver is blocked or timed out:**
- Park the yak: set `agent-status` to `blocked: shaver failed overnight`
- Append a note to the session yak's `comments.md`
- Skip to the next yak in the queue

### Step 3: SNIFF

Run an adversarial review on the completed work. The reviewer is a read-only
subagent (Agent tool, not yak-box spawn) — it inherits Yakob's auth, leaves
no stale sessions, and doesn't count against WIP.

First, collect the done yak's data:

```bash
context=$(yx context --show <yak-name>)
agent_status=$(yx field --show <yak-name> agent-status)
comments=$(yx field --show <yak-name> comments.md 2>/dev/null)
```

Mark under review:
```bash
echo "in-progress" | yx field <yak-name> review-status
```

Then launch a general-purpose subagent in the background with the adversarial
review prompt (see yak-sniff-test for the full template):

```
Agent tool call:
  subagent_type: "general-purpose"
  description: "Review <yak-name>"
  run_in_background: true
  prompt: |
    You are an adversarial reviewer for "<yak-name>".

    ## Original Brief
    $context

    ## Shaver's Done Summary
    $agent_status

    ## Shaver's Notes
    ${comments:-"No comments were left."}

    ## Your Task
    1. Check git log in the worktree for recent commits
    2. Verify each acceptance criterion against actual code
    3. Note which test commands should be run — do NOT run them yourself
    4. Report verdict: pass, fail, or needs-info with file/line evidence
```

When the subagent returns, **run the tests yourself** in the worktree:
```bash
cd "$worktree_path" && <test-command>  # e.g., go test ./..., cargo test, npm test
```

Record the verdict:
```bash
# On pass:
echo "pass: <summary>" | yx field <yak-name> review-status
echo "pass: <summary>" | yx field <yak-name> review-verdict

# On fail:
echo "fail: <summary>" | yx field <yak-name> review-status
echo "fail: <summary>" | yx field <yak-name> review-verdict
echo "<detailed findings>" | yx field <yak-name> review-notes
```

**On pass:** proceed to Step 4 (PR).

**On fail:** proceed to REMEDIATE (below).

### Step 4: PR

Create a pull request from the worktree branch.

```bash
# Find the worktree branch
worktree_branch=$(cd "$worktree_path" && git branch --show-current)

# Create PR (add --label "nightshift" if you've created that label in the repo)
gh pr create \
  --head "$worktree_branch" \
  --title "<yak-name>" \
  --body "$(cat <<'EOF'
## Summary

<paste context.md summary — what was asked>

## What was done

<paste agent-status — what was delivered>

## Review verdict

<paste review-verdict — sniff test result>

---

🌙 Generated by yak-nightshift
🐃 Shaved by <shaver-name>
EOF
)"

# Record the PR URL
echo "<pr-url>" | yx field <yak-name> pr-url
```

### Step 5: DONE

```bash
# Mark the yak done (if not already)
yx done <yak-name>

# Append to session yak's running log
existing=$(yx field --show "$session" comments.md 2>/dev/null)
cat <<EOF | yx field "$session" comments.md
${existing:+$existing

---

}### $(date '+%H:%M') — ✅ $yak_name
- **Shaved by:** $shaver_name
- **PR:** $pr_url
- **Sniff:** pass
EOF
```

Move to the next yak in the queue.

---

### REMEDIATE (on sniff failure)

When the sniff test fails, spawn a fresh shaver to fix the issues.

```bash
# Read the review findings
review_notes=$(yx field --show <yak-name> review-notes)

# Spawn a remediation shaver in the SAME worktree
yak-box spawn \
  --cwd "$worktree_path" \
  --yak-name "<yak-name>-fix" \
  --shaver-name "$shaver_name" \
  --tool claude \
  --runtime native \
  --yaks <yak-name> \
  $(echo $skill_flags) \
  "Your supervisor is $supervisor.

   This is a REMEDIATION pass. The original work on <yak-name> failed
   review. Fix the issues below, then push.

   ## Review Findings
   $review_notes

   ## Original Brief
   $(yx context --show <yak-name>)

   Fix the issues. Push when done. Mark the yak done with yx done <name>."
```

Wait for the remediation shaver to complete, then re-sniff.

- **Pass on re-sniff:** proceed to Step 4 (PR)
- **Fail on re-sniff:** park the yak

```bash
echo "blocked: failed sniff after remediation" | yx field <yak-name> agent-status

# Log to session yak
cat <<EOF | yx field "$session" comments.md
${existing:+$existing

---

}### $(date '+%H:%M') — 🅿️ $yak_name (parked)
- **Reason:** Failed sniff test after remediation
- **Findings:** $review_notes
EOF
```

Move to the next yak in the queue.

---

### Cancel the heartbeat

When the run loop exits — whether because the queue is empty, hard stop was
reached, or all remaining yaks are parked — cancel the heartbeat cron
**before** entering Phase 4. The heartbeat is only useful while shavers are
active; after the queue is exhausted it burns context window for no benefit.

```
CronDelete(id: <heartbeat_job_id>)
```

Use the job ID captured during Phase 2. If the ID is unavailable, call
`CronList` to find it (look for the `/loop` entry running `yx ls`) and delete
it. Do not proceed to Phase 4 without cancelling the heartbeat.

---

## Phase 4: Wrap

Triggered when:
- The queue is empty (all yaks processed)
- Hard stop is reached
- All remaining yaks are parked

### Morning summary

Before running yak-wrap, print the nightshift summary:

```
🌙 Nightshift Complete — YYYY-MM-DD

Shaved & PR'd:
  - yak-1 → PR #123 (https://...)
  - yak-2 → PR #124 (https://...)

Parked (needs human attention):
  - yak-3 — failed sniff after remediation
  - yak-4 — shaver blocked on missing test fixtures

Queue not reached (hard stop):
  - yak-5
  - yak-6

Duration: Xh Ym
Shavers spawned: N
```

### Run yak-wrap

Invoke `/yak-wrap` to harvest done yaks and generate the worklog.
The session yak's `comments.md` already has the running log from Phase 3 —
yak-wrap appends the formal summary.

### Worktree cleanup

Worktrees created by `--auto-worktree` persist after the session. For yaks
whose PRs have been merged, clean up with:
```bash
git worktree remove <worktree-path>
```

Worktrees for parked or unmerged yaks should be kept until the human reviews.

### Sync

```bash
yx sync
```

---

## Error Handling

| Scenario | Action |
|----------|--------|
| Shaver times out (no status change in 30+ min) | Park yak, move to next |
| Shaver reports `blocked:` | Park with reason, move to next |
| Sniff fails once | Remediation pass |
| Sniff fails twice | Park with findings, move to next |
| Hard stop mid-shave | Let current finish, skip rest, wrap |
| All yaks parked | Wrap early |
| `gh pr create` fails | Log error, still mark done, note missing PR |
| Worktree creation fails | Park yak, move to next |
| Heartbeat job ID lost | `CronList` to find it, then `CronDelete` |

## Quick Reference

| Phase | What | Trigger |
|-------|------|---------|
| Queue | Survey map, confirm order | Operator invokes `/yak-nightshift` |
| Commit | Create session yak, start heartbeat | Operator confirms queue |
| Run | Serial: shave → sniff → (remediate?) → PR → next | Automatic |
| Cancel heartbeat | `CronDelete` the heartbeat cron | Run loop exits (before Wrap) |
| Wrap | Worklog + sync + worktree cleanup | Queue empty or hard stop |

## Red Flags

- **Yaks with thin context** — will block the shaver with nobody to clarify. Skip them.
- **Yaks that share directories** — worktree conflicts. Don't queue together.
- **Strategic work in the queue** — nightshift is for grind work, not architecture decisions.
- **Skipping the sniff test** — "it's overnight, nobody's watching" is exactly when quality gates matter most.
- **Not pushing branches** — shavers must push so PRs can be created. Include in prompt.
- **Forgetting to cancel the heartbeat** — burns context window for hours after work is done. Always `CronDelete` before Phase 4.
- **Forgetting to sync** — morning session won't see nightshift state changes.
