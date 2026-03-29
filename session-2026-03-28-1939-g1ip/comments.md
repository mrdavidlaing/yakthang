## Yak Wrap — 2026-03-28 19:39 (supervised by David Laing)

### Highlights

- Completed 7 of 8 improve-yak-box code quality yaks in a single nightshift session
- First real sandbox nightshift — discovered and fixed two sandbox bugs (stale credentials, missing toolchain cache paths) during live testing
- Sequential shaver pattern worked well for nightshift: no rate limit issues, clean commits

---

## Yak Wrap — 2026-03-29 (supervised by David Laing)

### Highlights

- Fixed three sandbox stop bugs: worker home dir discovery, Zellij tab close name matching, and verified full spawn/stop cycle works cleanly
- Simplified native spawn by dropping CLAUDE_CONFIG_DIR — native workers now use the operator's real ~/.claude/
- Moved repo from wellmaintained/yakthang to mrdavidlaing/yakthang — Go module renamed across 23+ files
- Rewrote README to lead with ideas rather than implementation, added "tools with personality" section

### Shaved Yaks

#### sandbox runtime rework
- **fix sandbox worker home dir discovery** — StopSandboxWorker searched for @home/<yak-name> but dir is named after shaver. Fixed by storing HomeDir in Session at spawn time.
- **fix zellij tab close name matching** — findZellijTabIndex used exact match but tab names include display name. Changed to substring match.

#### worker runtime fixes
- **simplify native spawn** — Dropped CLAUDE_CONFIG_DIR and setupClaudeSettings for native workers. They now use the operator's real ~/.claude/ directly. API key set as ANTHROPIC_API_KEY instead of helper script.
- **suppress macos keychain prompt** — Marked parent done (root cause already fixed by CLAUDE_CONFIG_DIR approach, now further simplified).

#### yak-box CLI improvements
- **rename Go module to mrdavidlaing** — Changed module path from wellmaintained/yakthang to mrdavidlaing/yakthang across go.mod and 23+ Go files. Part of repo transfer.

#### docs
- **README rewrite** — Ideas-first framing with disclaimer. Dropped implementation details, added "tools with personality" section about the yak shaving metaphor.

### Interesting Findings

- Sandbox stop had THREE separate bugs stacked: wrong home dir lookup, wrong tab name match, and the process kill succeeded but tab didn't close. Each fix exposed the next.
- Repo transfer from org to personal was smooth with GitHub Transfer — automatic redirects mean old URLs keep working. The Go module rename was the bulk of the work (45 imports).

### Loose Ends

- spawn.go god-file refactor still parked (needs supervised session)
- improve yak-box and improve yak-map are empty parents after pruning
- sandbox runtime rework / add sandbox runtime mode has no remaining children after pruning — could be marked done
