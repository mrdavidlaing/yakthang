# shellcheck shell=bash

# Disable git hooks for all test repositories
export GIT_CONFIG_PARAMETERS="'core.hooksPath=/dev/null'"

# Prevent tests from polluting the main repository's git refs
GIT_CEILING_DIRECTORIES="$(pwd)"
export GIT_CEILING_DIRECTORIES

# Helper function to set up a git test repository
# Usage: setup_test_repo /path/to/repo [user_email] [user_name] [origin_url]
# Arguments:
#   repo_path: Path where the repo should be created
#   user_email: Git user email (default: "test@example.com")
#   user_name: Git user name (default: "Test User")
#   origin_url: Optional origin remote URL
setup_test_repo() {
	local repo_path="$1"
	local user_email="${2:-test@example.com}"
	local user_name="${3:-Test User}"
	local origin_url="${4:-}"

	git -C "$repo_path" init --initial-branch=main --quiet
	git -C "$repo_path" config user.email "$user_email"
	git -C "$repo_path" config user.name "$user_name"

	if [ -n "$origin_url" ]; then
		git -C "$repo_path" remote add origin "$origin_url"
	fi

	echo ".yaks" >"$repo_path/.gitignore"
	git -C "$repo_path" add .gitignore
	git -C "$repo_path" commit --quiet -m "Add .gitignore"
}

# Prerequisite check functions (return 0 = unavailable, for ShellSpec Skip)
docker_unavailable() { ! docker info >/dev/null 2>&1; }
zellij_unavailable() { ! zellij --version >/dev/null 2>&1; }
image_unavailable() { ! docker images -q yak-shaver:latest 2>/dev/null | grep -q .; }
yakbox_binary_unavailable() { ! command -v yak-box >/dev/null 2>&1; }

# Set up a clean test environment with a git repo, Docker worker naming, and Zellij session
setup_test_environment() {
	TEST_PROJECT_DIR=$(pwd)
	export PATH="$TEST_PROJECT_DIR/../../../bin:$PATH" # adds /home/yakob/yakthang/bin to PATH
	TEST_WORK_DIR=$(mktemp -d)
	cd "$TEST_WORK_DIR" || return
	setup_test_repo "."
	# Create .yaks task directory for spawn to consume
	mkdir -p ".yaks/test-task"
	echo "Integration test task" >".yaks/test-task/description"
	# Unique session and worker names to avoid conflicts
	TEST_ZELLIJ_SESSION="yak-box-test-$$"
	TEST_WORKER_NAME="test-worker-$$"
	export TEST_PROJECT_DIR TEST_WORK_DIR TEST_ZELLIJ_SESSION TEST_WORKER_NAME
}

# Clean up test environment
teardown_test_environment() {
	# Clean up Docker containers (try both name prefixes due to known bug)
	docker rm -f "yak-shaver-${TEST_WORKER_NAME}" 2>/dev/null || true
	docker rm -f "yak-worker-${TEST_WORKER_NAME}" 2>/dev/null || true
	# Wildcard cleanup to catch any variant
	docker ps -a --filter "name=${TEST_WORKER_NAME}" -q 2>/dev/null | xargs -r docker rm -f 2>/dev/null || true
	# Clean up Zellij tabs in test session (repeat to catch multiple tabs)
	zellij --session "$TEST_ZELLIJ_SESSION" action close-tab 2>/dev/null || true
	zellij --session "$TEST_ZELLIJ_SESSION" action close-tab 2>/dev/null || true
	zellij --session "$TEST_ZELLIJ_SESSION" action close-tab 2>/dev/null || true
	# Restore directory and remove temp work dir
	cd "$TEST_PROJECT_DIR" || return
	rm -rf "$TEST_WORK_DIR"
}

# Poll until container exists (use docker ps -a to include stopped containers)
# Returns 0 if found, 1 if timeout
wait_for_container() {
	local name="$1"
	local max_retries="${2:-30}"
	local count=0
	while [ "$count" -lt "$max_retries" ]; do
		if docker ps -a --filter "name=${name}" --format '{{.Names}}' 2>/dev/null | grep -q .; then
			return 0
		fi
		sleep 1
		count=$((count + 1))
	done
	return 1
}

# Poll until container is completely gone
# Returns 0 if gone, 1 if timeout
wait_for_container_gone() {
	local name="$1"
	local max_retries="${2:-30}"
	local count=0
	while [ "$count" -lt "$max_retries" ]; do
		if ! docker ps -a --filter "name=${name}" --format '{{.Names}}' 2>/dev/null | grep -q .; then
			return 0
		fi
		sleep 1
		count=$((count + 1))
	done
	return 1
}

# Discover actual Docker container name for the test worker
# Strategy: filter by TEST_WORKER_NAME as a substring (Docker filter is substring match)
# This finds the container regardless of whether it's yak-shaver-* or yak-worker-*
discover_container_name() {
	local found
	found=$(docker ps -a --filter "name=${TEST_WORKER_NAME}" --format '{{.Names}}' 2>/dev/null | head -1)
	if [ -n "$found" ]; then
		CONTAINER_NAME="$found"
		export CONTAINER_NAME
		return 0
	fi
	# Fallback: check sessions.json Container field
	local sessions_file="$TEST_WORK_DIR/.yak-boxes/sessions.json"
	if [ -f "$sessions_file" ]; then
		CONTAINER_NAME=$(grep -o '"container": *"[^"]*"' "$sessions_file" | head -1 | sed 's/.*"container": *"\([^"]*\)".*/\1/')
		export CONTAINER_NAME
		[ -n "$CONTAINER_NAME" ]
		return $?
	fi
	return 1
}

# ShellSpec hooks

# This callback function will be invoked only once before loading specfiles.
spec_helper_precheck() {
	: minimum_version "0.28.1"
}

# This callback function will be invoked after a specfile has been loaded.
spec_helper_loaded() {
	:
}

# This callback function will be invoked after core modules have been loaded.
spec_helper_configure() {
	before_all 'setup_test_environment'
	after_all 'teardown_test_environment'
}
