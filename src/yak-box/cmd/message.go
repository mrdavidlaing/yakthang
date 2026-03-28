package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/errors"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/sessions"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/ui"
)

var (
	messageFormat  string
	messageSession string
)

var messageCmd = &cobra.Command{
	Use:   "message <worker-name> <text>",
	Short: "Send a message to a running worker",
	Long: `Send a message directly to a running worker via its OpenCode session.

The message command:
1. Looks up the worker in .yak-boxes/sessions.json
2. Discovers its OpenCode session (via docker exec or opencode --dir)
3. Sends the message via opencode run --session

Works with both devcontainer (Docker) and native workers.`,
	Example: `  # Send a message to a worker
  yak-box message api-auth "Add error handling to the login endpoint"

  # Send with JSON output format
  yak-box message api-auth "Check test results" --format json

  # Send to a specific OpenCode session (skip auto-discovery)
  yak-box message api-auth "Fix the bug" --session ses_abc123`,
	Args: cobra.MinimumNArgs(2),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []error

		if strings.TrimSpace(args[0]) == "" {
			errs = append(errs, fmt.Errorf("worker name cannot be empty"))
		}

		if strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
			errs = append(errs, fmt.Errorf("message text cannot be empty"))
		}

		if messageFormat != "" && messageFormat != "default" && messageFormat != "json" {
			errs = append(errs, fmt.Errorf("--format must be 'default' or 'json' (got %q)", messageFormat))
		}

		if len(errs) > 0 {
			return errors.CombineValidation(errs)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMessage(args[0], strings.Join(args[1:], " ")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

func runMessage(workerName, text string) error {
	session, err := sessions.Get(workerName)
	if err != nil {
		workers, listErr := sessions.ListWorkers()
		if listErr != nil || len(workers) == 0 {
			return errors.NewValidationError(
				fmt.Sprintf("worker %q not found. No active workers registered", workerName), err)
		}
		return errors.NewValidationError(
			fmt.Sprintf("worker %q not found. Available workers: %s", workerName, strings.Join(workers, ", ")), err)
	}

	runner := &sessions.ExecRunner{}

	openCodeSessionID := messageSession
	if openCodeSessionID == "" {
		ui.Info("🔍 Discovering OpenCode sessions for %s...\n", workerName)
		ocSessions, err := sessions.DiscoverOpenCodeSessions(runner, session)
		if err != nil {
			return errors.NewRuntimeError(
				fmt.Sprintf("failed to discover sessions for %q. The worker might still be starting up, or it may have stopped", workerName), err)
		}

		if len(ocSessions) == 0 {
			return errors.NewRuntimeError(
				fmt.Sprintf("no active OpenCode sessions found for %q. The worker might still be starting up, or it may have stopped", workerName), nil)
		}

		most := sessions.FindMostRecentSession(ocSessions)
		if most == nil {
			return errors.NewRuntimeError("could not determine most recent session", nil)
		}
		openCodeSessionID = most.ID
		ui.Info("📡 Using session: %s\n", openCodeSessionID)
	}

	ui.Info("📨 Sending message to %s...\n", workerName)
	result, err := sessions.SendMessage(runner, session, openCodeSessionID, text, messageFormat)
	if err != nil {
		return errors.NewRuntimeError(
			fmt.Sprintf("failed to send message to %q", workerName), err)
	}

	if messageFormat == "json" {
		output := struct {
			Worker    string `json:"worker"`
			SessionID string `json:"session_id"`
			ExitCode  int    `json:"exit_code"`
			Output    string `json:"output"`
		}{
			Worker:    workerName,
			SessionID: openCodeSessionID,
			ExitCode:  result.ExitCode,
			Output:    result.Output,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
	} else {
		if result.Output != "" {
			fmt.Print(result.Output)
		}
		ui.Success("✅ Message delivered to %s\n", workerName)
	}

	return nil
}

func init() {
	messageCmd.Flags().StringVar(&messageFormat, "format", "", "Output format: 'default' or 'json'")
	messageCmd.Flags().StringVar(&messageSession, "session", "", "OpenCode session ID (skip auto-discovery)")
}
