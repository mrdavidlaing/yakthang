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
			PIDs:   1024,
			Tmpfs: map[string]string{
				"/tmp":                    "size=2g,exec,uid=1000,gid=1000",
				"/home/yak-shaver":        "size=1g,exec,uid=1000,gid=1000",
				"/home/yak-shaver/.cache": "size=1g,exec,uid=1000,gid=1000",
			},
		}
	default:
		return types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
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
func SpawnSandboxedWorker(worker *types.Worker, persona *types.Persona, prompt string, profile types.ResourceProfile) error {
	containerName := containerNamePrefix + worker.Name
	networkMode := GetNetworkMode()
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	// Create worker directory for temp files (persist until Zellij starts)
	workerDir, err := os.MkdirTemp("", "worker-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	// Don't clean up immediately - let Zellij use it

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

	// Create wrapper script that runs docker in background with -d flag for detached
	wrapperScript := filepath.Join(workerDir, "run.sh")
	wrapperContent := fmt.Sprintf(`#!/usr/bin/env bash
exec docker run -it --rm \\
	--name %s \\
	--user "%d:%d" \\
	--network %s \\
	--security-opt no-new-privileges \\
	--cap-drop ALL \\
	--tmpfs /tmp:rw,exec,size=2g \\
	--tmpfs /home/yak-shaver:rw,exec,size=1g \\
	--cpus %s \\
	--memory %s \\
	--pids-limit %d \\
	--stop-timeout 7200 \\
	-v "%s:%s:rw" \\
	-v "%s:%s:rw" \\
	-v "%s:/opt/worker/prompt.txt:ro" \\
	-v "%s:/opt/worker/start.sh:ro" \\
	-w "%s" \\
	-e HOME=/home/yak-shaver \
	-e GOPATH=/home/yak-shaver/.go \
	-e CARGO_HOME=/home/yak-shaver/.cargo \
	-e RUSTUP_HOME=/home/yak-shaver/.rustup \
	-e OPENCODE_API_KEY="${OPENCODE_API_KEY}" \\
	-e WORKER_NAME="%s" \\
	-e WORKER_EMOJI="%s" \\
	-e YAK_PATH="%s" \\
	yak-shaver:latest \\
	bash /opt/worker/start.sh build
`, containerName, os.Getuid(), os.Getgid(), networkMode, profile.CPUs, profile.Memory, profile.PIDs, workspaceRoot, workspaceRoot, worker.YakPath, worker.YakPath, promptFile, innerScript, worker.CWD, persona.Name, persona.Emoji, worker.YakPath)

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
        pane size="33%%" name="shell: %s"
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, wrapperScript, worker.CWD)

	if err := os.WriteFile(layoutFile, []byte(layoutContent), 0644); err != nil {
		return fmt.Errorf("failed to write layout file: %w", err)
	}

	// Spawn Zellij tab with the layout
	var zellijCmd *exec.Cmd
	sessionName := os.Getenv("ZELLIJ_SESSION_NAME")
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
	if sessionName != "" {
		prevTabCmd = exec.Command("zellij", "--session", sessionName, "action", "go-to-previous-tab")
	} else {
		prevTabCmd = exec.Command("zellij", "action", "go-to-previous-tab")
	}
	prevTabCmd.Run()

	// Clean up temp files after a delay (give Zellij time to read them)
	go func() {
		time.Sleep(5 * time.Second)
		os.RemoveAll(workerDir)
	}()

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
