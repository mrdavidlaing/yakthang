package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/errors"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/sessions"
)

var diffName string

var diffCmd = &cobra.Command{
	Use:   "diff --name <worker>",
	Short: "Show diffs for all repos in a worker's home",
	Long: `Show git diffs for all repos in a worker's home directory.

A worker's home directory contains flat repo directories (each a git worktree).
This command loops through them and shows changes against their default branch (main or master).`,
	Example: `  # Show all diffs for worker Yakira
  yak-box diff --name Yakira`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if diffName == "" {
			return errors.NewValidationError("--name is required (worker name)", nil)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDiff(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

func runDiff() error {
	homeDir, err := sessions.GetHomeDir(diffName)
	if err != nil {
		return fmt.Errorf("could not resolve home for worker %q: %w", diffName, err)
	}

	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		return fmt.Errorf("no home directory found for worker %q (expected %s)", diffName, homeDir)
	}

	entries, err := os.ReadDir(homeDir)
	if err != nil {
		return fmt.Errorf("failed to read home directory %s: %w", homeDir, err)
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(homeDir, entry.Name())
		if !hasOwnGitDir(repoPath) {
			continue
		}
		found = true
		branch := defaultBranch(repoPath)
		fmt.Printf("\n=== %s (diff against %s) ===\n", entry.Name(), branch)
		cmd := exec.Command("git", "-C", repoPath, "diff", branch+"...HEAD")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: git diff failed for %s: %v\n", entry.Name(), err)
		}
	}

	if !found {
		fmt.Printf("No git repos found in %s\n", homeDir)
	}

	return nil
}

// hasOwnGitDir reports whether path directly owns a .git entry (file or directory).
// Using git rev-parse would walk up to a parent repo, falsely identifying plain
// subdirectories as git repos. This strict check avoids that.
func hasOwnGitDir(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// defaultBranch returns "main" if it exists as a local or remote ref, otherwise "master".
func defaultBranch(repoPath string) string {
	for _, candidate := range []string{"main", "master"} {
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", candidate)
		if err := cmd.Run(); err == nil {
			return candidate
		}
		cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/"+candidate)
		if err := cmd.Run(); err == nil {
			return "origin/" + candidate
		}
	}
	return "main"
}

func init() {
	diffCmd.Flags().StringVar(&diffName, "name", "", "Worker name (required)")
	diffCmd.MarkFlagRequired("name")
}
