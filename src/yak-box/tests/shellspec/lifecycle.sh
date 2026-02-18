# shellcheck shell=bash
Describe 'yak-box lifecycle'
# Skip entire suite if any prerequisite is missing
Skip if "docker not available" docker_unavailable
Skip if "zellij not available" zellij_unavailable
Skip if "yak-shaver:latest image not found" image_unavailable
Skip if "yak-box binary not found" yakbox_binary_unavailable

It 'spawns a sandboxed worker'
run_spawn() {
	yak-box spawn \
		--cwd "$TEST_WORK_DIR" \
		--name "$TEST_WORKER_NAME" \
		--session "$TEST_ZELLIJ_SESSION" \
		--runtime sandboxed \
		--yak-path "$TEST_WORK_DIR/.yaks" \
		--yaks test-task
}
When call run_spawn
The status should be success
The output should include "Spawned"
The output should include "sandboxed"
End

It 'creates a Docker container (running or exited)'
check_container() {
	discover_container_name
	wait_for_container "$TEST_WORKER_NAME"
}
When call check_container
The status should be success
End

It 'registers session in sessions.json'
check_session() {
	local sessions_file="$TEST_WORK_DIR/.yak-boxes/sessions.json"
	test -f "$sessions_file" || {
		echo "sessions.json not found"
		return 1
	}
	grep -q "\"$TEST_WORKER_NAME\"" "$sessions_file" || {
		echo "worker not in sessions.json"
		return 1
	}
	grep -q '"runtime": "sandboxed"' "$sessions_file" || {
		echo "runtime not sandboxed"
		return 1
	}
	grep -q '"task": "test-task"' "$sessions_file" || {
		echo "task not test-task"
		return 1
	}
}
When call check_session
The status should be success
End

It 'creates assigned-to file'
check_assigned() {
	test -f "$TEST_WORK_DIR/.yaks/test-task/assigned-to"
}
When call check_assigned
The status should be success
End

It 'stops the worker and reports success'
run_stop() {
	cd "$TEST_WORK_DIR" || return 1
	yak-box stop --name "$TEST_WORKER_NAME"
}
When call run_stop
The status should be success
The output should include "Worker stopped"
End

It 'removes container after stop'
check_container_gone() {
	wait_for_container_gone "$TEST_WORKER_NAME"
}
When call check_container_gone
The status should be success
End

It 'removes session from sessions.json after stop'
check_session_gone() {
	local sessions_file="$TEST_WORK_DIR/.yak-boxes/sessions.json"
	if [ ! -f "$sessions_file" ]; then
		return 0
	fi
	! grep -q "\"${TEST_WORKER_NAME}\"" "$sessions_file"
}
When call check_session_gone
The status should be success
End

It 'clears assigned-to file after stop'
check_assigned_cleared() {
	! test -f "$TEST_WORK_DIR/.yaks/test-task/assigned-to"
}
When call check_assigned_cleared
The status should be success
End
End
