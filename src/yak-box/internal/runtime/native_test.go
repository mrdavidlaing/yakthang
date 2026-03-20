package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

func TestFileExists_Exists(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "exists")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(f) {
		t.Error("fileExists should be true for existing file")
	}
}

func TestFileExists_NotExists(t *testing.T) {
	if fileExists("/nonexistent/path/12345") {
		t.Error("fileExists should be false for non-existent path")
	}
}

func TestFileExists_Dir(t *testing.T) {
	tmp := t.TempDir()
	if !fileExists(tmp) {
		t.Error("fileExists is true for existing directory (os.Stat returns nil)")
	}
}

func TestGenerateNativeWrapperScript_Opencode(t *testing.T) {
	worker := &types.Worker{
		Tool:    "opencode",
		YakPath: "/test/yaks",
		CWD:     "/test/cwd",
	}
	content, paneName := generateNativeWrapperScript(worker, "/home/worker", "/host/home", "/prompt.txt", "/worker.pid", "")
	if paneName != "opencode (build) [native]" {
		t.Errorf("unexpected paneName: %q", paneName)
	}
	if !strings.Contains(content, "opencode") {
		t.Errorf("opencode wrapper must invoke opencode, got:\n%s", content)
	}
	if !strings.Contains(content, "--prompt") {
		t.Errorf("opencode wrapper must pass --prompt, got:\n%s", content)
	}
	if !strings.Contains(content, "/worker.pid") {
		t.Errorf("opencode wrapper must write PID to pidFile, got:\n%s", content)
	}
}

func TestGenerateNativeWrapperScript_UnknownToolDefaultsToOpencode(t *testing.T) {
	worker := &types.Worker{
		Tool:    "unknown-tool",
		YakPath: "/test/yaks",
		CWD:     "/test/cwd",
	}
	content, paneName := generateNativeWrapperScript(worker, "/home/worker", "", "/p.txt", "/pid", "")
	if paneName != "opencode (build) [native]" {
		t.Errorf("unknown tool should default to opencode pane name, got %q", paneName)
	}
	if !strings.Contains(content, "opencode") {
		t.Errorf("unknown tool should default to opencode script, got:\n%s", content)
	}
}

func TestKillNativeProcessTree_MissingPidFile(t *testing.T) {
	err := KillNativeProcessTree("/nonexistent/pid/file", time.Second)
	if err == nil {
		t.Error("expected error when pid file is missing")
	}
	if !strings.Contains(err.Error(), "failed to read pid file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKillNativeProcessTree_InvalidPidContent(t *testing.T) {
	tmp := t.TempDir()
	pidFile := filepath.Join(tmp, "pid")
	if err := os.WriteFile(pidFile, []byte("not-a-number"), 0644); err != nil {
		t.Fatal(err)
	}
	err := KillNativeProcessTree(pidFile, time.Second)
	if err == nil {
		t.Error("expected error when pid file contains non-numeric content")
	}
	if !strings.Contains(err.Error(), "invalid pid") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKillNativeProcessTree_ProcessNotExist(t *testing.T) {
	// PID that does not exist on the system; Signal(0) will fail and we remove pid file and return nil.
	tmp := t.TempDir()
	pidFile := filepath.Join(tmp, "pid")
	const nonexistentPID = 999999999
	if err := os.WriteFile(pidFile, []byte("999999999"), 0644); err != nil {
		t.Fatal(err)
	}
	err := KillNativeProcessTree(pidFile, 100*time.Millisecond)
	if err != nil {
		t.Errorf("KillNativeProcessTree for non-existent process should remove file and return nil: %v", err)
	}
	if _, statErr := os.Stat(pidFile); statErr == nil {
		t.Error("pid file should have been removed when process does not exist")
	}
}

func TestSetupClaudeSettings_NoGocccSkipsStatusline(t *testing.T) {
	homeDir := t.TempDir()
	// When goccc is not in PATH, setupClaudeSettings should still create .claude dirs and remote-settings
	// but may skip settings.json with statusline. We already have TestSetupClaudeSettings_PreseededClaudeJSON
	// in helpers_test. This test just ensures setupClaudeSettings with empty apiKey doesn't panic.
	if err := setupClaudeSettings(homeDir, "", ""); err != nil {
		t.Fatalf("setupClaudeSettings with empty key: %v", err)
	}
}

func TestSetupClaudeSettings_MergesHostHooks(t *testing.T) {
	workerHome := t.TempDir()
	hostHome := t.TempDir()

	hostClaudeDir := filepath.Join(hostHome, ".claude")
	if err := os.MkdirAll(hostClaudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	hostSettings := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "bash ~/.claude/hooks/block-git.sh",
            "statusMessage": "Checking for bare git commands..."
          }
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(hostClaudeDir, "settings.json"), []byte(hostSettings), 0644); err != nil {
		t.Fatal(err)
	}

	if err := setupClaudeSettings(workerHome, hostHome, ""); err != nil {
		t.Fatalf("setupClaudeSettings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workerHome, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading worker settings.json: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "PreToolUse") {
		t.Error("worker settings.json should contain PreToolUse hook")
	}
	// ~ should be rewritten to the absolute host home path
	if strings.Contains(content, "~/.claude") {
		t.Error("worker settings.json should not contain ~/  — tilde should be rewritten")
	}
	expectedCmd := "bash " + hostHome + "/.claude/hooks/block-git.sh"
	if !strings.Contains(content, expectedCmd) {
		t.Errorf("worker settings.json should contain rewritten path %q, got:\n%s", expectedCmd, content)
	}
}

func TestSetupClaudeSettings_NoHostHooksWhenHostSettingsMissing(t *testing.T) {
	workerHome := t.TempDir()
	hostHome := t.TempDir() // no .claude/settings.json written

	if err := setupClaudeSettings(workerHome, hostHome, ""); err != nil {
		t.Fatalf("setupClaudeSettings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(workerHome, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading worker settings.json: %v", err)
	}
	if strings.Contains(string(data), "hooks") {
		t.Errorf("worker settings.json should not contain hooks when host settings.json is absent, got:\n%s", data)
	}
}

func TestRewriteHookPaths_ReplacesLeadingTilde(t *testing.T) {
	hooks := map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Bash",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "bash ~/.claude/hooks/block-git.sh",
					},
				},
			},
		},
	}
	result := rewriteHookPaths(hooks, "/Users/testuser")
	outer, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	items := outer["PreToolUse"].([]any)
	inner := items[0].(map[string]any)
	hookList := inner["hooks"].([]any)
	hookEntry := hookList[0].(map[string]any)
	cmd := hookEntry["command"].(string)
	if cmd != "bash /Users/testuser/.claude/hooks/block-git.sh" {
		t.Errorf("expected rewritten path, got %q", cmd)
	}
}

func TestRewriteHookPaths_LeavesAbsolutePathsAlone(t *testing.T) {
	hooks := map[string]any{
		"command": "/absolute/path/hook.sh",
	}
	result := rewriteHookPaths(hooks, "/Users/testuser")
	m := result.(map[string]any)
	if m["command"] != "/absolute/path/hook.sh" {
		t.Errorf("absolute path should not be modified, got %q", m["command"])
	}
}
