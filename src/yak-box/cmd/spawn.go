package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/errors"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/pathutil"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/preflight"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/prompt"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/runtime"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/sessions"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/ui"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/devcontainer"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/types"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/worktree"
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
	spawnTool         string
	spawnModel        string
	spawnClean        bool
	spawnAutoWorktree bool
	spawnSkills       []string
	spawnShaverName   string
)

const (
	defaultClaudeModel = "default"
	defaultCursorModel = "auto"
)

var spawnCmd = &cobra.Command{
	Use:   "spawn --yak-name <tab-name> [flags]",
	Short: "Spawn a new worker",
	Long: `Spawn a new worker with specified configuration.

The spawn command creates a new worker (devcontainer or native) with the provided
name, assembles the appropriate prompt, and assigns tasks.

Runtime modes (--runtime is required):
  devcontainer: Uses Docker container with resource limits and isolation.
  sandbox: Uses bubblewrap sandbox with resource limits.
  native: Runs the AI tool directly on the host with full system access.

Tool selection:
  --tool claude (default): Uses Claude Code with --print mode and agent prompts.
  --tool opencode: Uses OpenCode with --agent build mode.
  --tool cursor: Uses Cursor agent CLI with --force mode.`,
	Example: `  # Spawn a worker for API authentication tasks
  yak-box spawn --cwd ./api --yak-name api-auth --yaks auth/api/login --yaks auth/api/logout

  # Spawn with automatic worktree creation
  yak-box spawn --cwd ./api --yak-name api-auth --yaks auth/api --auto-worktree

  # Spawn with heavy resources and native runtime
  yak-box spawn --cwd ./backend --yak-name backend-worker --resources heavy --runtime native

  # Spawn in plan mode with custom yak path
  yak-box spawn --cwd ./frontend --yak-name ui-worker --mode plan --yak-path /home/<username>/yakthang/.yaks`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []error

		if strings.TrimSpace(spawnName) == "" {
			errs = append(errs, fmt.Errorf("--yak-name is required (worker name used in logs and metadata)"))
		}

		if types.Mode(spawnMode) != types.ModePlan && types.Mode(spawnMode) != types.ModeBuild {
			errs = append(errs, fmt.Errorf("--mode must be 'plan' or 'build', got '%s'", spawnMode))
		}

		if spawnResources != "light" && spawnResources != "default" && spawnResources != "heavy" && spawnResources != "ram" {
			errs = append(errs, fmt.Errorf("--resources must be 'light', 'default', 'heavy', or 'ram', got '%s'", spawnResources))
		}

		if types.Runtime(spawnRuntime) != types.RuntimeDevcontainer && types.Runtime(spawnRuntime) != types.RuntimeNative && types.Runtime(spawnRuntime) != types.RuntimeSandbox {
			errs = append(errs, fmt.Errorf("--runtime is required: must be 'native', 'sandbox', or 'devcontainer', got '%s'", spawnRuntime))
		}

		if types.Tool(spawnTool) != types.ToolOpencode && types.Tool(spawnTool) != types.ToolClaude && types.Tool(spawnTool) != types.ToolCursor {
			errs = append(errs, fmt.Errorf("--tool must be 'opencode', 'claude', or 'cursor', got '%s'", spawnTool))
		}

		resolvedSkills := resolveSpawnSkills(spawnSkills)
		for _, skillPath := range resolvedSkills {
			info, err := os.Stat(skillPath)
			if err != nil {
				errs = append(errs, fmt.Errorf("--skill %q: folder not found", skillPath))
				continue
			}
			if !info.IsDir() {
				errs = append(errs, fmt.Errorf("--skill %q: must be a directory", skillPath))
				continue
			}
			skillMD := filepath.Join(skillPath, "SKILL.md")
			if _, err := os.Stat(skillMD); err != nil {
				errs = append(errs, fmt.Errorf("--skill %q: missing SKILL.md", skillPath))
			}
		}

		if len(errs) > 0 {
			return errors.CombineValidation(errs)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSpawn(cmd, cmd.Context(), args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

// resolveShaverName returns the identity of the shaver (person who spawned the worker),
// for tab titles and assigned-to. Prefers YAK_SHAVER_NAME, then USER, then "yak-shaver".
func resolveShaverName() string {
	if s := strings.TrimSpace(os.Getenv("YAK_SHAVER_NAME")); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("USER")); s != "" {
		return s
	}
	return "yak-shaver"
}

// formatDisplayName returns the Zellij tab title: "shaverName 🪒🐃 workerName" when
// they differ, or just shaverName when worker name is empty or the same.
func formatDisplayName(shaverName, workerName string) string {
	trimmedName := strings.TrimSpace(workerName)
	if trimmedName == "" || trimmedName == shaverName {
		return shaverName
	}
	return fmt.Sprintf("%s 🪒🐃 %s", shaverName, trimmedName)
}

func resolveSpawnModel(tool, model string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}

	switch types.Tool(tool) {
	case types.ToolClaude:
		return defaultClaudeModel
	case types.ToolCursor:
		return defaultCursorModel
	default:
		return ""
	}
}

func resolveSpawnSkills(explicitSkills []string) []string {
	skills := make([]string, 0, len(explicitSkills))
	seen := make(map[string]struct{}, len(explicitSkills))
	for _, skill := range explicitSkills {
		cleanPath := filepath.Clean(skill)
		if _, exists := seen[cleanPath]; exists {
			continue
		}
		seen[cleanPath] = struct{}{}
		skills = append(skills, cleanPath)
	}
	return skills
}

func runSpawn(cmd *cobra.Command, ctx context.Context, args []string) error {
	runtimeType := spawnRuntime

	var preflightDeps []preflight.Dep
	switch types.Runtime(runtimeType) {
	case types.RuntimeDevcontainer:
		preflightDeps = preflight.SpawnDevcontainerDeps()
	case types.RuntimeSandbox:
		preflightDeps = preflight.SpawnSandboxDeps()
	default:
		preflightDeps = preflight.SpawnNativeDeps(spawnTool)
	}
	if err := preflight.Run(preflightDeps, os.Stderr); err != nil {
		return err
	}
	startDir := "."
	if strings.TrimSpace(spawnCWD) != "" {
		startDir = spawnCWD
	}
	startAbsDir, err := filepath.Abs(startDir)
	if err != nil {
		return fmt.Errorf("failed to resolve start directory: %w. Suggestion: Ensure current directory or --cwd path is valid and accessible", err)
	}

	var absYakPath string
	if cmd.Flags().Changed("yak-path") {
		absYakPath, err = filepath.Abs(spawnYakPath)
		if err != nil {
			return errors.NewValidationError("failed to resolve yak path; ensure --yak-path exists and is accessible", err)
		}
	} else {
		absYakPath, err = findYakPath(startAbsDir, filepath.Base(spawnYakPath))
		if err != nil {
			return errors.NewValidationError(fmt.Sprintf("No .yaks found above %s. Use --yak-path to specify explicitly", startAbsDir), nil)
		}
	}

	// Resolve all --yaks values (IDs or display paths) to task dirs and display paths.
	type resolvedYak struct{ TaskDir, DisplayPath string }
	resolvedYaks := make([]resolvedYak, 0, len(spawnYaks))
	for _, y := range spawnYaks {
		taskDir, displayPath, errResolve := resolveYakValue(absYakPath, y)
		if errResolve != nil {
			return errors.NewValidationError(fmt.Sprintf("failed to resolve yak %q", y), errResolve)
		}
		resolvedYaks = append(resolvedYaks, resolvedYak{TaskDir: taskDir, DisplayPath: displayPath})
	}

	var (
		absCWD             string
		inheritedWorktrees []string
		worktreeBranch     string
	)
	if len(spawnYaks) > 0 {
		inheritedWorktrees, worktreeBranch, err = resolveInheritedWorktrees(absYakPath, spawnYaks[0])
		if err != nil {
			return fmt.Errorf("failed to resolve worktrees from yak %q: %w", spawnYaks[0], err)
		}
	}

	if strings.TrimSpace(spawnCWD) != "" {
		absCWD, err = filepath.Abs(spawnCWD)
		if err != nil {
			return fmt.Errorf("failed to resolve working directory: %w. Suggestion: Ensure --cwd path is valid and accessible", err)
		}
	} else if len(inheritedWorktrees) == 0 {
		return errors.NewValidationError("--cwd is required unless the assigned yak defines a worktrees field", nil)
	}

	worktreePath := ""
	if spawnAutoWorktree && len(resolvedYaks) > 0 {
		taskPath := resolvedYaks[0].DisplayPath
		fmt.Printf("Creating worktree for task: %s\n", taskPath)

		wt, err := worktree.EnsureWorktree(absCWD, taskPath, true)
		if err != nil {
			return fmt.Errorf("failed to ensure worktree: %w. Suggestion: Ensure you're in a git repository with proper permissions, or disable --auto-worktree", err)
		}

		worktreePath = wt
		absCWD = wt
		fmt.Printf("Using worktree: %s\n", wt)
	}

	workerName := strings.TrimSpace(spawnName)

	shaverName := strings.TrimSpace(spawnShaverName)
	if shaverName == "" {
		shaverName = resolveShaverName()
	}

	if spawnClean {
		fmt.Printf("Cleaning home directory for %s...\n", shaverName)
		if err := sessions.CleanHome(shaverName); err != nil {
			return fmt.Errorf("failed to clean home: %w. Suggestion: Ensure .yak-boxes directory exists and is writable", err)
		}
	}

	homeDir, err := sessions.EnsureHomeDir(shaverName)
	if err != nil {
		return fmt.Errorf("failed to ensure home directory: %w. Suggestion: Check that .yak-boxes directory exists and is writable", err)
	}

	if err := preflight.EnsureClaudeAuthEnv(spawnTool, runtimeType, homeDir, os.LookupEnv); err != nil {
		return err
	}

	if len(inheritedWorktrees) > 0 {
		seenDestinations := make(map[string]string, len(inheritedWorktrees))
		for _, repoPath := range inheritedWorktrees {
			repoName := filepath.Base(repoPath)
			destPath := filepath.Join(homeDir, repoName)
			if prior, exists := seenDestinations[repoName]; exists {
				return errors.NewValidationError(fmt.Sprintf("duplicate worktree destination %q for repos %q and %q", repoName, prior, repoPath), nil)
			}

			wtPath, err := worktree.EnsureWorktreeAtPath(repoPath, destPath, worktreeBranch, true)
			if err != nil {
				return fmt.Errorf("failed to ensure worktree for repo %s: %w", repoPath, err)
			}
			seenDestinations[repoName] = repoPath
			fmt.Printf("Using worktree: %s\n", wtPath)
		}
		absCWD = homeDir
		worktreePath = homeDir
		fmt.Printf("Using worker home for multi-repo worktrees: %s\n", homeDir)
	}

	resolvedSkills := resolveSpawnSkills(spawnSkills)

	if err := copySkillsToHome(resolvedSkills, homeDir, spawnTool); err != nil {
		return fmt.Errorf("failed to copy skills: %w", err)
	}

	devConfig, devWarnings, err := devcontainer.LoadConfig(absCWD)
	if err != nil {
		return fmt.Errorf("failed to load devcontainer config: %w. Suggestion: Ensure .devcontainer/devcontainer.json is valid JSON if it exists", err)
	}
	for _, w := range devWarnings {
		ui.Warning("WARNING: %s\n", w.Message)
	}

	profile := runtime.GetResourceProfile(spawnResources)

	userPrompt := "Work on the assigned tasks."
	if len(args) > 0 {
		userPrompt = args[0]
	}

	skillNames := make([]string, 0, len(resolvedSkills))
	for _, s := range resolvedSkills {
		skillNames = append(skillNames, filepath.Base(s))
	}
	displayPaths := make([]string, len(resolvedYaks))
	yakRwDirs := make([]string, 0, len(resolvedYaks))
	for i := range resolvedYaks {
		displayPaths[i] = resolvedYaks[i].DisplayPath
		taskDir := resolvedYaks[i].TaskDir
		if err := pathutil.ValidatePath(taskDir, absYakPath); err != nil {
			return fmt.Errorf("yak task dir %q escapes .yaks boundary: %w", taskDir, err)
		}
		yakRwDirs = append(yakRwDirs, taskDir)
	}
	workerPrompt := prompt.BuildPrompt(spawnMode, absYakPath, userPrompt, displayPaths, shaverName, skillNames)
	displayName := formatDisplayName(shaverName, spawnName)

	sanitizedName := strings.ReplaceAll(spawnName, " ", "-")
	sanitizedName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, sanitizedName)

	if types.Tool(spawnTool) == types.ToolOpencode && spawnModel != "" {
		ui.Warning("⚠️  --model is currently ignored for --tool opencode\n")
	}
	resolvedModel := resolveSpawnModel(spawnTool, spawnModel)

	worker := &types.Worker{
		Name:          spawnName,
		WorkerName:    workerName,
		DisplayName:   displayName,
		ContainerName: "yak-worker-" + sanitizedName,
		Runtime:       runtimeType,
		CWD:           absCWD,
		YakPath:       absYakPath,
		YakRwDirs:     yakRwDirs,
		Tasks:         displayPaths,
		SpawnedAt:     time.Now(),
		SessionName:   spawnSession,
		WorktreePath:  worktreePath,
		Tool:          spawnTool,
		Model:         resolvedModel,
		ShaverName:    shaverName,
	}

	switch types.Runtime(runtimeType) {
	case types.RuntimeDevcontainer:
		ui.Info("⏳ Building container...\n")
		if err := runtime.EnsureDevcontainer(); err != nil {
			ui.Error("❌ Build failed: %v\n", err)
			return fmt.Errorf("failed to ensure devcontainer: %w\n\nSuggestion: Install Docker or use native mode.\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}

		if err := runtime.SpawnDevcontainerWorker(ctx,
			runtime.WithWorker(worker),
			runtime.WithPrompt(workerPrompt),
			runtime.WithResourceProfile(profile),
			runtime.WithHomeDir(homeDir),
			runtime.WithDevConfig(devConfig),
		); err != nil {
			ui.Error("❌ Failed to spawn devcontainer worker: %v\n", err)
			return fmt.Errorf("failed to spawn devcontainer worker: %w\n\nSuggestion: Check Docker is running and has enough resources.\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}
		ui.Success("✅ Container ready\n")
	case types.RuntimeSandbox:
		ui.Info("⏳ Starting sandbox worker...\n")
		if err := runtime.SpawnSandboxWorker(ctx,
			runtime.WithWorker(worker),
			runtime.WithPrompt(workerPrompt),
			runtime.WithResourceProfile(profile),
			runtime.WithHomeDir(homeDir),
		); err != nil {
			ui.Error("❌ Failed to spawn sandbox worker: %v\n", err)
			return fmt.Errorf("failed to spawn sandbox worker: %w", err)
		}
		ui.Success("✅ Sandbox worker started\n")
	default:
		ui.Info("⏳ Starting native worker...\n")
		pidFile, err := runtime.SpawnNativeWorker(worker, workerPrompt, homeDir)
		if err != nil {
			ui.Error("❌ Failed to spawn native worker: %v\n", err)
			return fmt.Errorf("failed to spawn native worker: %w. Suggestion: Ensure Zellij is installed and running, or use --runtime=devcontainer instead", err)
		}
		worker.PidFile = pidFile
		ui.Success("✅ Native worker started\n")
	}

	taskName := ""
	var taskDirs []string
	if len(resolvedYaks) > 0 {
		taskName = resolvedYaks[0].DisplayPath
		taskDirs = make([]string, len(resolvedYaks))
		for i := range resolvedYaks {
			taskDirs[i] = resolvedYaks[i].TaskDir
		}
	}

	if err := sessions.Register(spawnName, sessions.Session{
		Worker:        workerName,
		Task:          taskName,
		TaskDirs:      taskDirs,
		Container:     worker.ContainerName,
		SpawnedAt:     worker.SpawnedAt,
		Runtime:       runtimeType,
		CWD:           absCWD,
		DisplayName:   displayName,
		ZellijSession: spawnSession,
		PidFile:       worker.PidFile,
		HomeDir:       homeDir,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to register session: %v\n", err)
	}

	for _, r := range resolvedYaks {
		taskFile := filepath.Join(r.TaskDir, "assigned-to")
		if err := os.WriteFile(taskFile, []byte(shaverName), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to assign task %s: %v\n", r.DisplayPath, err)
		}

		if worktreePath != "" {
			worktreeFile := filepath.Join(r.TaskDir, "worktree-path")
			if err := os.WriteFile(worktreeFile, []byte(worktreePath), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write worktree path for task %s: %v\n", r.DisplayPath, err)
			}
		}
	}

	fmt.Printf("Spawned %s (%s) in %s\n", workerName, spawnName, runtimeType)
	return nil
}

// resolveYakValue resolves a --yaks value (yak ID or display name path) to a task directory and display path.
// It first searches .yaks/ recursively for a directory whose .id file matches the value exactly.
// If found, returns that directory and the display path from .name (or the id if .name is missing).
// If not found, it falls back to:
//   1) SlugifyTaskPath + findTaskDir (legacy behavior),
//   2) lowercase-normalized SlugifyTaskPath + findTaskDir (matches yx directory naming).
func resolveYakValue(absYakPath, yakValue string) (taskDir, displayPath string, err error) {
	yakValue = strings.TrimSpace(yakValue)
	if yakValue == "" {
		return "", "", fmt.Errorf("empty yak value")
	}

	// 1. Search for a directory whose .id file matches exactly.
	var foundByID string
	errStop := fmt.Errorf("found")
	_ = filepath.Walk(absYakPath, func(path string, info os.FileInfo, errWalk error) error {
		if errWalk != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != ".id" {
			return nil
		}
		data, errRead := os.ReadFile(path)
		if errRead != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == yakValue {
			foundByID = filepath.Dir(path)
			return errStop
		}
		return nil
	})

	if foundByID != "" {
		return foundByID, readTaskDisplayPath(foundByID, yakValue), nil
	}

	// 2. Search for a directory whose .name file matches exactly.
	//    This handles the case where session.Task stores a display name
	//    (from .name) that doesn't match the directory slug or .id.
	var foundByName string
	_ = filepath.Walk(absYakPath, func(path string, info os.FileInfo, errWalk error) error {
		if errWalk != nil || info.IsDir() || info.Name() != ".name" {
			return nil
		}
		data, errRead := os.ReadFile(path)
		if errRead != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == yakValue {
			foundByName = filepath.Dir(path)
			return errStop
		}
		return nil
	})
	if foundByName != "" {
		return foundByName, readTaskDisplayPath(foundByName, yakValue), nil
	}

	// 4. Fall back to slugify + findTaskDir (legacy behavior).
	taskSlug := types.SlugifyTaskPath(yakValue)
	taskDir, err = findTaskDir(absYakPath, taskSlug)
	if err == nil {
		return taskDir, readTaskDisplayPath(taskDir, yakValue), nil
	}

	// 5. Try yx-style lowercase directory normalization for display names.
	normalizedTaskSlug := strings.ToLower(taskSlug)
	taskDir, err = findTaskDir(absYakPath, normalizedTaskSlug)
	if err != nil {
		return "", "", err
	}
	return taskDir, readTaskDisplayPath(taskDir, yakValue), nil
}

func readTaskDisplayPath(taskDir, fallback string) string {
	nameData, errRead := os.ReadFile(filepath.Join(taskDir, ".name"))
	if errRead != nil {
		return fallback
	}
	if n := strings.TrimSpace(string(nameData)); n != "" {
		return n
	}
	return fallback
}

// findTaskDir searches the .yaks/ tree for a directory matching the task slug.
// Tasks can be nested (e.g., "release-yakthang/yak-box/missing-tab-emoji"),
// so we walk the tree looking for a directory whose base name matches the slug.
func findTaskDir(yakPath, taskSlug string) (string, error) {
	// If the slug contains path separators, try the direct path first.
	directPath := filepath.Join(yakPath, taskSlug)
	if info, err := os.Stat(directPath); err == nil && info.IsDir() {
		return directPath, nil
	}

	// Otherwise, search for a directory with a matching leaf name.
	leafName := filepath.Base(taskSlug)
	var matches []string
	filepath.Walk(yakPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == leafName && path != yakPath {
			matches = append(matches, path)
		}
		return nil
	})

	if len(matches) == 0 {
		return "", fmt.Errorf("no directory matching %q found under %s", taskSlug, yakPath)
	}
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "Warning: multiple directories match %q, using first: %s\n", taskSlug, matches[0])
	}
	return matches[0], nil
}

// findYakPath walks up from startDir looking for a directory named yakDirName,
// similar to how git finds .git. Returns the full path if found, error if not.
// When used for spawn with --cwd in a subdirectory, this resolves to the workspace
// root's .yaks/, which is then mounted read-only in the sandbox so shavers cannot
// clobber workspace task state (yx and tests that write .yaks would otherwise overwrite it).
func findYakPath(startDir string, yakDirName string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, yakDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no .yaks directory found above %s — use --yak-path to specify", startDir)
}

func resolveInheritedWorktrees(absYakPath, yakValue string) ([]string, string, error) {
	taskDir, _, err := resolveYakValue(absYakPath, yakValue)
	if err != nil {
		return nil, "", err
	}
	rel, _ := filepath.Rel(absYakPath, taskDir)
	branchName := worktree.SanitizeBranchName(strings.Split(filepath.ToSlash(rel), "/")[0])

	workspaceRoot := filepath.Dir(absYakPath)
	searchDir := taskDir
	for {
		fieldPath := filepath.Join(searchDir, "worktrees")
		if info, err := os.Stat(fieldPath); err == nil && !info.IsDir() {
			data, err := os.ReadFile(fieldPath)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read %s: %w", fieldPath, err)
			}
			raw := strings.TrimSpace(string(data))
			if raw == "" {
				return nil, "", nil
			}
			entries := strings.Split(raw, ",")
			repos := make([]string, 0, len(entries))
			for _, entry := range entries {
				rel := strings.TrimSpace(entry)
				if rel == "" {
					continue
				}

				repoPath := filepath.Clean(filepath.Join(workspaceRoot, rel))
				info, err := os.Stat(repoPath)
				if err != nil || !info.IsDir() {
					return nil, "", fmt.Errorf("worktrees entry %q does not exist as a directory", rel)
				}
				if !worktree.IsGitRepo(repoPath) {
					return nil, "", fmt.Errorf("worktrees entry %q is not a git repository", rel)
				}
				repos = append(repos, repoPath)
			}

			if len(repos) == 0 {
				return nil, "", nil
			}
			return repos, branchName, nil
		}

		if searchDir == absYakPath {
			break
		}
		parent := filepath.Dir(searchDir)
		if parent == searchDir {
			break
		}
		searchDir = parent
	}

	return nil, "", nil
}

// copySkillsToHome copies each skill folder into the tool-appropriate location under homeDir.
// For Claude:   <homeDir>/.claude/skills/<skill-folder-name>/
// For Cursor:   <homeDir>/.claude/skills/<skill-folder-name>/
// For OpenCode: <homeDir>/.config/opencode/skills/<skill-folder-name>/
func copySkillsToHome(skillPaths []string, homeDir string, tool string) error {
	if len(skillPaths) == 0 {
		return nil
	}
	var destBase string
	switch types.Tool(tool) {
	case types.ToolClaude:
		destBase = filepath.Join(homeDir, ".claude", "skills")
	case types.ToolCursor:
		destBase = filepath.Join(homeDir, ".claude", "skills")
	case types.ToolOpencode:
		destBase = filepath.Join(homeDir, ".config", "opencode", "skills")
	default:
		return errors.NewValidationError(fmt.Sprintf("unsupported tool %q for skill copy", tool), nil)
	}
	if err := os.MkdirAll(destBase, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}
	for _, src := range skillPaths {
		skillName := filepath.Base(src)
		// Resolve symlinks so filepath.Walk sees the real directory.
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			return fmt.Errorf("failed to resolve skill path %q: %w", skillName, err)
		}
		dest := filepath.Join(destBase, skillName)
		if err := copyDirRecursive(resolved, dest); err != nil {
			return fmt.Errorf("failed to copy skill %q: %w", skillName, err)
		}
	}
	return nil
}

// copyDirRecursive copies src directory into dest, creating dest if needed.
// It handles symlinks within the directory by resolving them before copying.
func copyDirRecursive(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)

		// Handle symlinks: filepath.Walk uses Lstat, so symlinks appear as ModeSymlink.
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			resolvedInfo, err := os.Stat(resolved)
			if err != nil {
				return err
			}
			if resolvedInfo.IsDir() {
				return copyDirRecursive(resolved, target)
			}
			return copyFile(resolved, target, resolvedInfo.Mode())
		}

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func init() {
	spawnCmd.Flags().StringVar(&spawnCWD, "cwd", "", "Working directory for the worker (required unless yak worktrees field is set)")

	spawnCmd.Flags().StringVar(&spawnName, "yak-name", "", "Worker name used in logs and metadata (required)")
	spawnCmd.MarkFlagRequired("yak-name")

	spawnCmd.Flags().StringVar(&spawnSession, "session", "", "Zellij session name (default: auto-detect from ZELLIJ_SESSION_NAME)")

	spawnCmd.Flags().StringVar(&spawnMode, "mode", "build", "Agent mode: 'plan' or 'build'")
	spawnCmd.Flags().StringVar(&spawnResources, "resources", "default", "Resource profile: 'light', 'default', 'heavy', or 'ram'")
	spawnCmd.Flags().StringSliceVar(&spawnYaks, "yaks", []string{}, "Yak paths from .yaks/ to assign (can be repeated)")
	spawnCmd.Flags().StringSliceVar(&spawnYaks, "task", []string{}, "Alias for --yaks")
	spawnCmd.Flags().StringVar(&spawnYakPath, "yak-path", ".yaks", "Path to task state directory")
	spawnCmd.Flags().StringVar(&spawnRuntime, "runtime", "", "Runtime: 'native', 'sandbox', or 'devcontainer'")
	spawnCmd.MarkFlagRequired("runtime")
	spawnCmd.Flags().StringVar(&spawnTool, "tool", "claude", "AI tool: 'opencode', 'claude', or 'cursor'")
	spawnCmd.Flags().StringVar(&spawnModel, "model", "", "Optional model override (defaults: claude='default', cursor='auto')")
	spawnCmd.Flags().BoolVar(&spawnClean, "clean", false, "Clean worker home directory before spawning")
	spawnCmd.Flags().BoolVar(&spawnAutoWorktree, "auto-worktree", false, "Automatically create and use git worktree for the task")
	spawnCmd.Flags().StringArrayVar(&spawnSkills, "skill", []string{}, "Path to a skill folder to copy into the worker's home (can be repeated)")
	spawnCmd.Flags().StringVar(&spawnShaverName, "shaver-name", "", "Shaver identity for tab title and assigned-to (sets YAK_SHAVER_NAME in worker); default: env YAK_SHAVER_NAME → USER → yak-shaver")
}
