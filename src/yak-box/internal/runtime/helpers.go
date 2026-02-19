package runtime

import (
	"fmt"
	"os"
	"strings"

	"github.com/yakthang/yakbox/pkg/devcontainer"
)

func generateInitScript() string {
	return `#!/usr/bin/env bash
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
}

func generateWaitScript() string {
	return `#!/usr/bin/env bash
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
}

func generateRunScript(cfg *spawnConfig, workspaceRoot, promptFile, innerScript, passwdFile, groupFile, networkMode string) string {
	containerName := containerNamePrefix + cfg.worker.Name

	var sb strings.Builder
	sb.WriteString("#!/usr/bin/env bash\n")
	sb.WriteString("exec docker run -it --rm \\\n")
	sb.WriteString(fmt.Sprintf("\t--name %s \\\n", containerName))
	sb.WriteString(fmt.Sprintf("\t--user \"%d:%d\" \\\n", os.Getuid(), os.Getgid()))
	sb.WriteString(fmt.Sprintf("\t--network %s \\\n", networkMode))
	sb.WriteString("\t--security-opt no-new-privileges \\\n")
	sb.WriteString("\t--cap-drop ALL \\\n")
	sb.WriteString("\t--tmpfs /tmp:rw,exec,size=2g \\\n")
	sb.WriteString(fmt.Sprintf("\t--cpus %s \\\n", cfg.profile.CPUs))
	sb.WriteString(fmt.Sprintf("\t--memory %s \\\n", cfg.profile.Memory))

	if cfg.profile.Swap != "" {
		sb.WriteString(fmt.Sprintf("\t--memory-swap %s \\\n", cfg.profile.Swap))
	}

	sb.WriteString(fmt.Sprintf("\t--pids-limit %d \\\n", cfg.profile.PIDs))
	sb.WriteString("\t--stop-timeout 7200 \\\n")

	// Standard mounts
	sb.WriteString(fmt.Sprintf("\t-v \"%s:%s:rw\" \\\n", workspaceRoot, workspaceRoot))
	sb.WriteString(fmt.Sprintf("\t-v \"%s:/opt/worker/prompt.txt:ro\" \\\n", promptFile))
	sb.WriteString(fmt.Sprintf("\t-v \"%s:/opt/worker/start.sh:ro\" \\\n", innerScript))

	if cfg.homeDir != "" {
		sb.WriteString(fmt.Sprintf("\t-v \"%s:/home/yak-shaver:rw\" \\\n", cfg.homeDir))
	}

	if cfg.worker.WorktreePath != "" {
		sb.WriteString(fmt.Sprintf("\t-v \"%s:%s:rw\" \\\n", cfg.worker.WorktreePath, cfg.worker.WorktreePath))
	}

	homeDirHost := os.Getenv("HOME")
	sb.WriteString(fmt.Sprintf("\t-v \"%s/.local/share/opencode/auth.json:/home/yak-shaver/.local/share/opencode/auth.json:ro\" \\\n", homeDirHost))
	sb.WriteString(fmt.Sprintf("\t-v \"%s:/etc/passwd:ro\" \\\n", passwdFile))
	sb.WriteString(fmt.Sprintf("\t-v \"%s:/etc/group:ro\" \\\n", groupFile))

	// Devcontainer mounts
	if cfg.devConfig != nil {
		for _, mount := range cfg.devConfig.Mounts {
			sb.WriteString(fmt.Sprintf("\t-v \"%s\" \\\n", mount))
		}
	}

	sb.WriteString(fmt.Sprintf("\t-w \"%s\" \\\n", cfg.worker.CWD))
	sb.WriteString("\t-e HOME=/home/yak-shaver \\\n")
	sb.WriteString("\t-e TERM=\"${TERM:-xterm-256color}\" \\\n")
	sb.WriteString("\t-e GOPATH=/home/yak-shaver/.go \\\n")
	sb.WriteString("\t-e CARGO_HOME=/home/yak-shaver/.cargo \\\n")
	sb.WriteString("\t-e RUSTUP_HOME=/home/yak-shaver/.rustup \\\n")

	if cfg.profile.Name == "ram" {
		sb.WriteString("\t-e CARGO_BUILD_JOBS=4 \\\n")
	}

	sb.WriteString(fmt.Sprintf("\t-e WORKER_NAME=\"%s\" \\\n", cfg.persona.Name))
	sb.WriteString(fmt.Sprintf("\t-e WORKER_EMOJI=\"%s\" \\\n", cfg.persona.Emoji))
	sb.WriteString(fmt.Sprintf("\t-e YAK_PATH=\"%s\" \\\n", cfg.worker.YakPath))

	// Devcontainer envs
	imageName := "yak-shaver:latest"
	if cfg.devConfig != nil {
		if cfg.devConfig.Image != "" {
			imageName = cfg.devConfig.Image
		}

		ctx := &devcontainer.SubstituteContext{
			LocalWorkspaceFolder:     cfg.worker.CWD,
			ContainerWorkspaceFolder: cfg.worker.CWD,
			LocalEnv:                 make(map[string]string),
			ContainerEnv:             make(map[string]string),
		}

		for _, envVar := range os.Environ() {
			kv := strings.SplitN(envVar, "=", 2)
			if len(kv) == 2 {
				ctx.LocalEnv[kv[0]] = kv[1]
			}
		}

		resolvedEnv := cfg.devConfig.GetResolvedEnvironment(ctx)
		for k, v := range resolvedEnv {
			sb.WriteString(fmt.Sprintf("\t-e %s=\"%s\" \\\n", k, v))
		}
	}

	sb.WriteString(fmt.Sprintf("\t%s \\\n", imageName))
	sb.WriteString("\tbash /opt/worker/start.sh build\n")

	return sb.String()
}

func createZellijLayout(workerName, wrapperScript, shellExecScript, containerName string) string {
	return fmt.Sprintf(`layout {
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
`, workerName, wrapperScript, shellExecScript, containerName)
}
