package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wellmaintained/yakthang/src/yak-box/internal/zellij"
	"github.com/wellmaintained/yakthang/src/yak-box/pkg/devcontainer"
	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

func TestGenerateInitScript(t *testing.T) {
	script := generateInitScript()
	if !strings.Contains(script, "WORKSPACE_ROOT=") {
		t.Error("Init script missing WORKSPACE_ROOT")
	}
	if !strings.Contains(script, "opencode --prompt") {
		t.Error("Init script missing opencode command")
	}
}

func TestGenerateInitScript_ClaudeNoprint(t *testing.T) {
	script := generateInitScript()
	if strings.Contains(script, "claude --print") {
		t.Error("Init script must not use claude --print; workers should be interactive")
	}
	if !strings.Contains(script, "claude ") {
		t.Error("Init script missing claude invocation")
	}
}

func TestGenerateWaitScript(t *testing.T) {
	script := generateWaitScript()
	if !strings.Contains(script, "CONTAINER_NAME=\"$1\"") {
		t.Error("Wait script missing CONTAINER_NAME")
	}
	if !strings.Contains(script, "docker inspect") {
		t.Error("Wait script missing docker inspect")
	}
}

func TestGenerateRunScript(t *testing.T) {
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:        "test-worker",
			CWD:         "/test/cwd",
			YakPath:     "/test/yak",
			DisplayName: "Test Worker",
			WorkerName:  "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
	}

	workspaceRoot := "/test/workspace"
	promptFile := "/test/prompt.txt"
	innerScript := "/test/inner.sh"
	passwdFile := "/test/passwd"
	groupFile := "/test/group"
	networkMode := "test-net"

	script := generateRunScript(cfg, workspaceRoot, promptFile, innerScript, passwdFile, groupFile, networkMode)

	expected := []string{
		"exec docker run",
		"--name yak-worker-test-worker",
		"--network test-net",
		"--cpus 1.0",
		"--memory 2g",
		"-v \"/test/workspace:/test/workspace:rw\"",
		"-v \"/test/yak:/test/yak:ro\"", // .yaks read-only so shaver cannot clobber workspace task state
		"-w \"/test/cwd\"",
		`WORKER_NAME="TestWorker"`,
	}

	for _, exp := range expected {
		if !strings.Contains(script, exp) {
			t.Errorf("Run script missing expected string: %s", exp)
		}
	}
}

func TestGenerateRunScript_YakRwDirsAfterRo(t *testing.T) {
	roMount := "-v \"/ws/.yaks:/ws/.yaks:ro\""
	rwMount := "-v \"/ws/.yaks/yakthang-improvements/shaver-clobbered-yaks-directory-improvements:/ws/.yaks/yakthang-improvements/shaver-clobbered-yaks-directory-improvements:rw\""
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:        "test-worker",
			CWD:         "/test/cwd",
			YakPath:     "/ws/.yaks",
			YakRwDirs:   []string{"/ws/.yaks/yakthang-improvements/shaver-clobbered-yaks-directory-improvements"},
			DisplayName: "Test Worker",
			WorkerName:  "TestWorker",
		},
		profile: types.ResourceProfile{Name: "default", CPUs: "1.0", Memory: "2g", PIDs: 512},
	}
	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")
	roIdx := strings.Index(script, roMount)
	rwIdx := strings.Index(script, rwMount)
	if roIdx == -1 {
		t.Error("Run script must contain .yaks mount as :ro")
	}
	if rwIdx == -1 {
		t.Errorf("Run script must contain rw override for assigned yak dir, got:\n%s", script)
	}
	// Docker uses last-mount-wins: :rw must appear after :ro so the override takes effect.
	if roIdx != -1 && rwIdx != -1 && rwIdx <= roIdx {
		t.Errorf("Run script must emit :rw mount after :ro (ro at %d, rw at %d)", roIdx, rwIdx)
	}
}

func TestGenerateRunScript_MultipleYakRwDirs(t *testing.T) {
	roMount := "-v \"/ws/.yaks:/ws/.yaks:ro\""
	rw1 := "-v \"/ws/.yaks/team-a/task-one:/ws/.yaks/team-a/task-one:rw\""
	rw2 := "-v \"/ws/.yaks/team-b/task-two:/ws/.yaks/team-b/task-two:rw\""
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:        "test-worker",
			CWD:         "/test/cwd",
			YakPath:     "/ws/.yaks",
			YakRwDirs:   []string{"/ws/.yaks/team-a/task-one", "/ws/.yaks/team-b/task-two"},
			DisplayName: "Test Worker",
			WorkerName:  "TestWorker",
		},
		profile: types.ResourceProfile{Name: "default", CPUs: "1.0", Memory: "2g", PIDs: 512},
	}
	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")
	roIdx := strings.Index(script, roMount)
	idx1 := strings.Index(script, rw1)
	idx2 := strings.Index(script, rw2)
	if roIdx == -1 {
		t.Error("Run script must contain .yaks mount as :ro")
	}
	if idx1 == -1 {
		t.Errorf("Run script must contain first rw override, got:\n%s", script)
	}
	if idx2 == -1 {
		t.Errorf("Run script must contain second rw override, got:\n%s", script)
	}
	// Both :rw mounts must appear after :ro (Docker last-mount-wins).
	if roIdx != -1 && idx1 != -1 && idx1 <= roIdx {
		t.Errorf("First :rw must appear after :ro (ro at %d, rw1 at %d)", roIdx, idx1)
	}
	if roIdx != -1 && idx2 != -1 && idx2 <= roIdx {
		t.Errorf("Second :rw must appear after :ro (ro at %d, rw2 at %d)", roIdx, idx2)
	}
}

func TestGenerateRunScript_EmptyYakPathNoYaksMount(t *testing.T) {
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:        "test-worker",
			CWD:         "/test/cwd",
			YakPath:     "", // empty: no .yaks ro mount
			DisplayName: "Test Worker",
			WorkerName:  "TestWorker",
		},
		profile: types.ResourceProfile{Name: "default", CPUs: "1.0", Memory: "2g", PIDs: 512},
	}
	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")
	// Must not add a mount with empty path (would be -v "":"":ro)
	if strings.Contains(script, "-v \"\"") {
		t.Error("Run script must not add mount with empty path when YakPath is empty")
	}
	// Workspace mount still present
	if !strings.Contains(script, "-v \"/ws:/ws:rw\"") {
		t.Error("Run script must still contain workspace mount when YakPath is empty")
	}
}

func TestGenerateRunScript_WithDevConfig(t *testing.T) {
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:        "test-worker",
			CWD:         "/test/cwd",
			YakPath:     "/test/yak",
			DisplayName: "Test Worker",
			WorkerName:  "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
		devConfig: &devcontainer.Config{
			Image: "custom-image:latest",
			Mounts: []string{
				"source=/foo,target=/bar,type=bind",
			},
			RemoteEnv: map[string]string{
				"CUSTOM_ENV": "value",
			},
		},
	}

	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")

	expected := []string{
		"custom-image:latest",
		"-v \"source=/foo,target=/bar,type=bind\"",
		"-e CUSTOM_ENV=\"value\"",
	}

	for _, exp := range expected {
		if !strings.Contains(script, exp) {
			t.Errorf("Run script missing expected string: %s", exp)
		}
	}
}

func TestGenerateRunScript_AnthropicKeyFromEnv(t *testing.T) {
	t.Setenv("_ANTHROPIC_API_KEY", "test-api-key-from-env")
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:       "test-worker",
			CWD:        "/test/cwd",
			YakPath:    "/test/yak",
			WorkerName: "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
	}

	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")

	if !strings.Contains(script, `-e _ANTHROPIC_API_KEY="test-api-key-from-env"`) {
		t.Errorf("Run script missing _ANTHROPIC_API_KEY from env, script:\n%s", script)
	}
	if strings.Contains(script, `-e ANTHROPIC_API_KEY=`) {
		t.Errorf("Run script must not set ANTHROPIC_API_KEY, script:\n%s", script)
	}
}

func TestGenerateRunScript_AnthropicKeyAbsentWhenEmpty(t *testing.T) {
	t.Setenv("_ANTHROPIC_API_KEY", "")
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:       "test-worker",
			CWD:        "/test/cwd",
			YakPath:    "/test/yak",
			WorkerName: "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
	}

	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")

	// Key should be absent when the env var is empty, but script must never
	// contain an explicit empty assignment.
	if strings.Contains(script, `-e _ANTHROPIC_API_KEY=""`) {
		t.Error("Run script must not set _ANTHROPIC_API_KEY to empty string")
	}
	if strings.Contains(script, `-e ANTHROPIC_API_KEY=`) {
		t.Error("Run script must not set ANTHROPIC_API_KEY")
	}
}

func TestResolveAnthropicKey_DoesNotCallSecurityWhenEnvMissing(t *testing.T) {
	t.Setenv("_ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	tempDir := t.TempDir()
	markerFile := filepath.Join(tempDir, "security-called")
	securityPath := filepath.Join(tempDir, "security")
	script := fmt.Sprintf("#!/usr/bin/env bash\ntouch %q\nexit 0\n", markerFile)
	if err := os.WriteFile(securityPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake security binary: %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	key := resolveAnthropicKey()
	if key != "" {
		t.Fatalf("expected empty key when _ANTHROPIC_API_KEY is unset, got %q", key)
	}
	if _, err := os.Stat(markerFile); err == nil {
		t.Fatal("resolveAnthropicKey should not invoke security binary")
	}
}

func TestGenerateRunScript_OpencodeApiKey(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "test-opencode-key")
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:       "test-worker",
			CWD:        "/test/cwd",
			YakPath:    "/test/yak",
			WorkerName: "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
	}

	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")

	if !strings.Contains(script, `-e OPENCODE_API_KEY="test-opencode-key"`) {
		t.Errorf("Run script missing OPENCODE_API_KEY from env, script:\n%s", script)
	}
}

// TestGenerateRunScript_DevcontainerApiKeyNotClobbered verifies that a
// devcontainer remoteEnv entry for ANTHROPIC_API_KEY (which may resolve to
// empty string via ${localEnv:ANTHROPIC_API_KEY}) does not clobber the
// host/Keychain-resolved key.
func TestGenerateRunScript_DevcontainerApiKeyNotClobbered(t *testing.T) {
	t.Setenv("_ANTHROPIC_API_KEY", "host-resolved-key")
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:       "test-worker",
			CWD:        "/test/cwd",
			YakPath:    "/test/yak",
			WorkerName: "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
		devConfig: &devcontainer.Config{
			RemoteEnv: map[string]string{
				// Simulates ${localEnv:ANTHROPIC_API_KEY} resolving to empty
				"ANTHROPIC_API_KEY": "",
			},
		},
	}

	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")

	// Must contain exactly one underscore key entry with the correct value.
	count := strings.Count(script, `-e _ANTHROPIC_API_KEY=`)
	if count != 1 {
		t.Errorf("Expected exactly 1 _ANTHROPIC_API_KEY entry, got %d, script:\n%s", count, script)
	}
	if !strings.Contains(script, `-e _ANTHROPIC_API_KEY="host-resolved-key"`) {
		t.Errorf("Run script missing correct _ANTHROPIC_API_KEY value, script:\n%s", script)
	}
	if strings.Contains(script, `-e ANTHROPIC_API_KEY=`) {
		t.Errorf("Run script must not include ANTHROPIC_API_KEY from devcontainer or host env, script:\n%s", script)
	}
}

func TestGenerateRunScript_HostGitAndGhConfigMountedAndExported(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	gitConfigPath := filepath.Join(hostHome, ".gitconfig")
	if err := os.WriteFile(gitConfigPath, []byte("[user]\n\tname = Test\n"), 0644); err != nil {
		t.Fatalf("failed to write test .gitconfig: %v", err)
	}

	ghConfigDir := filepath.Join(hostHome, ".config", "gh")
	if err := os.MkdirAll(ghConfigDir, 0755); err != nil {
		t.Fatalf("failed to create test gh config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ghConfigDir, "hosts.yml"), []byte("github.com:\n"), 0644); err != nil {
		t.Fatalf("failed to write test gh hosts.yml: %v", err)
	}

	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:       "test-worker",
			CWD:        "/test/cwd",
			YakPath:    "/test/yak",
			WorkerName: "TestWorker",
		},
		profile: types.ResourceProfile{
			Name:   "default",
			CPUs:   "1.0",
			Memory: "2g",
			PIDs:   512,
		},
	}

	script := generateRunScript(cfg, "/ws", "/p", "/i", "/pw", "/g", "net")

	if !strings.Contains(script, fmt.Sprintf("-v \"%s:/home/yak-shaver/.host-gitconfig:ro\"", gitConfigPath)) {
		t.Errorf("Run script missing host gitconfig mount, script:\n%s", script)
	}
	if !strings.Contains(script, fmt.Sprintf("-v \"%s:/home/yak-shaver/.host-gh-config:ro\"", ghConfigDir)) {
		t.Errorf("Run script missing host gh config dir mount, script:\n%s", script)
	}
	if !strings.Contains(script, "-e GIT_CONFIG_GLOBAL=/home/yak-shaver/.host-gitconfig") {
		t.Errorf("Run script missing GIT_CONFIG_GLOBAL export, script:\n%s", script)
	}
	if !strings.Contains(script, "-e GH_CONFIG_DIR=/home/yak-shaver/.host-gh-config") {
		t.Errorf("Run script missing GH_CONFIG_DIR export, script:\n%s", script)
	}
}

func TestCreateZellijLayout(t *testing.T) {
	worker := &types.Worker{DisplayName: "Test Worker", CWD: "/workspace"}
	layout := zellij.GenerateLayout(worker, "sandboxed", "claude")

	expected := []string{
		`tab name="Test Worker"`,
		"claude", "build", "sandbox",
		"WRAPPER",
		"SHELL_EXEC_SCRIPT",
		"CONTAINER_NAME",
	}
	for _, exp := range expected {
		if !strings.Contains(layout, exp) {
			t.Errorf("Layout missing expected string: %s", exp)
		}
	}
}

func TestGenerateNativeWrapperScript_ClaudeHomeSetsWorkerHome(t *testing.T) {
	worker := &types.Worker{
		Tool:    "claude",
		YakPath: "/test/yaks",
		Model:   "default",
		CWD:     "/test/cwd",
	}
	homeDir := "/test/worker-home"
	content, paneName := generateNativeWrapperScript(worker, homeDir, "/host/home", "/test/prompt.txt", "/test/worker.pid", "")

	if paneName != "claude (build) [native]" {
		t.Errorf("unexpected paneName: %q", paneName)
	}
	// HOME must be the worker's homeDir, not a hardcoded path
	if !strings.Contains(content, `export HOME="/test/worker-home"`) {
		t.Errorf("native claude wrapper must set HOME to homeDir, got:\n%s", content)
	}
	if !strings.Contains(content, `export GIT_CONFIG_GLOBAL="/host/home/.gitconfig"`) {
		t.Errorf("native claude wrapper must preserve host git config path, got:\n%s", content)
	}
	if !strings.Contains(content, `export GH_CONFIG_DIR="/host/home/.config/gh"`) {
		t.Errorf("native claude wrapper must preserve host gh config path, got:\n%s", content)
	}
	if !strings.Contains(content, `export YAK_PATH="/test/yaks"`) {
		t.Errorf("native claude wrapper missing YAK_PATH, got:\n%s", content)
	}
	if !strings.Contains(content, `export YX_ROOT="/test"`) {
		t.Errorf("native claude wrapper missing YX_ROOT (workspace root), got:\n%s", content)
	}
	if !strings.Contains(content, "--dangerously-skip-permissions") {
		t.Errorf("native claude wrapper missing --dangerously-skip-permissions, got:\n%s", content)
	}
}

func TestGenerateNativeWrapperScript_ClaudeAnthropicKeyIncluded(t *testing.T) {
	worker := &types.Worker{
		Tool:    "claude",
		YakPath: "/test/yaks",
		Model:   "",
		CWD:     "/test/cwd",
	}
	content, _ := generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "test-key-abc")
	if !strings.Contains(content, `export _ANTHROPIC_API_KEY="test-key-abc"`) {
		t.Errorf("native claude wrapper must include _ANTHROPIC_API_KEY, got:\n%s", content)
	}
	if strings.Contains(content, `export ANTHROPIC_API_KEY=`) {
		t.Errorf("native claude wrapper must not include ANTHROPIC_API_KEY, got:\n%s", content)
	}
}

func TestGenerateNativeWrapperScript_ClaudeNoAnthropicKeyWhenEmpty(t *testing.T) {
	worker := &types.Worker{
		Tool:    "claude",
		YakPath: "/test/yaks",
		Model:   "",
		CWD:     "/test/cwd",
	}
	content, _ := generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")
	if strings.Contains(content, `ANTHROPIC_API_KEY`) {
		t.Errorf("native claude wrapper must not include ANTHROPIC_API_KEY when empty, got:\n%s", content)
	}
}

func TestGenerateNativeWrapperScript_CursorNoHomeOverride(t *testing.T) {
	worker := &types.Worker{
		Tool:    "cursor",
		YakPath: "/test/yaks",
		Model:   "",
		CWD:     "/test/cwd",
	}
	content, paneName := generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")
	if paneName != "cursor (build) [native]" {
		t.Errorf("unexpected paneName: %q", paneName)
	}
	// cursor does not need HOME override
	if strings.Contains(content, "export HOME=") {
		t.Errorf("cursor wrapper should not override HOME, got:\n%s", content)
	}
}

func TestGenerateNativeWrapperScript_ClaudeKeychainSetup(t *testing.T) {
	worker := &types.Worker{
		Tool:    "claude",
		YakPath: "/test/yaks",
		CWD:     "/test/cwd",
	}
	content, _ := generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")

	for _, expected := range []string{
		"worker.keychain-db",
		"security create-keychain",
		"security unlock-keychain",
		"security set-default-keychain",
		"_restore_keychain",
		"trap _restore_keychain EXIT",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("native claude wrapper missing keychain setup %q, got:\n%s", expected, content)
		}
	}

	// Keychain setup must appear BEFORE the claude invocation.
	keychainIdx := strings.Index(content, "security create-keychain")
	claudeIdx := strings.Index(content, "\nclaude ")
	if keychainIdx == -1 || claudeIdx == -1 || keychainIdx > claudeIdx {
		t.Errorf("keychain setup must appear before claude invocation (keychain at %d, claude at %d)", keychainIdx, claudeIdx)
	}

	// cursor and opencode wrappers must NOT contain keychain setup.
	for _, tool := range []string{"cursor", "opencode"} {
		worker.Tool = tool
		content, _ = generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")
		if strings.Contains(content, "keychain") {
			t.Errorf("%s wrapper must not contain keychain setup, got:\n%s", tool, content)
		}
	}
}

func TestGenerateNativeWrapperScript_ShaverNameEnvVar(t *testing.T) {
	worker := &types.Worker{
		Tool:       "claude",
		YakPath:    "/test/yaks",
		CWD:        "/test/cwd",
		ShaverName: "Yakoff",
	}
	content, _ := generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")
	if !strings.Contains(content, `export YAK_SHAVER_NAME="Yakoff"`) {
		t.Errorf("native wrapper must set YAK_SHAVER_NAME when worker.ShaverName is set, got:\n%s", content)
	}

	worker.ShaverName = ""
	content, _ = generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")
	if strings.Contains(content, "YAK_SHAVER_NAME") {
		t.Errorf("native wrapper must not set YAK_SHAVER_NAME when worker.ShaverName is empty, got:\n%s", content)
	}
}

func TestSetupClaudeSettings_PreseededClaudeJSON(t *testing.T) {
	homeDir := t.TempDir()
	apiKey := "sk-ant-test-xxxxxxxxxxxxxxxxxxxx"

	if err := setupClaudeSettings(homeDir, apiKey); err != nil {
		t.Fatalf("setupClaudeSettings returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(homeDir, ".claude.json"))
	if err != nil {
		t.Fatalf(".claude.json not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"hasCompletedOnboarding":true`) {
		t.Errorf(".claude.json missing hasCompletedOnboarding:true, got:\n%s", content)
	}
	if !strings.Contains(content, `"bypassPermissionsModeAccepted":true`) {
		t.Errorf(".claude.json missing bypassPermissionsModeAccepted:true, got:\n%s", content)
	}
	if !strings.Contains(content, `"customApiKeyResponses"`) {
		t.Errorf(".claude.json missing customApiKeyResponses, got:\n%s", content)
	}

	remoteSettingsPath := filepath.Join(homeDir, ".claude", "remote-settings.json")
	remoteSettings, err := os.ReadFile(remoteSettingsPath)
	if err != nil {
		t.Fatalf("remote-settings.json not created: %v", err)
	}
	if strings.TrimSpace(string(remoteSettings)) != "{}" {
		t.Errorf("remote-settings.json unexpected content: %q", string(remoteSettings))
	}

	apiKeyHelperPath := filepath.Join(homeDir, ".claude", "api-key-helper.sh")
	apiKeyHelper, err := os.ReadFile(apiKeyHelperPath)
	if err != nil {
		t.Fatalf("api-key-helper.sh not created: %v", err)
	}
	if !strings.Contains(string(apiKeyHelper), `echo "${_ANTHROPIC_API_KEY}"`) {
		t.Errorf("api-key-helper.sh has unexpected content: %q", string(apiKeyHelper))
	}

	settingsData, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	settings := string(settingsData)
	if !strings.Contains(settings, `"apiKeyHelper":`) {
		t.Errorf("settings.json missing apiKeyHelper, got:\n%s", settings)
	}
	if !strings.Contains(settings, "api-key-helper.sh") {
		t.Errorf("settings.json missing api-key-helper.sh path, got:\n%s", settings)
	}

	debugDirPath := filepath.Join(homeDir, ".claude", "debug")
	if info, err := os.Stat(debugDirPath); err != nil || !info.IsDir() {
		t.Fatalf("debug directory not created at %s", debugDirPath)
	}
}
