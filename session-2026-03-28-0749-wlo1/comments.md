## Yak Wrap — 2026-03-28 12:35 (supervised by David Laing)

### Highlights

- Investigated Claude Code's official sandboxing model and @anthropic-ai/sandbox-runtime CLI — established that srt can wrap yak-box workers with OS-level filesystem + network isolation
- Renamed --runtime sandboxed to --runtime devcontainer throughout yak-box, freeing up "sandbox" for the lighter srt-based approach
- Broke down the full sandbox runtime rework into 5 implementation yaks with context; first wave of 3 shavers completed (pending review)

---

## Yak Wrap — 2026-03-28 (supervised by David Laing)

### Highlights

- Built a complete --runtime sandbox mode for yak-box: submodule, compile, config, wrapper, preflight, pre-auth, smoke tests — from zero to working end-to-end in one session
- Fixed yak-map panic on Zellij 0.44 and added git-notes-hash change detection (40 bytes per tick instead of full directory walk)
- Removed --runtime auto — runtime is now an explicit, required choice by the orchestrator
- Attempted --permission-mode auto but discovered it requires Team/Enterprise plan; cleanly reverted

### Shaved Yaks

#### sandbox runtime rework (14 yaks)
- **rename sandboxed to devcontainer** — Pure rename across 21 files. Deprecated alias with warning. Committed 3b49c29.
- **submodule and compile srt** — Added @anthropic-ai/sandbox-runtime as git submodule. bun build --compile produces standalone ~100MB binary. Justfile targets, Dockerfile updated.
- **accept sandbox flag** — Added "sandbox" to valid --runtime values with stub SpawnSandboxWorker/StopSandboxWorker.
- **sandbox preflight** — SpawnSandboxDeps checks srt + platform deps (bwrap/socat on Linux, sandbox-exec on macOS).
- **generate srt config** — GenerateSrtConfig() builds hardcoded JSON config: allowWrite=[cwd, /tmp], denyRead=[~/.ssh, ~/.aws/credentials], 13 allowed domains, allowLocalBinding=true.
- **srt wrapper integration** — SpawnSandboxWorker wraps tool commands with srt --settings. Follows native pattern with functional options. Config path persisted for cleanup.
- **fix sandbox auth routing** — Sandbox workers now skip auth preflight (same as native, since they run on the host).
- **fix srt config for oauth** — Added allowLocalBinding:true and OAuth domains (anthropic.com, console.anthropic.com, claude.ai).
- **sandbox pre-auth flow** — Copies host ~/.claude/ OAuth credentials into worker's CLAUDE_CONFIG_DIR before srt wraps. Fixes bwrap network namespace isolation breaking OAuth callback.
- **sandbox smoke test** — 6 shellspec integration tests exercising yak-box's config generation, wrapper scripts, filesystem isolation, and cleanup. First version failed review (tested srt directly); rewritten as proper yak-box integration tests.
- **remove runtime auto detection** — --runtime is now required. DetectRuntime() deleted. Sandboxed deprecated alias removed (clean break).
- **switch to auto permission mode** — Implemented correctly but reverted (requires Team/Enterprise plan).
- **revert auto permission mode** — Back to --dangerously-skip-permissions. Re-investigate in May 2026.
- **default to sandbox runtime for spawns** — Yakob instructions updated to default --runtime sandbox for implementation shavers, native for research only.

#### improve yak-map (3 yaks)
- **upgrade zellij-tile to 0.44** — Fixed FloatingPaneCoordinates API change causing panic. Replaced unsafe unwrap() with if-let.
- **add filesystem change detection** — Timer reads .git/refs/notes/yaks commit hash (~40 bytes) per tick. Skips full tree rebuild when unchanged. 'r' forces refresh.
- **unit tests for change detection** — 6 tests covering timer skip/rebuild, refresh bypass, missing file edge cases. 76 total tests.

#### other
- **fix yak-mapping skill runtime reference** — Updated SKILL.md from --runtime native to --runtime sandbox.
- **investigate sandboxing docs** — Research on Claude Code native sandboxing model.
- **spike sandbox runtime CLI** — Full documentation of srt CLI: filesystem config, network config, platform differences, 10 gotchas.

### Interesting Findings

- srt's denyWrite always takes precedence over allowWrite (unlike reads). Can't do "deny .yaks/ except subdirs" — but .yaks/ is reconstructible from git notes, so doesn't matter.
- bun build --compile produces a standalone srt binary in <1 second. The project is already Bun-aware with Bun-specific code paths.
- bubblewrap creates a separate network namespace, so localhost inside the sandbox != host localhost. OAuth callbacks can't reach the sandboxed process. Solution: copy host credentials before wrapping.
- --permission-mode auto (the safer alternative to --dangerously-skip-permissions) requires Team/Enterprise plan. Not available on Claude Max personal subscription.
- git notes hash change detection for yak-map is elegant: one 40-byte read per tick catches all yx operations since every yx command commits to refs/notes/yaks.

### Loose Ends

- Smoke test has no runtime network filtering test (verifies config correctness but doesn't curl through srt)
- .desloppify/ metadata has stale "sandboxed" references — cosmetic
- GenerateSrtConfig doesn't validate empty or relative CWD paths
- StopSandboxWorker passes empty sessionName for Zellij tab close — best-effort, may miss tabs launched with explicit session names

### Remaining Yaks

See Phase 3 output for current map after pruning.
