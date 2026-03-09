Configured desloppify for yak-box Go codebase. Two commits landed on main.

Commit 1 — setup:
- Installed golangci-lint v1.64.8; added to .devcontainer/Dockerfile
- Fixed root cause of parser_error: nolint comment in cmd/check.go used em dash instead of // as reason separator, causing stderr output that broke desloppify JSON parsing
- Committed initial scan state (96.3% objective, 43 mechanical issues)
- Added .desloppify/.gitignore to exclude transient artifacts

Commit 2 — subjective review:
- Ran 20 holistic review batches in parallel via subagents
- Strict score: 38.5 → 70.6/100 (+32.1 pts), 68 new issues tracked

Key findings worth acting on:
- BUG: messageCmd is implemented and tested but never registered (rootCmd.AddCommand missing) — completely unreachable
- BUG: opencode spawn passes wrong variable as --agent arg (silent wrong behavior)
- SpawnNativeWorker silently swallows setup errors
- OAuth detection duplicated independently in runtime + preflight packages
- passwd/group strings copy-pasted between cmd/auth.go and helpers.go
- runSpawn (730 LOC, complexity 98) has zero test coverage for its orchestration path
- Runtime/Tool/Mode values are untyped strings — needs const types
- pkg/types and pkg/worktree should be under internal/
