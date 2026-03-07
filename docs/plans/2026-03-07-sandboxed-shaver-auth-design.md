# Sandboxed Shaver Auth Design

**Date:** 2026-03-07
**Status:** Design
**Author:** Yakira

---

## Problem

yak-box supports two runtimes: native (Zellij, host process) and sandboxed (Docker). Auth has two modes: API key and OAuth (Max/Pro subscription via `claude login`).

**Native runtime** works for both auth modes today:
- API key: passed via `_ANTHROPIC_API_KEY` env var
- OAuth: the shaver's persistent home dir (`.yak-boxes/@home/<name>/`) is set as `HOME`, so `~/.claude/` credentials are found naturally

**Sandboxed runtime** only works for API key mode today:
- `preflight.go:EnsureClaudeAuthEnv` hard-errors if `_ANTHROPIC_API_KEY` is not set
- The persistent home dir IS mounted at `/home/yak-shaver:rw`, so OAuth creds could theoretically persist — but there is no mechanism to perform the initial `claude login` inside the container, and the preflight blocks before we get there

---

## Key Architectural Insight

Each shaver name already has a persistent home directory:

```
.yak-boxes/@home/<shaver-name>/
  .claude/            # OAuth tokens stored here by claude login
  .claude.json        # pre-seeded by setupClaudeSettings
  scripts/            # generated run.sh, prompt.txt, etc.
```

This directory is mounted as a Docker volume (`-v <homeDir>:/home/yak-shaver:rw`). OAuth credentials written by `claude login` inside a container therefore persist across container respawns. The infrastructure is already there — we just need to:

1. Allow the initial login to happen
2. Detect which auth mode is active
3. Surface status to the operator

---

## Auth Detection

Auth mode is determined at spawn time in this priority order:

| Priority | Condition | Mode |
|----------|-----------|------|
| 1 | `_ANTHROPIC_API_KEY` or `ANTHROPIC_API_KEY` set in host env | API key |
| 2 | `<homeDir>/.claude/` contains valid OAuth credentials | OAuth (per-shaver) |
| 3 | Neither | Error: no auth configured |

The detection function (`detectClaudeAuth`) should return a typed result:

```go
type AuthMode int
const (
    AuthModeAPIKey AuthMode = iota
    AuthModeOAuth
    AuthModeNone
)

type AuthDetection struct {
    Mode    AuthMode
    APIKey  string  // non-empty when Mode == AuthModeAPIKey
}
```

**OAuth credential detection:** Check for the existence of `<homeDir>/.claude/` containing at least one of:
- `credentials.json`
- `.credentials.json`
- Any file matching `*.json` with a non-zero size

This is intentionally shallow — we do not try to validate token expiry at spawn time (that complexity belongs to a future `auth-status --validate` flag).

---

## Preflight Change: `EnsureClaudeAuthEnv`

**Current behaviour** (`preflight.go:139-155`):
- Native runtime: always passes (OAuth inherits from host)
- Sandboxed runtime: hard-errors if `_ANTHROPIC_API_KEY` is not set

**New behaviour:**
- Native runtime: unchanged (always passes)
- Sandboxed runtime: check for API key OR shaver OAuth creds; error only if neither present

The function signature should be extended:

```go
func EnsureClaudeAuth(tool, runtime, shaverHomeDir string, lookupEnv func(string) (string, bool)) error
```

When `shaverHomeDir` is non-empty, the function checks `<shaverHomeDir>/.claude/` for credentials before failing. This removes the current false requirement for OAuth users who have already logged in.

---

## `yak-box auth-status` Command

A new subcommand to surface auth state to the operator.

### Usage

```
yak-box auth-status [--shaver <name>] [--runtime native|sandboxed]
```

Without `--shaver`, shows host-level auth state. With `--shaver`, checks the named shaver's persistent home dir.

### Output format

```
Auth status for shaver: yakira
  Runtime: sandboxed
  Home: .yak-boxes/@home/yakira/
  Mode: oauth
  Credentials: present (.claude/credentials.json)

Auth status for shaver: yakoff
  Runtime: sandboxed
  Home: .yak-boxes/@home/yakoff/
  Mode: none
  Credentials: not found
  Action required: run 'yak-box auth-login --shaver yakoff'
```

### Implementation sketch

```go
// cmd/auth_status.go
func runAuthStatus(shaverName, runtime string) error {
    if shaverName == "" {
        // Show host-level: check _ANTHROPIC_API_KEY and host ~/.claude/
    } else {
        homeDir := resolveHomeDirForShaver(shaverName)
        detection := detectClaudeAuth(homeDir, os.LookupEnv)
        printAuthStatus(shaverName, homeDir, detection)
    }
}
```

---

## Per-Shaver OAuth Flow for Sandboxed Workers

### First spawn: shaver has no OAuth credentials

When `auth-status` detects no credentials and no API key, spawning should fail with a clear message:

```
Error: shaver 'yakira' has no auth configured.
  Option 1 (OAuth): run 'yak-box auth-login --shaver yakira' to complete device flow
  Option 2 (API key): set _ANTHROPIC_API_KEY in your environment
```

### `yak-box auth-login --shaver <name>`

A new command that:

1. Ensures the shaver's home dir exists (creates if needed)
2. Spawns a minimal Docker container with the same image and home-dir mount as a normal shaver container, but runs `claude login` instead of the shaver script
3. The device-flow URL prints to stdout; the user opens it in their browser
4. Container exits after successful login; OAuth creds are now in `<homeDir>/.claude/`
5. Subsequent `yak-box spawn --shaver yakira` picks up the creds automatically

The login container invocation:

```bash
docker run -it --rm \
  --name yak-auth-login-<shaver> \
  --user "<uid>:<gid>" \
  -v "<homeDir>:/home/yak-shaver:rw" \
  -e HOME=/home/yak-shaver \
  -e IS_DEMO=true \
  -v "<passwdFile>:/etc/passwd:ro" \
  -v "<groupFile>:/etc/group:ro" \
  yak-worker:latest \
  bash -c 'claude login'
```

No API key is passed. No `--dangerously-skip-permissions` flag. This is an interactive session.

### Subsequent spawns: shaver has OAuth credentials

`detectClaudeAuth` finds credentials in `<homeDir>/.claude/`. The preflight passes. `setupClaudeSettings` is called with `apiKey=""` (same as today for native OAuth workers), which:
- Skips writing `api-key-helper.sh`
- Writes a minimal `settings.json` without `apiKeyHelper`
- Claude Code falls through to OAuth credentials in `/home/yak-shaver/.claude/`

No code change needed in `setupClaudeSettings` — this already works correctly.

---

## Credential Propagation

### How it works today (for reference)

| Credential type | Native | Sandboxed (current) |
|----------------|--------|---------------------|
| `_ANTHROPIC_API_KEY` | via env in `run.sh` | via `-e` flag in `docker run` |
| OAuth tokens | inherited from `$HOME/.claude/` (host) | NOT SUPPORTED |
| opencode auth | host `~/.local/share/opencode/auth.json` ro-mounted | host file ro-mounted |
| git config | `GIT_CONFIG_GLOBAL` → host `.gitconfig` | host `.gitconfig` ro-mounted |
| gh config | `GH_CONFIG_DIR` → host `.config/gh` | host `.config/gh` ro-mounted |

### After this design

| Credential type | Sandboxed (new) |
|----------------|-----------------|
| `_ANTHROPIC_API_KEY` | via `-e` flag (unchanged) |
| OAuth tokens | shaver's persistent home dir (`<homeDir>/.claude/`) |

The persistent home dir mount (`-v "<homeDir>:/home/yak-shaver:rw"`) already exists in `generateRunScript` (helpers.go:124-126). OAuth creds written by `claude login` inside the container at `/home/yak-shaver/.claude/` map to `<homeDir>/.claude/` on the host.

**No new Docker volume mounts are required.** The auth-login command and subsequent spawns use the same mount. This is the core elegance of the design.

---

## Open Questions and Risks

### Token expiry mid-shave

Claude Code's OAuth tokens are issued by Anthropic. Max/Pro tokens appear to be long-lived (hours to days), but the exact TTL is not documented publicly. Mitigations:

- Accept the risk for now; shaves typically complete in under 30 minutes
- Future: `auth-status --validate` pings Anthropic to check token validity before spawn
- Future: detect auth errors in the shaver's exit code and surface "re-run `auth-login`" suggestion

### Token refresh inside containers

Claude Code handles token refresh automatically when it has a valid refresh token. Since the full `~/.claude/` directory is mounted read-write, refresh tokens written during the session persist. This should work transparently.

### Multiple shavers sharing a subscription

Each shaver has its own `claude login` identity. If all shavers use the same Max/Pro account, Anthropic's per-account rate limits apply to the aggregate. This is a billing/rate-limit concern, not a design concern here.

### `auth-status` across all shavers

A `yak-box auth-status --all` flag would scan `.yak-boxes/@home/` and report status for every known shaver. Useful but not required for the MVP.

---

## Implementation Plan

The following tasks are in scope for implementation (not this design task):

1. **`detectClaudeAuth` function** in `runtime` package — returns `AuthDetection` based on env and home dir inspection
2. **Update `EnsureClaudeAuthEnv`** to accept `shaverHomeDir` and call `detectClaudeAuth`; remove the hard API key requirement for sandboxed OAuth users
3. **`yak-box auth-login --shaver <name>`** command — spawns login container, waits for exit, reports result
4. **`yak-box auth-status [--shaver <name>]`** command — prints auth mode and credential presence
5. **Update spawn error messages** — when preflight fails for auth, guide user to `auth-login`
6. **Tests** for `detectClaudeAuth` and updated `EnsureClaudeAuthEnv`

Files likely to change:

- `src/yak-box/internal/preflight/preflight.go` — `EnsureClaudeAuthEnv` signature and logic
- `src/yak-box/internal/runtime/native.go` — call updated auth detection
- `src/yak-box/internal/runtime/sandboxed.go` — call updated auth detection
- `src/yak-box/cmd/` — new `auth-status` and `auth-login` commands
- New: `src/yak-box/internal/runtime/auth.go` — `detectClaudeAuth` and related types

---

## Non-Goals

- Automatic token refresh orchestration (Claude Code handles this)
- Credential sharing between shavers (per-shaver isolation is a feature)
- Support for tools other than `claude` (opencode uses its own auth mechanism already mounted)
- Revoking per-shaver credentials (delete `<homeDir>/.claude/` manually)
