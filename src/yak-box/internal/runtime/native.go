package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wellmaintained/yakthang/src/yak-box/internal/workspace"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/zellij"
	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

// SpawnNativeWorker spawns a worker in a Zellij session on the host.
// Returns the path to the PID file so callers can store it in the session for cleanup.
func SpawnNativeWorker(worker *types.Worker, prompt string, homeDir string) (pidFile string, err error) {
	// Use persistent scripts directory in worker's home
	workerDir := filepath.Join(homeDir, "scripts")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create scripts dir: %w", err)
	}

	promptFile := filepath.Join(workerDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return "", fmt.Errorf("failed to write prompt file: %w", err)
	}

	pidFile = filepath.Join(workerDir, "worker.pid")

	// Resolve API key once; shared by setupClaudeSettings and generateNativeWrapperScript.
	apiKey := ""
	if worker.Tool == "claude" {
		apiKey = resolveAnthropicKey()
		if err := setupClaudeSettings(homeDir, apiKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to setup Claude settings: %v\n", err)
		}
	}

	hostHomeDir := os.Getenv("HOME")
	wrapperContent, _ := generateNativeWrapperScript(worker, homeDir, hostHomeDir, promptFile, pidFile, apiKey)

	wrapperScript := filepath.Join(workerDir, "run.sh")
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		return "", fmt.Errorf("failed to write wrapper script: %w", err)
	}

	layoutFile := filepath.Join(workerDir, "layout.kdl")
	layoutContent := strings.ReplaceAll(zellij.GenerateLayout(worker, "native", worker.Tool), "%WRAPPER%", wrapperScript)
	if err := os.WriteFile(layoutFile, []byte(layoutContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write layout file: %w", err)
	}

	zellijSession := worker.SessionName
	var zellijCmd *exec.Cmd
	if zellijSession != "" {
		zellijCmd = exec.Command("zellij", "--session", zellijSession, "action", "new-tab", "--layout", layoutFile, "--name", worker.DisplayName, "--cwd", worker.CWD)
	} else {
		zellijCmd = exec.Command("zellij", "action", "new-tab", "--layout", layoutFile, "--name", worker.DisplayName, "--cwd", worker.CWD)
	}

	output, err := zellijCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create zellij tab: %w (output: %s)", err, string(output))
	}

	return pidFile, nil
}

// StopNativeWorker stops a native worker by closing the Zellij tab.
// Uses query-tab-names to find the tab's index, then navigates by index
// before closing. This avoids the race where go-to-tab-name fails silently
// and close-tab kills whatever tab happens to be focused.
func StopNativeWorker(name, sessionName string) error {
	root, _ := workspace.FindRoot()
	closeTabScript := filepath.Join(root, "close-zellij-tab.sh")

	// Prefer the script if available (handles edge cases)
	if fileExists(closeTabScript) {
		var cmd *exec.Cmd
		if sessionName != "" {
			cmd = exec.Command(closeTabScript, "--session", sessionName, name)
		} else {
			cmd = exec.Command(closeTabScript, name)
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to close zellij tab via script: %w", err)
		}
		return nil
	}

	tabIndex, err := findZellijTabIndex(name, sessionName)
	if err != nil {
		return err
	}
	if tabIndex == -1 {
		return nil
	}

	var goCmd, closeCmd *exec.Cmd
	if sessionName != "" {
		goCmd = exec.Command("zellij", "--session", sessionName, "action", "go-to-tab", fmt.Sprintf("%d", tabIndex))
		closeCmd = exec.Command("zellij", "--session", sessionName, "action", "close-tab")
	} else {
		goCmd = exec.Command("zellij", "action", "go-to-tab", fmt.Sprintf("%d", tabIndex))
		closeCmd = exec.Command("zellij", "action", "close-tab")
	}

	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed to navigate to tab index %d (%s): %w", tabIndex, name, err)
	}

	if err := closeCmd.Run(); err != nil {
		return fmt.Errorf("failed to close tab: %w", err)
	}

	return nil
}

// findZellijTabIndex queries Zellij for all tab names and returns the 1-based
// index of the tab matching the given name. Returns -1 if not found.
func findZellijTabIndex(name, sessionName string) (int, error) {
	var queryCmd *exec.Cmd
	if sessionName != "" {
		queryCmd = exec.Command("zellij", "--session", sessionName, "action", "query-tab-names")
	} else {
		queryCmd = exec.Command("zellij", "action", "query-tab-names")
	}

	output, err := queryCmd.Output()
	if err != nil {
		return -1, fmt.Errorf("failed to query tab names: %w", err)
	}

	tabs := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, tab := range tabs {
		if tab == name {
			return i + 1, nil // Zellij tabs are 1-indexed
		}
	}

	return -1, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// KillNativeProcessTree reads the PID from pidFile, sends SIGTERM to the
// process group, waits up to timeout, then escalates to SIGKILL.
// This ensures child processes (gopls, bash-language-server, etc.) are also killed.
func KillNativeProcessTree(pidFile string, timeout time.Duration) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read pid file %s: %w", pidFile, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid in %s: %w", pidFile, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	// Signal 0 checks if process is alive without killing it
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		os.Remove(pidFile)
		return nil
	}

	// Send SIGTERM to the entire process group (negative PID kills children too)
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		pgid = pid
	}

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		proc.Signal(syscall.SIGTERM)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			os.Remove(pidFile)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		proc.Signal(syscall.SIGKILL)
	}

	os.Remove(pidFile)
	return nil
}

// generateNativeWrapperScript builds the run.sh wrapper content and pane name
// for a native worker. For Claude, HOME is set to homeDir so that Claude Code
// resolves ~/.claude/skills/ to the worker's own skill directory rather than
// the invoking user's home. To preserve host git/gh auth, it also exports
// GIT_CONFIG_GLOBAL and GH_CONFIG_DIR pointing at hostHomeDir.
// apiKey is embedded directly when non-empty.
func generateNativeWrapperScript(worker *types.Worker, homeDir, hostHomeDir, promptFile, pidFile, apiKey string) (content, paneName string) {
	shaverNameLine := ""
	if worker.ShaverName != "" {
		shaverNameLine = fmt.Sprintf("export YAK_SHAVER_NAME=%q\n", worker.ShaverName)
	}
	switch worker.Tool {
	case "claude":
		paneName = "claude (build) [native]"
		// Set HOME to the worker's home directory so Claude Code finds skills,
		// settings, and other config at <homeDir>/.claude/ instead of the
		// invoking user's real home directory.
		apiKeyLine := ""
		if apiKey != "" {
			apiKeyLine = fmt.Sprintf("export _ANTHROPIC_API_KEY=%q", apiKey)
		}
		gitConfigGlobalLine := ""
		ghConfigDirLine := ""
		if hostHomeDir != "" {
			gitConfigGlobalLine = fmt.Sprintf("export GIT_CONFIG_GLOBAL=%q", filepath.Join(hostHomeDir, ".gitconfig"))
			ghConfigDirLine = fmt.Sprintf("export GH_CONFIG_DIR=%q", filepath.Join(hostHomeDir, ".config", "gh"))
		}
		// Clean CLAUDECODE env var to avoid nested session conflicts.
		content = fmt.Sprintf(`#!/usr/bin/env bash
export HOME=%q
%s
%s
export IS_DEMO=true
%sexport YAK_PATH="%s"
%s
unset CLAUDECODE
MODEL=%q
PROMPT_FILE=%q
CLAUDE_ARGS=(--dangerously-skip-permissions)
if [[ -n "$MODEL" ]]; then
  CLAUDE_ARGS+=(--model "$MODEL")
fi
# Suppress macOS keychain dialogs: Claude Code (or its Node.js/keytar dependency)
# tries to persist credentials to the default keychain. With HOME set to the
# worker directory the default keychain may be inaccessible, causing a macOS
# dialog. Create a worker-local keychain, make it the default, and restore the
# original on exit so the host user's keychain configuration is not permanently
# altered. All security(1) calls are silenced so non-macOS hosts are unaffected.
_ORIG_DEFAULT_KEYCHAIN=$(security default-keychain 2>/dev/null | tr -d '"' | xargs)
_WORKER_KEYCHAIN="$HOME/Library/Keychains/worker.keychain-db"
mkdir -p "$HOME/Library/Keychains"
security create-keychain -p "" "$_WORKER_KEYCHAIN" 2>/dev/null || true
security unlock-keychain -p "" "$_WORKER_KEYCHAIN" 2>/dev/null || true
security set-default-keychain "$_WORKER_KEYCHAIN" 2>/dev/null || true
_restore_keychain() { [[ -n "$_ORIG_DEFAULT_KEYCHAIN" ]] && security set-default-keychain "$_ORIG_DEFAULT_KEYCHAIN" 2>/dev/null; true; }
trap _restore_keychain EXIT
# Write PID before running Claude so yak-box stop can find and kill the process tree.
echo $$ > "%s"
claude "${CLAUDE_ARGS[@]}" @"$PROMPT_FILE"
`, homeDir, gitConfigGlobalLine, ghConfigDirLine, shaverNameLine, worker.YakPath, apiKeyLine, worker.Model, promptFile, pidFile)
	case "cursor":
		paneName = "cursor (build) [native]"
		content = fmt.Sprintf(`#!/usr/bin/env bash
%sexport YAK_PATH="%s"
PROMPT="$(cat "%s")"
MODEL=%q
# Write PID before exec so yak-box stop can find and kill the process tree.
echo $$ > "%s"
if [[ -n "$MODEL" ]]; then
  exec agent --force --model "$MODEL" --workspace "%s" "$PROMPT"
else
  exec agent --force --workspace "%s" "$PROMPT"
fi
`, shaverNameLine, worker.YakPath, promptFile, worker.Model, pidFile, worker.CWD, worker.CWD)
	default:
		paneName = "opencode (build) [native]"
		content = fmt.Sprintf(`#!/usr/bin/env bash
%sexport YAK_PATH="%s"
PROMPT="$(cat "%s")"
# Write PID before exec so yak-box stop can find and kill the process tree.
# exec replaces this process, so $$ will be the PID of opencode.
echo $$ > "%s"
exec opencode --prompt "$PROMPT" --agent build
`, shaverNameLine, worker.YakPath, promptFile, pidFile)
	}
	return content, paneName
}

// setupClaudeSettings configures Claude Code settings for the worker.
// When an API key is provided, it injects apiKeyHelper so workers use API key
// auth non-interactively. When no API key is present (OAuth mode), the helper
// is omitted so Claude Code falls through to its OAuth credentials.
// It also preserves statusline config when goccc exists.
func setupClaudeSettings(homeDir, apiKey string) error {
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}
	debugDir := filepath.Join(claudeDir, "debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude/debug directory: %w", err)
	}

	// Only write apiKeyHelper when an API key is available.
	// In OAuth mode (Max/Pro subscription), omitting the helper lets Claude Code
	// use its own OAuth credentials from ~/.claude/ instead.
	apiKeyHelperPath := ""
	if apiKey != "" {
		apiKeyHelperPath = filepath.Join(claudeDir, "api-key-helper.sh")
		apiKeyHelper := "#!/usr/bin/env bash\n" +
			"echo \"${_ANTHROPIC_API_KEY}\"\n"
		if err := os.WriteFile(apiKeyHelperPath, []byte(apiKeyHelper), 0755); err != nil {
			return fmt.Errorf("failed to write api-key-helper.sh: %w", err)
		}
	}

	// Pre-seed .claude.json so Claude Code starts without blocking on
	// onboarding or permissions prompts, and pre-approves the key suffix.
	claudeJSONPath := filepath.Join(homeDir, ".claude.json")
	suffix := apiKey
	if len(apiKey) > 20 {
		suffix = apiKey[len(apiKey)-20:]
	}
	if err := os.WriteFile(claudeJSONPath, []byte(buildClaudeJSONContent(suffix)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write .claude.json: %v\n", err)
	}
	remoteSettingsPath := filepath.Join(claudeDir, "remote-settings.json")
	if _, statErr := os.Stat(remoteSettingsPath); os.IsNotExist(statErr) {
		if err := os.WriteFile(remoteSettingsPath, []byte("{}"), 0644); err != nil {
			return fmt.Errorf("failed to write remote-settings.json: %w", err)
		}
	}

	settingsFile := filepath.Join(claudeDir, "settings.json")
	settings := map[string]any{}
	if apiKeyHelperPath != "" {
		settings["apiKeyHelper"] = apiKeyHelperPath
	}
	if _, err := exec.LookPath("goccc"); err == nil {
		settings["statusLine"] = map[string]string{
			"type":    "command",
			"command": "goccc -statusline",
		}
	}
	settingsData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Claude settings: %w", err)
	}
	settingsData = append(settingsData, '\n')
	if err := os.WriteFile(settingsFile, settingsData, 0644); err != nil {
		return fmt.Errorf("failed to write Claude settings: %w", err)
	}

	return nil
}
