## CLAUDE_CONFIG_DIR exists — fix implemented

### Finding
`CLAUDE_CONFIG_DIR` env var is supported by Claude Code (confirmed in binary v2.1.77).
When set, it redirects the `~/.claude/` config directory and `.claude.json` state file,
without touching `HOME`. The binary logic:
- Config dir: `CLAUDE_CONFIG_DIR ?? path.join(homedir(), ".claude")`
- State file: `path.join(CLAUDE_CONFIG_DIR || homedir(), ".claude.json")`

### What changed

**`src/yak-box/internal/runtime/native.go`**

`generateNativeWrapperScript` (claude case):
- Removed `export HOME=<workerDir>` — HOME is now the real user home
- Removed `GIT_CONFIG_GLOBAL`, `GH_CONFIG_DIR` exports (no longer needed)
- Removed git identity exports `resolveGitIdentityExports()` (no longer needed)
- Removed macOS keychain workaround (security create/unlock/set-default-keychain)
- Removed PATH pinning (`~/.local/bin` is always in PATH since HOME doesn't change)
- Added `export CLAUDE_CONFIG_DIR=<workerDir>/.claude` for config isolation
- Removed `hostHomeDir` parameter (unused after the above removals)

`setupClaudeSettings`:
- `.claude.json` now written to BOTH `homeDir/.claude.json` (for sandboxed/Docker workers
  that use `HOME=homeDir`) AND `homeDir/.claude/.claude.json` (for native workers where
  `CLAUDE_CONFIG_DIR=homeDir/.claude`).

### Also resolves
The "cannot find claude in PATH" symptom was caused by HOME override breaking node_modules/.bin
resolution. With real HOME, PATH is unmodified and claude is always findable.

### Not changed
- Docker/sandboxed runtime: HOME is still set to `/home/yak-shaver` inside containers.
  This is fine — containers don't have CLAUDE_CONFIG_DIR set and don't have keychain issues.
