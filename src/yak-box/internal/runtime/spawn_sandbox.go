package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wellmaintained/yakthang/src/yak-box/internal/zellij"
	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

// SpawnSandboxWorker spawns a worker in the sandbox runtime.
// This is the native runtime wrapped with srt for filesystem and network sandboxing.
func SpawnSandboxWorker(ctx context.Context, opts ...SpawnOption) error {
	cfg := &spawnConfig{
		commander: &defaultCommander{},
	}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("option error: %w", err)
		}
	}

	if cfg.worker == nil {
		return fmt.Errorf("worker is required")
	}

	workerDir := filepath.Join(cfg.homeDir, "scripts")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create scripts dir: %w", err)
	}

	promptFile := filepath.Join(workerDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(cfg.prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	pidFile := filepath.Join(workerDir, "worker.pid")

	// Generate srt sandbox config
	srtConfigPath, err := GenerateSrtConfig(cfg.worker.CWD)
	if err != nil {
		return fmt.Errorf("failed to generate srt config: %w", err)
	}

	// Store the srt config path so StopSandboxWorker can clean it up
	srtConfigRef := filepath.Join(workerDir, "srt-config-path")
	if err := os.WriteFile(srtConfigRef, []byte(srtConfigPath), 0644); err != nil {
		os.Remove(srtConfigPath)
		return fmt.Errorf("failed to write srt config ref: %w", err)
	}

	// Resolve API key; shared by setupClaudeSettings and the wrapper script.
	apiKey := ""
	if cfg.worker.Tool == "claude" {
		apiKey = resolveAnthropicKey()
		if err := setupClaudeSettings(cfg.homeDir, apiKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to setup Claude settings: %v\n", err)
		}
	}

	wrapperContent := generateSandboxWrapperScript(cfg.worker, cfg.homeDir, promptFile, pidFile, srtConfigPath, apiKey)

	wrapperScript := filepath.Join(workerDir, "run.sh")
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	layoutFile := filepath.Join(workerDir, "layout.kdl")
	layoutContent := strings.ReplaceAll(zellij.GenerateLayout(cfg.worker, "sandbox", cfg.worker.Tool), "%WRAPPER%", wrapperScript)
	if err := os.WriteFile(layoutFile, []byte(layoutContent), 0644); err != nil {
		return fmt.Errorf("failed to write layout file: %w", err)
	}

	var zellijCmd *exec.Cmd
	if cfg.worker.SessionName != "" {
		zellijCmd = cfg.commander.CommandContext(ctx, "zellij", "--session", cfg.worker.SessionName, "action", "new-tab", "--layout", layoutFile, "--name", cfg.worker.DisplayName, "--cwd", cfg.worker.CWD)
	} else {
		zellijCmd = cfg.commander.CommandContext(ctx, "zellij", "action", "new-tab", "--layout", layoutFile, "--name", cfg.worker.DisplayName, "--cwd", cfg.worker.CWD)
	}

	output, err := zellijCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create zellij tab: %w (output: %s)", err, string(output))
	}

	return nil
}

// StopSandboxWorker stops a sandbox worker by killing the process tree,
// closing the Zellij tab, and cleaning up the srt config temp file.
func StopSandboxWorker(name string, timeout time.Duration) error {
	// Find the worker's home dir to locate PID file and srt config
	homeDir, err := findWorkerHomeDir(name)
	if err != nil {
		return fmt.Errorf("failed to find worker home dir: %w", err)
	}

	workerDir := filepath.Join(homeDir, "scripts")
	pidFile := filepath.Join(workerDir, "worker.pid")

	// Kill the process tree if PID file exists
	if fileExists(pidFile) {
		if err := KillNativeProcessTree(pidFile, timeout); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill process tree: %v\n", err)
		}
	}

	// Clean up srt config temp file
	srtConfigRef := filepath.Join(workerDir, "srt-config-path")
	if data, err := os.ReadFile(srtConfigRef); err == nil {
		srtConfigPath := strings.TrimSpace(string(data))
		if srtConfigPath != "" {
			os.Remove(srtConfigPath)
		}
		os.Remove(srtConfigRef)
	}

	// Close the Zellij tab
	if err := StopNativeWorker(name, ""); err != nil {
		return fmt.Errorf("failed to close zellij tab: %w", err)
	}

	return nil
}

// findWorkerHomeDir locates the home directory for a named worker.
// Workers are stored under .yak-boxes/@home/<name>/ relative to the workspace root.
func findWorkerHomeDir(name string) (string, error) {
	// Walk up from CWD to find the workspace root containing .yak-boxes
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ".yak-boxes", "@home", name)
		if fileExists(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("worker home dir not found for %q", name)
}

// generateSandboxWrapperScript builds the run.sh wrapper that invokes the tool
// command through srt for filesystem and network sandboxing.
func generateSandboxWrapperScript(worker *types.Worker, homeDir, promptFile, pidFile, srtConfigPath, apiKey string) string {
	shaverNameLine := ""
	if worker.ShaverName != "" {
		shaverNameLine = fmt.Sprintf("export YAK_SHAVER_NAME=%q\n", worker.ShaverName)
	}

	// Build the inner tool command (same as native, but will be wrapped with srt)
	var toolCmd string
	switch worker.Tool {
	case "claude":
		apiKeyLine := ""
		if apiKey != "" {
			apiKeyLine = fmt.Sprintf("export _ANTHROPIC_API_KEY=%q\n", apiKey)
		}
		claudeConfigDir := filepath.Join(homeDir, ".claude")
		toolCmd = fmt.Sprintf(`export CLAUDE_CONFIG_DIR=%q
%sexport IS_DEMO=true
export YAK_PATH="%s"
%s
unset CLAUDECODE
MODEL=%q
PROMPT_FILE=%q
CLAUDE_ARGS=(--dangerously-skip-permissions)
if [[ -n "$MODEL" ]]; then
  CLAUDE_ARGS+=(--model "$MODEL")
fi
srt --settings %q -- claude "${CLAUDE_ARGS[@]}" @"$PROMPT_FILE"`,
			claudeConfigDir, shaverNameLine, worker.YakPath, apiKeyLine,
			worker.Model, promptFile, srtConfigPath)

	case "cursor":
		toolCmd = fmt.Sprintf(`%sexport YAK_PATH="%s"
PROMPT="$(cat "%s")"
MODEL=%q
if [[ -n "$MODEL" ]]; then
  srt --settings %q -- agent --force --model "$MODEL" --workspace "%s" "$PROMPT"
else
  srt --settings %q -- agent --force --workspace "%s" "$PROMPT"
fi`,
			shaverNameLine, worker.YakPath, promptFile, worker.Model,
			srtConfigPath, worker.CWD, srtConfigPath, worker.CWD)

	default: // opencode
		toolCmd = fmt.Sprintf(`%sexport YAK_PATH="%s"
PROMPT="$(cat "%s")"
srt --settings %q -- opencode --prompt "$PROMPT" --agent build`,
			shaverNameLine, worker.YakPath, promptFile, srtConfigPath)
	}

	return fmt.Sprintf(`#!/usr/bin/env bash
# Write PID before running tool so yak-box stop can find and kill the process tree.
echo $$ > "%s"
%s
`, pidFile, toolCmd)
}
