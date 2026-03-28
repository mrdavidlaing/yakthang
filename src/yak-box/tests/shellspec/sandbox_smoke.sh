# shellcheck shell=bash
Describe 'sandbox integration (yak-box)'
# Skip entire suite if sandbox deps are missing.
# These are the same deps checked by preflight.SpawnSandboxDeps() on Linux.
srt_unavailable() { ! command -v srt >/dev/null 2>&1; }
bwrap_unavailable() { ! command -v bwrap >/dev/null 2>&1; }
socat_unavailable() { ! command -v socat >/dev/null 2>&1; }
Skip if "srt not available" srt_unavailable
Skip if "bwrap not available" bwrap_unavailable
Skip if "socat not available" socat_unavailable

# --- helpers ---

setup_sandbox_env() {
	SANDBOX_CWD=$(mktemp -d)
	export SANDBOX_CWD

	# Build a fresh yak-box binary so preflight test can use it with restricted PATH
	YAKBOX_BIN="${TEST_PROJECT_DIR}/yak-box-test"
	if [ ! -x "$YAKBOX_BIN" ]; then
		(cd "$TEST_PROJECT_DIR" && go build -o yak-box-test .) || return 1
	fi
	export YAKBOX_BIN
}

teardown_sandbox_env() {
	rm -rf "$SANDBOX_CWD" 2>/dev/null || true
}

# Generate an srt config via yak-box's GenerateSrtConfig Go code.
# Prints the path to the generated config file; caller must clean up.
generate_srt_config() {
	local cwd="$1"
	SANDBOX_CWD="$cwd" go test ./internal/runtime/ \
		-run TestSrtConfigForShellspec -count=1 -v 2>&1 | \
		grep '^SRT_CONFIG_PATH=' | head -1 | cut -d= -f2-
}

Before 'setup_sandbox_env'
After 'teardown_sandbox_env'

# --- test cases ---

It 'preflight rejects missing srt when not in PATH'
	run_preflight_no_srt() {
		# Use a restricted PATH that excludes srt but keeps basic utils
		env PATH="/usr/bin:/bin" "$YAKBOX_BIN" spawn \
			--runtime sandbox \
			--cwd /tmp \
			--yak-name preflight-test 2>&1
	}
	When call run_preflight_no_srt
	The status should not be success
	The output should include "srt"
End

It 'GenerateSrtConfig produces valid JSON with CWD in allowWrite'
	run_config_validation() {
		cd "$TEST_PROJECT_DIR" || return 1
		SANDBOX_CWD="$SANDBOX_CWD" go test ./internal/runtime/ \
			-run TestSrtConfigForShellspec -count=1 -v 2>&1
	}
	When call run_config_validation
	The status should be success
	The output should include "CONFIG_VALID=true"
End

It 'Go unit tests for wrapper script generation pass'
	run_wrapper_tests() {
		cd "$TEST_PROJECT_DIR" || return 1
		go test ./internal/runtime/ \
			-run 'TestGenerateSandboxWrapperScript' -count=1 -v 2>&1
	}
	When call run_wrapper_tests
	The status should be success
	The output should include "PASS"
End

It 'allows writing to CWD via yak-box generated config'
	run_write_allowed() {
		cd "$TEST_PROJECT_DIR" || return 1
		local config_path
		config_path=$(generate_srt_config "$SANDBOX_CWD")

		if [ -z "$config_path" ]; then
			echo "FAILED: could not generate srt config"
			return 1
		fi

		# Run a command under srt using yak-box's generated config
		srt --settings "$config_path" -c \
			"echo test-payload > '${SANDBOX_CWD}/smoke-write.txt'" 2>/dev/null

		# Verify from host side
		if [ -f "${SANDBOX_CWD}/smoke-write.txt" ]; then
			echo "WRITE_SUCCEEDED"
			cat "${SANDBOX_CWD}/smoke-write.txt"
		else
			echo "WRITE_FAILED"
		fi

		rm -f "$config_path"
	}
	When call run_write_allowed
	The status should be success
	The output should include "WRITE_SUCCEEDED"
	The output should include "test-payload"
End

It 'blocks writing outside CWD via yak-box generated config'
	run_write_blocked() {
		cd "$TEST_PROJECT_DIR" || return 1
		# Use /var/tmp which is outside both CWD and /tmp (both in allowWrite)
		local blocked_dir="/var/tmp/sandbox-smoke-blocked-$$"
		mkdir -p "$blocked_dir"
		local config_path
		config_path=$(generate_srt_config "$SANDBOX_CWD")

		if [ -z "$config_path" ]; then
			echo "FAILED: could not generate srt config"
			rm -rf "$blocked_dir"
			return 1
		fi

		# Attempt write outside CWD — should fail inside sandbox
		srt --settings "$config_path" -c \
			"echo nope > '${blocked_dir}/blocked.txt' 2>&1 || echo WRITE_DENIED" 2>&1

		# Verify from host side that file was NOT created
		if [ ! -f "${blocked_dir}/blocked.txt" ]; then
			echo "FILE_NOT_CREATED"
		else
			echo "FILE_WAS_CREATED"
		fi

		rm -f "$config_path"
		rm -rf "$blocked_dir"
	}
	When call run_write_blocked
	The output should include "WRITE_DENIED"
	The output should include "FILE_NOT_CREATED"
End

It 'StopSandboxWorker cleans up srt config temp file'
	run_stop_cleanup() {
		cd "$TEST_PROJECT_DIR" || return 1
		go test ./internal/runtime/ \
			-run TestStopSandboxWorker_CleansUpSrtConfig -count=1 -v 2>&1
	}
	When call run_stop_cleanup
	The status should be success
	The output should include "PASS"
End

End
