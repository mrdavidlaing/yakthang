package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wellmaintained/yakthang/src/yak-box/pkg/devcontainer"
)

func generateInitScript() string {
	return `#!/usr/bin/env bash
WORKSPACE_ROOT="${WORKSPACE_ROOT:-${YAK_WORKSPACE:-/home/yakob/yak-box}}"
COST_DIR="${WORKSPACE_ROOT}/.worker-costs"
mkdir -p "$COST_DIR"

PROMPT_FILE="/opt/worker/prompt.txt"
PROMPT="$(cat "$PROMPT_FILE")"
TOOL="${YAK_TOOL:-opencode}"
MODEL="${YAK_MODEL:-}"
AGENT_NAME="${YAK_AGENT_NAME:-}"
WORKSPACE="${YAK_WORKSPACE:-$PWD}"

case "$TOOL" in
  claude)
    unset CLAUDECODE
    CLAUDE_ARGS=(--dangerously-skip-permissions)
    if [[ -n "$AGENT_NAME" ]]; then
      CLAUDE_ARGS=(--agent "$AGENT_NAME" "${CLAUDE_ARGS[@]}")
    fi
    if [[ -n "$MODEL" ]]; then
      CLAUDE_ARGS+=(--model "$MODEL")
    fi
    claude "${CLAUDE_ARGS[@]}" @"$PROMPT_FILE"
    ;;
  cursor)
    if [[ -n "$MODEL" ]]; then
      agent --force --model "$MODEL" --workspace "$WORKSPACE" "$PROMPT"
    else
      agent --force --workspace "$WORKSPACE" "$PROMPT"
    fi
    ;;
  *)
    opencode --prompt "$PROMPT" --agent "$1"
    ;;
esac
EXIT_CODE=$?

if [[ "$TOOL" == "opencode" ]]; then
  WORKER="${WORKER_NAME:-unknown}"
  TS="$(date -u +%Y%m%dT%H%M%SZ)"
  SID="$(opencode session list 2>/dev/null | tail -1 | awk '{print $1}')"
  if [[ -n "$SID" && "$SID" != "Session" ]]; then
    opencode export "$SID" > "${COST_DIR}/${WORKER}-${TS}.json" 2>/dev/null
  fi
  opencode stats --models > "${COST_DIR}/${WORKER}-${TS}.stats.txt" 2>/dev/null
fi

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

	// Standard mounts: workspace rw so the shaver can edit code; .yaks ro so the shaver
	// cannot clobber workspace task state (yx resolves .yaks by walking up from cwd and
	// would write to this same directory; overlay mount makes it read-only in container).
	sb.WriteString(fmt.Sprintf("\t-v \"%s:%s:rw\" \\\n", workspaceRoot, workspaceRoot))
	if cfg.worker.YakPath != "" {
		sb.WriteString(fmt.Sprintf("\t-v \"%s:%s:ro\" \\\n", cfg.worker.YakPath, cfg.worker.YakPath))
		// Override with :rw for each assigned yak directory so the shaver can write agent-status, done, etc.
		for _, dir := range cfg.worker.YakRwDirs {
			if dir != "" {
				sb.WriteString(fmt.Sprintf("\t-v \"%s:%s:rw\" \\\n", dir, dir))
			}
		}
	}
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
	hostGitConfigPath := filepath.Join(homeDirHost, ".gitconfig")
	if _, err := os.Stat(hostGitConfigPath); err == nil {
		sb.WriteString(fmt.Sprintf("\t-v \"%s:/home/yak-shaver/.host-gitconfig:ro\" \\\n", hostGitConfigPath))
	}
	hostGHConfigDir := filepath.Join(homeDirHost, ".config", "gh")
	if info, err := os.Stat(hostGHConfigDir); err == nil && info.IsDir() {
		sb.WriteString(fmt.Sprintf("\t-v \"%s:/home/yak-shaver/.host-gh-config:ro\" \\\n", hostGHConfigDir))
	}
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
	if _, err := os.Stat(hostGitConfigPath); err == nil {
		sb.WriteString("\t-e GIT_CONFIG_GLOBAL=/home/yak-shaver/.host-gitconfig \\\n")
	}
	if info, err := os.Stat(hostGHConfigDir); err == nil && info.IsDir() {
		sb.WriteString("\t-e GH_CONFIG_DIR=/home/yak-shaver/.host-gh-config \\\n")
	}
	sb.WriteString("\t-e TERM=xterm-256color \\\n")
	sb.WriteString("\t-e IS_DEMO=true \\\n") // Bypass Claude Code interactive onboarding wizard
	sb.WriteString("\t-e GOPATH=/home/yak-shaver/.go \\\n")
	sb.WriteString("\t-e CARGO_HOME=/home/yak-shaver/.cargo \\\n")
	sb.WriteString("\t-e RUSTUP_HOME=/home/yak-shaver/.rustup \\\n")

	if cfg.profile.Name == "ram" {
		sb.WriteString("\t-e CARGO_BUILD_JOBS=4 \\\n")
	}

	sb.WriteString(fmt.Sprintf("\t-e WORKER_NAME=\"%s\" \\\n", cfg.worker.WorkerName))
	sb.WriteString(fmt.Sprintf("\t-e YAK_PATH=\"%s\" \\\n", cfg.worker.YakPath))
	sb.WriteString(fmt.Sprintf("\t-e YAK_TOOL=\"%s\" \\\n", cfg.worker.Tool))
	sb.WriteString(fmt.Sprintf("\t-e YAK_WORKSPACE=\"%s\" \\\n", cfg.worker.CWD))

	if cfg.worker.ShaverName != "" {
		sb.WriteString(fmt.Sprintf("\t-e YAK_SHAVER_NAME=\"%s\" \\\n", cfg.worker.ShaverName))
	}

	if cfg.worker.Model != "" {
		sb.WriteString(fmt.Sprintf("\t-e YAK_MODEL=\"%s\" \\\n", cfg.worker.Model))
	}

	// API keys that will be explicitly set below — skip these from devcontainer env
	// to ensure only one -e entry per key in the generated script.
	apiKeyNames := map[string]bool{
		"ANTHROPIC_API_KEY": true,
		"_ANTHROPIC_API_KEY": true,
		"OPENAI_API_KEY":    true,
		"OPENCODE_API_KEY":  true,
		"CURSOR_API_KEY":    true,
	}

	// Devcontainer envs (API keys excluded; they are written after with correct values)
	imageName := "yak-worker:latest"
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
			if !apiKeyNames[k] {
				sb.WriteString(fmt.Sprintf("\t-e %s=\"%s\" \\\n", k, v))
			}
		}
	}

	// Direct API key passthrough from host env, written last so they override any
	// empty devcontainer remoteEnv substitutions (e.g. ${localEnv:ANTHROPIC_API_KEY}).
	anthropicKey := resolveAnthropicKey()
	if anthropicKey != "" {
		sb.WriteString(fmt.Sprintf("\t-e _ANTHROPIC_API_KEY=\"%s\" \\\n", anthropicKey))
	}
	for _, key := range []string{"OPENAI_API_KEY", "OPENCODE_API_KEY", "CURSOR_API_KEY"} {
		if val := os.Getenv(key); val != "" {
			sb.WriteString(fmt.Sprintf("\t-e %s=\"%s\" \\\n", key, val))
		}
	}

	sb.WriteString(fmt.Sprintf("\t%s \\\n", imageName))
	sb.WriteString("\tbash /opt/worker/start.sh build\n")

	return sb.String()
}

// resolveAnthropicKey returns the Anthropic API key from the environment.
// We intentionally do not shell out to macOS Keychain here because native
// workers override HOME for Claude skills/config isolation, and keychain
// fallback can trigger modal auth/keychain errors during worker spawn.
func resolveAnthropicKey() string {
	if key := os.Getenv("_ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	return ""
}

// resolveAnthropicKeySuffix returns the last 20 characters of the Anthropic API key,
// which is how Claude Code identifies approved custom API keys in .claude.json.
func resolveAnthropicKeySuffix() string {
	key := resolveAnthropicKey()
	if len(key) >= 20 {
		return key[len(key)-20:]
	}
	return key
}

// buildClaudeJSONContent returns the JSON content for $HOME/.claude.json.
// If apiKeySuffix is non-empty, the key is pre-approved so Claude Code does
// not prompt for confirmation when the same API key is used.
func buildClaudeJSONContent(apiKeySuffix string) string {
	if apiKeySuffix != "" {
		return fmt.Sprintf(
			`{"numStartups":1,"hasCompletedOnboarding":true,"theme":"dark","bypassPermissionsModeAccepted":true,"customApiKeyResponses":{"approved":["%s"],"rejected":[]}}`,
			apiKeySuffix,
		)
	}
	return `{"numStartups":1,"hasCompletedOnboarding":true,"theme":"dark","bypassPermissionsModeAccepted":true}`
}
