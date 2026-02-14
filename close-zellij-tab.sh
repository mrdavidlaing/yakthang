#!/usr/bin/env bash
# close-zellij-tab.sh - Close Zellij tabs by name or pattern
#
# Usage:
#   ./close-zellij-tab.sh <tab-name>
#   ./close-zellij-tab.sh --pattern <regex>
#   ./close-zellij-tab.sh --all-worker-tabs
#   ./close-zellij-tab.sh --session <session-name> <tab-name>
#
# Examples:
#   ./close-zellij-tab.sh "Yakira 🦬🧶 auth-api"
#   ./close-zellij-tab.sh --pattern "Yakira.*auth"
#   ./close-zellij-tab.sh --all-worker-tabs
#   ./close-zellij-tab.sh --session yak-orchestrator "Yakira 🦬🧶 auth-api"

set -euo pipefail

# Configuration
ORCHESTRATOR_TAB_PATTERN="Yakob"
SLEEP_BEFORE_CLOSE=0.1

# Parse arguments
SESSION=""
TAB_NAME=""
PATTERN=""
ALL_WORKER_TABS=false

usage() {
    cat <<EOF
Usage: $0 [OPTIONS] <tab-name>

Close Zellij tabs by name or pattern.

OPTIONS:
    --session <name>      Target specific Zellij session
    --pattern <regex>     Match tabs by regex pattern
    --all-worker-tabs     Close all worker tabs (non-orchestrator)
    --help, -h            Show this help

EXAMPLES:
    # Close specific tab by exact name
    $0 "Yakira 🦬🧶 auth-api"
    
    # Close by pattern
    $0 --pattern "Yakira.*auth"
    
    # Close all worker tabs (emergency cleanup)
    $0 --all-worker-tabs
    
    # Target specific session
    $0 --session yak-orchestrator "Yakira 🦬🧶 auth-api"

NOTES:
    - Refuses to close orchestrator tab (matches "$ORCHESTRATOR_TAB_PATTERN")
    - Uses two-step close: go-to-tab-name + close-tab
    - Focus automatically returns to adjacent tab
    - Safe to run multiple times (idempotent)

EOF
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --session)
            if [[ $# -lt 2 ]]; then
                echo "Error: --session requires a session name" >&2
                exit 1
            fi
            SESSION="$2"
            shift 2
            ;;
        --pattern)
            if [[ $# -lt 2 ]]; then
                echo "Error: --pattern requires a regex pattern" >&2
                exit 1
            fi
            PATTERN="$2"
            shift 2
            ;;
        --all-worker-tabs)
            ALL_WORKER_TABS=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            if [[ -z "$TAB_NAME" ]]; then
                TAB_NAME="$1"
                shift
            else
                echo "Error: Unknown option or too many arguments: $1" >&2
                echo "Use --help for usage information" >&2
                exit 1
            fi
            ;;
    esac
done

# Validate arguments
if [[ "$ALL_WORKER_TABS" == "false" && -z "$TAB_NAME" && -z "$PATTERN" ]]; then
    echo "Error: Must provide tab name, --pattern, or --all-worker-tabs" >&2
    echo "Use --help for usage information" >&2
    exit 1
fi

if [[ "$ALL_WORKER_TABS" == "true" && ( -n "$TAB_NAME" || -n "$PATTERN" ) ]]; then
    echo "Error: Cannot use --all-worker-tabs with tab name or pattern" >&2
    exit 1
fi

if [[ -n "$TAB_NAME" && -n "$PATTERN" ]]; then
    echo "Error: Cannot use both tab name and --pattern" >&2
    exit 1
fi

# Helper: run zellij action with optional --session
zellij_action() {
    if [[ -n "${SESSION}" ]]; then
        zellij --session "$SESSION" action "$@"
    else
        zellij action "$@"
    fi
}

# Helper: check if we're in a Zellij environment
check_zellij() {
    if [[ -z "${ZELLIJ:-}" && -z "${SESSION}" ]]; then
        echo "Error: Not running inside Zellij and no --session specified" >&2
        echo "Either run from inside Zellij or use --session <name>" >&2
        exit 1
    fi
}

# Main logic
main() {
    check_zellij
    
    # Get list of tabs
    if ! TABS=$(zellij_action query-tab-names 2>&1); then
        echo "Error: Failed to query tab names" >&2
        echo "$TABS" >&2
        exit 1
    fi
    
    if [[ -z "$TABS" ]]; then
        echo "No tabs found"
        exit 0
    fi
    
    # Match tabs based on mode
    MATCHED=""
    
    if [[ "$ALL_WORKER_TABS" == "true" ]]; then
        # Match all tabs except orchestrator
        MATCHED=$(echo "$TABS" | grep -vE "$ORCHESTRATOR_TAB_PATTERN" || true)
    elif [[ -n "$PATTERN" ]]; then
        # Match by regex pattern
        MATCHED=$(echo "$TABS" | grep -E "$PATTERN" || true)
    else
        # Match by exact name
        MATCHED=$(echo "$TABS" | grep -Fx "$TAB_NAME" || true)
    fi
    
    if [[ -z "$MATCHED" ]]; then
        echo "No matching tabs found"
        exit 0
    fi
    
    # Safety check: refuse to close orchestrator tab
    if echo "$MATCHED" | grep -qE "$ORCHESTRATOR_TAB_PATTERN"; then
        echo "Error: Refusing to close orchestrator tab" >&2
        echo "Matched tabs contain orchestrator pattern: $ORCHESTRATOR_TAB_PATTERN" >&2
        echo "Matched tabs:" >&2
        echo "$MATCHED" | sed 's/^/  /' >&2
        exit 1
    fi
    
    # Close each matched tab
    CLOSED_COUNT=0
    while IFS= read -r tab; do
        [[ -z "$tab" ]] && continue
        
        echo "Closing tab: $tab"
        
        # Two-step close: go-to-tab-name + close-tab
        if ! zellij_action go-to-tab-name "$tab" 2>/dev/null; then
            echo "  Warning: Failed to navigate to tab '$tab' (may already be closed)" >&2
            continue
        fi
        
        # Brief pause to let focus settle (prevents race conditions)
        sleep "$SLEEP_BEFORE_CLOSE"
        
        if ! zellij_action close-tab 2>/dev/null; then
            echo "  Warning: Failed to close tab '$tab'" >&2
            continue
        fi
        
        ((CLOSED_COUNT++))
        echo "  ✓ Closed"
    done <<< "$MATCHED"
    
    if [[ $CLOSED_COUNT -eq 0 ]]; then
        echo "No tabs were closed"
        exit 1
    else
        echo ""
        echo "Successfully closed $CLOSED_COUNT tab(s)"
        exit 0
    fi
}

main
