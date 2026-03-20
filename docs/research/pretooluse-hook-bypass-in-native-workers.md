# PreToolUse Hook Bypass in Native Workers

**Date:** 2026-03-20
**Investigator:** Yaklyn
**Status:** Diagnosed — fix not yet implemented

## Problem

The global `PreToolUse` hook that blocks bare `git` commands is configured in `~/.claude/settings.json` on the operator's machine. It fires correctly for the orchestrator session but does **not** fire for shavers spawned via `yak-box --runtime native`.

## Hook Configuration

`/Users/zell/.claude/settings.json`:
```json
"hooks": {
  "PreToolUse": [
    {
      "matcher": "Bash",
      "hooks": [
        {
          "type": "command",
          "command": "bash ~/.claude/hooks/block-git.sh",
          "statusMessage": "Checking for bare git commands..."
        }
      ]
    }
  ]
}
```

`~/.claude/hooks/block-git.sh` denies any Bash command matching `(^|[;&|]+\s*)git(\s|$)`.

## Root Cause 1 — Worker `settings.json` Is Written Without Hooks

`internal/runtime/native.go:setupClaudeSettings()` writes a fresh `settings.json` to
`<workerHome>/.claude/settings.json`. It only populates:

- `apiKeyHelper` (when API key auth is in use)
- `statusLine` (when `goccc` is installed)
- `skipDangerousModePermissionPrompt: true` (always)

The `hooks` section from the operator's real `~/.claude/settings.json` is **never read,
copied, or merged**. Claude Code running in the worker reads only the worker's settings
file and finds no hooks.

**Confirmed:** `.yak-boxes/@home/Yaklyn/.claude/settings.json` contains:
```json
{"skipDangerousModePermissionPrompt": true}
```

No hooks directory exists at `.yak-boxes/@home/Yaklyn/.claude/hooks/`.

## Root Cause 2 — `HOME` Override Severs the Hook Chain

The native worker wrapper script (`run.sh`) sets `export HOME=<workerHome>` so that Claude
Code finds worker-specific skills and settings at `<workerHome>/.claude/` rather than the
operator's real home. This is intentional isolation.

A side-effect: the hook command `bash ~/.claude/hooks/block-git.sh` would expand `~` to
`<workerHome>`, not `/Users/zell`. Even if the `hooks` config were naively copied, the
script path would resolve to a non-existent file in the worker home.

## Why It Works for the Orchestrator

The orchestrator runs with the real `HOME=/Users/zell`. Claude Code reads
`/Users/zell/.claude/settings.json`, which contains the `hooks` section, and resolves
`~/.claude/hooks/block-git.sh` correctly.

## Execution Path Summary

```
yak-box spawn --runtime native
  └─ runSpawn()
       └─ SpawnNativeWorker()
            ├─ setupClaudeSettings(homeDir)   <- writes minimal settings.json, no hooks
            └─ generateNativeWrapperScript()  <- emits: export HOME=<workerHome>
                                                         claude --dangerously-skip-permissions @prompt.txt

Worker Claude process:
  HOME = <workerHome>
  reads <workerHome>/.claude/settings.json  -> {"skipDangerousModePermissionPrompt": true}
  no hooks -> PreToolUse never fires
```

## Proposed Fix

Two changes are needed together:

### 1. Merge host hooks into the worker's `settings.json`

In `setupClaudeSettings(homeDir, apiKey string)`, accept the host home dir as an additional
parameter. Before writing `settings.json`, read the operator's
`<hostHomeDir>/.claude/settings.json`, extract the `hooks` key, and merge it into the
worker settings map.

### 2. Rewrite `~` to an absolute path in hook commands

When merging, walk every hook `command` string and replace a leading `~/` with
`<hostHomeDir>/`. This ensures the hook script resolves to the real host path even though
the worker runs with `HOME=<workerHome>`.

Alternatively, change the hook registration in the host settings to use an absolute path
from the start (e.g. `/Users/zell/.claude/hooks/block-git.sh`), making the rewrite step
unnecessary. Step 1 is still required regardless.

### Scope of change

- `internal/runtime/native.go` — `setupClaudeSettings()` signature + merge logic
- The call site in `generateNativeWrapperScript()` already has `hostHomeDir` in scope,
  so threading it through is straightforward.
- No changes needed to the sandboxed runtime (the Docker container gets a fully isolated
  environment by design and does not inherit host hooks).
