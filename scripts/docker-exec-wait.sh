#!/usr/bin/env bash
# Helper script to exec into a Docker container with retry logic
# Handles the race condition where the shell pane starts before the container is ready

set -euo pipefail

CONTAINER_NAME="${1:-}"
COMMAND="${2:-bash}"
MAX_RETRIES=30
RETRY_DELAY=1

if [[ -z "$CONTAINER_NAME" ]]; then
    echo "Usage: $0 <container-name> [command]"
    exit 1
fi

# Wait for container to be running
for i in $(seq 1 $MAX_RETRIES); do
    if docker inspect --format='{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null | grep -q "running"; then
        # Container is running, now exec into it
        exec docker exec -it "$CONTAINER_NAME" "$COMMAND"
    fi
    
    # Show progress indicator
    if [[ $i -eq 1 ]]; then
        echo "Waiting for container '$CONTAINER_NAME' to start..."
    fi
    
    sleep $RETRY_DELAY
done

# If we get here, container never started
echo "ERROR: Container '$CONTAINER_NAME' did not start within $MAX_RETRIES seconds"
echo "Try checking: docker ps -a | grep $CONTAINER_NAME"
exit 1
