package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yakthang/yakbox/pkg/types"
)

const (
	containerNamePrefix = "yak-shaver-"
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
			Memory: "4g",
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
			Memory: "8g",
			Swap:   "16g",
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
			Memory: "2g",
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

// DetectRuntime detects the available runtime (sandboxed/docker or native/zellij)
func DetectRuntime() string {
	if _, err := exec.LookPath("docker"); err == nil {
		if err := exec.Command("docker", "ps").Run(); err == nil {
			return "sandboxed"
		}
	}
	if _, err := exec.LookPath("zellij"); err == nil {
		return "native"
	}
	return "unknown"
}

// GetNetworkMode returns the network mode for Docker
func GetNetworkMode() string {
	cmd := exec.Command("docker", "network", "inspect", networkName)
	if err := cmd.Run(); err != nil {
		return "bridge"
	}
	return networkName
}

// SpawnSandboxedWorker spawns a worker in a Docker container via Zellij tab
func SpawnSandboxedWorker(worker *types.Worker, persona *types.Persona, prompt string, profile types.ResourceProfile, homeDir string) error {
	containerName := containerNamePrefix + worker.Name
	networkMode := GetNetworkMode()
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	// Create worker directory for scripts (persist in .yak-boxes)
	workerDir := filepath.Join(homeDir, "scripts")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create scripts dir: %w", err)
	}

	// Write prompt to file
	promptFile := filepath.Join(workerDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Create inner script that runs inside container
	innerScript := filepath.Join(workerDir, "inner.sh")
	innerContent := `#!/usr/bin/env bash
WORKSPACE_ROOT="${WORKSPACE_ROOT:-/home/yakob/yakthang}"
COST_DIR="${WORKSPACE_ROOT}/.worker-costs"
mkdir -p "$COST_DIR"

PROMPT="$(cat /opt/worker/prompt.txt)"
opencode --prompt "$PROMPT" --agent "$1"
EXIT_CODE=$?

WORKER="${WORKER_NAME:-unknown}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
SID="$(opencode session list 2>/dev/null | tail -1 | awk '{print $1}')"
if [[ -n "$SID" && "$SID" != "Session" ]]; then
  opencode export "$SID" > "${COST_DIR}/${WORKER}-${TS}.json" 2>/dev/null
fi
opencode stats --models > "${COST_DIR}/${WORKER}-${TS}.stats.txt" 2>/dev/null
exit $EXIT_CODE
`
	if err := os.WriteFile(innerScript, []byte(innerContent), 0755); err != nil {
		return fmt.Errorf("failed to write inner script: %w", err)
	}

	// Create shell-exec helper script that waits for container to be ready
	shellExecScript := filepath.Join(workerDir, "shell-exec.sh")
	shellExecContent := `#!/usr/bin/env bash
set -euo pipefail
CONTAINER_NAME="$1"
MAX_RETRIES=30
RETRY_DELAY=1

for i in $(seq 1 $MAX_RETRIES); do
    if docker inspect --format='{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null | grep -q "running"; then
        exec docker exec -it "$CONTAINER_NAME" bash
    fi
    if [[ $i -eq 1 ]]; then
        echo "Waiting for container to start..."
    fi
    sleep $RETRY_DELAY
done

echo "ERROR: Container did not start within $MAX_RETRIES seconds"
exit 1
`
	if err := os.WriteFile(shellExecScript, []byte(shellExecContent), 0755); err != nil {
		return fmt.Errorf("failed to write shell-exec script: %w", err)
	}

	// Create wrapper script that runs docker in background with -d flag for detached
	wrapperScript := filepath.Join(workerDir, "run.sh")

	// Build swap flag if specified
	swapFlag := ""
	if profile.Swap != "" {
		swapFlag = fmt.Sprintf("\t--memory-swap %s \\\\", profile.Swap)
	}

	cargoJobsEnv := ""
	if profile.Name == "ram" {
		cargoJobsEnv = "\t-e CARGO_BUILD_JOBS=4 \\"
	}

	homeMount := ""
	if homeDir != "" {
		homeMount = fmt.Sprintf("\t-v \"%s:/home/yak-shaver:rw\" \\", homeDir)
	}

	// Mount OpenCode auth.json for GitHub Copilot model access
	homeDir_host := os.Getenv("HOME")
	authMount := fmt.Sprintf("\t-v \"%s/.local/share/opencode/auth.json:/home/yak-shaver/.local/share/opencode/auth.json:ro\" \\", homeDir_host)

	wrapperContent := fmt.Sprintf(`#!/usr/bin/env bash
exec docker run -t --rm \
	--name %s \
	--user "%d:%d" \
	--network %s \
	--security-opt no-new-privileges \
	--cap-drop ALL \
	--tmpfs /tmp:rw,exec,size=2g \
	--cpus %s \
	--memory %s \
%s	--pids-limit %d \
	--stop-timeout 7200 \
	-v "%s:%s:rw" \
	-v "%s:%s:rw" \
	-v "%s:/opt/worker/prompt.txt:ro" \
	-v "%s:/opt/worker/start.sh:ro" \
%s
%s
	-w "%s" \
	-e HOME=/home/yak-shaver \
	-e GOPATH=/home/yak-shaver/.go \
	-e CARGO_HOME=/home/yak-shaver/.cargo \
	-e RUSTUP_HOME=/home/yak-shaver/.rustup \
%s	-e WORKER_NAME="%s" \
	-e WORKER_EMOJI="%s" \
	-e YAK_PATH="%s" \
	yak-shaver:latest \
	bash /opt/worker/start.sh build
`, containerName, os.Getuid(), os.Getgid(), networkMode, profile.CPUs, profile.Memory, swapFlag, profile.PIDs, workspaceRoot, workspaceRoot, worker.YakPath, worker.YakPath, promptFile, innerScript, homeMount, authMount, worker.CWD, cargoJobsEnv, persona.Name, persona.Emoji, worker.YakPath)

	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	// Create Zellij layout file
	layoutFile := filepath.Join(workerDir, "layout.kdl")
	layoutContent := fmt.Sprintf(`layout {
    tab name="%s" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane size="67%%" name="opencode (build) [docker]" focus=true {
            command "bash"
            args "%s"
        }
        pane size="33%%" name="shell: container" {
            command "bash"
            args "%s" "%s"
        }
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, wrapperScript, shellExecScript, containerName)

	if err := os.WriteFile(layoutFile, []byte(layoutContent), 0644); err != nil {
		return fmt.Errorf("failed to write layout file: %w", err)
	}

	// Spawn Zellij tab with the layout
	var zellijCmd *exec.Cmd
	sessionName := worker.SessionName
	if sessionName != "" {
		zellijCmd = exec.Command("zellij", "--session", sessionName, "action", "new-tab", "--layout", layoutFile, "--name", worker.DisplayName)
	} else {
		zellijCmd = exec.Command("zellij", "action", "new-tab", "--layout", layoutFile, "--name", worker.DisplayName)
	}

	if err := zellijCmd.Run(); err != nil {
		return fmt.Errorf("failed to create Zellij tab: %w", err)
	}

	// Small delay then go back to previous tab
	time.Sleep(300 * time.Millisecond)
	var prevTabCmd *exec.Cmd
	if worker.SessionName != "" {
		prevTabCmd = exec.Command("zellij", "--session", worker.SessionName, "action", "go-to-previous-tab")
	} else {
		prevTabCmd = exec.Command("zellij", "action", "go-to-previous-tab")
	}
	prevTabCmd.Run()

	return nil
}

// StopSandboxedWorker stops a sandboxed worker with timeout
func StopSandboxedWorker(name string, timeout time.Duration) error {
	containerName := containerNamePrefix + name

	// Check if container exists
	cmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=^%s$", containerName), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check container: %w", err)
	}

	if strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("container %s not found", containerName)
	}

	// Stop container
	stopCmd := exec.Command("docker", "stop", "-t", fmt.Sprintf("%d", int(timeout.Seconds())), containerName)
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove container
	rmCmd := exec.Command("docker", "rm", containerName)
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// ListRunningContainers returns list of running worker containers
func ListRunningContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name=yak-shaver-", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			containers = append(containers, line)
		}
	}
	return containers, nil
}

// ListAllContainers returns list of all worker containers (running and stopped)
func ListAllContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=yak-shaver-", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			containers = append(containers, line)
		}
	}
	return containers, nil
}

func findWorkspaceRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
