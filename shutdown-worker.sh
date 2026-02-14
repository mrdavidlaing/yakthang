#!/usr/bin/env bash
# shutdown-worker.sh - Comprehensive worker shutdown with full cleanup
#
# Usage:
#   ./shutdown-worker.sh <worker-name>
#   ./shutdown-worker.sh --timeout 60 <worker-name>
#   ./shutdown-worker.sh --dry-run <worker-name>
#
# Examples:
#   ./shutdown-worker.sh api-auth
#   ./shutdown-worker.sh --timeout 120 heavy-builder
#   ./shutdown-worker.sh --dry-run api-auth
#
# This script:
#   1. Loads worker metadata from .worker-cache/<name>.meta
#   2. Clears task assignments (assigned-to fields)
#   3. Performs runtime-dependent shutdown:
#      - Docker: stop container → close tab → remove container
#      - Zellij: close tab (sends SIGTERM to pane processes)
#   4. Deletes metadata file
#
# The script is idempotent and safe to run multiple times.

set -euo pipefail

# Configuration
DEFAULT_TIMEOUT=30
WORKSPACE_ROOT="$(git rev-parse --show-toplevel)"
WORKER_CACHE_DIR="${WORKSPACE_ROOT}/.worker-cache"
CLOSE_TAB_SCRIPT="${WORKSPACE_ROOT}/close-zellij-tab.sh"

# Parse arguments
WORKER_NAME=""
TIMEOUT="$DEFAULT_TIMEOUT"
DRY_RUN=false

usage() {
    cat <<EOF
Usage: $0 [OPTIONS] <worker-name>

Shutdown a worker with comprehensive cleanup.

OPTIONS:
    --timeout <seconds>   Docker stop timeout (default: $DEFAULT_TIMEOUT)
    --dry-run             Show what would be done without executing
    --help, -h            Show this help

EXAMPLES:
    # Shutdown worker by name
    $0 api-auth
    
    # Shutdown with longer timeout
    $0 --timeout 120 heavy-builder
    
    # Dry run to see what would happen
    $0 --dry-run api-auth

BEHAVIOR:
    - Loads metadata from .worker-cache/<name>.meta
    - Clears all task assignments
    - Docker runtime: stops container, closes tab, removes container
    - Zellij runtime: closes tab (which sends SIGTERM to processes)
    - Deletes metadata file
    - Idempotent: safe to run multiple times

EOF
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --timeout)
            if [[ $# -lt 2 ]]; then
                echo "Error: --timeout requires a value" >&2
                exit 1
            fi
            TIMEOUT="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            if [[ -z "$WORKER_NAME" ]]; then
                WORKER_NAME="$1"
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
if [[ -z "$WORKER_NAME" ]]; then
    echo "Error: Worker name required" >&2
    echo "Use --help for usage information" >&2
    exit 1
fi

# Helper: execute or print command
execute() {
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[dry-run] Would run: $*"
    else
        "$@"
    fi
}

# Helper: log actions
log_info() {
    echo "ℹ $*"
}

log_warn() {
    echo "⚠ Warning: $*" >&2
}

log_error() {
    echo "✗ Error: $*" >&2
}

log_success() {
    echo "✓ $*"
}

# Main logic
main() {
    echo "Shutting down worker: $WORKER_NAME"
    echo ""
    
    # Load metadata
    METADATA_FILE="${WORKER_CACHE_DIR}/${WORKER_NAME}.meta"
    
    if [[ ! -f "$METADATA_FILE" ]]; then
        log_warn "Metadata file not found: $METADATA_FILE"
        log_info "Attempting fallback detection..."
        
        # Fallback: try to detect runtime and tab name
        CONTAINER_NAME="yak-worker-${WORKER_NAME//[^a-zA-Z0-9_-]/}"
        
        # Try Docker
        if docker ps -a --filter "name=^${CONTAINER_NAME}$" --format "{{.Names}}" 2>/dev/null | grep -q .; then
            log_info "Found Docker container: $CONTAINER_NAME"
            RUNTIME="docker"
            DISPLAY_NAME=""
            TASKS=()
        # Try Zellij tabs
        elif command -v zellij >/dev/null 2>&1; then
            TABS=$(zellij action query-tab-names 2>/dev/null || true)
            MATCH=$(echo "$TABS" | grep -i "${WORKER_NAME}" || true)
            if [[ -n "$MATCH" ]]; then
                log_info "Found Zellij tab: $MATCH"
                RUNTIME="zellij"
                DISPLAY_NAME="$MATCH"
                CONTAINER_NAME=""
                TASKS=()
            else
                log_error "Worker not found in Docker or Zellij"
                log_info "Checked container name: $CONTAINER_NAME"
                log_info "Searched Zellij tabs for: $WORKER_NAME"
                exit 1
            fi
        else
            log_error "Worker not found and Zellij not available"
            exit 1
        fi
        
        log_warn "Without metadata, cannot clear task assignments"
        log_info "To manually clear assignments, run:"
        log_info "  find ${WORKSPACE_ROOT}/.yaks -name assigned-to -exec grep -l '${WORKER_NAME}' {} \\; | xargs rm -f"
        
        ZELLIJ_SESSION_NAME=""
    else
        log_info "Loading metadata from: $METADATA_FILE"
        
        # shellcheck disable=SC1090
        source "$METADATA_FILE"
        
        log_success "Loaded metadata:"
        log_info "  Display name: $DISPLAY_NAME"
        log_info "  Runtime: $RUNTIME"
        log_info "  Tab name: $TAB_NAME"
        log_info "  Container: ${CONTAINER_NAME:-<none>}"
        log_info "  Tasks: ${#TASKS[@]}"
    fi
    
    echo ""
    
    # Step 1: Clear task assignments
    if [[ ${#TASKS[@]} -gt 0 ]]; then
        log_info "Clearing task assignments..."
        
        for task in "${TASKS[@]}"; do
            ASSIGNMENT_FILE="${YAK_PATH}/${task}/assigned-to"
            
            if [[ -f "$ASSIGNMENT_FILE" ]]; then
                if [[ "$DRY_RUN" == "true" ]]; then
                    echo "[dry-run] Would delete: $ASSIGNMENT_FILE"
                else
                    rm -f "$ASSIGNMENT_FILE"
                    log_success "Cleared assignment: $task"
                fi
            else
                log_info "No assignment file for: $task (already cleared)"
            fi
        done
        echo ""
    else
        log_info "No tasks to clear (metadata not available or no tasks assigned)"
        echo ""
    fi
    
    # Step 2: Runtime-dependent shutdown
    if [[ "$RUNTIME" == "docker" ]]; then
        log_info "Docker runtime shutdown sequence:"
        
        # Check if container exists
        if docker ps -a --filter "name=^${CONTAINER_NAME}$" --format "{{.Names}}" 2>/dev/null | grep -q .; then
            CONTAINER_STATE=$(docker inspect -f '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "unknown")
            log_info "  Container state: $CONTAINER_STATE"
            
            # Stop container if running
            if [[ "$CONTAINER_STATE" == "running" ]]; then
                log_info "  Stopping container (timeout: ${TIMEOUT}s)..."
                if execute docker stop -t "$TIMEOUT" "$CONTAINER_NAME"; then
                    log_success "  Container stopped"
                else
                    log_warn "  Container stop failed or timed out"
                    log_info "  You may need to manually kill it: docker kill $CONTAINER_NAME"
                fi
            else
                log_info "  Container not running, skipping stop"
            fi
            
            # Close Zellij tab
            if [[ -n "$DISPLAY_NAME" ]] && command -v zellij >/dev/null 2>&1; then
                log_info "  Closing Zellij tab..."
                
                if [[ -f "$CLOSE_TAB_SCRIPT" ]]; then
                    if [[ -n "${ZELLIJ_SESSION_NAME}" ]]; then
                        execute "$CLOSE_TAB_SCRIPT" --session "$ZELLIJ_SESSION_NAME" "$DISPLAY_NAME" || log_warn "  Failed to close tab"
                    else
                        execute "$CLOSE_TAB_SCRIPT" "$DISPLAY_NAME" || log_warn "  Failed to close tab"
                    fi
                else
                    log_warn "  close-zellij-tab.sh not found at: $CLOSE_TAB_SCRIPT"
                fi
            else
                log_info "  No Zellij tab to close (or Zellij not available)"
            fi
            
            # Remove container
            log_info "  Removing container..."
            if execute docker rm "$CONTAINER_NAME"; then
                log_success "  Container removed"
            else
                log_warn "  Container removal failed (may already be removed)"
            fi
        else
            log_warn "Container not found: $CONTAINER_NAME (may already be removed)"
            
            # Still try to close the tab
            if [[ -n "$DISPLAY_NAME" ]] && command -v zellij >/dev/null 2>&1; then
                log_info "  Attempting to close Zellij tab anyway..."
                if [[ -f "$CLOSE_TAB_SCRIPT" ]]; then
                    if [[ -n "${ZELLIJ_SESSION_NAME}" ]]; then
                        execute "$CLOSE_TAB_SCRIPT" --session "$ZELLIJ_SESSION_NAME" "$DISPLAY_NAME" || log_warn "  Failed to close tab"
                    else
                        execute "$CLOSE_TAB_SCRIPT" "$DISPLAY_NAME" || log_warn "  Failed to close tab"
                    fi
                fi
            fi
        fi
        
    elif [[ "$RUNTIME" == "zellij" ]]; then
        log_info "Zellij runtime shutdown sequence:"
        
        # Close Zellij tab (this sends SIGTERM to all pane processes)
        if command -v zellij >/dev/null 2>&1; then
            if [[ -n "$DISPLAY_NAME" ]]; then
                log_info "  Closing Zellij tab (sends SIGTERM to processes)..."
                
                if [[ -f "$CLOSE_TAB_SCRIPT" ]]; then
                    if [[ -n "${ZELLIJ_SESSION_NAME}" ]]; then
                        if execute "$CLOSE_TAB_SCRIPT" --session "$ZELLIJ_SESSION_NAME" "$DISPLAY_NAME"; then
                            log_success "  Tab closed (worker processes terminated)"
                        else
                            log_warn "  Failed to close tab (may already be closed)"
                        fi
                    else
                        if execute "$CLOSE_TAB_SCRIPT" "$DISPLAY_NAME"; then
                            log_success "  Tab closed (worker processes terminated)"
                        else
                            log_warn "  Failed to close tab (may already be closed)"
                        fi
                    fi
                else
                    log_error "  close-zellij-tab.sh not found at: $CLOSE_TAB_SCRIPT"
                    log_info "  You may need to manually close the tab: $DISPLAY_NAME"
                fi
            else
                log_warn "No display name available, cannot close tab"
            fi
        else
            log_warn "Zellij not available, cannot close tab"
        fi
        
    else
        log_error "Unknown runtime: $RUNTIME"
        exit 1
    fi
    
    echo ""
    
    # Step 3: Delete metadata file
    if [[ -f "$METADATA_FILE" ]]; then
        log_info "Deleting metadata file..."
        if execute rm -f "$METADATA_FILE"; then
            log_success "Metadata deleted: $METADATA_FILE"
        else
            log_warn "Failed to delete metadata file"
        fi
    else
        log_info "No metadata file to delete (already removed or not found)"
    fi
    
    echo ""
    log_success "Worker shutdown complete: $WORKER_NAME"
}

# Run main
main
