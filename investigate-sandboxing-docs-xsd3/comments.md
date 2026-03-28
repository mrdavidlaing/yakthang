# Sandboxing Research Findings

## Source
Official docs at https://code.claude.com/docs/en/sandboxing (redirected from docs.anthropic.com). Also read the security page at /en/security.

## What Claude Code provides natively

Claude Code has a built-in **sandboxed bash tool** using OS-level primitives:
- **macOS**: Seatbelt (built-in, no install needed)
- **Linux/WSL2**: bubblewrap + socat (apt install required)
- WSL1 not supported; native Windows planned

**Filesystem isolation**: writes restricted to CWD by default; reads allowed broadly except denied dirs. Configurable via `sandbox.filesystem.allowWrite`, `denyWrite`, `denyRead`, `allowRead` in settings.json. Paths merge across settings scopes (managed, user, project).

**Network isolation**: proxy-based domain filtering. Only approved domains reachable. New domains trigger permission prompts. Custom proxy support available (`sandbox.network.httpProxyPort`, `socksProxyPort`).

**Two modes** (toggled via `/sandbox`):
1. **Auto-allow**: sandboxed bash runs without permission prompts; unsandboxable commands fall back to normal flow
2. **Regular permissions**: all commands still prompt, but sandbox restrictions still enforced

**Key settings**:
- `sandbox.enabled: true` — master switch
- `sandbox.failIfUnavailable: true` — hard-fail if sandbox can't start (good for managed deployments)
- `sandbox.allowUnsandboxedCommands: false` — disables the escape hatch where Claude retries failed commands outside sandbox
- `excludedCommands` — commands that always run outside sandbox (e.g. docker)
- `enableWeakerNestedSandbox` — runs inside Docker without privileged namespaces (weaker security)

**Escape hatch**: if a command fails due to sandbox restrictions, Claude can retry with `dangerouslyDisableSandbox` (goes through normal permissions). Disableable via `allowUnsandboxedCommands: false`.

**Open source**: sandbox runtime available as `npx @anthropic-ai/sandbox-runtime <command>` — usable for sandboxing any program (e.g. MCP servers). Source: https://github.com/anthropic-experimental/sandbox-runtime

## How this relates to yak-box --runtime sandboxed vs native

**yak-box sandboxed mode** uses Docker containers with:
- `--cap-drop ALL`, `--security-opt no-new-privileges`
- `.yaks/` mounted read-only (assigned tasks get rw override)
- Resource limits (CPU, memory, PIDs via profiles: light/default/heavy/ram)
- Non-root execution as `yakshaver` user
- Path traversal protection (CWE-22)

**yak-box native mode** uses host processes with:
- CLAUDE_CONFIG_DIR isolation (per-worker .claude/ dir)
- Zellij tab management
- Process group cleanup (SIGTERM → SIGKILL)
- No filesystem isolation beyond process permissions

**Key overlap**: Claude Code's native sandbox (bubblewrap on Linux) provides filesystem + network isolation at the OS level, which would apply *within* a native-spawned worker. This means native mode workers could get meaningful isolation without Docker overhead — if Claude Code's sandbox is enabled.

**Key gap**: yak-box's Docker sandboxing protects `.yaks/` with read-only mounts. Claude Code's native sandbox would need explicit `denyWrite` rules to achieve the same protection for `.yaks/`. The `.yaks/` read-only mount is yak-box's strongest defense against the "shaver wipes .yaks/" class of incidents.

## Are there guardrails we're missing for "shaver wipes .yaks/"?

**Current protection (sandboxed mode)**: `.yaks/` mounted read-only — strong; workers physically cannot write outside assigned dirs.

**Current protection (native mode)**: None. A native worker has full host filesystem access. Claude Code's `--dangerously-skip-permissions` (which yak-box uses for native spawn) bypasses ALL permission checks including sandbox.

**What native sandbox could add**: If we enabled Claude Code's sandbox for native workers and configured `sandbox.filesystem.denyWrite: [".yaks/"]` while allowing writes to specific assigned task dirs, we'd get OS-level protection without Docker. BUT: yak-box currently passes `--dangerously-skip-permissions` which likely disables the sandbox too.

**Recommendation**: Investigate whether `--dangerously-skip-permissions` disables the sandbox. If not, enabling sandbox + denyWrite rules would be a significant safety improvement for native mode. If it does disable sandbox, we'd need a different flag combination.

## Could native sandboxing replace or simplify our devcontainer approach?

**Partially, with caveats:**

Pros of switching to native sandbox:
- No Docker dependency (bubblewrap is much lighter)
- No image build/cache management
- No devcontainer.json parsing complexity
- Faster worker startup (no container spin-up)
- OS-level enforcement covers ALL subprocesses (same as Docker)

Cons / what Docker still provides that native sandbox doesn't:
- **Resource limits** (CPU, memory, PIDs) — bubblewrap doesn't do cgroup limits; Docker does
- **Network namespace isolation** — Claude's sandbox does domain filtering via proxy, but Docker provides full network namespacing
- **Clean environment** — Docker gives a fresh filesystem; native inherits host state
- **Reproducibility** — devcontainer ensures consistent tooling; native depends on host setup
- **The `enableWeakerNestedSandbox` caveat** — running Claude's sandbox inside Docker requires the weaker mode, which "considerably weakens security"

**Verdict**: For workers that need strong isolation (untrusted code, resource limits, clean environments), Docker/devcontainer remains superior. For trusted workers doing routine tasks where the main risk is accidental `.yaks/` corruption, native sandbox + denyWrite rules could be a simpler, faster alternative. A hybrid approach makes sense: keep `--runtime sandboxed` for high-risk work, but make `--runtime native` safer by enabling Claude's built-in sandbox with appropriate filesystem restrictions.

**Next investigation needed**: Does `--dangerously-skip-permissions` disable the sandbox? This is the critical question that determines feasibility.
