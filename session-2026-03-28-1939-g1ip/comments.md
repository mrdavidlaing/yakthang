## Yak Wrap — 2026-03-29 09:01 (supervised by David Laing)

### Highlights

- Completed 7 of 8 improve-yak-box code quality yaks in a single nightshift session
- First real sandbox nightshift — discovered and fixed two sandbox bugs (stale credentials, missing toolchain cache paths) during live testing
- Sequential shaver pattern worked well for nightshift: no rate limit issues, clean commits

### Shaved Yaks

#### improve yak-box (5 yaks)
- **dead code removal** — Deleted resolveAnthropicKeySuffix (unused), messageCmd (unregistered), generateDevContainerID dead branch, simplified native wrapper return. -345 lines.
- **regex recompile hotpath** — Verified already correct (varPattern compiled at package level). No changes needed.
- **untyped string constants** — Added typed constants for Runtime, Tool, Mode, Severity in pkg/types/types.go. Replaced raw string comparisons across 11 files.
- **oauth duplication** — Shared HasOAuthCredentials in auth.go, removed duplicate from preflight.go.
- **unmarshal json duplication** — Already completed by previous partial sandbox run. Clean embedded *Alias pattern confirmed.

#### sandbox runtime fixes (2 yaks)
- **fix pre-auth credential overwrite** — Removed "skip if exists" check in copyHostOAuthCredentials. Stale OAuth tokens in persistent worker homes now always overwritten.
- **fix srt config for toolchain caches** — Added GOPATH, CARGO_HOME, ~/.cache, mise, RUSTUP_HOME, ~/.bun to allowWrite. Fixed "read-only file system" errors for go build inside sandbox.

#### parked
- **spawn.go god-file refactor** — Explicitly marked "not for nightshift" in context. Parked as sleeping.

### Interesting Findings

- Sandbox pre-auth copies credentials but never overwrites existing ones — stale tokens in persistent worker homes caused 401s. Simple fix but a good reminder that persistent state + immutable copies = bugs.
- Go module cache writes to ~/go/pkg/mod/cache/ which is outside the sandbox allowWrite paths. Any toolchain that caches outside cwd/tmp will hit this. We added 6 common cache paths.
- The regex hotpath was already fixed — the desloppify analysis that created the yak was stale. Worth checking freshness of code quality findings before spawning.

### Loose Ends

- spawn.go god-file refactor needs a supervised session (730 LOC, 264-line runSpawn)
- Stale yak-box sessions cleaned up (7 from March 7-21)
- suppress-macos-keychain-prompt parent yak discussed but not marked done — operator hasn't confirmed
