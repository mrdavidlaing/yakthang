Changed 3 files:
- src/yak-box/internal/runtime/native.go: CLAUDE_ARGS reverted to --dangerously-skip-permissions
- src/yak-box/internal/runtime/spawn_sandbox.go: same
- src/yak-box/internal/runtime/helpers_test.go: test assertion and error message updated

agents/yakob.md had no --permission-mode auto references — no changes needed there.
yakstead/orchestrator.kdl was already reverted per the brief.

go test ./... passes.
