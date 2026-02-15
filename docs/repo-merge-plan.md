# Plan: Merge OpenClaw Workspace into Main Git Repo

**Task:** `openclaw-orchestrator/repo-merge`
**Status:** Implemented
**Author:** Yakriel (analysis agent)
**Date:** 2026-02-14

---

## TL;DR

Remove `.openclaw/workspace/.git` and track the workspace markdown files directly
in the yakthang repo. OpenClaw doesn't require `.git` in the workspace — confirmed
via source code analysis. The only side effect is a harmless tip from `openclaw doctor`.

---

## Evidence Summary

### Q1: Does OpenClaw require `.git` in the workspace?

**No.** Source: `/usr/lib/node_modules/openclaw/dist/agent-scope-CnY2bb9p.js` lines 107-116.

```js
async function ensureGitRepo(dir, isBrandNewWorkspace) {
  if (!isBrandNewWorkspace) return;       // Skip if files already exist
  if (await hasGitRepo(dir)) return;      // Skip if .git present
  if (!await isGitAvailable()) return;    // Skip if no git binary
  try {
    await runCommandWithTimeout(["git", "init"], { cwd: dir, timeoutMs: 1e4 });
  } catch {}                              // Silently ignore failures
}
```

Key behaviors:
- `git init` only runs for **brand-new workspaces** (zero identity files exist)
- If git is unavailable or init fails, **workspace creation still succeeds**
- The runtime (`ensureAgentWorkspace`) only needs the files, never reads `.git`
- OpenClaw never auto-commits after initial setup

### Q2: Can workspace point to a subdirectory of the main repo?

**Yes, already does.** The config at `~/.openclaw/openclaw.json` has:

```json
"agents": {
  "defaults": {
    "workspace": "/home/yakob/yakthang/.openclaw/workspace"
  }
}
```

This path is configurable via:
- `agents.defaults.workspace` in config
- `openclaw setup --workspace <dir>` CLI flag
- `OPENCLAW_WORKSPACE_DIR` env var (Docker setups)
- Per-agent override: `agents.list[].workspace`

### Q3: What breaks if we remove `.openclaw/workspace/.git`?

**Almost nothing:**

| Component | Impact |
|-----------|--------|
| Agent runtime | **None** — reads files directly, ignores `.git` |
| `openclaw doctor` | **Cosmetic** — shows a tip: "back up the workspace in a private git repo" |
| `ensureAgentWorkspace()` | **None** — won't re-run `git init` because workspace already has files (the `isBrandNewWorkspace` check sees existing SOUL.md, etc.) |
| Channel plugin onboarding | **Minor** — `hasGitWorkspace()` is used to allow local plugin paths. Only matters during `openclaw onboard` channel setup, and the parent repo's `.git` may satisfy this anyway |
| Memory system | **None** — reads MEMORY.md / memory/ directly |
| Skills system | **None** — reads skills/ directly |

### Q4: Is a git submodule the right approach?

**No.** The workspace repo is local-only (no remote, 2 commits). Submodules require
a remote URL. The files are small, stable markdown — direct tracking is simpler.

### Q5: What's version-controllable vs machine-specific?

| File/Dir | Version control? | Reason |
|----------|-----------------|--------|
| `.openclaw/workspace/SOUL.md` | **Yes** | Personality — project identity |
| `.openclaw/workspace/AGENTS.md` | **Yes** | Operating procedures |
| `.openclaw/workspace/HEARTBEAT.md` | **Yes** | Monitoring checklist |
| `.openclaw/workspace/IDENTITY.md` | **Yes** | Agent name/emoji |
| `.openclaw/workspace/USER.md` | **Yes** | User context |
| `.openclaw/workspace/TOOLS.md` | **Yes** | Tool notes |
| `.openclaw/workspace/.gitignore` | **Yes** | Workspace gitignore (ignores .yaks) |
| `.openclaw/workspace/.yaks` | **No** | Symlink — machine-specific path |
| `.openclaw/workspace/.git/` | **Remove** | Nested repo — the whole point |
| `.openclaw/workspace/memory/` | **Yes** | Daily logs — versioned alongside identity |
| `.openclaw/workspace/skills/` | **Yes** (if exists) | Custom workspace skills |
| `~/.openclaw/openclaw.json` | **No** | Contains gateway auth token |
| `~/.openclaw/credentials/` | **No** | OAuth tokens, API keys |
| `~/.openclaw/agents/*/sessions/` | **No** | Session transcripts |

---

## Recommendation: Direct Tracking (Option A)

### What to do

1. **Remove the nested git repo:**
   ```bash
   rm -rf .openclaw/workspace/.git
   ```

2. **Update `.gitignore`** — replace the blanket `/.openclaw/` ignore with selective rules:
   ```gitignore
   # OpenClaw workspace — track identity files, ignore runtime state
   /.openclaw/workspace/.yaks
   /.openclaw/workspace/memory/
   ```
   (Remove the `/.openclaw/` line that currently ignores everything.)

3. **Add workspace files to main repo:**
   ```bash
   git add .openclaw/workspace/SOUL.md
   git add .openclaw/workspace/AGENTS.md
   git add .openclaw/workspace/HEARTBEAT.md
   git add .openclaw/workspace/IDENTITY.md
   git add .openclaw/workspace/USER.md
   git add .openclaw/workspace/TOOLS.md
   git add .openclaw/workspace/.gitignore
   ```

4. **Commit:**
   ```bash
   git commit -m "Track OpenClaw workspace identity files in main repo"
   ```

5. **Update `setup-vm.sh`** — now generates `~/.openclaw/openclaw.json`,
   prompts for secrets (OPENCODE_API_KEY, Slack tokens), and writes the
   systemd override. No import scripts needed.

6. **Remove `export-config.sh` and `import-config.sh`** — replaced by
   git-tracked config + setup-vm.sh secret prompts.

### What NOT to do

- Don't change the workspace path — `/.openclaw/workspace` is already established
  in config, scripts, and documentation. Changing it means updating
  `~/.openclaw/openclaw.json`, `setup-vm.sh`, `export-config.sh`, `import-config.sh`,
  and `docs/openclaw-migration-plan.md`.
- Don't use submodules — no remote repo, overhead not justified for 6 markdown files.

### Fresh install flow after this change

```
1. git clone <repo>                    # Gets workspace identity files
2. ./setup-vm.sh                       # Creates .yaks symlink, installs openclaw
3. openclaw onboard --workspace /home/yakob/yakthang/.openclaw/workspace
   # Onboard sees existing files, skips template creation, skips git init
   # Sets up config, credentials, channels
4. Done — workspace files came from git, not from onboarding templates
```

### Alternatives considered

**Option B: Submodule** — Requires a remote repo URL. The workspace has no remote
(local-only, 2 commits). Would need to create a separate GitHub repo for 6 markdown
files. Overkill.

**Option C: Move workspace outside `.openclaw/`** — e.g., `yakthang/workspace/`.
Cleaner path, but requires updating `~/.openclaw/openclaw.json`,
`setup-vm.sh`, `export-config.sh`, `import-config.sh`, and all docs.
Higher blast radius for marginal benefit.

**Option D: Symlink from workspace into repo** — Keep `.openclaw/workspace/.git` but
symlink individual files from a tracked directory. Fragile, confusing.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `openclaw doctor` shows backup tip | Certain | Cosmetic | Ignore or suppress with `--no-workspace-suggestions` if available |
| Future OpenClaw update changes workspace assumptions | Low | Medium | Pin openclaw version; workspace file format is stable (just markdown) |
| `ensureAgentWorkspace` tries to re-create `.git` | Won't happen | N/A | Only triggers for brand-new workspaces (no files exist). Our files exist. |
| Channel plugin onboarding can't resolve local paths | Low | Low | Parent `.git/` at repo root may satisfy the check; re-test during next channel setup |
| Merge conflicts on workspace files | Low | Low | Files change rarely (identity/procedures are stable) |

---

## Verification Steps (post-implementation)

1. `openclaw doctor` — should pass (may show workspace backup tip — that's OK)
2. `openclaw agents list` — should still show Yakob with correct workspace path
3. `openclaw agent --message "What is your name?"` — should respond as Yakob
4. Heartbeat should still work (reads HEARTBEAT.md from workspace)
5. `git status` — workspace files tracked, no nested repo warnings
6. Fresh clone test: clone to `/tmp`, verify workspace files are present
