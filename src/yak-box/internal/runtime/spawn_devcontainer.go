package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/workspace"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/zellij"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/types"
)

const (
	containerNamePrefix = "yak-worker-"
	workerCacheDir      = ".yak-boxes"
	networkName         = "yak-shavers"
)

// GetResourceProfile returns the resource profile for a given name
func GetResourceProfile(name string) types.ResourceProfile {
	switch name {
	case "light":
		return types.ResourceProfile{
			Name:   "light",
			CPUs:   "0.5",
			Memory: "1g",
			Swap:   "",
			PIDs:   256,
			Tmpfs: map[string]string{
				"/tmp":                    "size=1g,exec,uid=1000,gid=1000",
				"/home/yak-shaver":        "size=512m,exec,uid=1000,gid=1000",
				"/home/yak-shaver/.cache": "size=512m,exec,uid=1000,gid=1000",
			},
		}
	case "heavy":
		return types.ResourceProfile{
			Name:   "heavy",
			CPUs:   "2.0",
			Memory: "6g",
			Swap:   "",
			PIDs:   1024,
			Tmpfs: map[string]string{
				"/tmp":                    "size=2g,exec,uid=1000,gid=1000",
				"/home/yak-shaver":        "size=1g,exec,uid=1000,gid=1000",
				"/home/yak-shaver/.cache": "size=1g,exec,uid=1000,gid=1000",
			},
		}
	case "ram":
		return types.ResourceProfile{
			Name:   "ram",
			CPUs:   "0",
			Memory: "12g",
			Swap:   "24g",
			PIDs:   2048,
			Tmpfs: map[string]string{
				"/tmp":                    "size=4g,exec,uid=1000,gid=1000",
				"/home/yak-shaver":        "size=2g,exec,uid=1000,gid=1000",
				"/home/yak-shaver/.cache": "size=2g,exec,uid=1000,gid=1000",
			},
		}
	default:
		return types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "3g",
			Swap:   "",
			PIDs:   512,
			Tmpfs: map[string]string{
				"/tmp":                    "size=1g,exec,uid=1000,gid=1000",
				"/home/yak-shaver":        "size=512m,exec,uid=1000,gid=1000",
				"/home/yak-shaver/.cache": "size=512m,exec,uid=1000,gid=1000",
			},
		}
	}
}

// GetNetworkMode returns the network mode for Docker
func GetNetworkMode(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", networkName)
	if err := cmd.Run(); err != nil {
		return "bridge"
	}
	return networkName
}

// SpawnDevcontainerWorker spawns a worker in a Docker container via Zellij tab
func SpawnDevcontainerWorker(ctx context.Context, opts ...SpawnOption) error {
	cfg := &spawnConfig{
		commander: &defaultCommander{},
		profile:   GetResourceProfile("default"),
	}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("option error: %w. Suggestion: Check all spawn options (worker, prompt, resources, etc.) are provided correctly", err)
		}
	}

	if cfg.worker == nil {
		return fmt.Errorf("worker is required. Suggestion: Ensure worker config is provided via spawn options")
	}

	containerName := containerNamePrefix + cfg.worker.Name
	networkMode := GetNetworkMode(ctx)
	workspaceRoot, err := workspace.FindRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w. Suggestion: Ensure you're in a valid yak-box workspace with a .yak-box directory", err)
	}

	// Create worker directory for scripts (persist in .yak-boxes)
	workerDir := filepath.Join(cfg.homeDir, "scripts")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create scripts dir: %w. Suggestion: Check that .yak-boxes home directory is writable", err)
	}

	// Prepare Claude settings using the shared native+sandbox setup path.
	// This keeps Claude bootstrap behavior consistent across runtimes; sandbox
	// differs only in execution inside a container.
	if err := setupClaudeSettings(cfg.homeDir, resolveAnthropicKey()); err != nil {
		return fmt.Errorf("failed to setup Claude settings: %w. Suggestion: Ensure the .yak-boxes directory is writable", err)
	}

	// Write prompt to file
	promptFile := filepath.Join(workerDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(cfg.prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w. Suggestion: Ensure the .yak-boxes directory is writable and has sufficient disk space", err)
	}

	// Create inner script that runs inside container
	innerScript := filepath.Join(workerDir, "inner.sh")
	if err := os.WriteFile(innerScript, []byte(generateInitScript()), 0755); err != nil {
		return fmt.Errorf("failed to write inner script: %w. Suggestion: Check disk space and file permissions in .yak-boxes directory", err)
	}

	// Create shell-exec helper script that waits for container to be ready
	shellExecScript := filepath.Join(workerDir, "shell-exec.sh")
	if err := os.WriteFile(shellExecScript, []byte(generateWaitScript()), 0755); err != nil {
		return fmt.Errorf("failed to write shell-exec script: %w. Suggestion: Check .yak-boxes directory exists and is writable", err)
	}

	// Generate custom /etc/passwd and /etc/group for the container
	uid := os.Getuid()
	gid := os.Getgid()
	passwdContent := fmt.Sprintf("root:x:0:0:root:/root:/bin/bash\nyakshaver:x:%d:%d:Yak Shaver:/home/yak-shaver:/bin/bash\n", uid, gid)
	groupContent := fmt.Sprintf("root:x:0:\nyakshaver:x:%d:\n", gid)
	passwdFile := filepath.Join(workerDir, "passwd")
	groupFile := filepath.Join(workerDir, "group")
	if err := os.WriteFile(passwdFile, []byte(passwdContent), 0644); err != nil {
		return fmt.Errorf("failed to write passwd file: %w. Suggestion: Ensure .yak-boxes directory is writable and has sufficient space", err)
	}
	if err := os.WriteFile(groupFile, []byte(groupContent), 0644); err != nil {
		return fmt.Errorf("failed to write group file: %w. Suggestion: Ensure .yak-boxes directory is writable", err)
	}

	// Create wrapper script that runs docker in background with -d flag for detached
	wrapperScript := filepath.Join(workerDir, "run.sh")
	runScriptContent := generateRunScript(cfg, workspaceRoot, promptFile, innerScript, passwdFile, groupFile, networkMode)

	if err := os.WriteFile(wrapperScript, []byte(runScriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w. Suggestion: Check .yak-boxes directory permissions and disk space", err)
	}

	// Create Zellij layout file (single generator from internal/zellij)
	layoutFile := filepath.Join(workerDir, "layout.kdl")
	layoutContent := zellij.GenerateLayout(cfg.worker, string(types.RuntimeDevcontainer), cfg.worker.Tool)
	layoutContent = strings.ReplaceAll(layoutContent, "%WRAPPER%", wrapperScript)
	layoutContent = strings.ReplaceAll(layoutContent, "%SHELL_EXEC_SCRIPT%", shellExecScript)
	layoutContent = strings.ReplaceAll(layoutContent, "%CONTAINER_NAME%", containerName)

	if err := os.WriteFile(layoutFile, []byte(layoutContent), 0644); err != nil {
		return fmt.Errorf("failed to write layout file: %w. Suggestion: Ensure .yak-boxes directory is writable", err)
	}

	// Spawn Zellij tab with the layout
	var zellijCmd *exec.Cmd
	sessionName := cfg.worker.SessionName
	if sessionName != "" {
		zellijCmd = cfg.commander.CommandContext(ctx, "zellij", "--session", sessionName, "action", "new-tab", "--layout", layoutFile, "--name", cfg.worker.DisplayName)
	} else {
		zellijCmd = cfg.commander.CommandContext(ctx, "zellij", "action", "new-tab", "--layout", layoutFile, "--name", cfg.worker.DisplayName)
	}

	if err := zellijCmd.Run(); err != nil {
		return fmt.Errorf("failed to create Zellij tab: %w. Suggestion: Ensure Zellij is installed and you're in a Zellij session, or use --runtime=devcontainer", err)
	}

	return nil
}

// StopDevcontainerWorker stops a devcontainer worker with timeout
func StopDevcontainerWorker(name string, timeout time.Duration) error {
	containerName := containerNamePrefix + name

	// Check if container exists
	cmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=^%s$", containerName), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check container: %w. Suggestion: Ensure Docker is running with 'docker ps'", err)
	}

	if strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("container %s not found. Suggestion: Use 'docker ps -a' to see available containers, or check worker name is correct", containerName)
	}

	// Stop container
	stopCmd := exec.Command("docker", "stop", "-t", fmt.Sprintf("%d", int(timeout.Seconds())), containerName)
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %w. Suggestion: Check Docker is running or try 'docker stop %s' manually", err, containerName)
	}

	// Remove container
	rmCmd := exec.Command("docker", "rm", containerName)
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w. Suggestion: The container may still be running; try 'docker rm -f %s' manually", err, containerName)
	}

	return nil
}

// ListRunningContainers returns list of running worker containers.
// Errors are wrapped with context so callers can distinguish Docker failures from an empty list.
func ListRunningContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name=yak-worker-", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list running containers (is Docker running?): %w", err)
	}

	var containers []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			containers = append(containers, line)
		}
	}
	return containers, nil
}

// ListAllContainers returns list of all worker containers (running and stopped).
// Errors are wrapped with context so callers get actionable messages.
func ListAllContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=yak-worker-", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers (is Docker running?): %w", err)
	}

	var containers []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			containers = append(containers, line)
		}
	}
	return containers, nil
}
