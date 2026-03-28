Added 'sandbox' as a valid --runtime value with routing to stub functions.

Changes:
- src/yak-box/internal/runtime/spawn_sandbox.go (new): SpawnSandboxWorker and StopSandboxWorker stubs that return 'sandbox runtime not yet implemented'
- src/yak-box/cmd/spawn.go: runtime dispatch switch now routes sandbox to SpawnSandboxWorker; updated flag help text
- src/yak-box/cmd/stop.go: added sandbox branch to stop dispatch, routes to StopSandboxWorker

The validation (line 88) and preflight dispatch (line 194) already had sandbox support from a prior commit. go test ./... passes.
