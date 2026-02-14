#!/usr/bin/env bash
set -euo pipefail

# cost-openclaw.sh - Extract cost data from OpenClaw Gateway
# Parses JSONL transcripts to compute cost by session type

OPENCLAW_SESSIONS="${HOME}/.openclaw/agents/main/sessions"
DAYS="${1:-1}"

# Pricing model (approximate - used when cost not in transcript)
declare -A PRICING=(
    [claude-opus-4-5]="15.00 75.00 0.60 0.30"
    [claude-opus-4-6]="15.00 75.00 0.60 0.30"
    [claude-sonnet-4-5]="3.00 15.00 0.30 0.15"
    [claude-sonnet-4-4]="3.00 15.00 0.30 0.15"
    [claude-haiku-4-5]="0.80 4.00 0.10 0.05"
    [claude-haiku-4-4]="0.80 4.00 0.10 0.05"
)

usage() {
    echo "Usage: $0 [--today|--week|--month|--all] [days]"
    echo "  --today   Show today's costs (default)"
    echo "  --week    Show last 7 days"
    echo "  --month   Show last 30 days"
    echo "  --all     Show all time"
    echo "  days      Custom number of days (default: 1)"
    exit 1
}

case "${1:-}" in
    --today) DAYS=1 ;;
    --week) DAYS=7 ;;
    --month) DAYS=30 ;;
    --all) DAYS=3650 ;;
    -h|--help) usage ;;
esac

# Calculate cutoff timestamp
CUTOFF=$(date -d "${DAYS} days ago" +%s)

# Session type classification
classify_session() {
    local key="$1"
    if [[ "$key" == *"cron"* ]]; then
        echo "cron"
    elif [[ "$key" == *"slack"* ]]; then
        echo "slack"
    elif [[ "$key" == *"heartbeat"* ]]; then
        echo "heartbeat"
    else
        echo "interactive"
    fi
}

# Compute cost from usage if not in transcript
compute_cost() {
    local input="$1" output="$2" cache_read="$3" cache_write="$4" model="$5"
    
    if [[ -v "PRICING[$model]" ]]; then
        read -r i o c_r c_w <<< "${PRICING[$model]}"
        # Prices per 1M tokens
        input_cost=$(echo "scale=6; $input * $i / 1000000" | bc)
        output_cost=$(echo "scale=6; $output * $o / 1000000" | bc)
        cache_read_cost=$(echo "scale=6; $cache_read * $c_r / 1000000" | bc)
        cache_write_cost=$(echo "scale=6; $cache_write * $c_w / 1000000" | bc)
        total=$(echo "scale=6; $input_cost + $output_cost + $cache_read_cost + $cache_write_cost" | bc)
        echo "$total"
    else
        echo "0"
    fi
}

# Parse sessions.json
declare -A session_files
while IFS= read -r key; do
    session_file=$(jq -r ".[\"$key\"].sessionFile // empty" "${OPENCLAW_SESSIONS}/sessions.json" 2>/dev/null)
    if [[ -n "$session_file" && -f "$session_file" ]]; then
        session_files["$key"]="$session_file"
    fi
done < <(jq -r 'keys[]' "${OPENCLAW_SESSIONS}/sessions.json" 2>/dev/null)

# Aggregate costs by session type
declare -A type_costs
declare -A type_tokens
declare -A model_costs
total_cost=0

for key in "${!session_files[@]}"; do
    session_file="${session_files[$key]}"
    type=$(classify_session "$key")
    
    # Get session timestamp from first entry
    session_time=$(head -1 "$session_file" 2>/dev/null | jq -r '.timestamp // empty' | sed 's/Z//' | sed 's/T/ /')
    if [[ -z "$session_time" ]]; then
        continue
    fi
    
    session_ts=$(date -d "$session_time" +%s 2>/dev/null || continue)
    if [[ "$session_ts" -lt "$CUTOFF" ]]; then
        continue
    fi
    
    # Parse JSONL for usage/cost data
    while IFS= read -r line; do
        msg=$(echo "$line" | jq -r '.message // empty' 2>/dev/null)
        if [[ -z "$msg" || "$msg" == "null" ]]; then
            continue
        fi
        
        role=$(echo "$msg" | jq -r '.role // empty')
        if [[ "$role" != "assistant" ]]; then
            continue
        fi
        
        # Try to get cost directly
        cost=$(echo "$msg" | jq -r '.usage.cost.total // empty')
        model=$(echo "$msg" | jq -r '.model // empty')
        
        if [[ -z "$cost" || "$cost" == "null" || "$cost" == "0" ]]; then
            # Compute from usage
            input=$(echo "$msg" | jq -r '.usage.input // 0')
            output=$(echo "$msg" | jq -r '.usage.output // 0')
            cache_read=$(echo "$msg" | jq -r '.usage.cacheRead // 0')
            cache_write=$(echo "$msg" | jq -r '.usage.cacheWrite // 0')
            cost=$(compute_cost "$input" "$output" "$cache_read" "$cache_write" "$model")
        fi
        
        if [[ -n "$cost" && "$cost" != "null" ]]; then
            type_costs["$type"]=$(echo "${type_costs[$type]:-0} + $cost" | bc)
            total_cost=$(echo "$total_cost + $cost" | bc)
            
            if [[ -n "$model" && "$model" != "null" ]]; then
                model_costs["$model"]=$(echo "${model_costs[$model]:-0} + $cost" | bc)
            fi
        fi
    done < "$session_file"
done

# Output results
echo "OpenClaw Cost Report (last ${DAYS} day(s))"
echo "============================================="
echo ""

for type in interactive cron slack heartbeat; do
    cost="${type_costs[$type]:-0}"
    if [[ "$(echo "$cost > 0.001" | bc)" -eq 1 ]]; then
        printf "%-20s \$%.2f\n" "${type^}:" "$cost"
    fi
done

echo ""
printf "%-20s \$%.2f\n" "Total:" "$total_cost"
echo ""
echo "By Model:"
for model in "${!model_costs[@]}"; do
    cost="${model_costs[$model]}"
    # Skip zero-cost or delivery models
    if [[ "$(echo "$cost > 0.001" | bc)" -eq 0 || "$model" == "delivery-mirror" ]]; then
        continue
    fi
    pct=$(echo "scale=1; $cost * 100 / $total_cost" | bc)
    printf "  %-20s \$%.2f (%s%%)\n" "$model:" "$cost" "$pct"
done
