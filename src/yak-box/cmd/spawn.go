package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yakthang/yakbox/internal/errors"
	"github.com/yakthang/yakbox/internal/prompt"
	"github.com/yakthang/yakbox/internal/runtime"
	"github.com/yakthang/yakbox/internal/sessions"
	"github.com/yakthang/yakbox/internal/ui"
	"github.com/yakthang/yakbox/pkg/devcontainer"
	"github.com/yakthang/yakbox/pkg/types"
	"github.com/yakthang/yakbox/pkg/worktree"
)

var (
	spawnCWD          string
	spawnName         string
	spawnSession      string
	spawnMode         string
	spawnResources    string
	spawnYaks         []string
	spawnYakPath      string
	spawnRuntime      string
	spawnClean        bool
	spawnAutoWorktree bool
)

var spawnCmd = &cobra.Command{
	Use:   "spawn --cwd <dir> --name <tab-name> [flags]",
	Short: "Spawn a new worker",
	Long: `Spawn a new worker with specified configuration.

The spawn command creates a new worker (sandboxed or native) with a randomly
selected name, assembles the appropriate prompt, and assigns tasks.

Sandboxed mode (default): Uses Docker container with resource limits and isolation.
Native mode: Runs opencode directly on the host with full system access.`,
	Example: `  # Spawn a worker for API authentication tasks
  yak-box spawn --cwd ./api --name api-auth --yaks auth/api/login --yaks auth/api/logout

  # Spawn with automatic worktree creation
  yak-box spawn --cwd ./api --name api-auth --yaks auth/api --auto-worktree

  # Spawn with heavy resources and native runtime
  yak-box spawn --cwd ./backend --name backend-worker --resources heavy --runtime native

  # Spawn in plan mode with custom yak path
  yak-box spawn --cwd ./frontend --name ui-worker --mode plan --yak-path .tasks`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []error

		if spawnCWD == "" {
			errs = append(errs, fmt.Errorf("--cwd is required (working directory for the worker)"))
		}
		if spawnName == "" {
			errs = append(errs, fmt.Errorf("--name is required (worker name used in logs and metadata)"))
		}

		if spawnMode != "plan" && spawnMode != "build" {
			errs = append(errs, fmt.Errorf("--mode must be 'plan' or 'build', got '%s'", spawnMode))
		}

		if spawnResources != "light" && spawnResources != "default" && spawnResources != "heavy" && spawnResources != "ram" {
			errs = append(errs, fmt.Errorf("--resources must be 'light', 'default', 'heavy', or 'ram', got '%s'", spawnResources))
		}

		if spawnRuntime != "auto" && spawnRuntime != "sandboxed" && spawnRuntime != "native" {
			errs = append(errs, fmt.Errorf("--runtime must be 'auto', 'sandboxed', or 'native', got '%s'", spawnRuntime))
		}

		if len(errs) > 0 {
			combined := "Validation errors:\n"
			for _, err := range errs {
				combined += fmt.Sprintf("  - %s\n", err)
			}
			return errors.NewValidationError(combined, nil)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSpawn(cmd.Context(), args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

func pickWorkerName() string {
	return types.WorkerNames[rand.Intn(len(types.WorkerNames))]
}

func runSpawn(ctx context.Context, args []string) error {
	runtimeType := spawnRuntime
	if runtimeType == "auto" {
		runtimeType = runtime.DetectRuntime()
		if runtimeType == "unknown" {
			return fmt.Errorf("no runtime available (docker or zellij). Suggestion: Install Docker and start the daemon, or install Zellij. Force with --runtime=sandboxed or --runtime=native")
		}
	}

	absCWD, err := filepath.Abs(spawnCWD)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w. Suggestion: Ensure --cwd path is valid and accessible", err)
	}

	absYakPath, err := filepath.Abs(spawnYakPath)
	if err != nil {
		return fmt.Errorf("failed to resolve yak path: %w. Suggestion: Ensure --yak-path exists and is accessible (default: .yaks)", err)
	}

	worktreePath := ""
	if spawnAutoWorktree && len(spawnYaks) > 0 {
		taskPath := spawnYaks[0]
		fmt.Printf("Creating worktree for task: %s\n", taskPath)

		wt, err := worktree.EnsureWorktree(absCWD, taskPath, true)
		if err != nil {
			return fmt.Errorf("failed to ensure worktree: %w. Suggestion: Ensure you're in a git repository with proper permissions, or disable --auto-worktree", err)
		}

		worktreePath = wt
		absCWD = wt
		fmt.Printf("Using worktree: %s\n", wt)
	}

	workerName := pickWorkerName()

	if spawnClean {
		fmt.Printf("Cleaning home directory for %s...\n", workerName)
		if err := sessions.CleanHome(workerName); err != nil {
			return fmt.Errorf("failed to clean home: %w. Suggestion: Ensure .yak-boxes directory exists and is writable", err)
		}
	}

	homeDir, err := sessions.EnsureHomeDir(workerName)
	if err != nil {
		return fmt.Errorf("failed to ensure home directory: %w. Suggestion: Check that .yak-boxes directory exists and is writable", err)
	}

	devConfig, err := devcontainer.LoadConfig(absCWD)
	if err != nil {
		return fmt.Errorf("failed to load devcontainer config: %w. Suggestion: Ensure .devcontainer/devcontainer.json is valid JSON if it exists", err)
	}

	profile := runtime.GetResourceProfile(spawnResources)

	userPrompt := "Work on the assigned tasks."
	if len(args) > 0 {
		userPrompt = args[0]
	}

	workerPrompt := prompt.BuildPrompt(spawnMode, spawnYakPath, userPrompt, spawnYaks)

	yakTitle := ""
	if len(spawnYaks) > 0 {
		yakTitle = spawnYaks[0]
		for i := 1; i < len(spawnYaks); i++ {
			_, name := filepath.Split(spawnYaks[i])
			yakTitle += ", " + name
		}
	}

	displayName := workerName
	if yakTitle != "" {
		displayName += " " + yakTitle
	}

	sanitizedName := strings.ReplaceAll(spawnName, " ", "-")
	sanitizedName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, sanitizedName)

	worker := &types.Worker{
		Name:          spawnName,
		WorkerName:    workerName,
		DisplayName:   displayName,
		ContainerName: "yak-worker-" + sanitizedName,
		Runtime:       runtimeType,
		CWD:           absCWD,
		YakPath:       absYakPath,
		Tasks:         spawnYaks,
		SpawnedAt:     time.Now(),
		SessionName:   spawnSession,
		WorktreePath:  worktreePath,
	}

	if runtimeType == "sandboxed" {
		ui.Info("⏳ Building container...\n")
		if err := runtime.EnsureDevcontainer(); err != nil {
			ui.Error("❌ Build failed: %v\n", err)
			return fmt.Errorf("failed to ensure devcontainer: %w\n\nSuggestion: Install Docker or use native mode.\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}

		if err := runtime.SpawnSandboxedWorker(ctx,
			runtime.WithWorker(worker),
			runtime.WithPrompt(workerPrompt),
			runtime.WithResourceProfile(profile),
			runtime.WithHomeDir(homeDir),
			runtime.WithDevConfig(devConfig),
		); err != nil {
			ui.Error("❌ Failed to spawn sandboxed worker: %v\n", err)
			return fmt.Errorf("failed to spawn sandboxed worker: %w\n\nSuggestion: Check Docker is running and has enough resources.\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}
		ui.Success("✅ Container ready\n")
	} else {
		ui.Info("⏳ Starting native worker...\n")
		pidFile, err := runtime.SpawnNativeWorker(worker, workerPrompt, homeDir)
		if err != nil {
			ui.Error("❌ Failed to spawn native worker: %v\n", err)
			return fmt.Errorf("failed to spawn native worker: %w. Suggestion: Ensure Zellij is installed and running, or use --runtime=sandboxed instead", err)
		}
		worker.PidFile = pidFile
		ui.Success("✅ Native worker started\n")
	}

	taskName := ""
	if len(spawnYaks) > 0 {
		taskName = spawnYaks[0]
	}

	if err := sessions.Register(spawnName, sessions.Session{
		Worker:        workerName,
		Task:          taskName,
		Container:     worker.ContainerName,
		SpawnedAt:     worker.SpawnedAt,
		Runtime:       runtimeType,
		CWD:           absCWD,
		DisplayName:   displayName,
		ZellijSession: spawnSession,
		PidFile:       worker.PidFile,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to register session: %v\n", err)
	}

	for _, task := range spawnYaks {
		taskFile := filepath.Join(absYakPath, task, "assigned-to")
		if err := os.WriteFile(taskFile, []byte(workerName), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to assign task %s: %v\n", task, err)
		}

		if worktreePath != "" {
			worktreeFile := filepath.Join(absYakPath, task, "worktree-path")
			if err := os.WriteFile(worktreeFile, []byte(worktreePath), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write worktree path for task %s: %v\n", task, err)
			}
		}
	}

	fmt.Printf("Spawned %s (%s) in %s\n", workerName, spawnName, runtimeType)
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
	spawnCmd.Flags().BoolVar(&spawnClean, "clean", false, "Clean worker home directory before spawning")
	spawnCmd.Flags().BoolVar(&spawnAutoWorktree, "auto-worktree", false, "Automatically create and use git worktree for the task")
}
