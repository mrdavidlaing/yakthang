# Desloppify Triage — yak-box

Scan date: 2026-03-09
Objective score: 96.3/100
Strict score: 38.5/100 (low because 20 subjective dimensions unassessed)

## Mechanical issues: 43 open

### Fix now (high signal, low effort)

**golangci-lint: unused code** (Tier 2, 4 items)
- `internal/runtime/helpers.go:256` — `resolveAnthropicKeySuffix` unused function
- `internal/runtime/sandboxed_test.go:20,30,31,44` — unused fields (`responses`, `output`, `err`) and `getCall` method in `TestCommander`

These are dead code; safe to delete. Keeps the codebase clean.

**golangci-lint: gosimple** (Tier 2, 1 item)
- `internal/preflight/preflight.go:166` — `fmt.Sprintf` with no format verbs; use string literal instead

Quick one-liner fix.

### Fix later (high effort or low signal)

**golangci-lint: errcheck** (Tier 2, ~29 items)
Most are in test files (`os.MkdirAll`, `os.Chmod`, `os.Chdir` without checking errors).
A handful are in production code (`filepath.Walk`, `MarkFlagRequired`, `io.Copy`, etc.).

Assessment: the test file violations are low-risk (test infra failures are immediately visible).
The production `MarkFlagRequired` calls are cobra wiring that only fail if called on non-existent flags
— which would be caught by tests. The `filepath.Walk` and `io.Copy` cases are real but low severity.

Recommended: batch-fix in a dedicated yak; don't rush.

**Structural: large files** (Tier 3, 5 items)
- `cmd/spawn.go` — 730 LOC, complexity 98 (only PRODUCTION large file)
- `cmd/spawn_test.go`, `internal/runtime/helpers_test.go`, `internal/runtime/sandboxed_test.go`, `internal/sessions/sessions_test.go` — large test files

The test files are large because they cover complex integration scenarios; splitting them would
require extracting test helpers. Not urgent. `cmd/spawn.go` is the real debt — complexity 98
warrants eventual decomposition into sub-commands or a `spawner` package.

**Test coverage: untested modules** (Tier 3, 3 items)
- `main.go` — 17 LOC entrypoint; acceptable, integration tests cover it
- `pkg/devcontainer/build.go` — 108 LOC; Docker build logic; hard to unit test
- `pkg/devcontainer/lifecycle.go` — 164 LOC; container lifecycle; hard to unit test

Both devcontainer files are infrastructure wrappers around Docker. Unit testing them requires
mocking the Docker daemon. Worth doing eventually; not blocking.

## Subjective review: 34 issues (unassessed)

The strict score will remain at 38.5 until the 20 subjective dimensions are reviewed.
Running the full subjective review requires 20 LLM subagent calls.

To run it:
```
desloppify review --run-batches --dry-run  # generates prompts
# then launch 20 subagents, one per prompt in .desloppify/subagents/runs/<run>/prompts/
desloppify review --import-run <run-dir> --scan-after-import
```

Biggest weighted drags (from score recipe):
- High elegance: -10.73 pts (17.9% of subjective pool)
- Mid elegance: -10.73 pts (17.9% of subjective pool)
- Type safety: -5.85 pts (9.8%)
- Contracts: -5.85 pts (9.8%)
- Low elegance: -5.85 pts (9.8%)

## Areas with most structural debt

1. `internal/runtime` — 8 issues (T3:2, T4:6)
2. `pkg/devcontainer` — 8 issues (T3:2, T4:6)
3. `internal/sessions` — 3 issues (T3:1, T4:2)

These are the two hardest areas to test and also the most complex.
