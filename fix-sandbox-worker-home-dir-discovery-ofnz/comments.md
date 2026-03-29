Root cause: StopSandboxWorker received the yak-name (session key) but searched for .yak-boxes/@home/<yak-name>/ — the directory is actually named after the shaver (e.g. Yakoff), not the task.

Fix: added HomeDir field to Session struct, stored at spawn time, passed directly to StopSandboxWorker. The function now accepts homeDir as a parameter and only falls back to the walk-up discovery when homeDir is empty (backwards compat for sessions created before this fix).

Files changed:
- internal/sessions/sessions.go — added HomeDir field to Session
- cmd/spawn.go — stores homeDir in session registration
- internal/runtime/spawn_sandbox.go — StopSandboxWorker now takes homeDir param
- cmd/stop.go — passes session.HomeDir to StopSandboxWorker
- internal/runtime/spawn_sandbox_test.go — updated existing test, added regression test for name mismatch
