package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yakthang/yakbox/internal/workspace"
	"github.com/yakthang/yakbox/pkg/types"
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

	wrapperScript := filepath.Join(workerDir, "run.sh")
	wrapperContent := fmt.Sprintf(`#!/usr/bin/env bash
export YAK_PATH="%s"
PROMPT="$(cat "%s")"
# Write PID before exec so yak-box stop can find and kill the process tree.
# exec replaces this process, so $$ will be the PID of opencode.
echo $$ > "%s"
exec opencode --prompt "$PROMPT" --agent build
`, worker.YakPath, promptFile, pidFile)
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		return "", fmt.Errorf("failed to write wrapper script: %w", err)
	}

	layoutFile := filepath.Join(workerDir, "layout.kdl")
	layoutContent := fmt.Sprintf(`layout {
    tab name="%s" cwd="%s" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane size="67%%" name="opencode (build)" focus=true {
            command "bash"
            args "%s"
        }
        pane size="33%%" name="shell: %s"
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, worker.CWD, wrapperScript, worker.CWD)
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

	if err := zellijCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create zellij tab: %w", err)
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
