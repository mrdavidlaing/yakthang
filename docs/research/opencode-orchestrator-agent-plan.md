# OpenCode Orchestrator Agent Plan

## Executive Summary

This plan proposes extracting the Yakob orchestrator personality and core orchestration instructions from CLAUDE.md into a dedicated OpenCode agent definition. This would leverage OpenCode's native agent system instead of relying on static markdown files.

## Research Summary

### OpenCode Agent System

OpenCode supports two types of custom agents:

1. **Primary agents** - Switchable via Tab key, handle main conversation
2. **Subagents** - Invokable via @mention or the Task tool, specialized tasks

**Agent definition locations:**
- Global: `~/.config/opencode/agents/`
- Project: `.opencode/agents/`
- Config: `opencode.json` (JSON format)

**Agent definition formats:**
- **Markdown** (recommended): Frontmatter + prompt content
- **JSON**: Full config in opencode.json

**Key agent properties:**
- `description` - When to use the agent (required)
- `mode` - "primary" or "subagent"
- `prompt` - System instructions (inline or file reference)
- `model` - Override the default model
- `temperature` - Control creativity (0.0-1.0)
- `steps` - Max agentic iterations
- `tools` - Enable/disable specific tools
- `permission` - Control tool permissions
- `color` - UI appearance
- `hidden` - Hide from autocomplete (subagents only)

### Current Yakob Architecture

**CLAUDE.md structure:**
- Yakob personality (calm shepherd of workers, dry yak puns)
- Core role: PLANNER and COORDINATOR, not implementer
- Architecture documentation (yx, spawn-worker.sh, .yaks/)
- Task management workflows
- Worker spawning patterns
- Monitoring and feedback protocols
- 7 core rules

**spawn-worker.sh mechanics:**
- Takes --mode flag (plan or build)
- Maps to: `opencode --prompt "$PROMPT" --agent ${MODE}`
- Injects random worker personality inline (Yakriel, Yakueline, Yakov, Yakira)
- Injects full yx instructions inline
- Creates temporary layout and wrapper script

**Design philosophy:**
- Sub-repos stay COMPLETELY clean (no orchestration files)
- All worker knowledge comes via inline prompts
- Workers are disposable, orchestrator is persistent

### Inspiration from agent-deck

**Conductor pattern:**
- Template-based CLAUDE.md with variable substitution (e.g., {PROFILE})
- Shared knowledge base + per-instance identity
- Persistent state management (state.json, task-log.md)
- Clear separation of concerns

## Proposed Design

### Option A: Yakob as Primary Agent (Recommended)

Create a custom primary agent named "yakob" that the user explicitly selects when launching orchestrator sessions.

**Implementation:**

```markdown
# ~/.config/opencode/agents/yakob.md

---
description: Orchestrates multi-agent workspaces via yx and spawn-worker.sh. Plans work, spawns workers, monitors progress. Never implements directly.
mode: primary
temperature: 0.3
tools:
  write: false    # Orchestrators don't write code
  edit: false     # Orchestrators don't edit code
  bash: true      # Needed for yx commands and spawn-worker.sh
permission:
  bash:
    "*": "ask"
    "yx *": "allow"
    "spawn-worker.sh *": "allow"
    "check-workers.sh *": "allow"
    "git status": "allow"
    "git add *": "allow"
    "git commit *": "allow"
color: "#8B7355"  # Yak brown
---

You are **Yakob** — a calm, methodical shepherd of workers. Your name is a play
on "yak," because someone has to keep all this yak-shaving organized. You speak
in short, clear sentences. You take pride in clean task breakdowns and
well-scoped workers. You occasionally make dry yak-related puns — sparingly,
like a good shepherd rations salt licks. When the herd wanders, you guide them
back. When a worker is blocked, you don't panic — you just move the fence.

[... rest of current CLAUDE.md content ...]
```

**Launch pattern:**

```bash
# In orchestrator.kdl, specify the agent explicitly
opencode --agent yakob
```

**Pros:**
- Clean separation: orchestrator role is explicit
- Users can switch away from Yakob (to build/plan) if needed
- Leverages OpenCode's native agent system
- Tool permissions enforce the "no implementation" rule
- Portable: ~/.config/opencode/agents/ works across projects

**Cons:**
- User must remember to use --agent yakob flag
- Slightly more setup (but only once globally)

### Option B: Modified AGENTS.md

Instead of a custom agent, create a project-specific AGENTS.md that includes the Yakob persona when in orchestrator mode.

**Implementation:**

Keep CLAUDE.md but make it conditional on session type:

```markdown
# CLAUDE.md

<!-- If launched with --cwd pointing to orchestrator root -->

You are **Yakob** — orchestrator for this multi-agent workspace...

[orchestration instructions]

<!-- Otherwise, standard project instructions -->
```

**Pros:**
- No new agent definition needed
- Works with existing OpenCode conventions
- CLAUDE.md already project-specific

**Cons:**
- Doesn't leverage OpenCode agent system
- No tool permission enforcement
- Conditional logic in CLAUDE.md is fragile
- Less portable

### Option C: Hybrid Approach

Yakob as a primary agent + minimal project CLAUDE.md with orchestrator-specific context.

**Implementation:**

1. `~/.config/opencode/agents/yakob.md` - Core personality and workflows
2. Project `CLAUDE.md` - Project-specific paths and conventions

```markdown
# yurt-workspace/CLAUDE.md

## Orchestrator Context

This workspace uses the Yakob orchestrator agent. Key locations:

- Task state: `.yaks/`
- Worker spawner: `spawn-worker.sh`
- Worker monitor: `check-workers.sh`
- Layout: `orchestrator.kdl`

See ~/.config/opencode/agents/yakob.md for core orchestration instructions.
```

**Pros:**
- Best of both worlds
- Agent system handles personality and permissions
- CLAUDE.md provides project-specific context
- DRY: orchestration knowledge lives in one place

**Cons:**
- Slight complexity: instructions split across two files
- Need to coordinate updates

## Recommended Approach: Option A (Yakob as Primary Agent)

**Rationale:**
1. **Explicit is better than implicit** - Orchestrator role is clearly defined
2. **Tool permissions enforce behavior** - Can't accidentally write code
3. **Portable** - Works across all orchestrated projects
4. **Leverage OpenCode's system** - Uses native agent capabilities
5. **Clean separation** - Workers remain separate via inline prompts

## Implementation Plan

### Phase 1: Create the Agent Definition

**File:** `~/.config/opencode/agents/yakob.md`

**Content structure:**
1. Frontmatter (mode, tools, permissions, temperature, color)
2. Yakob personality (shepherd, yak puns, calm methodical)
3. Role definition (PLANNER and COORDINATOR, not implementer)
4. Architecture documentation (yx, spawn-worker.sh, .yaks/)
5. Task management workflows (lifecycle, writing context)
6. Spawning workers (scoping, plan vs build modes)
7. Monitoring & feedback protocols
8. Core rules (7 rules from current CLAUDE.md)

**Tool configuration:**
```yaml
tools:
  write: false
  edit: false
  bash: true
  read: true
  glob: true
  grep: true
  todowrite: true
permission:
  bash:
    "*": "ask"
    "yx *": "allow"
    "spawn-worker.sh *": "allow"
    "check-workers.sh *": "allow"
    "git status": "allow"
    "git add *": "allow"
    "git commit *": "allow"
```

### Phase 2: Update orchestrator.kdl

Modify the Zellij layout to use the agent explicitly:

```kdl
pane size="67%" name="Yakob" focus=true {
    command "opencode"
    args "--agent" "yakob"
}
```

### Phase 3: Update Worker Spawning (No Changes Needed!)

**Key insight:** spawn-worker.sh already uses `--agent ${MODE}` where MODE is "plan" or "build". These are OpenCode's built-in agents. No changes needed!

The architecture already separates orchestrator from workers:
- Orchestrator: `opencode --agent yakob`
- Workers: `opencode --agent plan` or `opencode --agent build`

### Phase 4: Migrate CLAUDE.md Content

**What moves to yakob.md:**
- Yakob personality
- Orchestrator role ("PLANNER and COORDINATOR")
- Architecture overview
- Task management with yx
- Worker spawning instructions
- Monitoring protocols
- Core rules

**What stays in CLAUDE.md (if anything):**
- Project-specific context (optional)
- Custom conventions for this workspace
- Integration notes

**Alternative:** Delete CLAUDE.md entirely if all content moves to yakob.md

### Phase 5: Test the Migration

1. Launch orchestrator: `zellij --layout orchestrator.kdl`
2. Verify Yakob agent loads correctly
3. Test yx commands work (should be allowed)
4. Test spawn-worker.sh (should prompt for approval, then allow)
5. Verify workers spawn correctly with plan/build agents
6. Test git operations (should be allowed for status/commit)
7. Verify file editing is blocked (try to write a file, should fail)

### Phase 6: Documentation Updates

Update these files:
- `docs/plan-build-modes.md` - Note that orchestrator uses yakob agent
- `README.md` or setup docs - Mention the yakob agent requirement
- Add section about creating the agent on first setup

## Worker Personalities

**Current approach:** spawn-worker.sh randomly assigns personalities (Yakriel, Yakueline, Yakov, Yakira) inline.

**Recommendation:** KEEP THIS AS-IS.

**Rationale:**
1. Workers are ephemeral and disposable
2. Inline injection keeps sub-repos clean (core design principle)
3. Randomization adds variety and prevents "same agent" confusion
4. No benefit to making these formal agent definitions

**Alternative (not recommended):** Create 4 subagent definitions for each worker personality. This would:
- Add complexity
- Violate the "keep sub-repos clean" principle
- Make spawn-worker.sh more complex
- Provide minimal benefit

## Project-Specific Context

**Question:** What happens when different projects have different orchestration needs?

**Answer:** Use CLAUDE.md for project-specific conventions.

Example:

```markdown
# project-alpha/CLAUDE.md

## Orchestrator Notes for This Project

This is a Python monorepo with these sub-packages:
- `api/` - FastAPI backend
- `worker/` - Celery workers  
- `web/` - Next.js frontend

When spawning workers:
- Use `--cwd ./api` for backend tasks
- Use `--cwd ./web` for frontend tasks
- Integration tests need `--cwd .` for full repo access

The test suite requires Docker. Remind workers to start services via:
`docker compose up -d` before running tests.
```

**This keeps:**
- Yakob's core knowledge in the agent definition (portable)
- Project-specific context in CLAUDE.md (not portable, that's OK)

## Migration Path from Current Setup

**Current state:** CLAUDE.md contains everything

**Migration steps:**

1. **Create yakob.md** - Copy most of CLAUDE.md to `~/.config/opencode/agents/yakob.md`
2. **Strip CLAUDE.md** - Remove orchestration content, keep project-specific notes (if any)
3. **Update orchestrator.kdl** - Add `--agent yakob` flag
4. **Test** - Launch orchestrator, verify agent loads and works
5. **Iterate** - Adjust tool permissions if needed

**Backwards compatibility:** Old sessions using CLAUDE.md will still work until orchestrator.kdl is updated.

## Pros and Cons vs Current Approach

### Pros of Agent-Based Approach

1. **Explicit role definition** - "I'm Yakob the orchestrator" vs "I'm Claude reading CLAUDE.md"
2. **Permission enforcement** - Can't accidentally write code, enforced by OpenCode
3. **Portable** - Works across all orchestrated projects
4. **Discoverable** - Users can see available agents via Tab key
5. **OpenCode-native** - Leverages built-in capabilities
6. **Tool control** - Fine-grained bash command permissions
7. **Visual distinction** - Custom color in UI

### Cons of Agent-Based Approach

1. **Setup step** - User must create yakob.md in ~/.config/opencode/agents/
2. **Learning curve** - Slight: "use --agent yakob for orchestrator sessions"
3. **Documentation** - Need to explain the agent approach
4. **Not discoverable in TUI** - Can't switch to Yakob agent mid-session without restart

### Pros of Current CLAUDE.md Approach

1. **Simple** - Just one file in the repo
2. **Self-contained** - All instructions in the project
3. **No setup** - Works immediately

### Cons of Current CLAUDE.md Approach

1. **No permission enforcement** - Relies on prompt to prevent code edits
2. **Not portable** - Must copy CLAUDE.md to each project
3. **Invisible role** - No explicit "orchestrator mode"
4. **Tool access** - Can't restrict bash commands easily
5. **Duplication** - Yakob personality duplicated across projects

## Alternative: Keep CLAUDE.md + Add Tool Restrictions

**Middle ground:** Keep the current CLAUDE.md approach but add tool restrictions via project opencode.json:

```json
{
  "tools": {
    "write": false,
    "edit": false
  }
}
```

**Pros:**
- Simpler than full agent definition
- Some permission enforcement

**Cons:**
- No explicit orchestrator role/personality
- Not portable (must duplicate config + CLAUDE.md)
- Doesn't leverage agent system
- No visual distinction

## Recommendation Summary

**Implement Option A: Yakob as Primary Agent**

**Key actions:**
1. Create `~/.config/opencode/agents/yakob.md` with personality + instructions
2. Update `orchestrator.kdl` to use `--agent yakob`
3. Keep worker spawning as-is (inline personalities)
4. Optionally keep minimal project CLAUDE.md for project-specific context
5. Update documentation to explain the agent approach

**Benefits:**
- Explicit orchestrator role
- Permission enforcement prevents accidents
- Portable across projects
- Leverages OpenCode's native agent system
- Maintains clean sub-repo principle

**Trade-offs:**
- One-time setup to create the agent
- Instructions split across agent definition + project CLAUDE.md (if used)

## Open Questions for Review

1. **Agent scope:** Should Yakob be a global agent (~/.config) or per-project (.opencode/agents/)? 
   - **Recommendation:** Global, since orchestration patterns are reusable

2. **CLAUDE.md fate:** Keep a minimal project CLAUDE.md for context, or delete entirely?
   - **Recommendation:** Keep minimal version for project-specific notes

3. **Worker agents:** Should worker personalities become formal subagents?
   - **Recommendation:** No, keep inline injection for disposability

4. **Switching agents:** Should users be able to switch to build/plan from Yakob?
   - **Recommendation:** Yes, keep flexibility

5. **Mode flag:** Should spawn-worker.sh --mode plan/build stay as-is?
   - **Recommendation:** Yes, it already uses OpenCode's built-in agents

6. **Permission granularity:** Are the bash permissions too restrictive or too permissive?
   - **Recommendation:** Start conservative (ask for most, allow for yx/spawn/git), iterate based on usage

7. **Multi-profile support:** Should we support multiple orchestrator variants (e.g., yakob-strict, yakob-permissive)?
   - **Recommendation:** Start with one, add variants if needed

## Next Steps

1. **Review this plan** - Get feedback on the approach
2. **Prototype yakob.md** - Create the agent definition
3. **Test in isolation** - Launch with `opencode --agent yakob` outside orchestrator
4. **Integration test** - Update orchestrator.kdl and test full workflow
5. **Refine permissions** - Adjust based on real usage
6. **Document** - Update setup instructions
7. **Iterate** - Gather feedback and improve

---

**Status:** Plan ready for review
**Author:** Yakriel 🦬🪒
**Date:** 2026-02-13
