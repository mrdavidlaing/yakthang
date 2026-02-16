# Investigation: Matt Wynne's YAKS Project Skills

## Executive Summary

This report investigates the skills available in [mattwynne/yaks](https://github.com/mattwynne/yaks) for potential adoption in yakthang. The yaks project is a Rust-based DAG task tracker with extensive workflow skills developed by Matt Wynne.

**Key Finding:** Several skills are already partially adopted in `src/yaks/.claude/skills/` (the yaks CLI project), but there are opportunities to adopt additional skills into the yakthang orchestration layer at `.openclaw/workspace/skills/`.

---

## Skills Inventory

### 1. cqrs-event-sourcing

**Purpose:** Architecture guidance for event-driven systems (CQRS + Event Sourcing)

**What it provides:**
- Decision framework for aggregate boundaries
- Commands vs Events vs Queries distinction
- Read model / projection design
- Policies (reactors) and Sagas (process managers)
- Anti-patterns to avoid

**yakthang Relevance:** MEDIUM
- The yaks CLI project already references this skill in CLAUDE.md and has ADR 0002 documenting CQRS/ES adoption
- For yakthang orchestration, this is less relevant — our system doesn't require event sourcing
- Could be useful if we ever build event-driven worker communication

**Recommendation:** Already adopted in `src/yaks/` — no action needed for yakthang

---

### 2. incremental-tdd

**Purpose:** Test-Driven Development workflow — one failing test at a time

**What it provides:**
- The Iron Law: exactly one failing test at a time
- RED-GREEN-REFACTOR cycle steps
- Two stages of RED (compilation error → behavioral failure)
- ADR review integration during REFACTOR
- Common rationalizations for bad TDD

**yakthang Relevance:** HIGH
- Improves code quality for any implementation work
- Already present in `src/yaks/` but not exposed at orchestration level
- Workers implementing features should use this pattern

**Recommendation:** HIGH PRIORITY — Create skill at `.openclaw/workspace/skills/incremental-tdd/SKILL.md`

**Adaptation Notes:**
- Keep existing content mostly intact
- Remove references to specific yaks CLI commands if needed
- Emphasize this applies to any code the worker writes

---

### 3. preparing-a-yak

**Purpose:** Requirements gathering before implementation — prepares a yak with context, examples, and plan

**What it provides:**
- Three phases: Brainstorm → Example Map → Implementation Plan
- Stores outputs on yak using yx fields
- Integration with /brainstorming, /example-mapping, /writing-plans skills

**yakthang Relevance:** HIGH
- Directly applicable to orchestrator preparing work for workers
- Already present in `src/yaks/` but could be enhanced for orchestration

**Recommendation:** HIGH PRIORITY — Adapt for `.openclaw/workspace/skills/preparing-a-yak/SKILL.md`

**Adaptation Notes:**
- Rename to "preparing-work-for-workers" or keep "preparing-a-yak" if workers understand the terminology
- Replace references to yx with orchestrator field commands
- Add integration points with yak-box spawn commands

---

### 4. yak-worktree-workflow

**Purpose:** Git worktree-based isolation for parallel agent work

**What it provides:**
- Step-by-step workflow: check yaks → read context → create worktree → mark WIP → implement → demo → merge → cleanup
- Critical emphasis on reading context before starting
- Dogfooding warnings about .yaks directory

**yakthang Relevance:** MEDIUM (for solo worker) / LOW (for orchestration)
- yakthang uses Docker containers and Zellij sessions, not git worktrees
- Workers operate on sub-repos, not isolated worktrees within the same repo
- Pattern is similar but implementation differs

**Recommendation:** LOW PRIORITY — The workflow pattern is valuable but the git worktree implementation is specific to yaks CLI development

**Alternative:** Document the equivalent pattern for yakthang: workers spawn in separate Docker containers with their own git clones

---

### 5. discovery-tree-workflow

**Purpose:** Hierarchical task breakdown — just-in-time planning

**What it provides:**
- Start simple, break down only what's needed
- Emergent complexity handling
- Epic + Root Task + Subtasks structure
- Visual status tracking

**yakthang Relevance:** MEDIUM
- Very similar to how yakthang already uses yx with hierarchical tasks
- Could formalize as a skill for orchestrator planning

**Recommendation:** MEDIUM PRIORITY — Document existing yakthang patterns as a skill

---

### 6. mikado-method

**Purpose:** Discover refactoring dependencies through experiments

**What it provides:**
- Try naive change → run tests → identify blockers → revert → repeat
- Mikado graph visualization
- Documentation pattern for tracking experiments

**yakthang Relevance:** MEDIUM (for complex refactoring)
- Useful for large refactoring tasks
- Requires test suite to guide discovery

**Recommendation:** MEDIUM PRIORITY — Add as skill for complex refactoring scenarios

---

### 7. parallel-yak-implementation

**Purpose:** Dispatch multiple agents to work on independent leaf yaks concurrently

**What it provides:**
- Identify ready leaves → mark all WIP → dispatch agents → verify results
- Integration with dispatching-parallel-agents skill
- Common mistakes to avoid

**yakthang Relevance:** HIGH
- This is essentially what yakthang does! Orchestrator spawns multiple workers in parallel
- Already partially implemented in yak-box spawn

**Recommendation:** HIGH PRIORITY — Adapt as orchestration-level skill to formalize parallel worker dispatch pattern

**Adaptation Notes:**
- Replace "dispatching-parallel-agents" references with yakthang-specific commands
- Document how yak-box spawn creates parallel workers
- Add verification steps using yak-box check

---

### 8. yak-mapping

**Purpose:** Emergent planning through approaching goals and discovering blockers

**What it provides:**
- The Approach Pattern: add one yak → show map → add context → explore
- CRITICAL: show map after EVERY add
- Context pattern: Goal + Definition of Done + Known Knowns + Known Unknowns
- Integration with structuring-yak-dependencies

**yakthang Relevance:** HIGH
- Core planning pattern for orchestrator
- Already present in `src/yaks/` 

**Recommendation:** HIGH PRIORITY — Ensure this skill is available at orchestration level

---

### 9. structuring-yak-dependencies

**Purpose:** Model emergent prerequisites through parent-child nesting

**What it provides:**
- Parent-child nesting enforces order (parent can't be done until children complete)
- Work deepest-first (leaves before parents)
- Reorganizing flat yaks into hierarchy

**yakthang Relevance:** HIGH
- Direct fit for task dependency management in yakthang
- Already present in `src/yaks/`

**Recommendation:** Already adopted — ensure orchestrator uses this pattern

---

### 10. shellspec

**Purpose:** Shell testing framework (external tool, not a skill)

**Status:** Not applicable to yakthang

---

## Summary Table

| Skill | Current Status | Recommendation | Priority |
|-------|----------------|----------------|----------|
| cqrs-event-sourcing | In `src/yaks/` | Already adopted | None |
| incremental-tdd | In `src/yaks/` | Create at orchestration level | HIGH |
| preparing-a-yak | In `src/yaks/` | Adapt for orchestration | HIGH |
| yak-worktree-workflow | In `src/yaks/` | Not applicable (different isolation model) | LOW |
| discovery-tree-workflow | In `src/yaks/` | Formalize existing pattern | MEDIUM |
| mikado-method | In `src/yaks/` | Add for complex refactoring | MEDIUM |
| parallel-yak-implementation | In `src/yaks/` | Adapt for orchestration | HIGH |
| yak-mapping | In `src/yaks/` | Ensure available at orchestration level | HIGH |
| structuring-yak-dependencies | In `src/yaks/` | Already adopted | None |

---

## Recommended Adoption Plan

### Phase 1: High Priority Skills

1. **incremental-tdd** — Copy to `.openclaw/workspace/skills/incremental-tdd/SKILL.md`
   - Minor adaptations: remove yaks CLI-specific commands
   - Add reference to test commands available in worker environments

2. **preparing-a-yak** — Adapt for orchestrator at `.openclaw/workspace/skills/preparing-a-yak/SKILL.md`
   - Emphasize: orchestrator prepares work, workers execute
   - Integrate with yak-box spawn workflow

3. **parallel-yak-implementation** — Formalize at `.openclaw/workspace/skills/parallel-yak-implementation/SKILL.md`
   - Document yak-box spawn --parallel flag or equivalent
   - Add verification using yak-box check

4. **yak-mapping** — Ensure exists at `.openclaw/workspace/skills/yak-mapping/SKILL.md`
   - Verify integration with orchestration workflow

### Phase 2: Medium Priority Skills

5. **discovery-tree-workflow** — Document as `.openclaw/workspace/skills/discovery-tree-workflow/SKILL.md`
   - Focus on orchestrator planning patterns

6. **mikado-method** — Add as `.opencloak/workspace/skills/mikado-method/SKILL.md`
   - For large refactoring tasks

### Phase 3: Documentation

7. Update `.openclaw/workspace/AGENTS.md` to reference adopted skills

---

## Architecture Considerations

### Where Skills Should Live

| Skill Type | Location |
|------------|----------|
| Orchestrator planning | `.openclaw/workspace/skills/` |
| Worker execution | Per-project or shared in worker template |
| yaks CLI specific | `src/yaks/.claude/skills/` (already present) |

### Integration with Docker/Zellij Setup

The skills assume git worktrees for isolation. yakthang uses:
- **Docker containers** for worker isolation
- **Zellij sessions** for terminal management

Skills that reference worktrees should be adapted to reference:
- Docker container lifecycle
- Zellij session management
- yak-box spawn/stop commands

---

## Conclusion

Matt Wynne's yaks project has a mature set of workflow skills that align well with yakthang's orchestration needs. The key opportunities are:

1. **Adopt incremental-tdd** at the orchestration level to ensure workers follow TDD
2. **Adapt preparing-a-yak** for the orchestrator-to-worker workflow
3. **Formalize parallel-yak-implementation** around yak-box spawn capabilities
4. **Ensure yak-mapping** is available for orchestrator planning

The existing skills in `src/yaks/` provide a good foundation, but should be mirrored (with adaptations) to `.openclaw/workspace/skills/` for orchestration-level use.

---

## Appendix: Current Skill Locations

```
yakthang/
├── .openclaw/workspace/skills/
│   └── zellij/SKILL.md          # Already adopted
├── src/yaks/.claude/skills/     # yaks CLI skills (reference)
│   ├── cqrs-event-sourcing/
│   ├── discovery-tree-workflow/
│   ├── incremental-tdd/
│   ├── mikado-method/
│   ├── parallel-yak-implementation/
│   ├── preparing-a-yak/
│   ├── shellspec/
│   ├── structuring-yak-dependencies/
│   ├── yak-mapping/
│   └── yak-worktree-workflow/
```

---

*Report generated: 2026-02-16*
*Task: yakthang-v2/investigate-mattwynne-skills*
---

## Update: workgraph Connection

**Source:** Matt Wynne mentioned (Feb 2026) he's taking inspiration from https://github.com/graphwork/workgraph for yaks.

This provides additional context for understanding the conceptual foundation of his skills. The workgraph project may inform:
- Graph-based task representation
- Workflow modeling patterns
- Task dependency management

See `curate-skills/investigate-workgraph` for deeper investigation into this connection.
