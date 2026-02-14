#!/usr/bin/env bash
set -euo pipefail

# check-workers.sh - Show agent-status for all tasks that have one
#
# Used by the orchestrator to poll worker feedback.
# Reads the agent-status field from every task that has reported status.
#
# Usage:
#   check-workers.sh              Show all statuses
#   check-workers.sh --blocked    Show only blocked tasks
#   check-workers.sh --wip        Show only in-progress tasks
#   check-workers.sh <prefix>     Show statuses under a task prefix

YAK_PATH="${YAK_PATH:-.yaks}"
FILTER=""
PREFIX=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--blocked)
		FILTER="blocked"
		shift
		;;
	--wip)
		FILTER="wip"
		shift
		;;
	*)
		PREFIX="$1"
		shift
		;;
	esac
done

SEARCH_PATH="${YAK_PATH}"
if [[ -n "$PREFIX" ]]; then
	SEARCH_PATH="${YAK_PATH}/${PREFIX}"
fi

if [[ ! -d "$SEARCH_PATH" ]]; then
	echo "No tasks found under ${SEARCH_PATH}" >&2
	exit 1
fi

found=0
while IFS= read -r status_file; do
	# Get the task path relative to .yaks
	task_dir="$(dirname "$status_file")"
	task_name="${task_dir#"${YAK_PATH}"/}"

	status="$(cat "$status_file")"

	# Apply filter if set
	if [[ -n "$FILTER" ]]; then
		case "$status" in
		"${FILTER}"*) ;; # matches
		*) continue ;;   # skip
		esac
	fi

	printf "%-50s %s\n" "$task_name" "$status"
	found=1
done < <(find "$SEARCH_PATH" -name "agent-status" -type f 2>/dev/null | sort)

if [[ "$found" -eq 0 ]]; then
	if [[ -n "$FILTER" ]]; then
		echo "No tasks with status '${FILTER}' found."
	else
		echo "No tasks have reported agent-status yet."
	fi
fi

echo ""
echo "=== Running Workers (Docker) ==="
echo ""

DOCKER_WORKERS=$(docker ps --filter "name=yak-worker-" --format "{{.Names}}\t{{.Status}}\t{{.RunningFor}}" 2>/dev/null || true)

if [[ -z "$DOCKER_WORKERS" ]]; then
	echo "No running worker containers."
else
	echo "Container Name                    Status              Running For"
	echo "----------------------------------------------------------------"
	echo "$DOCKER_WORKERS"
	echo ""
	echo "Live Cost:"
	for container in $(docker ps --filter "name=yak-worker-" --format '{{.Names}}' 2>/dev/null); do
		cost=$(docker exec "$container" opencode stats 2>/dev/null | grep "Total Cost" | awk '{print $NF}' || echo "n/a")
		printf "  %-30s %s\n" "$container" "${cost:-n/a}"
	done
fi

echo ""
echo "=== Stopped Workers (Docker) ==="
echo ""

STOPPED_WORKERS=$(docker ps -a --filter "name=yak-worker-" --filter "status=exited" --format "{{.Names}}\t{{.Status}}" 2>/dev/null || true)

if [[ -z "$STOPPED_WORKERS" ]]; then
	echo "No stopped worker containers."
else
	echo "Container Name                    Status"
	echo "----------------------------------------------------------------"
	echo "$STOPPED_WORKERS"
	echo ""
	echo "Run './cleanup-workers.sh' to remove stopped containers."
fi
