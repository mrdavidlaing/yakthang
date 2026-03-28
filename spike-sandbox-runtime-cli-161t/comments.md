# Spike: @anthropic-ai/sandbox-runtime CLI Interface

## 1. How to invoke

Binary name: `srt` (installed via `npm install -g @anthropic-ai/sandbox-runtime`)

```bash
srt <command>                           # positional args joined with space
srt -c "<command>"                      # pass command string directly (like sh -c)
srt --settings /path/to/config.json <cmd>  # custom config file
srt --debug <cmd>                       # debug logging (sets DEBUG=true)
srt --control-fd <fd> <cmd>             # dynamic config updates via fd (JSON lines)
```

No subcommands — srt is a single-purpose wrapper. It takes a command, wraps it
in OS-level sandboxing, and exits with the child's exit code.

Can also be used as a Node library:
```typescript
import { SandboxManager } from '@anthropic-ai/sandbox-runtime'
await SandboxManager.initialize(config)
const wrapped = await SandboxManager.wrapWithSandbox('some command')
```

## 2. Filesystem config

Two separate patterns:

**Reads** (deny-then-allow — reads allowed by default):
- `filesystem.denyRead` — paths to block reads on (e.g. `["~/.ssh"]`)
- `filesystem.allowRead` — re-allow within denied regions; **takes precedence over denyRead**

**Writes** (allow-only — writes denied by default):
- `filesystem.allowWrite` — paths to allow writes (e.g. `[".", "/tmp"]`)
- `filesystem.denyWrite` — carve out protected regions; **takes precedence over allowWrite**

Note the asymmetry: allowRead > denyRead, but denyWrite > allowWrite. This is intentional.

**Mandatory deny paths** (always blocked from writes, even within allowWrite):
- Shell configs: .bashrc, .bash_profile, .zshrc, .zprofile, .profile
- Git configs: .gitconfig, .gitmodules, .git/hooks/, .git/config
- IDE/Claude dirs: .vscode/, .idea/, .claude/commands/, .claude/agents/
- Other: .ripgreprc, .mcp.json

**Linux caveat**: mandatory deny only blocks *existing* files (bubblewrap bind-mount limitation).
macOS uses globs so it blocks creation too.

**Path syntax**:
- macOS: git-style globs (*, **, ?, [abc])
- Linux: **literal paths only, no glob support**
- Both: absolute or relative paths; ~ expands to $HOME

**mandatoryDenySearchDepth** (Linux only): controls how deep ripgrep scans for
dangerous files in allowWrite dirs. Default 3, range 1-10.

## 3. Network config

**Allow-only pattern** — all network denied by default.

- `network.allowedDomains` — array of permitted domains (supports wildcards: `*.github.com`)
- `network.deniedDomains` — blocklist, checked first, takes precedence
- `network.allowLocalBinding` — allow binding to local ports (default: false)

**Unix sockets** (blocked by default on both platforms):
- macOS: `allowUnixSockets: ["/var/run/docker.sock"]` or `allowAllUnixSockets: true`
- Linux: uses seccomp BPF filters (x64/arm64). `allowAllUnixSockets: true` disables blocking.

Network traffic routes through proxy servers (HTTP proxy + SOCKS5 proxy) running on the host:
- Linux: via Unix domain sockets bind-mounted into the bwrap namespace
- macOS: via localhost ports allowed by the Seatbelt profile

## 4. Platform support

| Platform | Mechanism | Dependencies |
|----------|-----------|-------------|
| macOS | sandbox-exec (Seatbelt profiles) | ripgrep |
| Linux | bubblewrap (bwrap) | bubblewrap, socat, ripgrep |
| Windows | Not supported | — |

**Linux-specific**: network namespace is fully removed; all traffic must go through
the host-side proxies. Seccomp BPF filters block Unix socket creation at syscall level.

**macOS-specific**: Seatbelt profiles are dynamically generated. Violation monitoring
via system log store gives real-time alerts. Linux has no built-in violation
reporting (use strace).

## 5. Exit codes and error handling

- Child exit code is forwarded directly: `process.exit(code ?? 0)`
- SIGINT/SIGTERM on child → exit 0
- Other signals on child → exit 1
- No command specified → exit 1 with "No command specified" error
- Spawn failure → exit 1 with "Failed to execute command" error
- Any initialization error → exit 1

After child exits, `SandboxManager.cleanupAfterCommand()` removes bwrap mount
artifacts (empty files created for non-existent deny paths on Linux).

## 6. Config file support

- Default location: `~/.srt-settings.json`
- Override with: `srt --settings /path/to/file.json`
- **Dynamic updates**: `--control-fd <fd>` reads JSON lines from a file descriptor,
  calling `SandboxManager.updateConfig()` on each valid line. This lets a parent
  process update sandbox rules at runtime without restarting.

If no config file found, defaults to empty arrays (no network, no writes, full reads).

## 7. Sample wrapper for yak-box

Config file (`yak-box-sandbox.json`):
```json
{
  "filesystem": {
    "allowWrite": ["/home/user/project"],
    "denyWrite": [
      "/home/user/project/.yaks",
      "/home/user/project/.git/hooks",
      "/home/user/project/.git/config"
    ],
    "denyRead": [],
    "allowRead": []
  },
  "network": {
    "allowedDomains": [
      "github.com",
      "*.github.com",
      "api.anthropic.com",
      "*.anthropic.com"
    ],
    "deniedDomains": []
  }
}
```

Invocation:
```bash
srt --settings yak-box-sandbox.json claude --dangerously-skip-permissions
```

**Problem**: denyWrite within allowWrite takes precedence, so `.yaks/` is fully
protected. But we want to allow writes to *specific* subdirs like
`.yaks/<task>/shaver-message`. Unfortunately, there's no way to re-allow within
a denyWrite region — denyWrite always wins. So the approach would be:

**Option A**: Don't deny .yaks/ at all; rely on convention + file permissions.
**Option B**: Use the library API with `--control-fd` to dynamically update
write permissions as tasks are assigned (the parent process sends updated config
via the fd when a task starts).
**Option C**: Have shavers write to a staging area outside .yaks/ and have the
orchestrator copy results in.

## 8. Gotchas and limitations

1. **Linux has no glob support** — all paths must be literal. This makes per-file
   deny rules cumbersome (must list every file explicitly).

2. **No re-allow within denyWrite** — once a path is in denyWrite, it's blocked
   period. No allowWrite override. The asymmetry (allowRead > denyRead, but
   denyWrite > allowWrite) means you can't do "deny .yaks/ except .yaks/task/output".

3. **Mandatory deny paths are hardcoded** — .bashrc, .gitconfig, .claude/commands/
   etc. are always blocked. If yak-box needs to write .claude/ files for worker
   setup, that won't work under srt. Workers would need their .claude/ setup
   done before srt wraps them.

4. **Linux mandatory deny only blocks existing files** — if a dangerous file
   doesn't exist yet, bubblewrap can't block its creation. macOS globs handle this.

5. **Network filtering relies on proxy env vars on Linux** — programs that ignore
   HTTP_PROXY/HTTPS_PROXY/ALL_PROXY won't route through the proxy and will simply
   fail to connect (network namespace is removed, so they get no connectivity
   rather than unfiltered connectivity — this is actually safe, just confusing).

6. **Unix socket blocking is seccomp-based on Linux** — only x64/arm64. Other
   architectures get a warning and unrestricted sockets.

7. **No Windows support**.

8. **`enableWeakerNestedSandbox`** is needed for Docker environments but
   considerably weakens security. Relevant if yak-box workers run in containers.

9. **Exit code passthrough is clean** — srt preserves the child's exit code,
   so callers can detect failures. SIGINT/SIGTERM get swallowed to exit 0 though.

10. **Dynamic config via --control-fd** is the most interesting feature for
    yak-box: the orchestrator can adjust sandbox permissions at runtime without
    restarting the wrapped process. JSON lines protocol, one config per line.
