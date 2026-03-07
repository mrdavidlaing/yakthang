#!/usr/bin/env bash
set -euo pipefail

# cost-summary.sh - Worker cost reporting
# Usage: cost-summary.sh [--today|--week|--month|--all] [--append-csv]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKER_COSTS="${SCRIPT_DIR}/.worker-costs"
CSV_FILE="${SCRIPT_DIR}/.worker-costs/daily-totals.csv"

DAYS=""
APPEND_CSV=false

usage() {
    echo "Usage: $0 [--today|--week|--month|--all] [--append-csv]"
    echo "  --today    Show today's costs (default)"
    echo "  --week     Show last 7 days"
    echo "  --month    Show last 30 days"
    echo "  --all      Show all time"
    echo "  --append-csv  Append today's totals to CSV history"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --today) DAYS="--today" ;;
        --week) DAYS="--week" ;;
        --month) DAYS="--month" ;;
        --all) DAYS="--all" ;;
        --append-csv) APPEND_CSV=true ;;
        -h|--help) usage ;;
        *) usage ;;
    esac
    shift
done

if [[ -z "$DAYS" ]]; then
    DAYS="--today"
fi

# Header
DATE_STR=$(date +%Y-%m-%d)
echo "═══ Cost Report: ${DATE_STR} ═══"
echo ""

# Worker costs
echo "Workers:"
total_worker_cost=0
if [[ -d "$WORKER_COSTS" ]]; then
    declare -A worker_costs

    for json_file in "${WORKER_COSTS}"/*.json; do
        [[ -f "$json_file" ]] || continue

        # Extract worker name from filename (e.g., Yakriel-20260214T123456Z.json)
        worker=$(basename "$json_file" | cut -d'-' -f1)

        # Sum costs from session export - find all "cost": number patterns
        cost=$(grep -oP '"cost":\s*\K[0-9.]+' "$json_file" 2>/dev/null | awk '{sum+=$1} END {print sum+0}')

        worker_costs["$worker"]=$(awk "BEGIN {print ${worker_costs[$worker]:-0} + $cost}" 2>/dev/null || echo "0")
    done

    for worker in "${!worker_costs[@]}"; do
        cost="${worker_costs[$worker]}"
        total_worker_cost=$(awk "BEGIN {print $total_worker_cost + $cost}" 2>/dev/null || echo "$total_worker_cost")
        printf "  %-20s \$%.2f\n" "$worker:" "$cost"
    done

    if [[ -z "${worker_costs[@]}" ]]; then
        echo "  (no worker runs in period)"
        total_worker_cost=0
    fi
else
    echo "  (no cost data captured yet)"
    total_worker_cost=0
fi

echo ""
printf "                          Total: \$%.2f\n" "$total_worker_cost"

# Append to CSV if requested
if [[ "$APPEND_CSV" == "true" ]]; then
    mkdir -p "$(dirname "$CSV_FILE")"
    if [[ ! -f "$CSV_FILE" ]]; then
        echo "date,worker_cost,total_cost,workers" > "$CSV_FILE"
    fi

    workers=$(find "$WORKER_COSTS" -name "*.json" -mtime 0 2>/dev/null | wc -l)
    [[ -z "$workers" ]] && workers=0

    echo "${DATE_STR},${total_worker_cost},${total_worker_cost},${workers}" >> "$CSV_FILE"
    echo ""
    echo "Appended to $CSV_FILE"
fi
