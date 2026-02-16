package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yakthang/yakbox/internal/runtime"
	"github.com/yakthang/yakbox/internal/sessions"
)

var (
	stopName    string
	stopTimeout string
	stopForce   bool
	stopDryRun  bool
)

var stopCmd = &cobra.Command{
	Use:   "stop --name <worker-name> [flags]",
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
  yak-box stop --name api-auth

  # Force stop without cleanup (immediate termination)
  yak-box stop --name api-auth --force

  # Dry run to see what would happen
  yak-box stop --name api-auth --dry-run

  # Stop with custom timeout
  yak-box stop --name backend-worker --timeout 60s`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runStop() error {
	fmt.Printf("Stopping worker: %s\n", stopName)

	timeout, err := time.ParseDuration(stopTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
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
						Runtime:     "sandboxed",
						Container:   containerName,
						DisplayName: stopName,
					}
					break
				}
			}
		}

		if session == nil {
			return fmt.Errorf("worker not found")
		}
	}

	yakPath := ".yaks"
	if !stopForce && session.Task != "" {
		fmt.Println("Clearing task assignments...")
		taskFile := filepath.Join(yakPath, session.Task, "assigned-to")
		if err := os.Remove(taskFile); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: Failed to clear assignment for %s: %v\n", session.Task, err)
		} else {
			fmt.Printf("Cleared assignment: %s\n", session.Task)
		}
	}

	if session.Runtime == "sandboxed" {
		if stopDryRun {
			fmt.Printf("[dry-run] Would stop container: %s\n", session.Container)
			fmt.Printf("[dry-run] Would close Zellij tab: %s\n", session.DisplayName)
		} else {
			fmt.Println("Stopping container...")
			if err := runtime.StopSandboxedWorker(stopName, timeout); err != nil {
				fmt.Printf("Warning: %v\n", err)
			}
			fmt.Println("Closing Zellij tab...")
			if err := runtime.StopNativeWorker(session.DisplayName, session.ZellijSession); err != nil {
				fmt.Printf("Warning: failed to close tab: %v\n", err)
			}
		}
	} else if session.Runtime == "native" {
		if stopDryRun {
			fmt.Printf("[dry-run] Would close Zellij tab: %s\n", session.DisplayName)
		} else {
			fmt.Println("Closing Zellij tab...")
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

	fmt.Printf("Worker stopped: %s\n", stopName)
	return nil
}

func init() {
	stopCmd.Flags().StringVar(&stopName, "name", "", "Worker name to stop (required)")
	stopCmd.MarkFlagRequired("name")

	stopCmd.Flags().StringVar(&stopTimeout, "timeout", "30s", "Docker stop timeout (e.g., '30s', '1m')")
	stopCmd.Flags().BoolVarP(&stopForce, "force", "f", false, "Skip task cleanup and stop immediately")
	stopCmd.Flags().BoolVar(&stopDryRun, "dry-run", false, "Show what would happen without actually stopping")
}
