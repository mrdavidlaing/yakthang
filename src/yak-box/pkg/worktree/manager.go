// Package worktree manages Git worktrees for yak-box projects.
package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetermineWorktreePath calculates the path for a worktree
// Uses XDG-compliant location: ~/.local/share/yakthang/worktrees/<project>/<task-path>
func DetermineWorktreePath(projectPath, taskPath string) string {
	projectName := filepath.Base(projectPath)
	sanitizedName := sanitizeTaskPath(taskPath)

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to old behavior if can't get home
		parentDir := filepath.Dir(projectPath)
		return filepath.Join(parentDir, fmt.Sprintf("%s-%s", projectName, sanitizedName))
	}

	// XDG-compliant path: ~/.local/share/yakthang/worktrees/<project>/<task-path>
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(homeDir, ".local", "share")
	}

	worktreePath := filepath.Join(xdgDataHome, "yakthang", "worktrees", projectName, sanitizedName)

	// Ensure parent directory exists
	_ = os.MkdirAll(filepath.Dir(worktreePath), 0755)

	return worktreePath
}

// sanitizeTaskPath converts task path to filesystem-safe name
func sanitizeTaskPath(taskPath string) string {
	// Replace slashes with dashes
	name := strings.ReplaceAll(taskPath, "/", "-")
	// Remove other problematic characters
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

// IsGitRepo checks if a directory is a git repository
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// WorktreeExists checks if a worktree with the given name exists
// Checks in the context of the projectPath git repository
func WorktreeExists(projectPath, worktreeName string) (bool, error) {
	cmd := exec.Command("git", "-C", projectPath, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	// Parse worktree list output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if branch == worktreeName {
				return true, nil
			}
		}
	}
	return false, nil
}

// GetWorktreePath gets the actual path of an existing worktree
// Searches in the context of the projectPath git repository
func GetWorktreePath(projectPath, worktreeName string) (string, error) {
	cmd := exec.Command("git", "-C", projectPath, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse worktree list output
	// Format is:
	// worktree <path>
	// HEAD <sha>
	// branch refs/heads/<name>
	// (blank line between entries)
	lines := strings.Split(string(output), "\n")
	var currentPath string
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if branch == worktreeName {
				return currentPath, nil
			}
		}
	}
	return "", fmt.Errorf("worktree '%s' not found", worktreeName)
}

// CreateWorktree creates a new worktree
// Creates it in the context of the projectPath git repository
func CreateWorktree(projectPath, worktreePath, branchName string, verbose bool) error {
	// Check if branch already exists
	checkCmd := exec.Command("git", "-C", projectPath, "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", branchName))
	branchExists := checkCmd.Run() == nil

	var cmd *exec.Cmd
	if branchExists {
		// Branch exists, check it out in the worktree
		cmd = exec.Command("git", "-C", projectPath, "worktree", "add", worktreePath, branchName)
		if verbose {
			fmt.Fprintf(os.Stderr, "+ git -C %s worktree add %s %s\n", projectPath, worktreePath, branchName)
		}
	} else {
		// Branch doesn't exist, create it
		cmd = exec.Command("git", "-C", projectPath, "worktree", "add", worktreePath, "-b", branchName)
		if verbose {
			fmt.Fprintf(os.Stderr, "+ git -C %s worktree add %s -b %s\n", projectPath, worktreePath, branchName)
		}
	}

	if verbose {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// EnsureWorktree ensures a worktree exists, creating it if necessary
// Returns the path to the worktree
func EnsureWorktree(projectPath, taskPath string, verbose bool) (string, error) {
	// Verify projectPath is a git repo
	if !IsGitRepo(projectPath) {
		return "", fmt.Errorf("not a git repository: %s", projectPath)
	}

	// Convert task path to branch name (replace / with -)
	branchName := strings.ReplaceAll(taskPath, "/", "-")

	// Check if worktree already exists
	exists, err := WorktreeExists(projectPath, branchName)
	if err != nil {
		return "", fmt.Errorf("failed to check worktree existence: %w", err)
	}

	if exists {
		// Get existing worktree path
		path, err := GetWorktreePath(projectPath, branchName)
		if err != nil {
			return "", fmt.Errorf("failed to get worktree path: %w", err)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "Using existing worktree: %s\n", path)
		}
		return path, nil
	}

	// Determine where to create the worktree
	worktreePath := DetermineWorktreePath(projectPath, taskPath)

	// Create the worktree
	if err := CreateWorktree(projectPath, worktreePath, branchName, verbose); err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Created worktree: %s\n", worktreePath)
	}

	return worktreePath, nil
}
