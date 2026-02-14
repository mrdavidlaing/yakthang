#!/usr/bin/env bash
set -euo pipefail

# spawn-worker.sh - Spawn an opencode worker in a new Zellij tab
#
# Called by the orchestrator to delegate work to sub-repos.
# The worker gets its own tab, runs in the sub-repo's directory, and receives
# all yx instructions inline.
#
# Usage:
#   spawn-worker.sh --cwd <dir> --name <tab-name> [--mode plan|build] [--resources light|default|heavy] [--task <task>...] "<prompt>"
#   spawn-worker.sh --cwd ./api --name "api-auth" --task auth/api/login "Work on auth/api/* tasks..."
#   spawn-worker.sh --cwd ./api --name "api-planner" --mode plan "Plan the auth tasks"
#   spawn-worker.sh --cwd ./api --name "api-heavy" --resources heavy "Work on heavy tasks"
#
# Options:
#   --cwd <dir>           Working directory for the worker (required)
#   --name <name>         Zellij tab name (required)
#   --mode <mode>         Agent mode: plan or build (default: build)
#   --resources <profile> Resource profile: light, default, or heavy (default: default)
#   --task <task>         Task path to assign to this worker (can be specified multiple times)
#   --yak-path <dir>      Path to .yaks directory (default: $PWD/.yaks)

YAK_PATH="${YAK_PATH:-$PWD/.yaks}"
MODE="build"
CWD=""
TAB_NAME=""
PROMPT=""
TASKS=()
SETUP_NETWORK=false
RESOURCES="default"

# --- Yak-shaver personalities ------------------------------------------------
# Each worker gets a random identity. Yakob (the orchestrator) is the supervisor;
# these are the yak shavers. The yaks (tasks) are what get shaved.
# Personalities are now read from .opencode/personalities/ files.
WORKSPACE_ROOT="$(git rev-parse --show-toplevel)"
PERSONALITY_DIR="${WORKSPACE_ROOT}/.opencode/personalities"

WORKER_NAMES=(yakriel yakueline yakov yakira)
WORKER_DISPLAY_NAMES=(Yakriel Yakueline Yakov Yakira)
WORKER_EMOJIS=("🦬🪒" "🦬💈" "🦬🔔" "🦬🧶")

# Pick a random worker
SHAVER_INDEX=$((RANDOM % ${#WORKER_NAMES[@]}))
SHAVER_NAME="${WORKER_DISPLAY_NAMES[$SHAVER_INDEX]}"
SHAVER_EMOJI="${WORKER_EMOJIS[$SHAVER_INDEX]}"

# Read personality from file
PERSONALITY_FILE="${PERSONALITY_DIR}/${WORKER_NAMES[$SHAVER_INDEX]}-worker.md"
if [[ ! -f "$PERSONALITY_FILE" ]]; then
	echo "Error: Personality file not found: $PERSONALITY_FILE" >&2
	exit 1
fi
SHAVER_PERSONALITY="$(cat "$PERSONALITY_FILE")"

# --- Runtime Detection --------------------------------------------------------
# Detect runtime: Docker if available, else Zellij
# Can be overridden with RUNTIME environment variable (docker|zellij)
RUNTIME="${RUNTIME:-}"
if [[ -z "$RUNTIME" ]]; then
	if docker ps >/dev/null 2>&1; then
		RUNTIME="docker"
	elif command -v zellij >/dev/null 2>&1; then
		RUNTIME="zellij"
	else
		echo "Error: Neither Docker nor Zellij available" >&2
		exit 1
	fi
fi

while [[ $# -gt 0 ]]; do
	case "$1" in
	--cwd)
		CWD="$2"
		shift 2
		;;
	--name)
		TAB_NAME="$2"
		shift 2
		;;
	--mode)
		MODE="$2"
		if [[ "$MODE" != "plan" && "$MODE" != "build" ]]; then
			echo "Error: --mode must be 'plan' or 'build'" >&2
			echo "Usage: spawn-worker.sh --cwd <dir> --name <tab-name> [--mode plan|build] \"<prompt>\"" >&2
			exit 1
		fi
		shift 2
		;;
	--yak-path)
		YAK_PATH="$2"
		shift 2
		;;
	--task)
		TASKS+=("$2")
		shift 2
		;;
	--setup-network)
		SETUP_NETWORK=true
		shift
		;;
	--resources)
		RESOURCES="$2"
		if [[ "$RESOURCES" != "light" && "$RESOURCES" != "default" && "$RESOURCES" != "heavy" ]]; then
			echo "Error: --resources must be 'light', 'default', or 'heavy'" >&2
			exit 1
		fi
		shift 2
		;;
	*)
		if [[ -z "$PROMPT" ]]; then
			PROMPT="$1"
		else
			PROMPT="$PROMPT $1"
		fi
		shift
		;;
	esac
done

if [[ -z "$CWD" || -z "$TAB_NAME" || -z "$PROMPT" ]]; then
	echo "Usage: spawn-worker.sh --cwd <dir> --name <tab-name> [--mode plan|build] \"<prompt>\"" >&2
	exit 1
fi

# Resolve to absolute path
CWD="$(cd "$CWD" && pwd)"

# Validate ZELLIJ_SESSION_NAME for Zellij runtime
# When running inside Zellij, this env var pins zellij action commands to the correct session
if [[ "$RUNTIME" == "zellij" ]]; then
	if [[ -z "${ZELLIJ_SESSION_NAME:-}" ]]; then
		echo "Warning: ZELLIJ_SESSION_NAME not set. Workers may spawn in wrong session if multiple Zellij sessions are running." >&2
		echo "This is expected if not running inside a Zellij pane." >&2
	fi
fi

# Extract yak title from tasks (strip parent path prefix for conciseness)
YAK_TITLE=""
if [[ ${#TASKS[@]} -gt 0 ]]; then
	# Take the first task, strip parent prefix (e.g. "yurt-poc/openclaw" -> "openclaw")
	FIRST_TASK="${TASKS[0]}"
	YAK_TITLE="${FIRST_TASK##*/}"

	# If multiple tasks, join them with commas
	if [[ ${#TASKS[@]} -gt 1 ]]; then
		for ((i = 1; i < ${#TASKS[@]}; i++)); do
			TASK_NAME="${TASKS[$i]##*/}"
			YAK_TITLE="${YAK_TITLE}, ${TASK_NAME}"
		done
	fi
fi

# Format the tab name with the shaver identity and yak title
if [[ -n "$YAK_TITLE" ]]; then
	DISPLAY_NAME="${SHAVER_NAME} ${SHAVER_EMOJI} ${YAK_TITLE}"
else
	DISPLAY_NAME="${SHAVER_NAME} ${SHAVER_EMOJI}"
fi

# Build the inline system prompt that teaches the worker about yx.
# This is the key design choice: sub-repos have NO CLAUDE.md about orchestration.
# Everything the worker needs to know comes in this prompt.
if [[ "$MODE" == "plan" ]]; then
	ROLE_DESCRIPTION="Your supervisor is Yakob. The yaks are tasks — your job is to scout them and plan the shave. Do NOT pick up the clippers."
else
	ROLE_DESCRIPTION="Your supervisor is Yakob. The yaks are tasks — your job is to shave them clean."
fi

WORKER_PROMPT="$(
	cat <<PROMPT_EOF
${SHAVER_PERSONALITY}

${ROLE_DESCRIPTION}

${PROMPT}

---
TASK TRACKER (yx)

You have access to a task tracker called yx. The task state lives in ${YAK_PATH}.

Commands:
  yx ls                     Show all tasks and their states
  yx context --show <name>  Read the context/requirements for a task
  yx done <name>            Mark a task as complete
  yx state <name> wip       Mark a task as in-progress

Reporting status (IMPORTANT -- the orchestrator monitors these fields):
  echo "<status>" | yx field <name> agent-status

  Write agent-status at each transition:
  - When starting:  echo "wip: starting" | yx field <name> agent-status
  - Progress:       echo "wip: <what you're doing>" | yx field <name> agent-status
  - When blocked:   echo "blocked: <reason>" | yx field <name> agent-status
  - When done:      echo "done: <summary>" | yx field <name> agent-status

PROMPT_EOF
)"

if [[ "$MODE" == "plan" ]]; then
	WORKER_PROMPT="${WORKER_PROMPT}

Workflow:
1. Run 'yx ls' to see available tasks
2. Pick a task, read its context with 'yx context --show <name>'
3. Set it to wip: 'yx state <name> wip'
4. Report status: echo \"wip: starting plan\" | yx field <name> agent-status
5. Analyze the codebase, understand the problem, and write a detailed plan
6. Save the plan where it makes sense (e.g. a markdown file, or in yx context)
7. Report: echo \"blocked: plan ready for review\" | yx field <name> agent-status
8. STOP and wait — do NOT implement. Your job is to plan, not build.

Focus on the tasks assigned to you. Do not modify tasks outside your scope."
else
	WORKER_PROMPT="${WORKER_PROMPT}

Workflow:
1. Run 'yx ls' to see available tasks
2. Pick a task, read its context with 'yx context --show <name>'
3. Set it to wip: 'yx state <name> wip'
4. Report status: echo \"wip: starting\" | yx field <name> agent-status
5. Do the work (update agent-status as you make progress)
6. When done: 'yx done <name>' then echo \"done: <summary>\" | yx field <name> agent-status
7. If blocked: echo \"blocked: <reason>\" | yx field <name> agent-status

Focus on the tasks assigned to you. Do not modify tasks outside your scope."
fi

if [[ "$RUNTIME" == "docker" ]]; then
	WORKSPACE_ROOT="$(git rev-parse --show-toplevel)"
	CONTAINER_NAME="yak-worker-${TAB_NAME//[^a-zA-Z0-9_-]/}"

	# Network mode: bridge by default (workers need LLM API access)
	# TODO: implement proper network filtering (see docker-workers/network-filtering)
	NETWORK_MODE="bridge"

	# Determine resource limits based on profile
	case "$RESOURCES" in
	light)
		CPUS="0.5"
		MEMORY="1g"
		PIDS_LIMIT="256"
		;;
	default)
		CPUS="1.0"
		MEMORY="2g"
		PIDS_LIMIT="512"
		;;
	heavy)
		CPUS="2.0"
		MEMORY="4g"
		PIDS_LIMIT="1024"
		;;
	esac

	# Write prompt and wrapper script to a temp dir, then launch in a Zellij tab
	# so the container gets a TTY for the interactive opencode TUI.
	WORKER_DIR="$(mktemp -d "${TMPDIR:-/tmp}/worker-XXXXXX")"
	PROMPT_FILE="${WORKER_DIR}/prompt.txt"
	WRAPPER="${WORKER_DIR}/run.sh"

	printf '%s' "$WORKER_PROMPT" >"$PROMPT_FILE"

	# Inner script that runs inside the container — reads prompt file and runs opencode
	INNER="${WORKER_DIR}/inner.sh"
	cat >"$INNER" <<'INNER_EOF'
#!/usr/bin/env bash
# Cost capture setup - write to workspace-mounted directory
WORKSPACE_ROOT="${WORKSPACE_ROOT:-/home/yakob/yakthang}"
COST_DIR="${WORKSPACE_ROOT}/.worker-costs"
mkdir -p "$COST_DIR"

PROMPT="$(cat /opt/worker/prompt.txt)"

# Run opencode (NOT exec - we need post-run cleanup)
opencode --prompt "$PROMPT" --agent "$1"
EXIT_CODE=$?

# Capture cost data to persistent storage (survives container stop)
WORKER="${WORKER_NAME:-unknown}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"

# Full session export (per-message costs)
SID="$(opencode session list 2>/dev/null | tail -1 | awk '{print $1}')"
if [[ -n "$SID" && "$SID" != "Session" ]]; then
  opencode export "$SID" > "${COST_DIR}/${WORKER}-${TS}.json" 2>/dev/null
fi

# Human-readable stats
opencode stats --models > "${COST_DIR}/${WORKER}-${TS}.stats.txt" 2>/dev/null

exit $EXIT_CODE
INNER_EOF
	chmod +x "$INNER"

	# ANTHROPIC_API_KEY must be set in the environment
	if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
		echo "Error: ANTHROPIC_API_KEY not set. Docker workers need this to access the LLM API." >&2
		exit 1
	fi

	cat >"$WRAPPER" <<WRAPPER_EOF
#!/usr/bin/env bash
exec docker run -it --rm \\
	--name "$CONTAINER_NAME" \\
	--user "$(id -u):$(id -g)" \\
	--network "$NETWORK_MODE" \\
	--security-opt no-new-privileges \\
	--cap-drop ALL \\
	--read-only \\
	--tmpfs /tmp:rw,exec,size=2g \\
	--tmpfs /home/worker:rw,exec,size=1g,uid=$(id -u),gid=$(id -g) \\
	--tmpfs /home/worker/.cache:rw,exec,size=1g,uid=$(id -u),gid=$(id -g) \\
	--cpus "$CPUS" \\
	--memory "$MEMORY" \\
	--pids-limit "$PIDS_LIMIT" \\
	--stop-timeout 7200 \\
	-v "${WORKSPACE_ROOT}:${WORKSPACE_ROOT}:rw" \\
	-v "${YAK_PATH}:${YAK_PATH}:rw" \\
	-v "${PROMPT_FILE}:/opt/worker/prompt.txt:ro" \\
	-v "${INNER}:/opt/worker/start.sh:ro" \\
	-w "$CWD" \\
	-e HOME=/home/worker \\
	-e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \\
	-e WORKER_NAME="$SHAVER_NAME" \\
	-e WORKER_EMOJI="$SHAVER_EMOJI" \\
	-e YAK_PATH="$YAK_PATH" \\
	yak-worker:latest \\
	bash /opt/worker/start.sh ${MODE}
WRAPPER_EOF
	chmod +x "$WRAPPER"

	WORKER_LAYOUT="${WORKER_DIR}/layout.kdl"

	cat >"$WORKER_LAYOUT" <<LAYOUT_EOF
layout {
    tab name="${DISPLAY_NAME}" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane size="67%" name="opencode (${MODE}) [docker]" focus=true {
            command "bash"
            args "${WRAPPER}"
        }
        pane size="33%" name="shell: ${CWD}" cwd="${CWD}"
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
LAYOUT_EOF

	zellij action new-tab --layout "$WORKER_LAYOUT" --name "$DISPLAY_NAME"

	for task in "${TASKS[@]}"; do
		echo "${SHAVER_NAME} ${SHAVER_EMOJI}" | yx field "$task" assigned-to
	done

	# --- Write worker metadata for shutdown ---
	WORKER_CACHE_DIR="${WORKSPACE_ROOT}/.worker-cache"
	mkdir -p "$WORKER_CACHE_DIR"

	# Format TASKS as bash array literal
	TASKS_LITERAL="("
	for t in "${TASKS[@]}"; do TASKS_LITERAL+="\"$t\" "; done
	TASKS_LITERAL+=")"

	cat > "${WORKER_CACHE_DIR}/${TAB_NAME}.meta" <<META_EOF
SHAVER_NAME="${SHAVER_NAME}"
SHAVER_EMOJI="${SHAVER_EMOJI}"
DISPLAY_NAME="${DISPLAY_NAME}"
TAB_NAME="${TAB_NAME}"
CONTAINER_NAME="${CONTAINER_NAME}"
RUNTIME="${RUNTIME}"
CWD="${CWD}"
SPAWNED_AT=$(date +%s)
YAK_PATH="${YAK_PATH}"
ZELLIJ_SESSION_NAME="${ZELLIJ_SESSION_NAME:-}"
TASKS=${TASKS_LITERAL}
META_EOF

	sleep 0.3
	zellij action go-to-previous-tab

	echo "Spawned ${SHAVER_NAME} (${TAB_NAME}) in Docker container ${CONTAINER_NAME}"

elif [[ "$RUNTIME" == "zellij" ]]; then
	WORKER_DIR="$(mktemp -d "${TMPDIR:-/tmp}/worker-XXXXXX")"
	PROMPT_FILE="${WORKER_DIR}/prompt.txt"
	WRAPPER="${WORKER_DIR}/run.sh"

	printf '%s' "$WORKER_PROMPT" >"$PROMPT_FILE"

	cat >"$WRAPPER" <<WRAPPER_EOF
#!/usr/bin/env bash
PROMPT="\$(cat "${PROMPT_FILE}")"
rm -rf "${WORKER_DIR}"
exec opencode --prompt "\$PROMPT" --agent ${MODE}
WRAPPER_EOF
	chmod +x "$WRAPPER"

	WORKER_LAYOUT="${WORKER_DIR}/layout.kdl"

	cat >"$WORKER_LAYOUT" <<LAYOUT_EOF
layout {
    tab name="${DISPLAY_NAME}" cwd="${CWD}" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane size="67%" name="opencode (${MODE})" focus=true {
            command "bash"
            args "${WRAPPER}" "${PROMPT_FILE}"
        }
        pane size="33%" name="shell: ${CWD}"
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
LAYOUT_EOF

	if [[ -n "${ZELLIJ_SESSION_NAME:-}" ]]; then
		zellij --session "$ZELLIJ_SESSION_NAME" action new-tab --layout "$WORKER_LAYOUT" --name "$DISPLAY_NAME" --cwd "$CWD"
	else
		zellij action new-tab --layout "$WORKER_LAYOUT" --name "$DISPLAY_NAME" --cwd "$CWD"
	fi

	for task in "${TASKS[@]}"; do
		echo "${SHAVER_NAME} ${SHAVER_EMOJI}" | yx field "$task" assigned-to
	done

	# --- Write worker metadata for shutdown ---
	WORKER_CACHE_DIR="${WORKSPACE_ROOT}/.worker-cache"
	mkdir -p "$WORKER_CACHE_DIR"

	# Format TASKS as bash array literal
	TASKS_LITERAL="("
	for t in "${TASKS[@]}"; do TASKS_LITERAL+="\"$t\" "; done
	TASKS_LITERAL+=")"

	cat > "${WORKER_CACHE_DIR}/${TAB_NAME}.meta" <<META_EOF
SHAVER_NAME="${SHAVER_NAME}"
SHAVER_EMOJI="${SHAVER_EMOJI}"
DISPLAY_NAME="${DISPLAY_NAME}"
TAB_NAME="${TAB_NAME}"
CONTAINER_NAME=""
RUNTIME="${RUNTIME}"
CWD="${CWD}"
SPAWNED_AT=$(date +%s)
YAK_PATH="${YAK_PATH}"
ZELLIJ_SESSION_NAME="${ZELLIJ_SESSION_NAME:-}"
TASKS=${TASKS_LITERAL}
META_EOF

	sleep 0.3
	if [[ -n "${ZELLIJ_SESSION_NAME:-}" ]]; then
		zellij --session "$ZELLIJ_SESSION_NAME" action go-to-previous-tab
	else
		zellij action go-to-previous-tab
	fi

	echo "Spawned ${SHAVER_NAME} (${TAB_NAME}) in ${CWD}"

else
	echo "Error: Unknown RUNTIME value: $RUNTIME" >&2
	exit 1
fi
