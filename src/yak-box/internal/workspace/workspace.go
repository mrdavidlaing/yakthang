// Package workspace provides utilities for finding the workspace root directory.
package workspace

import (
	"os/exec"
	"strings"
)

// FindRoot finds the git repository root directory by running `git rev-parse --show-toplevel`.
// It returns the root directory path or an error if the command fails.
func FindRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
