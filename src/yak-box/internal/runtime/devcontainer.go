package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/workspace"
)

const devcontainerPath = ".devcontainer"
const workerImageName = "yak-worker:latest"

func getStoredDevcontainerCommit() (string, error) {
	cmd := exec.Command("docker", "image", "inspect", workerImageName, "--format", "{{index .Config.Labels \"yak-box.devcontainer.commit\"}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func isImageUpToDate() (bool, error) {
	workspaceRoot, err := workspace.FindRoot()
	if err != nil {
		return false, err
	}

	currentCommit, err := getDevcontainerCommit(workspaceRoot)
	if err != nil {
		return false, err
	}

	storedCommit, err := getStoredDevcontainerCommit()
	if err != nil {
		return false, err
	}

	if currentCommit != storedCommit {
		return false, nil
	}

	dirty, err := isDevcontainerDirty(workspaceRoot)
	if err != nil {
		return false, err
	}
	if dirty {
		return false, nil
	}

	return true, nil
}

func isDevcontainerDirty(workspaceRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain", ".")
	cmd.Dir = workspaceRoot + "/" + devcontainerPath
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}

// RebuildDevcontainer rebuilds the yak-worker Docker image from the .devcontainer directory.
func RebuildDevcontainer() error {
	workspaceRoot, err := workspace.FindRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	commitHash, err := getDevcontainerCommit(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to get devcontainer commit: %w", err)
	}

	fmt.Println("Rebuilding yak-worker image...")

	cmd := exec.Command("docker", "build",
		"-t", workerImageName,
		"-f", devcontainerPath+"/Dockerfile",
		"--label", "yak-box.devcontainer.commit="+commitHash,
		".")
	cmd.Dir = workspaceRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to rebuild docker image: %w", err)
	}

	fmt.Println("Image rebuilt successfully")
	return nil
}

func getDevcontainerCommit(workspaceRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workspaceRoot + "/" + devcontainerPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// EnsureDevcontainer ensures the Docker image exists and is up-to-date
func EnsureDevcontainer() error {
	workspaceRoot, err := workspace.FindRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	devcontainerDir := filepath.Join(workspaceRoot, devcontainerPath)
	hasDevcontainer := dirExists(devcontainerDir)

	if imageExists, err := ImageExists(); err == nil && imageExists {
		// If there's no .devcontainer dir, image is considered up-to-date as-is
		if !hasDevcontainer {
			return nil
		}
		upToDate, err := isImageUpToDate()
		if err != nil {
			return fmt.Errorf("failed to check image status: %w", err)
		}
		if !upToDate {
			return RebuildDevcontainer()
		}
		return nil
	}

	// Image doesn't exist
	if !hasDevcontainer {
		return fmt.Errorf("yak-worker:latest image not found and no .devcontainer/Dockerfile to build from")
	}

	fmt.Println("Building yak-worker image for the first time...")
	return RebuildDevcontainer()
}

// ImageExists checks if the yak-worker Docker image exists locally
func ImageExists() (bool, error) {
	cmd := exec.Command("docker", "image", "inspect", workerImageName)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	// Check if it's a "not found" error
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
	}
	return false, err
}
