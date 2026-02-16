package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yakthang/yakbox/pkg/types"
)

// SpawnNativeWorker spawns a worker in a Zellij session on the host
func SpawnNativeWorker(worker *types.Worker, persona *types.Persona, prompt string) error {
	workerDir, err := os.MkdirTemp("", "worker-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workerDir)

	promptFile := filepath.Join(workerDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	wrapperScript := filepath.Join(workerDir, "run.sh")
	wrapperContent := fmt.Sprintf(`#!/usr/bin/env bash
PROMPT="$(cat "%s")"
rm -rf "%s"
exec opencode --prompt "$PROMPT" --agent build
`, promptFile, workerDir)
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
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
		return fmt.Errorf("failed to write layout file: %w", err)
	}

	zellijSession := worker.SessionName
	var zellijCmd *exec.Cmd
	if zellijSession != "" {
		zellijCmd = exec.Command("zellij", "--session", zellijSession, "action", "new-tab", "--layout", layoutFile, "--name", worker.DisplayName, "--cwd", worker.CWD)
	} else {
		zellijCmd = exec.Command("zellij", "action", "new-tab", "--layout", layoutFile, "--name", worker.DisplayName, "--cwd", worker.CWD)
	}

	if err := zellijCmd.Run(); err != nil {
		return fmt.Errorf("failed to create zellij tab: %w", err)
	}

	return nil
}

// StopNativeWorker stops a native worker by closing the Zellij tab
func StopNativeWorker(name, sessionName string) error {
	root, _ := findWorkspaceRoot()
	closeTabScript := filepath.Join(root, "close-zellij-tab.sh")

	zellijSession := sessionName

	var cmd *exec.Cmd
	if zellijSession != "" && fileExists(closeTabScript) {
		cmd = exec.Command(closeTabScript, "--session", zellijSession, name)
	} else if fileExists(closeTabScript) {
		cmd = exec.Command(closeTabScript, name)
	} else {
		if zellijSession != "" {
			cmd = exec.Command("zellij", "--session", zellijSession, "action", "close-tab", "-n", name)
		} else {
			cmd = exec.Command("zellij", "action", "close-tab", "-n", name)
		}
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to close zellij tab: %w", err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
