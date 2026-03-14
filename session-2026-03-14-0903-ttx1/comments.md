## Yak Wrap — 2026-03-14 11:55 (supervised by David Laing)

### Highlights

- **Completed the yak-map quality journey**: Decomposed a 1,310-line monolith (main.rs) into 6 focused Rust modules with clear dependency boundaries, reducing max cognitive complexity from 28 to well below threshold.
- **Codified architectural conventions into a skill**: The patterns learned during refactoring are now embedded in `.claude/skills/yak-map-architecture/SKILL.md` so future agents know where to put code, how to test, and *why* the patterns exist.
- **Fixed yak-box stop assignment cleanup**: Resolved the persistent bug where `yak-box stop` couldn't find task directories to clear `assigned-to`, using stored TaskDirs at spawn time + `.name` file fallback.

### Shaved Yaks

#### Quick Wins (4 yaks)
- **task state from str** — Added `FromStr` impl to `TaskState` enum, replacing inline magic string matching with `.parse().ok()`.
- **agent status kind enum** — Created `AgentStatusKind` enum with `from_status_string()`, refactored `task_color()` and `status_symbol()` to exhaustive match.
- **review status kind enum** — Created `ReviewStatusKind` enum with `from_status_string()`, refactored `review_status_emoji()`.
- **extract utilities module** — Moved 5 utility functions to `util.rs` (escape, clipboard, base64, ANSI strip, display width).

#### Structural Refactors (4 yaks + 1 fix)
- **extract rendering module** — 6 rendering functions moved to `render.rs` with 22 tests.
- **extract tree builder** — Tree-building logic into `tree.rs`; `refresh_tasks()` delegates to `tree::build()`.
- **introduce task source trait** — `TaskSource` trait with `TaskRepository` (filesystem) and `InMemoryTaskSource` (tests), contract test macro for behavioral parity.
- **split event handler** — `update()` reduced to 10-line dispatcher routing to `handle_*` methods. Required a fix sub-yak to extract `handle_key()` and rename inconsistent methods.

#### Module Split (1 yak)
- **multi-file module split** — Final extraction of `model.rs` (domain types, ansi constants) and `repository.rs` (TaskSource trait, both impls, contract tests). 51 tests pass.

#### Architecture Skill (1 yak)
- **write yak-map architecture skill** — Created SKILL.md covering all 11 conventions with rationale + deviation guidance. Passed adversarial review with 4 cross-references verified.

#### Side Quest: yak-box fix (1 yak)
- **fix yak-box stop assignment cleanup** — Two-pronged fix: store resolved TaskDirs in session at spawn time, add `.name` file search as fallback in `resolveYakValue`.

### Interesting Findings

- The **ports-and-adapters pattern** (TaskSource trait) made the monolith split possible — once State was parameterized on the trait, modules could be extracted independently without breaking tests.
- The **contract test macro** (`task_source_tests!`) is a surprisingly effective pattern: one macro, invoked in both `InMemoryTaskSource` and filesystem test modules, guarantees behavioral parity without duplication.
- **ADRs vs skills debate**: Decided to embed architectural rationale directly in skills (convention + rationale + when to deviate) rather than maintaining separate ADRs. The reasoning lives where the guidance lives.
- The **yak-box stop bug** was caused by worker `--yak-name` not matching the filesystem slug — a naming mismatch that only surfaced during cleanup.

### Loose Ends

- `refactor-agent-status-rendering` — Deferred deliberately. The current render.rs works but could be cleaner with the new enum patterns.
- `extract-tree-builder-explainer.html` — Interactive playground file was created in repo root during session. May want to move or clean up.
- Session had a WIP limit bump from 1→2 for the yak-box side quest. Worked well for independent codebases (Rust + Go).

### Session Stats

- **12 yaks shaved** (10 yak-map quality + 1 fix re-shave + 1 yak-box fix)
- **Shavers used**: Yakira, Yakoff, Yakueline
- **All adversarial reviews passed**
- main.rs: 1,310 lines → 276 lines (79% reduction)
- Test count: 51 tests, all passing
- Quality gate: bin/dev check green throughout
