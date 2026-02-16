package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const devcontainerPath = ".devcontainer"
const workerImageName = "yak-shaver:latest"

func getStoredDevcontainerCommit() (string, error) {
	cmd := exec.Command("docker", "image", "inspect", workerImageName, "--format", "{{index .Config.Labels \"yakthang.devcontainer.commit\"}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func isImageUpToDate() (bool, error) {
	workspaceRoot, err := findWorkspaceRoot()
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

func RebuildDevcontainer() error {
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	commitHash, err := getDevcontainerCommit(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to get devcontainer commit: %w", err)
	}

	fmt.Println("Rebuilding yak-shaver image...")

	cmd := exec.Command("docker", "build",
		"-t", workerImageName,
		"-f", devcontainerPath+"/Dockerfile",
		"--label", "yakthang.devcontainer.commit="+commitHash,
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

// EnsureDevcontainer ensures the Docker image exists and is up-to-date
func EnsureDevcontainer() error {
	if imageExists, err := ImageExists(); err == nil && imageExists {
		upToDate, err := isImageUpToDate()
		if err != nil {
			return fmt.Errorf("failed to check image status: %w", err)
		}
		if !upToDate {
			return RebuildDevcontainer()
		}
		return nil
	}

	fmt.Println("Building yak-shaver image for the first time...")
	return RebuildDevcontainer()
}

// ImageExists checks if the yak-shaver Docker image exists locally
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
