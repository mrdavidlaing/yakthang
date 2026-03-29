package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/errors"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/preflight"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/runtime"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/sessions"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/ui"
)

var (
	checkBlocked bool
	checkWIP     bool
	checkPrefix  string
)

var checkCmd = &cobra.Command{
	Use:   "check [flags]",
	Short: "Check worker and task status",
	Long: `Check the status of workers and tasks.

The check command displays:
1. Task statuses from .yaks directory (agent-status field)
2. Running workers with container name, status, and uptime
3. Live cost information from OpenCode for each running container

Filters can be applied to show only specific task states or prefixes.`,
	Example: `  # Check all workers and tasks
  yak-box check

  # Show only blocked tasks
  yak-box check --blocked

  # Show only work-in-progress tasks
  yak-box check --wip

  # Filter tasks by prefix
  yak-box check --prefix auth/api

  # Combine filters
  yak-box check --wip --prefix backend`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []error

		// Validate flag combinations
		if checkBlocked && checkWIP {
			errs = append(errs, fmt.Errorf("--blocked and --wip are mutually exclusive (cannot filter for both states simultaneously)"))
		}

		if len(errs) > 0 {
			return errors.CombineValidation(errs)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

func runCheck() error {
	preflight.Run(preflight.CheckDeps(), os.Stderr) //nolint:errcheck // check deps are all optional

	fmt.Println("=== Active Sessions ===")
	activeSessions, err := sessions.List()
	if err != nil {
		fmt.Printf("Warning: Could not load sessions: %v\n", err)
	} else if len(activeSessions) == 0 {
		fmt.Println("No active sessions.")
	} else {
		headers := []string{"Session", "Worker", "Runtime", "Task"}
		var rows [][]string
		for id, session := range activeSessions {
			rows = append(rows, []string{id, session.Worker, session.Runtime, session.Task})
		}
		ui.PrintTable(os.Stdout, headers, rows)
	}

	fmt.Println("\n=== Worker Homes ===")
	homes, err := sessions.ListHomes()
	if err != nil {
		fmt.Printf("Warning: Could not list homes: %v\n", err)
	} else if len(homes) == 0 {
		fmt.Println("No persistent worker homes.")
	} else {
		for _, home := range homes {
			homePath, _ := sessions.GetHomeDir(home)
			var size int64
			filepath.Walk(homePath, func(_ string, info os.FileInfo, err error) error {
				if err == nil && info != nil && !info.IsDir() {
					size += info.Size()
				}
				return nil
			})
			fmt.Printf("  %s (~%.1f MB)\n", home, float64(size)/1024/1024)
		}
	}

	yakPath := ".yaks"
	if prefix := checkPrefix; prefix != "" {
		yakPath = filepath.Join(yakPath, prefix)
	}

	if _, err := os.Stat(yakPath); os.IsNotExist(err) {
		fmt.Printf("No tasks found under %s\n", yakPath)
	} else {
		err := filepath.Walk(yakPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Name() == "agent-status" {
				taskDir := filepath.Dir(path)
				taskName := strings.TrimPrefix(taskDir, ".yaks/")
				taskName = strings.TrimPrefix(taskName, ".yaks\\")

				status, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				statusStr := strings.TrimSpace(string(status))
				if checkBlocked && !strings.HasPrefix(statusStr, "blocked") {
					return nil
				}
				if checkWIP && !strings.HasPrefix(statusStr, "wip") {
					return nil
				}

				// Color-code the status output
				if strings.HasPrefix(statusStr, "wip") {
					ui.Info("%-50s %s\n", taskName, statusStr)
				} else if strings.HasPrefix(statusStr, "blocked") {
					ui.Warning("%-50s %s\n", taskName, statusStr)
				} else {
					fmt.Printf("%-50s %s\n", taskName, statusStr)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Warning: Error walking task directory: %v\n", err)
		}
	}

	fmt.Println("\n=== Running Workers (Docker) ===")
	containers, err := runtime.ListRunningContainers()
	if err != nil {
		fmt.Printf("Warning: Could not list running containers (is Docker running?): %v\n", err)
	} else if len(containers) == 0 {
		fmt.Println("No running worker containers.")
	} else {
		cmd := exec.Command("docker", "ps", "--filter", "name=yak-worker-", "--format", "{{.Names}}\t{{.Status}}\t{{.RunningFor}}")
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) != "" {
			headers := []string{"Container Name", "Status", "Running For"}
			var rows [][]string
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if strings.TrimSpace(line) != "" {
					parts := strings.Split(line, "\t")
					if len(parts) >= 3 {
						rows = append(rows, []string{parts[0], parts[1], parts[2]})
					}
				}
			}
			ui.PrintTable(os.Stdout, headers, rows)
		}

		fmt.Println("\nLive Cost:")
		for _, container := range containers {
			cost := getContainerCost(container)
			if cost != "" {
				fmt.Printf("  %-30s %s\n", container, cost)
			}
		}
	}

	fmt.Println("\n=== Stopped Workers (Docker) ===")
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=yak-worker-", "--filter", "status=exited", "--format", "{{.Names}}\t{{.Status}}")
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) == "" {
		fmt.Println("No stopped worker containers.")
	} else {
		headers := []string{"Container Name", "Status"}
		var rows [][]string
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if strings.TrimSpace(line) != "" {
				parts := strings.Split(line, "\t")
				if len(parts) >= 2 {
					rows = append(rows, []string{parts[0], parts[1]})
				}
			}
		}
		ui.PrintTable(os.Stdout, headers, rows)
		fmt.Println("\nRun 'yak-box stop --name <worker>' to clean up stopped containers.")
	}

	return nil
}

func getContainerCost(container string) string {
	cmd := exec.Command("docker", "exec", container, "opencode", "stats")
	output, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, "Total Cost") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					return parts[len(parts)-1]
				}
			}
		}
	}

	cmd = exec.Command("docker", "exec", container, "goccc", "-days", "0", "-json")
	output, err = cmd.Output()
	if err == nil {
		cmd = exec.Command("docker", "exec", container, "sh", "-c", "echo '"+string(output)+"' | jq -r '.summary.total_cost // \"0\"'")
		costOutput, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(costOutput)) != "0" {
			return "$" + strings.TrimSpace(string(costOutput))
		}
	}

	return ""
}

func init() {
	checkCmd.Flags().BoolVar(&checkBlocked, "blocked", false, "Show only blocked tasks")
	checkCmd.Flags().BoolVar(&checkWIP, "wip", false, "Show only work-in-progress tasks")
	checkCmd.Flags().StringVar(&checkPrefix, "prefix", "", "Filter tasks by prefix (e.g., 'auth/api')")
}
