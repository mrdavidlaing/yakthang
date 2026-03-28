Deleted the old sandbox_smoke.sh that tested srt directly with hand-crafted configs. Rewrote with 6 integration tests that exercise yak-box's code:

1. Preflight rejection — uses pre-built yak-box binary with restricted PATH
2. Config validation — calls GenerateSrtConfig via Go test helper, verifies CWD in allowWrite, domains, allowLocalBinding
3. Wrapper script generation — runs existing Go unit tests for generateSandboxWrapperScript
4. Filesystem allow — generates config via yak-box's Go code, then uses srt to verify write to CWD succeeds
5. Filesystem block — same config, verifies write to /var/tmp (outside allowWrite) is blocked
6. Stop cleanup — runs existing Go unit test for StopSandboxWorker config cleanup

Added srt_config_shellspec_test.go as a committed Go test helper that shellspec calls via `go test -run`. It calls GenerateSrtConfig (the same codepath as yak-box spawn) and validates the JSON structure. Skips when SANDBOX_CWD is not set, so it doesn't run in normal `go test`.

Key design choice: tests 4-5 use yak-box's ACTUAL generated config (via the Go helper) and then call srt directly to verify the isolation. This tests the integration between yak-box's config generation and srt's enforcement.
