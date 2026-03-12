## Findings Summary

Wrote a prioritized list of 12 improvements to the yak context, organized as:
- 5 Quick Wins (QW-1 through QW-5): ~2 hours total
- 4 Medium Refactors (MR-1 through MR-4): ~5-7 hours total
- 3 Larger Refactors (LR-1 through LR-3): ~1.5 days total

Top recommendation: do QW-1,2,4,5 as one commit, then tackle MR-2 (tree builder extraction from the 90-line refresh_tasks method) as the single biggest readability win.

## Additional Patterns from Matt's Tour (David's feedback)

David flagged 4 additional yx patterns worth studying for yak-map:

### 1. Inner Loop Unit Tests
yx uses #[test] modules throughout with fast assertions (~1s). yak-map already has good test coverage (52% of file), but tests are all in one massive test block at the end of main.rs. Extracting modules (MR-1, MR-2) would naturally co-locate tests with the code they exercise.

Also notable: yx's contract test pattern (yak_store_tests! macro) runs identical tests against InMemoryStorage and DirectoryStorage — ensuring adapter swaps don't change behavior. This is exactly what MR-4 (TaskRepository trait) would enable for yak-map.

### 2. Outer Loop Cucumber Specs
yx has 27 .feature files with a clever dual-world runner: InProcessWorld (fast, ~1s) and FullStackWorld (spawns binary, ~39s). Both implement a TestWorld trait so step definitions are written once.

yak-map doesn't have behavioral specs. Given it's a Zellij WASM plugin, cucumber may not apply directly — but the *pattern* of specifying behavior as human-readable scenarios before implementing is valuable. The preparing-a-yak skill (below) could generate these.

### 3. Preparing-a-Yak Skill
Located at src/yaks/skills/preparing-a-yak/SKILL.md. A structured interview process that takes a vague yak idea through: brainstorming → example mapping (for features) or ADR (for refactorings) → sub-yak planning. Outputs stored via yx context and yx field.

This skill is directly applicable to *how* yak-map improvements get specced before implementation. Each MR/LR item in the plan could go through this preparation flow.

### 4. bin/dev Quality Checks
src/yaks/bin/dev (978 lines) is a comprehensive quality gate:
- `dev lint`: clippy + rustfmt + shellcheck + dead step detection
- `dev cx`: cognitive complexity ≤ 42, cyclomatic ≤ 34 (uses rust-code-analysis-cli)
- `dev mutate-diff`: mutation testing on changed files only (~seconds)
- `dev check`: runs ALL checks in sequence (build → shellspec → tests → cucumber → lint → complexity → audit → mutants)

yak-map has no equivalent. Creating a bin/dev (or adding yak-map to the existing one) would systematically enforce quality as improvements land. The complexity thresholds alone would flag the 90-line refresh_tasks() and 73-line update() methods.

## Surprise

The yx codebase has mutation testing infrastructure that auto-generates yaks for missed mutants via `dev mutate-sync`. This is a self-improving quality loop — the tool creates its own improvement tasks.
