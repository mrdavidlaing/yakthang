package preflight_test

import (
	"bytes"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/wellmaintained/yakthang/src/yak-box/internal/preflight"
)

func TestCheck_RequiredMissing(t *testing.T) {
	deps := []preflight.Dep{
		{Name: "this-binary-does-not-exist-99999", Required: true, Hint: "install it somehow"},
	}
	result := preflight.Check(deps)
	if len(result.Missing) != 1 {
		t.Fatalf("expected 1 missing dep, got %d", len(result.Missing))
	}
	if result.Missing[0].Name != "this-binary-does-not-exist-99999" {
		t.Errorf("unexpected missing dep name: %s", result.Missing[0].Name)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings for required dep, got %v", result.Warnings)
	}
}

func TestCheck_OptionalMissing(t *testing.T) {
	deps := []preflight.Dep{
		{Name: "this-binary-does-not-exist-99999", Required: false, Hint: "cost tracking will be disabled"},
	}
	result := preflight.Check(deps)
	if len(result.Missing) != 0 {
		t.Errorf("optional dep should not appear in Missing, got %v", result.Missing)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning for optional dep, got %d", len(result.Warnings))
	}
	if !strings.Contains(result.Warnings[0], "cost tracking will be disabled") {
		t.Errorf("warning missing hint text: %s", result.Warnings[0])
	}
}

func TestCheck_PresentBinary(t *testing.T) {
	// "sh" is present on any Unix-like system used by this project.
	deps := []preflight.Dep{
		{Name: "sh", Required: true, Hint: "should never be missing"},
	}
	result := preflight.Check(deps)
	if len(result.Missing) != 0 {
		t.Errorf("sh should be present, got missing: %v", result.Missing)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("no warnings expected for present dep, got %v", result.Warnings)
	}
}

func TestRun_RequiredMissingReturnsError(t *testing.T) {
	deps := []preflight.Dep{
		{Name: "this-binary-does-not-exist-99999", Required: true, Hint: "install it somehow"},
	}
	var buf bytes.Buffer
	err := preflight.Run(deps, &buf)
	if err == nil {
		t.Fatal("expected error for missing required dep, got nil")
	}
	if !strings.Contains(err.Error(), "preflight check failed") {
		t.Errorf("error message should mention preflight check failed: %v", err)
	}
	if !strings.Contains(err.Error(), "this-binary-does-not-exist-99999") {
		t.Errorf("error message should name the missing binary: %v", err)
	}
}

func TestRun_OptionalMissingWritesWarning(t *testing.T) {
	deps := []preflight.Dep{
		{Name: "this-binary-does-not-exist-99999", Required: false, Hint: "cost tracking will be disabled"},
	}
	var buf bytes.Buffer
	err := preflight.Run(deps, &buf)
	if err != nil {
		t.Fatalf("optional missing dep should not return error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Warning:") {
		t.Errorf("expected Warning: in output, got: %q", out)
	}
}

func TestRun_AllPresent(t *testing.T) {
	deps := []preflight.Dep{
		{Name: "sh", Required: true, Hint: "should never be missing"},
	}
	var buf bytes.Buffer
	err := preflight.Run(deps, &buf)
	if err != nil {
		t.Fatalf("expected no error when all deps present, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when all deps present, got: %q", buf.String())
	}
}

func TestSpawnNativeDeps_Claude(t *testing.T) {
	deps := preflight.SpawnNativeDeps("claude")
	names := depNames(deps)
	requireContains(t, names, "zellij")
	requireContains(t, names, "yx")
	requireContains(t, names, "claude")
	requireContains(t, names, "goccc")
}

func TestSpawnNativeDeps_Cursor(t *testing.T) {
	deps := preflight.SpawnNativeDeps("cursor")
	names := depNames(deps)
	requireContains(t, names, "zellij")
	requireContains(t, names, "yx")
	requireContains(t, names, "agent")
}

func TestSpawnNativeDeps_Opencode(t *testing.T) {
	deps := preflight.SpawnNativeDeps("opencode")
	names := depNames(deps)
	requireContains(t, names, "zellij")
	requireContains(t, names, "yx")
	requireContains(t, names, "opencode")
}

func TestSpawnSandboxDeps(t *testing.T) {
	deps := preflight.SpawnSandboxDeps()
	names := depNames(deps)
	requireContains(t, names, "srt")
	requireContains(t, names, "zellij")
	requireContains(t, names, "yx")

	// Platform-specific deps should be present based on the current OS.
	switch runtime.GOOS {
	case "linux":
		requireContains(t, names, "bwrap")
		requireContains(t, names, "socat")
	case "darwin":
		requireContains(t, names, "sandbox-exec")
	}
}

func TestSpawnDevcontainerDeps(t *testing.T) {
	deps := preflight.SpawnDevcontainerDeps()
	names := depNames(deps)
	requireContains(t, names, "docker")
	requireContains(t, names, "zellij")
	requireContains(t, names, "yx")
}

func TestEnsureClaudeAuthEnv_DevcontainerClaudeMissing(t *testing.T) {
	err := preflight.EnsureClaudeAuthEnv("claude", "devcontainer", "", func(string) (string, bool) {
		return "", false
	})
	if err == nil {
		t.Fatal("expected error when no auth configured for devcontainer claude")
	}
	if !strings.Contains(err.Error(), "devcontainer") {
		t.Errorf("error should mention devcontainer runtime, got: %v", err)
	}
}

func TestEnsureClaudeAuthEnv_DevcontainerClaudeEmpty(t *testing.T) {
	err := preflight.EnsureClaudeAuthEnv("claude", "devcontainer", "", func(string) (string, bool) {
		return "   ", true
	})
	if err == nil {
		t.Fatal("expected error when _ANTHROPIC_API_KEY is blank for devcontainer claude")
	}
}

func TestEnsureClaudeAuthEnv_DevcontainerClaudePresent(t *testing.T) {
	err := preflight.EnsureClaudeAuthEnv("claude", "devcontainer", "", func(string) (string, bool) {
		return "sk-ant-valid", true
	})
	if err != nil {
		t.Fatalf("expected no error when _ANTHROPIC_API_KEY is set for devcontainer claude, got: %v", err)
	}
}

func TestEnsureClaudeAuthEnv_DevcontainerClaudeOAuth(t *testing.T) {
	// Create a temp home dir with a fake OAuth credential file.
	tmpHome := t.TempDir()
	claudeDir := tmpHome + "/.claude"
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeDir+"/credentials.json", []byte(`{"token":"fake"}`), 0600); err != nil {
		t.Fatal(err)
	}
	err := preflight.EnsureClaudeAuthEnv("claude", "devcontainer", tmpHome, func(string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatalf("expected no error when OAuth creds present in shaverHomeDir, got: %v", err)
	}
}

func TestEnsureClaudeAuthEnv_NativeSkipsKeyCheck(t *testing.T) {
	err := preflight.EnsureClaudeAuthEnv("claude", "native", "", func(string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatalf("expected no error for native runtime without API key, got: %v", err)
	}
}

func TestEnsureClaudeAuthEnv_NativeWithKeyStillWorks(t *testing.T) {
	err := preflight.EnsureClaudeAuthEnv("claude", "native", "", func(string) (string, bool) {
		return "sk-ant-valid", true
	})
	if err != nil {
		t.Fatalf("expected no error for native runtime with API key, got: %v", err)
	}
}

func TestEnsureClaudeAuthEnv_NonClaudeIgnored(t *testing.T) {
	tools := []string{"cursor", "opencode"}
	for _, tool := range tools {
		err := preflight.EnsureClaudeAuthEnv(tool, "devcontainer", "", func(string) (string, bool) {
			return "", false
		})
		if err != nil {
			t.Fatalf("expected no error for tool %q when _ANTHROPIC_API_KEY is missing, got: %v", tool, err)
		}
	}
}

func depNames(deps []preflight.Dep) []string {
	names := make([]string, len(deps))
	for i, d := range deps {
		names[i] = d.Name
	}
	return names
}

func requireContains(t *testing.T, names []string, want string) {
	t.Helper()
	if !slices.Contains(names, want) {
		t.Errorf("expected dep %q in list %v", want, names)
	}
}
