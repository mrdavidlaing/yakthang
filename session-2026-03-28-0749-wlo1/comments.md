## Yak Wrap — 2026-03-28 12:35 (supervised by David Laing)

### Highlights

- Investigated Claude Code's official sandboxing model and @anthropic-ai/sandbox-runtime CLI — established that srt can wrap yak-box workers with OS-level filesystem + network isolation
- Renamed --runtime sandboxed to --runtime devcontainer throughout yak-box, freeing up "sandbox" for the lighter srt-based approach
- Broke down the full sandbox runtime rework into 5 implementation yaks with context; first wave of 3 shavers completed (pending review)

### Shaved Yaks

#### future and research
- **investigate sandboxing docs** — Researched Claude Code's native sandboxing. Key insight: native sandbox could protect .yaks/ but --dangerously-skip-permissions interaction needs investigation. Operator noted .yaks/ is just a projection of git notes, so protection isn't critical.

#### sandbox runtime rework
- **spike sandbox runtime CLI** — Fully documented srt CLI: invocation patterns, filesystem config (denyWrite > allowWrite asymmetry), network allowlists, platform differences (bubblewrap/Seatbelt), exit codes, config files, --control-fd dynamic updates. Identified 10 gotchas. Passed adversarial review against source repo.
- **rename sandboxed to devcontainer** — Pure rename across 21 files. --runtime sandboxed kept as deprecated alias with warning. All 16 Go test packages pass. Committed as 3b49c29. Passed adversarial review.

### Interesting Findings

- srt's denyWrite always takes precedence over allowWrite (unlike reads where allowRead > denyRead). This means you can't do "deny .yaks/ except .yaks/task/output" — but since .yaks/ is reconstructible from git notes, this doesn't matter.
- srt is already Bun-aware (has Bun-specific code paths, uses bun test). `bun build --compile` produces a ~100MB standalone binary in under a second — no Node runtime needed at deploy time.
- Linux has no glob support in srt paths (literal only), and mandatory deny only blocks existing files (bubblewrap limitation). macOS uses Seatbelt globs which handle creation too.
- --control-fd is the most interesting srt feature for future work: parent process can adjust sandbox permissions at runtime via JSON lines protocol.

### Loose Ends

- Three first-wave implementation yaks completed by shavers but not yet sniff-tested: submodule-and-compile-srt, accept-sandbox-flag, sandbox-preflight. Left at ready-for-sniff-test for next session.
- Two implementation yaks not yet started: generate-srt-config, srt-wrapper-integration
- remove-runtime-auto-detection is independent and ready to shave anytime
- .desloppify/ metadata has stale "sandboxed" references from the rename — cosmetic, will age out

### Remaining Yaks

See Phase 3 output for current map after pruning.
