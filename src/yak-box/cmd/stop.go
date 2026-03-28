package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/errors"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/preflight"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/runtime"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/sessions"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/ui"
)

var (
	stopName    string
	stopTimeout string
	stopForce   bool
	stopDryRun  bool
)

var stopCmd = &cobra.Command{
	Use:   "stop --yak-name <worker-name> [flags]",
	Short: "Stop a worker",
	Long: `Stop a running worker, optionally forcing termination.

The stop command gracefully shuts down a worker by:
1. Loading session from .yak-boxes/sessions.json
2. Clearing task assignments (unless --force is set)
3. Stopping the container or closing the Zellij tab
4. Unregistering the session (home directory is preserved)

If session is missing, the command attempts to detect the worker
via Docker ps or Zellij tabs as a fallback.`,
	Example: `  # Gracefully stop a worker (clears task assignments)
  yak-box stop --yak-name api-auth

  # Force stop without cleanup (immediate termination)
  yak-box stop --yak-name api-auth --force

  # Dry run to see what would happen
  yak-box stop --yak-name api-auth --dry-run

  # Stop with custom timeout
  yak-box stop --yak-name backend-worker --timeout 60s`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []error

		// Validate required flags
		if stopName == "" {
			errs = append(errs, fmt.Errorf("--yak-name is required (worker name to stop)"))
		}

		// Validate timeout format
		if stopTimeout != "" {
			if _, err := time.ParseDuration(stopTimeout); err != nil {
				errs = append(errs, fmt.Errorf("--timeout has invalid format: %v (use '30s', '1m', '5m30s', etc.)", err))
			}
		}

		if len(errs) > 0 {
			return errors.CombineValidation(errs)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

func runStop() error {
	if err := preflight.Run(preflight.StopDeps(), os.Stderr); err != nil {
		return err
	}

	ui.Info("⏳ Stopping worker: %s...\n", stopName)

	timeout, err := time.ParseDuration(stopTimeout)
	if err != nil {
		return errors.NewValidationError("invalid timeout format. Use a valid duration like '30s', '1m', or '5m30s'", err)
	}

	session, err := sessions.Get(stopName)
	if err != nil {
		fmt.Printf("Warning: Could not load session: %v\n", err)
		fmt.Println("Attempting fallback detection...")

		containerName := "yak-worker-" + stopName
		workers, err := runtime.ListAllContainers()
		if err == nil && len(workers) > 0 {
			for _, w := range workers {
				if w == containerName {
					session = &sessions.Session{
						Runtime:     "devcontainer",
						Container:   containerName,
						DisplayName: stopName,
					}
					break
				}
			}
		}

		if session == nil {
			return errors.NewValidationError("worker not found. Use 'docker ps' or 'zellij list-sessions' to find running workers, or check .yak-boxes/sessions.json", nil)
		}
	}

	yakPath := ".yaks"
	absYakPath, _ := filepath.Abs(yakPath)

	// Resolve task directories: prefer stored TaskDirs, fall back to resolveYakValue.
	var resolvedTaskDirs []string
	if !stopForce && (len(session.TaskDirs) > 0 || session.Task != "") {
		if len(session.TaskDirs) > 0 {
			resolvedTaskDirs = session.TaskDirs
		} else {
			taskDir, _, errResolve := resolveYakValue(absYakPath, session.Task)
			if errResolve != nil {
				fmt.Printf("Warning: Failed to find task directory for %s: %v\n", session.Task, errResolve)
			} else {
				resolvedTaskDirs = []string{taskDir}
			}
		}
	}

	if !stopForce && len(resolvedTaskDirs) > 0 {
		ui.Info("⏳ Clearing task assignments...\n")
		for _, taskDir := range resolvedTaskDirs {
			taskFile := filepath.Join(taskDir, "assigned-to")
			if err := os.Remove(taskFile); err != nil && !os.IsNotExist(err) {
				fmt.Printf("Warning: Failed to clear assignment for %s: %v\n", taskDir, err)
			} else {
				ui.Success("✅ Cleared assignment: %s\n", taskDir)
			}
		}
	}

	if !stopForce && len(resolvedTaskDirs) > 0 {
		cost := extractWorkerCost(session)
		if cost != "" {
			ui.Info("💰 Session cost: %s\n", cost)
			for _, taskDir := range resolvedTaskDirs {
				spendFile := filepath.Join(taskDir, "spend")
				if err := os.WriteFile(spendFile, []byte(cost), 0644); err != nil {
					fmt.Printf("Warning: Failed to write spend field: %v\n", err)
				}
			}
		}
	}

	if session.Runtime == "devcontainer" {
		if stopDryRun {
			fmt.Printf("[dry-run] Would close Zellij tab: %s\n", session.DisplayName)
			fmt.Printf("[dry-run] Would stop container: %s\n", session.Container)
		} else {
			ui.Info("⏳ Closing Zellij tab...\n")
			if err := runtime.StopNativeWorker(session.DisplayName, session.ZellijSession); err != nil {
				fmt.Printf("Warning: failed to close tab: %v\n", err)
			}
			ui.Info("⏳ Stopping container...\n")
			if err := runtime.StopDevcontainerWorker(stopName, timeout); err != nil {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	} else if session.Runtime == "native" {
		if stopDryRun {
			fmt.Printf("[dry-run] Would kill native process tree via PID file: %s\n", session.PidFile)
			fmt.Printf("[dry-run] Would close Zellij tab: %s\n", session.DisplayName)
		} else {
			if session.PidFile != "" {
				ui.Info("⏳ Killing native process tree...\n")
				if err := runtime.KillNativeProcessTree(session.PidFile, timeout); err != nil {
					fmt.Printf("Warning: failed to kill process tree: %v\n", err)
				} else {
					ui.Success("✅ Process tree terminated\n")
				}
			}
			ui.Info("⏳ Closing Zellij tab...\n")
			if err := runtime.StopNativeWorker(session.DisplayName, session.ZellijSession); err != nil {
				fmt.Printf("Warning: failed to close tab: %v\n", err)
			}
		}
	}

	if !stopDryRun {
		if err := sessions.Unregister(stopName); err != nil {
			fmt.Printf("Warning: Failed to unregister session: %v\n", err)
		}
	}

	ui.Success("✅ Stopped: %s\n", stopName)
	return nil
}

func extractWorkerCost(session *sessions.Session) string {
	if session.Runtime == "devcontainer" {
		cmd := exec.Command("docker", "exec", session.Container, "goccc", "-days", "0", "-json")
		output, err := cmd.Output()
		if err == nil {
			jqCmd := exec.Command("docker", "exec", session.Container, "sh", "-c", "echo '"+string(output)+"' | jq -r '.summary.total_cost // \"0\"'")
			costOutput, err := jqCmd.Output()
			if err == nil {
				cost := strings.TrimSpace(string(costOutput))
				if cost != "0" && cost != "" {
					return cost
				}
			}
		}
	} else if session.Runtime == "native" {
		cmd := exec.Command("goccc", "-days", "0", "-json")
		output, err := cmd.Output()
		if err == nil {
			jqCmd := exec.Command("sh", "-c", "echo '"+string(output)+"' | jq -r '.summary.total_cost // \"0\"'")
			costOutput, err := jqCmd.Output()
			if err == nil {
				cost := strings.TrimSpace(string(costOutput))
				if cost != "0" && cost != "" {
					return cost
				}
			}
		}
	}
	return ""
}

func init() {
	stopCmd.Flags().StringVar(&stopName, "yak-name", "", "Worker name to stop (required)")
	stopCmd.MarkFlagRequired("yak-name")

	stopCmd.Flags().StringVar(&stopTimeout, "timeout", "30s", "Docker stop timeout (e.g., '30s', '1m')")
	stopCmd.Flags().BoolVarP(&stopForce, "force", "f", false, "Skip task cleanup and stop immediately")
	stopCmd.Flags().BoolVar(&stopDryRun, "dry-run", false, "Show what would happen without actually stopping")
}
