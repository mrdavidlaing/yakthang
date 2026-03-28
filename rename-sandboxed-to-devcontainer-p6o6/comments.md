Renamed --runtime sandboxed to --runtime devcontainer throughout yak-box.

Changes:
- Renamed sandboxed.go → spawn_devcontainer.go (sandboxed_test.go → spawn_devcontainer_test.go)
  Note: devcontainer.go already existed with Docker image management code, so used spawn_devcontainer.go to avoid collision.
- Renamed functions: SpawnSandboxedWorker → SpawnDevcontainerWorker, StopSandboxedWorker → StopDevcontainerWorker, SpawnSandboxedDeps → SpawnDevcontainerDeps
- Changed runtime string "sandboxed" → "devcontainer" everywhere (DetectRuntime, session registration, layout generation, opencode dispatch, stop dispatch, preflight checks, tests, shellspec, README)
- Added deprecated alias: --runtime=sandboxed still accepted but prints warning and maps to devcontainer
- Updated all test names and assertions to use "devcontainer"
- All 16 Go test packages pass
