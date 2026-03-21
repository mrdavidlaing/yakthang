# OpenShell vs yak-box: Research Comparison

> Research round 1. Sources: NVIDIA/OpenShell README (March 2026), src/yak-box/ source.

---

## 1. OpenShell Summary

OpenShell (https://github.com/NVIDIA/OpenShell) is a **secure agent runtime** from NVIDIA — currently alpha/single-user.

Its core pitch: *"the safe, private runtime for autonomous AI agents."* It provides sandboxed execution environments governed by declarative YAML policies that prevent unauthorized file access, data exfiltration, and uncontrolled network activity.

**Architecture:**
- Python CLI, installable via uv/PyPI or binary
- Each sandbox runs in its own Docker container; the control plane is K3s (Kubernetes) running *inside* a single Docker container
- A persistent **gateway** (the K3s cluster) manages sandbox lifecycle, credential injection, and egress proxying
- Agents run inside sandbox containers and connect out through the policy-enforced proxy

**Key capabilities:**
- `openshell sandbox create -- claude` — spin up an isolated agent sandbox
- Declarative YAML **network policies** (hot-reloadable at runtime, no sandbox restart needed)
- **Inference routing** — strips caller credentials, injects backend credentials, proxies LLM API calls through a controlled endpoint
- **GPU passthrough** for local inference workloads
- Credential **providers** — named bundles injected as env vars at creation; never written to container filesystem
- Community sandbox catalog (`--from openclaw`, `--from ./my-sandbox-dir`)
- TUI dashboard (`openshell term`) for monitoring gateways, sandboxes, providers — k9s-inspired
- Protection layers: filesystem (locked at creation), network (hot-reload), process/seccomp (locked at creation), inference routing (hot-reload)

**Target audience:** Teams running autonomous agents that touch sensitive data or infrastructure; enterprise-grade/multi-tenant use cases (roadmap). Agents: Claude Code, OpenCode, Codex, OpenClaw.

---

## 2. yak-box Summary

yak-box is a **task-orchestration worker launcher** — a Go CLI that spawns, manages, and stops agent workers (primarily Claude Code) as part of the yakthang workflow.

Its core pitch: spawn a worker, hand it a yak (task), let it shave.

**Architecture:**
- Go binary (`yak-box spawn/stop/check/auth/diff`)
- Two runtimes: **sandboxed** (Docker container + Zellij tab) and **native** (Zellij tab only, no container)
- Workers are ephemeral; each gets a generated home dir under `.yak-boxes/` with a prompt, scripts, and Claude settings
- Integrates deeply with the `yx` task tracker: injects task context, yak path, and agent persona into the worker prompt
- Worker identity delivered via a text prompt file (`prompt.txt`) read by Claude at startup

**Key capabilities:**
- `yak-box spawn --yak <name> --runtime sandboxed/native` — spin up an agent worker
- Resource **profiles** (light/heavy/ram/default): CPU, memory, PID, tmpfs limits — set at spawn time
- Auth detection: API key env vars or OAuth credentials from `~/.claude/`; credentials injected at runtime
- Zellij tab management: each worker gets a dedicated tab with a build pane + shell pane
- **Cross-repo worktrees**: `worktrees` field on a yak coordinates the same branch across multiple repos
- devcontainer.json support: can use custom Docker images, env vars, mounts per-project
- Security validation: warns on dangerous devcontainer capabilities (SYS_ADMIN, NET_ADMIN, etc.), seccomp/AppArmor overrides
- Skill delivery: skills are folders copied into the worker's `@home` directory, loaded at runtime
- `yak-box stop` tears down container + Zellij tab and cleans up `.yak-boxes/` state

**Target audience:** Individual developers or small teams using the yakthang workflow with yx task tracking. Single-developer today.

---

## 3. Feature Comparison

| Feature | yak-box | OpenShell |
|---|---|---|
| **Primary purpose** | Task-orchestration worker launcher | Secure agent runtime / sandbox platform |
| **Task tracking integration** | Deep (yx-native, passes yak context to agents) | None (task-agnostic) |
| **Container runtime** | Docker (direct `docker run`) | Docker + K3s cluster inside Docker |
| **Non-container runtime** | Yes (native Zellij, no isolation) | No |
| **Sandboxing model** | Docker resource limits + tmpfs | Docker + K3s + seccomp + eBPF filesystem |
| **Network egress control** | None (Docker bridge, unrestricted outbound) | Yes — declarative L7 YAML policies, hot-reloadable |
| **Inference routing / proxy** | No (credentials injected directly) | Yes — strips/injects credentials, routes to controlled backends |
| **Credential injection** | API key env var or OAuth files mounted | Named providers injected as env vars at creation |
| **Resource profiles** | Yes (light/heavy/ram/default — static at spawn) | Not documented; container-level |
| **GPU support** | No | Yes (`--gpu` flag) |
| **Terminal / multiplexer** | Zellij tabs (each worker = one tab) | Own TUI (`openshell term`) + SSH into sandbox |
| **Policy system** | Static devcontainer.json + security warnings | Declarative YAML, hot-reloadable, enforced at proxy |
| **Community images** | devcontainer.json (local) | Community catalog (`--from openclaw`), BYOC |
| **Skill/extension model** | Skills as folders injected into worker @home | Agent skills in `.agents/skills/` for project automation |
| **Worktree / multi-repo** | Yes (cross-repo worktrees coordinated by yak) | No |
| **Security audit/validation** | Warns on dangerous devcontainer config | Enforces via policy engine (filesystem, process, network) |
| **Implementation language** | Go | Python |
| **Maturity** | In active use (yakthang daily driver) | Alpha — single-player |
| **Infrastructure weight** | Light (Docker + Zellij) | Heavy (K3s cluster in Docker, always-on gateway) |

---

## 4. Key Differences

**Different problems.** The most important difference: these tools solve adjacent but distinct problems. yak-box is a *workflow tool* — it knows about tasks, personas, skills, and workspaces. OpenShell is a *security platform* — it knows about policies, egress control, and credential isolation. They're not competitors; they occupy different layers.

**Network security.** OpenShell's L7 egress proxy is the most significant capability gap in yak-box. yak-box workers have unrestricted outbound network access inside the Docker bridge. An agent can call any API, exfiltrate data, or incur unexpected costs without any enforcement layer. OpenShell prevents this with per-sandbox policies (HTTP method + path + destination). This is genuinely valuable for any agent touching sensitive data.

**Inference routing.** OpenShell can proxy LLM API calls through a controlled endpoint — stripping the caller's credentials and injecting backend credentials. This decouples credential management from agent permissions and could enable cost attribution, rate limiting, and model-switching. yak-box passes credentials directly into the container environment.

**Infrastructure weight.** OpenShell's K3s-in-Docker gateway is an always-on cluster. That's significant overhead for a single developer. yak-box uses Docker and Zellij directly — much lighter, faster to spawn, zero infrastructure to maintain.

**Task integration.** yak-box understands yaks, contexts, assigned-to files, supervised-by, and agent-status. OpenShell is task-agnostic — it provides the sandbox; you bring the workflow.

**Hot-reload.** OpenShell policies can be updated on a running sandbox without restart. yak-box resource profiles are fixed at spawn time; changing them requires a stop + respawn.

---

## 5. Ideas Worth Borrowing

**1. Egress policy layer (high value, non-trivial effort)**
The declarative L7 egress YAML is compelling. For yakthang, a lightweight version — even just allowlist-based domain filtering via a MITM proxy in the Docker network — would prevent accidental data exfiltration and runaway API calls. Could be implemented as a `yak-shavers` network policy sidecar rather than a full K3s cluster.

**2. Inference routing / cost attribution (medium value)**
If yakthang scales to multiple developers or billing matters, routing LLM calls through a proxy that attributes cost per-yak would be very useful. OpenShell's privacy router architecture (strip caller creds, inject backend creds) is the right pattern. Not needed today.

**3. Named credential providers (low effort, nice UX)**
OpenShell's provider model (`openshell provider create --type claude --from-existing`) is cleaner than yak-box's current API-key-or-OAuth detection. A `yak-box auth create` subcommand that names and validates credential bundles could improve the setup experience — especially for teams with multiple agents.

**4. Community sandbox catalog (low priority)**
OpenShell's `--from openclaw` community image pattern is a nice UX improvement over yak-box's devcontainer.json approach. Worth watching; not worth building now when yak-box has a small user base.

**5. TUI monitoring dashboard (low priority)**
`openshell term` (k9s-inspired live view of sandboxes + policies) is useful at scale. yak-box's current approach (Zellij tabs + yak-map) is good enough for single-developer use. A dedicated monitoring view would matter more if running 10+ concurrent workers.

---

## 6. Recommendation: Park

**Park this investigation for now; re-evaluate in 6–12 months.**

OpenShell is alpha software solving an enterprise-grade problem that yakthang doesn't yet face. The infrastructure overhead (K3s gateway, persistent cluster) would add significant friction to a tool optimized for fast, lightweight worker spawning.

The one idea genuinely worth acting on today is **lightweight egress filtering** — not via OpenShell, but as a standalone addition to the yak-box Docker network. A simple allowlist-based DNS/HTTP proxy on the `yak-shavers` bridge would capture most of the value (preventing data exfiltration, runaway LLM costs) without the K3s complexity. That's a separate yak if and when it becomes a priority.

Watch OpenShell as it matures from alpha toward multi-tenant beta. If yakthang ever needs to run agents on behalf of multiple developers with different credential scopes, OpenShell's architecture becomes relevant.
