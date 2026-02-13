# Persistent VM Implementation Plan

**Status**: Ready for Review (Metis-reviewed)  
**Author**: Prometheus  
**Date**: 2026-02-13  
**Target**: Multi-tenant persistent Yak orchestration VM  
**Cloud Provider**: GCP Compute Engine

---

## Key Decisions (Resolved)

| Question | Decision | Rationale |
|----------|----------|-----------|
| **API Key Mechanism** | Environment variable (`-e ANTHROPIC_API_KEY`) | Simple, secure (yakob env → container) |
| **Worker Image Runtimes** | Per-project extends base | Minimal base image; projects add `.devcontainer/Dockerfile` |
| **Dependencies Strategy** | Full network for setup, then restricted | Two-phase: `--network bridge` during npm/pip install, then `--network none` |
| **Cloud Provider** | GCP Compute Engine | Good gcloud CLI integration |

---

## TL;DR

> **Quick Summary**: Transform the current local Zellij-based Yak orchestration system into a cloud-hosted persistent VM deployment with Docker-isolated workers. Three security layers: VM boundary, yakob user isolation, and ephemeral worker containers.
> 
> **Deliverables**:
> - Worker container image (Dockerfile + build scripts)
> - Modified spawn-worker.sh with Docker runtime support
> - VM provisioning script (setup-vm.sh)
> - Security hardening configuration
> - Operations documentation
> 
> **Estimated Effort**: Large (8-12 engineering days)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Phase 1 → Phase 2 → Phase 3 → Phase 4

---

## Context

### Original Request
Implement a persistent multi-tenant Yak orchestration VM that allows:
- Team members to connect remotely via SSH
- Workers to run in isolated Docker containers
- Shared .yaks/ state across containers
- Three-layer security model (VM, yakob user, containers)

### Current State Analysis

**What Works Well (Keep This)**:
- Clean separation: Orchestrator knows about yx; workers learn it inline
- Sub-repo purity: No orchestration files in sub-repos
- Shaver personalities: Random identity assignment creates engaging worker personas
- Plan/build modes: Two-phase workflow with hard permission boundaries
- Worker feedback protocol: Pull-based status via yx fields
- State management: .yaks/ shared directory with yx CLI

**What Changes for VM Deployment**:
- Worker spawning: `zellij action new-tab` → `docker run`
- Orchestrator lifecycle: Interactive Zellij session → systemd-managed session
- Network access: Full network → restricted/none
- Credentials: User SSH keys → layered credential management
- Resource limits: OS-level → explicit Docker constraints
- Monitoring: Direct yx field reads → Docker + yx status

### Research Findings

**From spawn-worker.sh analysis**:
- Currently spawns Zellij tabs with KDL layouts
- Injects yx instructions inline via WORKER_PROMPT
- Supports --mode plan|build with different behaviors
- Writes assigned-to field for task tracking

**From yx analysis**:
- File-based state in .yaks/ directory
- Supports concurrent access via atomic file operations
- Fields stored as plain files (e.g., .yaks/task/agent-status)

---

## Work Objectives

### Core Objective
Enable persistent, multi-tenant Yak orchestration on a cloud VM with Docker-isolated workers while maintaining backward compatibility with local Zellij mode.

### Concrete Deliverables
- `worker.Dockerfile` - Generic worker container image
- Modified `spawn-worker.sh` - Docker runtime support with auto-detection
- `setup-vm.sh` - VM provisioning script
- `kill-worker.sh` - Worker container management
- `cleanup-workers.sh` - Container cleanup utilities
- Enhanced `check-workers.sh` - Docker + yx status integration
- Documentation: DEPLOYMENT.md, OPERATIONS.md, SECURITY.md

### Definition of Done
- [ ] `RUNTIME=docker ./spawn-worker.sh` spawns Docker container
- [ ] Workers can read/write .yaks/ state from inside containers
- [ ] Workers can commit to git using yakob's identity
- [ ] `setup-vm.sh` provisions fresh Ubuntu VM cleanly
- [ ] Team members can SSH and attach to Zellij orchestrator
- [ ] Security checklist 100% complete

### Must Have
- Docker runtime detection and container spawning
- UID/GID mapping for file permissions
- Resource limits (CPU, memory, pids)
- Network isolation (--network none)
- Systemd service for orchestrator lifecycle
- Layered credential management

### Must NOT Have (Guardrails)
- DO NOT break existing Zellij mode (backward compatibility required)
- DO NOT put credentials in Docker images
- DO NOT give workers network access by default (except during setup phase)
- DO NOT give yakob user sudo privileges
- DO NOT push to remote git (workers commit locally only)
- DO NOT hardcode UID in Dockerfile (use runtime `--user $(id -u):$(id -g)`)
- DO NOT use `--read-only` without `--tmpfs /home/worker/.cache` (opencode needs cache)
- DO NOT modify spawn-worker.sh argument parsing for Zellij mode (Docker branch is additive)
- DO NOT implement AppArmor/SELinux in this plan (defer to Phase 2 Security)
- DO NOT add monitoring/alerting infrastructure (defer to follow-on work)

### Deferred to Phase 2 (Out of Scope)
- AppArmor/SELinux container profiles
- Trivy/Docker Content Trust image scanning
- Centralized logging (ELK/Loki)
- Monitoring dashboards (Grafana/Prometheus)
- Multi-VM orchestration

---

## Architecture

### Three Security Layers

```
┌─────────────────────────────────────────────────────┐
│ VM (GCP Compute Engine)                             │
│ • Firewall: SSH (22) only                          │
│ • Team cloud credentials (root-owned, read-only)   │
│ • GCP OS Login for team SSH access                 │
│ └─────────────────────────────────────────────────┐│
│   │ yakob user (non-root)                         ││
│   │ • No sudo access                              ││
│   │ • Docker group membership only                ││
│   │ • Git: commit locally, no push credentials    ││
│   │ • ANTHROPIC_API_KEY in environment            ││
│   │ • Runs: Zellij orchestrator + spawn-worker.sh ││
│   └───────────────────────────────────────────────┐││
│     │ Worker Containers (ephemeral)               │││
│     │ • Non-root user (runtime UID mapping)       │││
│     │ • Two-phase network: bridge→none            │││
│     │ • API key via -e flag from yakob env        │││
│     │ • Read-write workspace mount                │││
│     │ • Shared .yaks/ volume                      │││
│     │ • Resource limits + 2hr timeout             │││
│     │ • tmpfs for /home/worker/.cache             │││
│     └─────────────────────────────────────────────┘││
└─────────────────────────────────────────────────────┘
```

### Two-Phase Network Strategy

Workers use a two-phase network approach for dependency installation:

```
Phase 1 (Setup): --network bridge
  └── npm install / pip install / cargo build
  └── Duration: Until deps installed
  
Phase 2 (Work): --network none  
  └── Actual task execution
  └── No external network access
```

This is controlled via `--setup-network` flag in spawn-worker.sh.

### Data Flow

```
Team Member                    Persistent VM                     Workers
    │                              │                               │
    ├──SSH─────────────────────────>│                               │
    │                              │                               │
    ├──zellij attach──────────────>│ (yakob user)                  │
    │                              │                               │
    │                         ┌────┴────┐                          │
    │                         │ Yakob   │                          │
    │                         │ (orch)  │                          │
    │                         └────┬────┘                          │
    │                              │                               │
    │                              ├──spawn-worker.sh─────────────>│
    │                              │   docker run ...              │
    │                              │                          ┌────┴────┐
    │                              │                          │ Worker  │
    │                              │                          │(opencode)│
    │                              │                          └────┬────┘
    │                              │                               │
    │                              │<──────yx field (status)───────┤
    │                              │                               │
    │<────yx ls (via Zellij)───────┤                               │
```

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO (new Docker setup)
- **Automated tests**: Tests-after (bash script integration tests)
- **Framework**: Bash scripts with assertions

### Agent-Executed QA Scenarios

Every task includes QA scenarios that the executing agent will run directly using Bash commands to verify functionality. No human intervention required.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately):
└── Task 1: Worker Container Image (no dependencies)

Wave 2 (After Wave 1):
└── Task 2: Docker-based spawn-worker.sh (depends: 1)

Wave 3 (After Wave 2):
├── Task 3: Shared State & Volumes (depends: 2)
├── Task 4: Container Networking (depends: 2)
└── Task 5: Resource Limits (depends: 2)

Wave 4 (After Wave 3):
├── Task 6: VM Provisioning Script (depends: 3, 4, 5)
└── Task 7: Credential Management (depends: 3)

Wave 5 (After Wave 4):
├── Task 8: Orchestrator Lifecycle (depends: 6, 7)
├── Task 9: Security Hardening (depends: 6)
└── Task 10: Worker Lifecycle Management (depends: 2, 6)

Wave 6 (After Wave 5):
└── Task 11: Documentation (depends: all)

Critical Path: 1 → 2 → 3 → 6 → 8 → 11
```

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|---------------------|
| 1 | None | 2 | None |
| 2 | 1 | 3, 4, 5 | None |
| 3 | 2 | 6, 7 | 4, 5 |
| 4 | 2 | 6 | 3, 5 |
| 5 | 2 | 6 | 3, 4 |
| 6 | 3, 4, 5 | 8, 9, 10 | 7 |
| 7 | 3 | 8 | 6 |
| 8 | 6, 7 | 11 | 9, 10 |
| 9 | 6 | 11 | 8, 10 |
| 10 | 2, 6 | 11 | 8, 9 |
| 11 | All | None | None |

---

## TODOs

### Task Tracking with yx (MANDATORY)

**Every agent MUST track their work using yx. This is NOT optional.**

**FIRST thing when starting ANY task:**
```bash
yx state <task-path> wip
echo "wip: starting <description>" | yx field <task-path> agent-status
```

**During work (update periodically):**
```bash
echo "wip: <current activity>" | yx field <task-path> agent-status
```

**LAST thing when task is complete:**
```bash
yx done <task-path>
echo "done: <summary of what was accomplished>" | yx field <task-path> agent-status
```

**If blocked:**
```bash
echo "blocked: <reason>" | yx field <task-path> agent-status
```

> ⚠️ **All task "What to do" sections begin with START (yx wip) and end with END (yx done).
> Agents MUST execute these steps.**

**Task Path Mapping:**

| Plan Task | yx Task Path |
|-----------|--------------|
| Task 1: Worker Container Image | `persistent-vm/worker-container-image` |
| Task 2: Docker spawn-worker.sh | `persistent-vm/docker-spawn-worker` |
| Task 3: Shared State & Volumes | `persistent-vm/shared-state-volumes` |
| Task 4: Container Networking | `persistent-vm/container-networking` |
| Task 5: Resource Limits | `persistent-vm/resource-limits` |
| Task 6: VM Provisioning Script | `persistent-vm/vm-provisioning` |
| Task 7: Credential Management | `persistent-vm/credential-management` |
| Task 8: Orchestrator Lifecycle | `persistent-vm/orchestrator-lifecycle` |
| Task 9: Security Hardening | `persistent-vm/security-hardening` |
| Task 10: Worker Lifecycle | `persistent-vm/worker-lifecycle` |
| Task 11: Documentation | `persistent-vm/documentation` |

---

### Task 1: Worker Container Image
**yx task:** `persistent-vm/worker-container-image`

- [x] 1. Create minimal worker.Dockerfile with OpenCode and yx

  **What to do**:
  1. **START**: `yx state persistent-vm/worker-container-image wip` + report `wip: starting`
  2. Create `worker.Dockerfile` in repository root (MINIMAL base image)
  3. Base on `ubuntu:24.04`
  4. Install ONLY: ca-certificates, curl, git, bash
  5. Install OpenCode CLI via install script
  6. Install yx via install script
  7. DO NOT include language runtimes (Node, Python, Go) - projects extend base
  8. Create non-root `worker` user (UID set at runtime via --user flag, NOT hardcoded)
  9. Set ENTRYPOINT to opencode
  10. Add LABEL for extension pattern documentation
  11. Test image builds and runs
  12. **END**: `yx done persistent-vm/worker-container-image` + report `done: <summary>`

  **Per-Project Extension Pattern** (document in Dockerfile comments):
  ```dockerfile
  # Projects can extend this base image:
  # FROM yak-worker:latest
  # RUN apt-get update && apt-get install -y nodejs npm
  # Or use .devcontainer/Dockerfile in project root
  ```

  **Must NOT do**:
  - DO NOT include any credentials or API keys in image
  - DO NOT install language runtimes (Node, Python, Go, Rust) - keep minimal
  - DO NOT hardcode UID (use runtime --user flag instead)
  - DO NOT use root user as default

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file creation with well-defined structure
  - **Skills**: [`git-master`]
    - `git-master`: For committing the Dockerfile

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (alone)
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:
  - `spawn-worker.sh:1-50` - Understand what tools workers need (opencode, yx)
  - `CLAUDE.md:85-103` - Worker spawning patterns and requirements
  - Official Docker documentation for multi-stage builds

  **Acceptance Criteria**:

  ```
  Scenario: Worker image builds successfully
    Tool: Bash
    Preconditions: Docker installed and running
    Steps:
      1. docker build -f worker.Dockerfile -t yak-worker:latest .
      2. Assert: Exit code 0
      3. Assert: Image exists in docker images output
    Expected Result: Image built without errors
    Evidence: Build log captured

  Scenario: Worker container runs opencode
    Tool: Bash
    Preconditions: yak-worker:latest image exists
    Steps:
      1. docker run --rm yak-worker:latest --version
      2. Assert: Exit code 0
      3. Assert: Output contains version number
    Expected Result: OpenCode CLI responds
    Evidence: Version output captured

  Scenario: Worker user is non-root (uses runtime UID)
    Tool: Bash
    Preconditions: yak-worker:latest image exists
    Steps:
      1. docker run --rm --user $(id -u):$(id -g) yak-worker:latest id
      2. Assert: Output shows current user's UID (not root)
      3. Assert: Output does NOT show uid=0(root)
    Expected Result: Container runs as mapped non-root user
    Evidence: id command output captured

  Scenario: yx is available in container
    Tool: Bash
    Preconditions: yak-worker:latest image exists
    Steps:
      1. docker run --rm --entrypoint yx yak-worker:latest --version
      2. Assert: Exit code 0
    Expected Result: yx CLI responds
    Evidence: Version output captured

  Scenario: opencode works without TTY (Metis-added validation)
    Tool: Bash
    Preconditions: yak-worker:latest image exists
    Steps:
      1. docker run --rm -d yak-worker:latest --agent build --prompt "echo hello"
      2. Wait for container to complete (docker wait)
      3. docker logs <container>
      4. Assert: No "TTY required" or "interactive" errors
      5. Assert: Container exited successfully (exit code 0 or expected agent exit)
    Expected Result: opencode runs in detached mode without TTY issues
    Evidence: Container logs showing successful execution
  ```

  **Commit**: YES
  - Message: `feat(docker): add minimal worker container image with opencode and yx`
  - Files: `worker.Dockerfile`
  - Pre-commit: `docker build -f worker.Dockerfile -t yak-worker:latest .`

---

### Task 2: Docker-based spawn-worker.sh
**yx task:** `persistent-vm/docker-spawn-worker`

- [x] 2. Add Docker runtime support to spawn-worker.sh

  **What to do**:
  1. **START**: `yx state persistent-vm/docker-spawn-worker wip` + report `wip: starting`
  2. Add runtime detection at top of script (Docker vs Zellij)
  3. Add RUNTIME environment variable override option
  4. Implement Docker mode branch with docker run command
  5. Mount workspace root and .yaks/ directory
  6. Pass environment variables (WORKER_NAME, WORKER_EMOJI, YAK_PATH, ANTHROPIC_API_KEY)
  7. Apply resource limits (--cpus, --memory, --pids-limit)
  8. Apply security options (--network none, --security-opt no-new-privileges)
  9. Add --tmpfs /home/worker/.cache for opencode cache
  10. Use --detach for background execution
  11. Use runtime UID mapping: --user $(id -u):$(id -g)
  12. Maintain existing Zellij mode unchanged
  13. **END**: `yx done persistent-vm/docker-spawn-worker` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT break existing Zellij mode
  - DO NOT give workers network access (except with --setup-network)
  - DO NOT run containers as root
  - DO NOT hardcode paths (use git rev-parse for workspace root)
  - DO NOT hardcode UID (use runtime --user flag)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Core infrastructure change requiring careful modification of existing script
  - **Skills**: [`git-master`]
    - `git-master`: For committing changes

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (alone)
  - **Blocks**: Tasks 3, 4, 5
  - **Blocked By**: Task 1

  **References**:
  - `spawn-worker.sh:1-218` - Current implementation to modify
  - `spawn-worker.sh:47-83` - Argument parsing (add new flags here)
  - `spawn-worker.sh:99-163` - Mode-specific prompt construction
  - `spawn-worker.sh:183-216` - Zellij spawning (parallel Docker implementation)
  - `docs/plan-build-modes.md:1-96` - Plan vs build mode documentation
  - `check-workers.sh:1-64` - How workers are monitored (ensure compatibility)

  **Acceptance Criteria**:

  ```
  Scenario: Docker runtime is auto-detected when Docker available
    Tool: Bash
    Preconditions: Docker installed and user in docker group
    Steps:
      1. Create test script that sources spawn-worker.sh runtime detection
      2. Assert: RUNTIME variable equals "docker"
    Expected Result: Docker mode auto-selected
    Evidence: Variable value captured

  Scenario: Zellij runtime is auto-detected when Docker unavailable
    Tool: Bash
    Preconditions: Docker not available or user not in docker group, Zellij installed
    Steps:
      1. Run spawn-worker.sh with PATH excluding docker
      2. Assert: RUNTIME variable equals "zellij"
    Expected Result: Zellij mode auto-selected
    Evidence: Variable value captured

  Scenario: RUNTIME env var overrides auto-detection
    Tool: Bash
    Preconditions: Both Docker and Zellij available
    Steps:
      1. RUNTIME=zellij ./spawn-worker.sh --cwd . --name test "echo test"
      2. Assert: No docker commands executed
      3. Assert: Zellij tab created (or attempted)
    Expected Result: Override respected
    Evidence: Command execution log

  Scenario: Docker container spawns with correct flags
    Tool: Bash
    Preconditions: Docker available, yak-worker:latest image exists
    Steps:
      1. Create .yaks/ directory if not exists
      2. RUNTIME=docker ./spawn-worker.sh --cwd . --name "test-worker" "echo hello"
      3. docker ps -a --filter "name=yak-worker-" --format "{{.Names}}"
      4. Assert: Container name matches pattern yak-worker-*
      5. docker inspect <container> --format '{{.HostConfig.NetworkMode}}'
      6. Assert: NetworkMode equals "none"
    Expected Result: Container spawned with security flags
    Evidence: docker inspect output captured

  Scenario: Worker can access .yaks/ directory
    Tool: Bash
    Preconditions: Docker available, .yaks/ exists with test task
    Steps:
      1. mkdir -p .yaks/test-docker
      2. echo "todo" > .yaks/test-docker/state
      3. RUNTIME=docker ./spawn-worker.sh --cwd . --name "volume-test" "yx ls"
      4. Wait for container to complete (docker wait)
      5. docker logs <container>
      6. Assert: Output shows test-docker task
    Expected Result: Container can read .yaks/
    Evidence: Container logs captured

  Scenario: Worker receives ANTHROPIC_API_KEY (Metis-added)
    Tool: Bash
    Preconditions: Docker available, ANTHROPIC_API_KEY set in yakob env
    Steps:
      1. export ANTHROPIC_API_KEY="test-key-12345"
      2. RUNTIME=docker ./spawn-worker.sh --cwd . --name "key-test" "printenv ANTHROPIC_API_KEY"
      3. Wait for container to complete (docker wait)
      4. docker logs <container>
      5. Assert: Output contains "test-key-12345"
    Expected Result: API key passed to container via -e flag
    Evidence: Container logs showing key value

  Scenario: Container uses runtime UID/GID mapping (Metis-added)
    Tool: Bash
    Preconditions: Docker available
    Steps:
      1. RUNTIME=docker ./spawn-worker.sh --cwd . --name "uid-test" "id"
      2. docker logs <container>
      3. Assert: UID matches host yakob user (not hardcoded 1001)
    Expected Result: Runtime UID mapping works
    Evidence: id command output
  ```

  **Commit**: YES
  - Message: `feat(spawn-worker): add Docker runtime support with auto-detection`
  - Files: `spawn-worker.sh`
  - Pre-commit: `RUNTIME=docker ./spawn-worker.sh --cwd . --name test "echo test" && docker ps -a | grep yak-worker`

---

### Task 3: Shared State & Volumes
**yx task:** `persistent-vm/shared-state-volumes`

- [ ] 3. Configure volume mounting for .yaks/ and workspace

  **What to do**:
  1. **START**: `yx state persistent-vm/shared-state-volumes wip` + report `wip: starting`
  2. Verify bind mount configuration in spawn-worker.sh
  3. Add git config mount (read-only) for worker identity
  4. Test UID/GID matching between host and container
  5. Test concurrent access from multiple workers
  6. Document volume mount strategy
  7. **END**: `yx done persistent-vm/shared-state-volumes` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT use Docker named volumes (use bind mounts)
  - DO NOT allow workers to modify git config

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Configuration verification and minor additions
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 4, 5)
  - **Blocks**: Tasks 6, 7
  - **Blocked By**: Task 2

  **References**:
  - `spawn-worker.sh` - Volume mount implementation
  - `.yaks/` directory structure
  - `docs/task-management.md:79-95` - .yaks/ storage explanation

  **Acceptance Criteria**:

  ```
  Scenario: Worker can write to .yaks/ state
    Tool: Bash
    Preconditions: Docker mode working
    Steps:
      1. Create task: mkdir -p .yaks/test-write && echo "todo" > .yaks/test-write/state
      2. Spawn worker that runs: echo "done" > .yaks/test-write/state
      3. Wait for container completion
      4. cat .yaks/test-write/state
      5. Assert: Contents equal "done"
    Expected Result: Worker successfully wrote state
    Evidence: State file contents

  Scenario: Multiple workers can access .yaks/ concurrently
    Tool: Bash
    Preconditions: Docker mode working
    Steps:
      1. Create 3 test tasks
      2. Spawn 3 workers in parallel (background)
      3. Each worker marks its task done
      4. Wait for all containers
      5. Verify all 3 tasks show done state
    Expected Result: No corruption, all tasks completed
    Evidence: yx ls output showing all done

  Scenario: Worker inherits git config for commits
    Tool: Bash
    Preconditions: Docker mode with git config mount
    Steps:
      1. Spawn worker that runs: git config user.name
      2. Assert: Output matches yakob's configured name
    Expected Result: Git identity inherited
    Evidence: Git config output
  ```

  **Commit**: YES (if changes needed)
  - Message: `feat(volumes): add git config mount and verify .yaks/ access`
  - Files: `spawn-worker.sh`

---

### Task 4: Container Networking
**yx task:** `persistent-vm/container-networking`

- [ ] 4. Implement two-phase network isolation policy

  **What to do**:
  1. **START**: `yx state persistent-vm/container-networking wip` + report `wip: starting`
  2. Implement two-phase network strategy:
     - Phase 1 (Setup): `--network bridge` for npm/pip install
     - Phase 2 (Work): `--network none` for actual task execution
  3. Add `--setup-network` flag to spawn-worker.sh for Phase 1
  4. Default remains `--network none` (Phase 2 behavior)
  5. Document network policy decisions and two-phase rationale
  6. Test that workers cannot make external connections in Phase 2
  7. **END**: `yx done persistent-vm/container-networking` + report `done: <summary>`

  **Two-Phase Implementation**:
  ```bash
  # Phase 1: Setup with network (for dependency installation)
  ./spawn-worker.sh --setup-network --cwd ./project --name "setup" "npm install"
  
  # Phase 2: Work without network (default)
  ./spawn-worker.sh --cwd ./project --name "build" "npm run build"
  ```

  **Must NOT do**:
  - DO NOT enable network by default for work tasks
  - DO NOT allow arbitrary network access during work phase

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Verification and minor flag addition
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 3, 5)
  - **Blocks**: Task 6
  - **Blocked By**: Task 2

  **References**:
  - `.yaks/persistent-vm/container-networking/context.md` - Requirements
  - `spawn-worker.sh` - Docker run command

  **Acceptance Criteria**:

  ```
  Scenario: Workers have no network access by default
    Tool: Bash
    Preconditions: Docker mode working
    Steps:
      1. Spawn worker that attempts: curl -s --max-time 5 https://google.com
      2. Wait for container
      3. Assert: curl command failed (exit code non-zero or timeout)
    Expected Result: Network request fails
    Evidence: Container logs showing failure

  Scenario: Network mode flag works when specified
    Tool: Bash
    Preconditions: spawn-worker.sh supports --network-mode flag
    Steps:
      1. ./spawn-worker.sh --network-mode bridge --cwd . --name net-test "curl -s https://google.com"
      2. Wait for container
      3. Assert: curl succeeded (if bridge mode implemented)
    Expected Result: Bridge mode allows network (optional feature)
    Evidence: Container logs
  ```

  **Commit**: YES
  - Message: `feat(network): verify and document network isolation policy`
  - Files: `spawn-worker.sh`, `docs/SECURITY.md`

---

### Task 5: Resource Limits
**yx task:** `persistent-vm/resource-limits`

- [ ] 5. Implement resource limit profiles

  **What to do**:
  1. **START**: `yx state persistent-vm/resource-limits wip` + report `wip: starting`
  2. Add --resources flag to spawn-worker.sh (light|default|heavy)
  3. Implement CPU limits: 0.5/1.0/2.0 cores
  4. Implement memory limits: 1g/2g/4g
  5. Implement pids-limit: 256/512/1024
  6. Add --stop-timeout 7200 for 2-hour timeout
  7. Test resource limits are enforced
  8. **END**: `yx done persistent-vm/resource-limits` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT allow unlimited resources
  - DO NOT set limits too low for basic operation

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Flag addition with predefined values
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 3, 4)
  - **Blocks**: Task 6
  - **Blocked By**: Task 2

  **References**:
  - `.yaks/persistent-vm/resource-limits/context.md` - Requirements
  - `spawn-worker.sh` - Docker run command

  **Acceptance Criteria**:

  ```
  Scenario: Default resource limits applied
    Tool: Bash
    Preconditions: Docker mode working
    Steps:
      1. Spawn worker with default settings
      2. docker inspect <container> --format '{{.HostConfig.NanoCpus}}'
      3. Assert: CPU limit corresponds to 1.0 cores (1000000000 nanocpus)
      4. docker inspect <container> --format '{{.HostConfig.Memory}}'
      5. Assert: Memory limit is 2GB (2147483648 bytes)
    Expected Result: Default limits applied
    Evidence: docker inspect output

  Scenario: Heavy resource profile applies correct limits
    Tool: Bash
    Preconditions: spawn-worker.sh supports --resources flag
    Steps:
      1. ./spawn-worker.sh --resources heavy --cwd . --name heavy-test "sleep 5"
      2. docker inspect <container> --format '{{.HostConfig.NanoCpus}}'
      3. Assert: CPU limit corresponds to 2.0 cores
      4. docker inspect <container> --format '{{.HostConfig.Memory}}'
      5. Assert: Memory limit is 4GB
    Expected Result: Heavy limits applied
    Evidence: docker inspect output
  ```

  **Commit**: YES
  - Message: `feat(resources): add resource limit profiles (light/default/heavy)`
  - Files: `spawn-worker.sh`

---

### Task 6: VM Provisioning Script
**yx task:** `persistent-vm/vm-provisioning`

- [ ] 6. Create setup-vm.sh provisioning script for GCP

  **What to do**:
  1. **START**: `yx state persistent-vm/vm-provisioning wip` + report `wip: starting`
  2. Create `setup-vm.sh` that runs as root on fresh Ubuntu 24.04 (GCP Compute Engine)
  3. Install: Docker Engine, OpenCode CLI, yx, git, zellij, watch, jq
  4. Create yakob user with docker group membership
  5. Set up yakob's git config (name/email only)
  6. Create /home/yakob/workspace directory
  7. Build worker container image
  8. Create systemd service file for orchestrator
  9. Make script idempotent (can run multiple times safely)
  10. Add GCP-specific notes (OS Login, metadata, firewall)
  11. **END**: `yx done persistent-vm/vm-provisioning` + report `done: <summary>`

  **GCP Deployment Notes** (document in script comments):
  ```bash
  # Create VM:
  # gcloud compute instances create yak-orchestrator \
  #   --zone=us-central1-a \
  #   --machine-type=e2-standard-2 \
  #   --image-family=ubuntu-2404-lts-amd64 \
  #   --image-project=ubuntu-os-cloud \
  #   --boot-disk-size=50GB
  #
  # Copy and run:
  # gcloud compute scp setup-vm.sh yak-orchestrator:~ --zone=us-central1-a
  # gcloud compute ssh yak-orchestrator --zone=us-central1-a -- sudo bash setup-vm.sh
  ```

  **Must NOT do**:
  - DO NOT include credentials in script
  - DO NOT give yakob sudo access
  - DO NOT start services automatically (leave for manual step)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Complex multi-step provisioning script
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 7)
  - **Blocks**: Tasks 8, 9, 10
  - **Blocked By**: Tasks 3, 4, 5

  **References**:
  - `.yaks/persistent-vm/vm-provisioning/context.md` - Requirements
  - `orchestrator.kdl` - Layout file that systemd will reference
  - GCP Compute Engine documentation
  - Ubuntu 24.04 Docker installation docs

  **Acceptance Criteria**:

  ```
  Scenario: Script syntax is valid
    Tool: Bash
    Preconditions: setup-vm.sh exists
    Steps:
      1. bash -n setup-vm.sh
      2. Assert: Exit code 0
    Expected Result: No syntax errors
    Evidence: Validation output

  Scenario: Script is idempotent (dry-run check)
    Tool: Bash
    Preconditions: setup-vm.sh exists
    Steps:
      1. Review script for idempotent patterns (useradd -m checks, apt-get install -y)
      2. Assert: All package installs use -y
      3. Assert: User creation checks if user exists first
    Expected Result: Script can run multiple times
    Evidence: Script review findings

  Scenario: setup-vm.sh provisions GCP VM correctly (Metis-added)
    Tool: Bash (with gcloud)
    Preconditions: GCP project configured, gcloud authenticated
    Steps:
      1. Create fresh Ubuntu 24.04 VM: gcloud compute instances create test-yak-vm ...
      2. Copy setup-vm.sh: gcloud compute scp setup-vm.sh test-yak-vm:~
      3. Run: gcloud compute ssh test-yak-vm -- sudo bash setup-vm.sh
      4. Assert: yakob user exists (id yakob)
      5. Assert: yakob in docker group (groups yakob | grep docker)
      6. Assert: docker works as yakob (sudo -u yakob docker ps)
      7. Assert: Worker image exists (docker images | grep yak-worker)
      8. Assert: Systemd service file exists
      9. Cleanup: gcloud compute instances delete test-yak-vm --quiet
    Expected Result: VM fully provisioned
    Evidence: Command outputs captured
  ```

  **Commit**: YES
  - Message: `feat(vm): add setup-vm.sh provisioning script`
  - Files: `setup-vm.sh`
  - Pre-commit: `bash -n setup-vm.sh`

---

### Task 7: Credential Management
**yx task:** `persistent-vm/credential-management`

- [ ] 7. Document credential management layers

  **What to do**:
  1. **START**: `yx state persistent-vm/credential-management wip` + report `wip: starting`
  2. Document three credential layers in SECURITY.md
  3. Create /etc/yak-creds/ directory structure documentation
  4. Document API key handling (environment variable approach)
  5. Add credential setup steps to DEPLOYMENT.md
  6. **END**: `yx done persistent-vm/credential-management` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT include actual credentials in documentation
  - DO NOT create files with placeholder credentials

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation task
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 6)
  - **Blocks**: Task 8
  - **Blocked By**: Task 3

  **References**:
  - `.yaks/persistent-vm/credential-management/context.md` - Requirements

  **Acceptance Criteria**:

  ```
  Scenario: Documentation covers all credential layers
    Tool: Bash
    Preconditions: docs/SECURITY.md exists
    Steps:
      1. grep -c "Layer 1\|Layer 2\|Layer 3" docs/SECURITY.md
      2. Assert: Count >= 3
      3. grep "yakob" docs/SECURITY.md
      4. Assert: yakob user credentials documented
    Expected Result: All layers documented
    Evidence: grep output
  ```

  **Commit**: YES
  - Message: `docs(security): add credential management documentation`
  - Files: `docs/SECURITY.md`

---

### Task 8: Orchestrator Lifecycle
**yx task:** `persistent-vm/orchestrator-lifecycle`

- [ ] 8. Create systemd service and team access documentation

  **What to do**:
  1. **START**: `yx state persistent-vm/orchestrator-lifecycle wip` + report `wip: starting`
  2. Create systemd service file template in setup-vm.sh
  3. Document service start/stop/status commands
  4. Document team SSH + zellij attach workflow
  5. Document multi-user session attachment
  6. **END**: `yx done persistent-vm/orchestrator-lifecycle` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT include API keys directly in service file
  - DO NOT auto-start service on install

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Service file and documentation
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 9, 10)
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 6, 7

  **References**:
  - `.yaks/persistent-vm/orchestrator-lifecycle/context.md` - Requirements
  - `orchestrator.kdl` - Layout to reference in service

  **Acceptance Criteria**:

  ```
  Scenario: Systemd service file syntax valid
    Tool: Bash
    Preconditions: setup-vm.sh contains service file
    Steps:
      1. Extract service file content from setup-vm.sh
      2. systemd-analyze verify (if available) or syntax check
    Expected Result: Valid service file
    Evidence: Analysis output

  Scenario: Team member can SSH and attach to orchestrator (Metis-added)
    Tool: Bash
    Preconditions: VM provisioned, orchestrator running
    Steps:
      1. SSH to VM as team-user (gcloud compute ssh or direct SSH)
      2. Run: zellij attach yak-orchestrator
      3. Assert: Zellij session displays with orchestrator layout
      4. Assert: yx ls command works and shows tasks
      5. Detach with Ctrl+o, d
    Expected Result: Multi-user Zellij attach works
    Evidence: Screenshot or session output

  Scenario: Orchestrator survives SSH disconnect
    Tool: Bash
    Preconditions: VM provisioned, orchestrator running, team attached
    Steps:
      1. SSH to VM and attach to orchestrator
      2. Disconnect SSH abruptly (close terminal)
      3. Re-SSH and re-attach
      4. Assert: Orchestrator still running
      5. Assert: Any running workers still executing
    Expected Result: Zellij session persists across disconnects
    Evidence: Process list showing zellij still running
  ```

  **Commit**: YES
  - Message: `feat(lifecycle): add systemd service and operations docs`
  - Files: `setup-vm.sh`, `docs/OPERATIONS.md`

---

### Task 9: Security Hardening
**yx task:** `persistent-vm/security-hardening`

- [ ] 9. Add VM and container security hardening

  **What to do**:
  1. **START**: `yx state persistent-vm/security-hardening wip` + report `wip: starting`
  2. Add UFW firewall setup to setup-vm.sh
  3. Add SSH hardening configuration
  4. Add fail2ban installation
  5. Add Docker daemon configuration (/etc/docker/daemon.json)
  6. Add container security flags to spawn-worker.sh (--cap-drop ALL, --read-only)
  7. Document security checklist
  8. **END**: `yx done persistent-vm/security-hardening` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT disable security features for convenience
  - DO NOT allow password authentication

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Security-critical configuration
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 8, 10)
  - **Blocks**: Task 11
  - **Blocked By**: Task 6

  **References**:
  - `.yaks/persistent-vm/security-hardening/context.md` - Requirements
  - Docker security best practices documentation

  **Acceptance Criteria**:

  ```
  Scenario: Container drops all capabilities
    Tool: Bash
    Preconditions: spawn-worker.sh updated with security flags
    Steps:
      1. Spawn worker
      2. docker inspect <container> --format '{{.HostConfig.CapDrop}}'
      3. Assert: Output contains "ALL" or lists all capabilities
    Expected Result: Capabilities dropped
    Evidence: docker inspect output

  Scenario: Container has read-only rootfs
    Tool: Bash
    Preconditions: spawn-worker.sh updated
    Steps:
      1. Spawn worker
      2. docker inspect <container> --format '{{.HostConfig.ReadonlyRootfs}}'
      3. Assert: Output is "true"
    Expected Result: Read-only root
    Evidence: docker inspect output

  Scenario: Worker cannot escape container (Metis-added)
    Tool: Bash
    Preconditions: spawn-worker.sh with security flags
    Steps:
      1. Spawn worker that attempts: mount /dev/sda1 /mnt 2>&1 || echo "BLOCKED"
      2. docker logs <container>
      3. Assert: Output contains "BLOCKED" or "permission denied" or "Operation not permitted"
      4. Spawn worker that attempts: chroot / /bin/bash 2>&1 || echo "BLOCKED"
      5. Assert: Output contains "BLOCKED" or "Operation not permitted"
    Expected Result: Privileged operations fail
    Evidence: Container logs showing permission denied

  Scenario: Container has tmpfs for cache (Metis-added)
    Tool: Bash
    Preconditions: spawn-worker.sh updated
    Steps:
      1. Spawn worker
      2. docker inspect <container> --format '{{.HostConfig.Tmpfs}}'
      3. Assert: Output contains "/home/worker/.cache"
    Expected Result: Cache directory is tmpfs (writable despite read-only rootfs)
    Evidence: docker inspect output
  ```

  **Commit**: YES
  - Message: `feat(security): add VM and container hardening`
  - Files: `setup-vm.sh`, `spawn-worker.sh`, `docs/SECURITY.md`

---

### Task 10: Worker Lifecycle Management
**yx task:** `persistent-vm/worker-lifecycle`

- [ ] 10. Create worker management scripts

  **What to do**:
  1. **START**: `yx state persistent-vm/worker-lifecycle wip` + report `wip: starting`
  2. Create `kill-worker.sh` to stop specific worker containers
  3. Create `cleanup-workers.sh` to prune stopped containers
  4. Enhance `check-workers.sh` to show Docker container status
  5. Add timeout mechanism to spawn-worker.sh (2 hour max using --stop-timeout)
  6. **END**: `yx done persistent-vm/worker-lifecycle` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT kill workers without preserving yx state
  - DO NOT auto-cleanup running containers

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Utility scripts with clear requirements
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 8, 9)
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 2, 6

  **References**:
  - `.yaks/persistent-vm/worker-lifecycle/context.md` - Requirements
  - `check-workers.sh` - Current implementation to enhance

  **Acceptance Criteria**:

  ```
  Scenario: kill-worker.sh stops named worker
    Tool: Bash
    Preconditions: Worker container running
    Steps:
      1. Spawn worker named "test-kill"
      2. ./kill-worker.sh test-kill
      3. docker ps --filter "name=yak-worker-test-kill"
      4. Assert: No running container found
    Expected Result: Worker stopped
    Evidence: docker ps output

  Scenario: check-workers.sh shows Docker status
    Tool: Bash
    Preconditions: Worker containers running
    Steps:
      1. Spawn 2 workers
      2. ./check-workers.sh
      3. Assert: Output contains "Running Workers (Docker)" section
      4. Assert: Output contains both worker names
    Expected Result: Docker status integrated
    Evidence: check-workers.sh output
  ```

  **Commit**: YES
  - Message: `feat(lifecycle): add worker management scripts`
  - Files: `kill-worker.sh`, `cleanup-workers.sh`, `check-workers.sh`, `spawn-worker.sh`

---

### Task 11: Documentation
**yx task:** `persistent-vm/documentation`

- [ ] 11. Create comprehensive documentation

  **What to do**:
  1. **START**: `yx state persistent-vm/documentation wip` + report `wip: starting`
  2. Create `docs/deployment/DEPLOYMENT.md` - VM provisioning guide
  3. Create `docs/deployment/OPERATIONS.md` - Day-to-day operations
  4. Create `docs/deployment/SECURITY.md` - Security architecture
  5. Create `docs/deployment/TROUBLESHOOTING.md` - Common issues
  6. Create `docs/development/DOCKER-MODE.md` - Local Docker testing
  7. Update main docs to reference new files
  8. **END**: `yx done persistent-vm/documentation` + report `done: <summary>`

  **Must NOT do**:
  - DO NOT include credentials or secrets
  - DO NOT duplicate information (link instead)

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation creation
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 6 (final)
  - **Blocks**: None (final task)
  - **Blocked By**: All previous tasks

  **References**:
  - `.yaks/persistent-vm/documentation/context.md` - Requirements
  - All other persistent-vm task contexts for content

  **Acceptance Criteria**:

  ```
  Scenario: All required documentation exists
    Tool: Bash
    Preconditions: Documentation tasks complete
    Steps:
      1. ls docs/deployment/
      2. Assert: DEPLOYMENT.md exists
      3. Assert: OPERATIONS.md exists
      4. Assert: SECURITY.md exists
      5. Assert: TROUBLESHOOTING.md exists
    Expected Result: All docs present
    Evidence: ls output

  Scenario: Documentation has no broken links
    Tool: Bash
    Preconditions: Docs exist
    Steps:
      1. grep -r "\](\./" docs/ | extract paths
      2. For each path, verify file exists
    Expected Result: All internal links valid
    Evidence: Link check output
  ```

  **Commit**: YES
  - Message: `docs: add comprehensive VM deployment documentation`
  - Files: `docs/deployment/*`, `docs/development/*`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `feat(docker): add worker container image` | worker.Dockerfile | docker build |
| 2 | `feat(spawn-worker): add Docker runtime support` | spawn-worker.sh | RUNTIME=docker test |
| 3 | `feat(volumes): verify .yaks/ access` | spawn-worker.sh | volume mount test |
| 4 | `feat(network): document network isolation` | spawn-worker.sh, docs | network test |
| 5 | `feat(resources): add resource profiles` | spawn-worker.sh | docker inspect |
| 6 | `feat(vm): add setup-vm.sh` | setup-vm.sh | bash -n |
| 7 | `docs(security): add credential docs` | docs/SECURITY.md | grep check |
| 8 | `feat(lifecycle): add systemd service` | setup-vm.sh, docs | syntax check |
| 9 | `feat(security): add hardening` | setup-vm.sh, spawn-worker.sh | docker inspect |
| 10 | `feat(lifecycle): add management scripts` | kill-worker.sh, cleanup-workers.sh, check-workers.sh | script test |
| 11 | `docs: add VM deployment docs` | docs/* | file exists |

---

## Success Criteria

### Phase 1-2 (MVP)
- [ ] spawn-worker.sh detects Docker runtime
- [ ] Workers spawn as containers, not tabs
- [ ] Workers can read/write .yaks/ state
- [ ] Workers can commit to git (using yakob's identity)
- [ ] check-workers.sh shows Docker container status
- [ ] Zellij mode still works locally

### Phase 3-4 (Deployment)
- [ ] setup-vm.sh provisions VM cleanly
- [ ] Orchestrator starts via systemd
- [ ] Team members can SSH and attach to Zellij
- [ ] Workers spawn on VM with resource limits
- [ ] No network access for workers (verified)

### Phase 5-6 (Production)
- [ ] Security checklist 100% complete
- [ ] Documentation published
- [ ] Concurrent test: 3 workers spawn and complete without yx corruption
- [ ] API key injection verified (worker can call opencode)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Container escape | High | Non-root user, no-new-privileges, seccomp, regular updates |
| Resource exhaustion | Medium | CPU/memory limits, pids-limit, monitoring + alerts |
| Disk space runaway | Medium | Auto-cleanup (--rm), periodic prune, disk monitoring |
| Credential leak | High | Layered creds, minimal privileges, audit logging |
| Worker deadlock | Low | Timeout mechanism, kill-worker.sh manual override |
| Network isolation bypass | Low | --network none, test egress, firewall |
| yx state corruption | Low | Atomic file ops (yx design), test concurrent access |
| Zellij session crash | Medium | Systemd auto-restart, workers continue (detached containers) |

---

## Timeline Estimate

| Phase | Tasks | Complexity | Estimate |
|-------|-------|------------|----------|
| 1 | Worker Container Image | Medium | 1 day |
| 2 | Docker spawn-worker.sh | High | 2-3 days |
| 3 | State/Volumes/Network/Resources | Medium | 1-2 days |
| 4 | VM Provisioning | Medium | 1-2 days |
| 5 | Security Hardening | High | 2-3 days |
| 6 | Documentation | Low | 1 day |

**Total**: 8-12 days (engineering time)
