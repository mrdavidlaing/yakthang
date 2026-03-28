Implemented SpawnSandboxWorker and StopSandboxWorker in spawn_sandbox.go.

SpawnSandboxWorker follows the native worker pattern: creates scripts dir, prompt file, PID file, generates srt config via GenerateSrtConfig, then builds a wrapper script that invokes the tool command through 'srt --settings <config-path> -- <tool-command>'. Launches via Zellij tab.

StopSandboxWorker finds the worker home dir by walking up from cwd, kills the process tree via PID file, cleans up the srt config temp file, and closes the Zellij tab.

Also created spawn_sandbox_test.go with 7 tests covering wrapper generation for all tools (claude/cursor/opencode), shaver name propagation, home dir discovery, and srt config cleanup on stop.
