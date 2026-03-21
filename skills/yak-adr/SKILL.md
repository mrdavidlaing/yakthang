---
name: yak-adr
description: Architecture Decision Records. Create, manage, and cross-reference ADRs — no external CLI needed.
allowed-tools: Read, Write, Edit, Grep, Glob
argument-hint: <create|supersede|amend|update-index|list> [title or ADR number]
---

# Architecture Decision Records

User instructions: $ARGUMENTS

## Argument Routing

Parse `$ARGUMENTS` to determine which operation to perform:

- **`create "Title"`** or just a title string — create a new ADR
- **`supersede NNNN "New Title"`** — create a new ADR that supersedes #NNNN
- **`amend NNNN "New Title"`** — create a new ADR that amends #NNNN
- **`update-index`** — regenerate the README.md index table
- **`list`** — show all existing ADRs and their statuses

If `$ARGUMENTS` is empty or unclear, ask the user what they want to do.

---

## When to Write an ADR

Write an ADR when making decisions that:
- Change the architecture or core design patterns
- Introduce new dependencies or technologies
- Affect multiple components or the public API
- Have long-term maintenance implications
- Future maintainers will ask "why did we do it this way?"

**Not for:** minor implementation details, bug fixes, refactoring, configuration changes.

If the user's request doesn't clearly warrant an ADR, mention this guidance before proceeding. Don't refuse — the user may have good reasons.

---

## Conventions

| Convention | Value |
|-----------|-------|
| ADR location | `docs/adr/` relative to repository root |
| Filename format | `NNNN-kebab-case-title.md` (4-digit zero-padded) |
| Index file | `docs/adr/README.md` with markdown table |
| New ADR status | `proposed` |
| Date format | `YYYY-MM-DD` |

**Supported statuses:** proposed, accepted, rejected, superseded, amended, deprecated

---

## Process: Determine Next ADR Number

1. Use Glob to find existing ADR files: pattern `docs/adr/[0-9][0-9][0-9][0-9]-*.md`
2. If no files exist, the next number is `0001`
3. Otherwise, extract the number from the last file (sorted), increment by 1, zero-pad to 4 digits

## Process: Directory Bootstrap

Before any operation, check if `docs/adr/` exists. If not:

1. Create `docs/adr/` directory
2. Create `docs/adr/README.md` with the initial template (see Operation: Update Index for the full template)
3. Inform the user: "Created docs/adr/ directory and initial README.md"

## Process: Convert Title to Kebab-Case

1. Lowercase all characters
2. Replace spaces and special characters with hyphens
3. Remove consecutive hyphens
4. Strip leading/trailing hyphens
5. Examples:
   - "Adopt CQRS and Event Sourcing" → `adopt-cqrs-and-event-sourcing`
   - "Keep main.rs Thin" → `keep-main-rs-thin`
   - "UUID for Migration Events" → `uuid-for-migration-events`

---

## ADR Template

When creating a new ADR, write the file with this exact structure:

```markdown
# NNNN. Title of the Decision

Date: YYYY-MM-DD

## Status

proposed

## Context

[Describe the issue, forces at play, and what motivates this decision.
Include enough background that a reader unfamiliar with the project
can understand why this decision matters.]

## Decision

[State the decision clearly. Use active voice: "We will..." or
"Use X for Y because Z."]

### Relation to other ADRs

None.

## Consequences

### Benefits

[What becomes easier or better because of this decision.]

### Trade-offs

[What becomes harder or more constrained. Be honest.]

### Future considerations

[Open questions, things to watch for, conditions under which
this decision should be revisited.]
```

---

## Operation: Create

1. Bootstrap the directory if needed (see Process: Directory Bootstrap)
2. Determine the next ADR number (see Process: Determine Next ADR Number)
3. Convert the title to kebab-case (see Process: Convert Title to Kebab-Case)
4. Create the file at `docs/adr/NNNN-kebab-case-title.md` using the ADR Template
5. Fill in:
   - Title as `NNNN. Title` (preserving the user's original casing)
   - Date as today's date
   - Status as `proposed`
   - If the user provided context about the decision, fill in Context, Decision, and Consequences sections with that content
   - Otherwise, leave the placeholder text for the user to fill in
6. Update the index (see Operation: Update Index)
7. Report what was created:
   ```
   Created: docs/adr/NNNN-kebab-case-title.md
   Status: proposed

   Next steps:
   - Fill in Context, Decision, and Consequences (if not already done)
   - Change status to "accepted" once the decision is agreed upon
   - Commit the ADR with the related code changes
   ```

---

## Operation: Supersede

When a new decision replaces an older one entirely:

1. Read the old ADR file (`docs/adr/NNNN-*.md`) to confirm it exists and get its title
2. Create the new ADR (follow Operation: Create steps 1–5)
3. In the **new** ADR's Status section, write: `accepted (supersedes ADR NNNN)`
4. In the **new** ADR's "Relation to other ADRs" subsection, explain what changed and why the old decision is being replaced
5. Update the **old** ADR's Status section to: `superseded by ADR MMMM — Short Title of New ADR`
   - Do NOT change anything else in the old ADR
6. Update the index (see Operation: Update Index)
7. Report both files modified:
   ```
   Created: docs/adr/MMMM-new-title.md (supersedes ADR NNNN)
   Updated: docs/adr/NNNN-old-title.md → status: superseded

   Both ADRs cross-reference each other.
   ```

---

## Operation: Amend

When a new decision modifies (but does not replace) an older one:

1. Read the old ADR file (`docs/adr/NNNN-*.md`) to confirm it exists and get its title
2. Create the new ADR (follow Operation: Create steps 1–5)
3. In the **new** ADR's Status section, write: `accepted (amends ADR NNNN)`
4. In the **new** ADR's "Relation to other ADRs" subsection, explain what is being modified and why
5. Update the **old** ADR's Status section by appending on a new line: `amended by ADR MMMM — Short Title of New ADR`
   - Keep the old ADR's existing status on its own line (e.g., `accepted` remains)
   - Example result:
     ```
     accepted
     amended by ADR MMMM — Short Title of New ADR
     ```
   - Do NOT change anything else in the old ADR
6. Update the index (see Operation: Update Index)
7. Report both files modified:
   ```
   Created: docs/adr/MMMM-new-title.md (amends ADR NNNN)
   Updated: docs/adr/NNNN-old-title.md → status: amended

   Both ADRs cross-reference each other.
   ```

---

## Operation: Update Index

Regenerate `docs/adr/README.md` from the current ADR files:

1. Find all ADR files matching `docs/adr/[0-9][0-9][0-9][0-9]-*.md` using Glob
2. For each file, extract:
   - **Number**: first 4 digits of the filename
   - **Title**: from the first `# ` heading, stripping the `NNNN. ` prefix
   - **Status**: the first non-empty line after `## Status` — use only the **first word** for the index table (e.g., `superseded` not `superseded by ADR 0019 — Title`)
3. Sort rows by ADR number ascending
4. Write `docs/adr/README.md` with this structure:

```markdown
# Architecture Decision Records

ADRs document significant architectural and design decisions.

## Index

| # | Decision | Status |
|---|----------|--------|
| [NNNN](NNNN-filename.md) | Title | status |

## When to Write an ADR

Write an ADR when making decisions that:
- Change the architecture or core design patterns
- Introduce new dependencies or technologies
- Affect multiple components or the public API
- Have long-term maintenance implications
- Future maintainers will ask "why did we do it this way?"

Not for: minor implementation details, bug fixes, refactoring,
configuration changes.
```

---

## Operation: List

1. Read `docs/adr/README.md` if it exists and display the Index table
2. If no README.md exists, scan for ADR files and display a summary table
3. If no ADR files exist, report: "No ADRs found in docs/adr/"

---

## Quick Reference

| Operation | Invocation | What it does |
|-----------|-----------|--------------|
| Create | `/yak-adr create "Title"` | New ADR with next sequential number |
| Supersede | `/yak-adr supersede 0015 "New approach"` | New ADR, marks old as superseded |
| Amend | `/yak-adr amend 0012 "Refinement"` | New ADR, marks old as amended |
| Update index | `/yak-adr update-index` | Regenerate README.md table |
| List | `/yak-adr list` | Show all ADRs and statuses |

## Status Lifecycle

| Status | Meaning | How it's set |
|--------|---------|-------------|
| proposed | Under discussion, not yet accepted | Default for new ADRs |
| accepted | Decision is in effect | User manually changes from proposed |
| rejected | Considered but not adopted | User manually changes from proposed |
| superseded | Replaced by a newer ADR | Set by supersede operation |
| amended | Modified by a newer ADR (still partially in effect) | Set by amend operation |
| deprecated | No longer relevant but not replaced | User manually sets |
