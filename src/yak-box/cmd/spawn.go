package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yakthang/yakbox/internal/metadata"
	"github.com/yakthang/yakbox/internal/persona"
	"github.com/yakthang/yakbox/internal/prompt"
	"github.com/yakthang/yakbox/internal/runtime"
	"github.com/yakthang/yakbox/pkg/types"
)

var (
	spawnCWD       string
	spawnName      string
	spawnSession   string
	spawnMode      string
	spawnResources string
	spawnYaks      []string
	spawnYakPath   string
	spawnRuntime   string
)

var spawnCmd = &cobra.Command{
	Use:   "spawn --cwd <dir> --name <tab-name> [flags]",
	Short: "Spawn a new worker",
	Long: `Spawn a new worker with specified configuration.

The spawn command creates a new worker (sandboxed or native) with a randomly
selected personality, assembles the appropriate prompt, and assigns tasks.

Sandboxed mode (default): Uses Docker container with resource limits and isolation.
Native mode: Runs opencode directly on the host with full system access.`,
	Example: `  # Spawn a worker for API authentication tasks
  yak-box spawn --cwd ./api --name api-auth --yaks auth/api/login --yaks auth/api/logout

  # Spawn with heavy resources and native runtime
  yak-box spawn --cwd ./backend --name backend-worker --resources heavy --runtime native

  # Spawn in plan mode with custom yak path
  yak-box spawn --cwd ./frontend --name ui-worker --mode plan --yak-path .tasks`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSpawn(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runSpawn(args []string) error {
	runtimeType := spawnRuntime
	if runtimeType == "auto" {
		runtimeType = runtime.DetectRuntime()
		if runtimeType == "unknown" {
			return fmt.Errorf("no runtime available (docker or zellij). Use --runtime=sandboxed or --runtime=native to force a specific runtime")
		}
	}

	absCWD, err := filepath.Abs(spawnCWD)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	persona := persona.GetRandomPersona()

	profile := runtime.GetResourceProfile(spawnResources)

	userPrompt := "Work on the assigned tasks."
	if len(args) > 0 {
		userPrompt = args[0]
	}

	workerPrompt := prompt.BuildPrompt(persona, spawnMode, spawnYakPath, userPrompt, spawnYaks)

	yakTitle := ""
	if len(spawnYaks) > 0 {
		yakTitle = spawnYaks[0]
		for i := 1; i < len(spawnYaks); i++ {
			_, name := filepath.Split(spawnYaks[i])
			yakTitle += ", " + name
		}
	}

	displayName := persona.Name + " " + persona.Emoji
	if yakTitle != "" {
		displayName += " " + yakTitle
	}

	// Sanitize spawnName for use as Docker container name (only allow [a-zA-Z0-9_-])
	sanitizedName := strings.ReplaceAll(spawnName, " ", "-")
	sanitizedName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, sanitizedName)

	worker := &types.Worker{
		Name:          spawnName,
		DisplayName:   displayName,
		ContainerName: "yak-worker-" + sanitizedName,
		Runtime:       runtimeType,
		CWD:           absCWD,
		YakPath:       spawnYakPath,
		Tasks:         spawnYaks,
		SpawnedAt:     time.Now(),
		SessionName:   spawnSession,
	}

	if runtimeType == "sandboxed" {
		if err := runtime.EnsureDevcontainer(); err != nil {
			return fmt.Errorf("failed to ensure devcontainer: %w\n\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}

		if err := runtime.SpawnSandboxedWorker(worker, &persona, workerPrompt, profile); err != nil {
			return fmt.Errorf("failed to spawn sandboxed worker: %w\n\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}
	} else {
		if err := runtime.SpawnNativeWorker(worker, &persona, workerPrompt); err != nil {
			return fmt.Errorf("failed to spawn native worker: %w", err)
		}
	}

	if err := metadata.SaveMetadata(worker, &persona, spawnYaks); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save metadata: %v\n", err)
	}

	for _, task := range spawnYaks {
		assignment := persona.Name + " " + persona.Emoji
		taskFile := filepath.Join(spawnYakPath, task, "assigned-to")
		if err := os.WriteFile(taskFile, []byte(assignment), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to assign task %s: %v\n", task, err)
		}
	}

	fmt.Printf("Spawned %s (%s) in %s\n", persona.Name, spawnName, runtimeType)
	return nil
}

func init() {
	spawnCmd.Flags().StringVar(&spawnCWD, "cwd", "", "Working directory for the worker (required)")
	spawnCmd.MarkFlagRequired("cwd")

	spawnCmd.Flags().StringVar(&spawnName, "name", "", "Worker name used in logs and metadata (required)")
	spawnCmd.MarkFlagRequired("name")

	spawnCmd.Flags().StringVar(&spawnSession, "session", "yakthang", "Zellij session name (overrides ZELLIJ_SESSION_NAME)")

	spawnCmd.Flags().StringVar(&spawnMode, "mode", "build", "Agent mode: 'plan' or 'build'")
	spawnCmd.Flags().StringVar(&spawnResources, "resources", "default", "Resource profile: 'light', 'default', 'heavy', or 'ram'")
	spawnCmd.Flags().StringSliceVar(&spawnYaks, "yaks", []string{}, "Yak paths from .yaks/ to assign (can be repeated)")
	spawnCmd.Flags().StringSliceVar(&spawnYaks, "task", []string{}, "Alias for --yaks")
	spawnCmd.Flags().StringVar(&spawnYakPath, "yak-path", ".yaks", "Path to task state directory")
	spawnCmd.Flags().StringVar(&spawnRuntime, "runtime", "auto", "Runtime: 'auto', 'sandboxed', or 'native'")
}
