#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESSION_NAME="${1:-$(basename "$(pwd)")}"
SESSIONS=$(zellij list-sessions 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' || true)

# Clean up dead sessions
if echo "$SESSIONS" | grep -q "^${SESSION_NAME}.*EXITED"; then
    echo "Cleaning up dead session '${SESSION_NAME}'..."
    zellij delete-session "${SESSION_NAME}"
    SESSIONS=$(zellij list-sessions 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' || true)
fi

if echo "$SESSIONS" | grep -q "^${SESSION_NAME} "; then
    echo "Attaching to existing session '${SESSION_NAME}'..."
    zellij attach "${SESSION_NAME}"
else
    echo "Starting new session '${SESSION_NAME}'..."
    zellij -s "${SESSION_NAME}" --layout "$SCRIPT_DIR/orchestrator.kdl"
fi
