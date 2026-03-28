package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

func TestGenerateSandboxWrapperScript_Claude(t *testing.T) {
	worker := &types.Worker{
		Tool:    "claude",
		YakPath: "/test/yaks",
		CWD:     "/test/cwd",
		Model:   "sonnet",
	}
	content := generateSandboxWrapperScript(worker, "/home/worker", "/prompt.txt", "/worker.pid", "/tmp/srt-config-123.json", "sk-test-key")

	if !strings.Contains(content, "srt --settings") {
		t.Errorf("sandbox wrapper must invoke srt, got:\n%s", content)
	}
	if !strings.Contains(content, "/tmp/srt-config-123.json") {
		t.Errorf("sandbox wrapper must reference srt config path, got:\n%s", content)
	}
	if !strings.Contains(content, "claude") {
		t.Errorf("sandbox wrapper must invoke claude, got:\n%s", content)
	}
	if !strings.Contains(content, "/worker.pid") {
		t.Errorf("sandbox wrapper must write PID file, got:\n%s", content)
	}
	if !strings.Contains(content, "CLAUDE_CONFIG_DIR") {
		t.Errorf("sandbox wrapper must set CLAUDE_CONFIG_DIR, got:\n%s", content)
	}
	if !strings.Contains(content, "_ANTHROPIC_API_KEY") {
		t.Errorf("sandbox wrapper must set API key, got:\n%s", content)
	}
	if !strings.Contains(content, "srt --settings \"/tmp/srt-config-123.json\" -- claude") {
		t.Errorf("sandbox wrapper must wrap claude with srt, got:\n%s", content)
	}
}

func TestGenerateSandboxWrapperScript_Opencode(t *testing.T) {
	worker := &types.Worker{
		Tool:    "opencode",
		YakPath: "/test/yaks",
		CWD:     "/test/cwd",
	}
	content := generateSandboxWrapperScript(worker, "/home/worker", "/prompt.txt", "/worker.pid", "/tmp/srt-config-456.json", "")

	if !strings.Contains(content, "srt --settings") {
		t.Errorf("sandbox wrapper must invoke srt, got:\n%s", content)
	}
	if !strings.Contains(content, "opencode") {
		t.Errorf("sandbox wrapper must invoke opencode, got:\n%s", content)
	}
	if !strings.Contains(content, "srt --settings \"/tmp/srt-config-456.json\" -- opencode") {
		t.Errorf("sandbox wrapper must wrap opencode with srt, got:\n%s", content)
	}
}

func TestGenerateSandboxWrapperScript_Cursor(t *testing.T) {
	worker := &types.Worker{
		Tool:    "cursor",
		YakPath: "/test/yaks",
		CWD:     "/test/cwd",
		Model:   "gpt-4",
	}
	content := generateSandboxWrapperScript(worker, "/home/worker", "/prompt.txt", "/worker.pid", "/tmp/srt-config-789.json", "")

	if !strings.Contains(content, "srt --settings") {
		t.Errorf("sandbox wrapper must invoke srt, got:\n%s", content)
	}
	if !strings.Contains(content, "agent --force") {
		t.Errorf("sandbox wrapper must invoke cursor agent, got:\n%s", content)
	}
}

func TestGenerateSandboxWrapperScript_ShaverName(t *testing.T) {
	worker := &types.Worker{
		Tool:       "claude",
		YakPath:    "/test/yaks",
		CWD:        "/test/cwd",
		ShaverName: "Yakitty",
	}
	content := generateSandboxWrapperScript(worker, "/home/worker", "/prompt.txt", "/worker.pid", "/tmp/srt.json", "")

	if !strings.Contains(content, "YAK_SHAVER_NAME") {
		t.Errorf("sandbox wrapper must set YAK_SHAVER_NAME when ShaverName is set, got:\n%s", content)
	}
	if !strings.Contains(content, "Yakitty") {
		t.Errorf("sandbox wrapper must include shaver name, got:\n%s", content)
	}
}

func TestFindWorkerHomeDir_NotFound(t *testing.T) {
	// findWorkerHomeDir should return error when no .yak-boxes exists
	oldWd, _ := os.Getwd()
	tmp := t.TempDir()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	_, err := findWorkerHomeDir("nonexistent-worker")
	if err == nil {
		t.Error("expected error for nonexistent worker")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindWorkerHomeDir_Found(t *testing.T) {
	tmp := t.TempDir()
	workerHome := filepath.Join(tmp, ".yak-boxes", "@home", "test-worker")
	if err := os.MkdirAll(workerHome, 0755); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	// Change to a subdirectory so it has to walk up
	subDir := filepath.Join(tmp, "sub", "deep")
	os.MkdirAll(subDir, 0755)
	os.Chdir(subDir)
	defer os.Chdir(oldWd)

	got, err := findWorkerHomeDir("test-worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != workerHome {
		t.Errorf("findWorkerHomeDir = %q, want %q", got, workerHome)
	}
}

func TestCopyHostOAuthCredentials_CopiesOAuthFiles(t *testing.T) {
	// Set up a fake host home with OAuth credential files
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	hostClaudeDir := filepath.Join(hostHome, ".claude")
	os.MkdirAll(hostClaudeDir, 0755)

	// OAuth token file (should be copied)
	os.WriteFile(filepath.Join(hostClaudeDir, "oauth-token-abc123.json"), []byte(`{"token":"secret"}`), 0644)

	// Another OAuth file (should be copied)
	os.WriteFile(filepath.Join(hostClaudeDir, "credentials.json"), []byte(`{"cred":"data"}`), 0644)

	// Managed files (should NOT be copied)
	os.WriteFile(filepath.Join(hostClaudeDir, ".claude.json"), []byte(`{"managed":true}`), 0644)
	os.WriteFile(filepath.Join(hostClaudeDir, "settings.json"), []byte(`{"managed":true}`), 0644)
	os.WriteFile(filepath.Join(hostClaudeDir, "remote-settings.json"), []byte(`{}`), 0644)

	// Non-JSON file (should NOT be copied)
	os.WriteFile(filepath.Join(hostClaudeDir, "api-key-helper.sh"), []byte("#!/bin/bash"), 0755)
	os.WriteFile(filepath.Join(hostClaudeDir, "some-log.txt"), []byte("log data"), 0644)

	// Directory (should be skipped)
	os.MkdirAll(filepath.Join(hostClaudeDir, "debug"), 0755)

	// Set up worker home
	workerHome := t.TempDir()
	workerClaudeDir := filepath.Join(workerHome, ".claude")
	os.MkdirAll(workerClaudeDir, 0755)

	// Pre-existing worker settings (should NOT be overwritten)
	os.WriteFile(filepath.Join(workerClaudeDir, "settings.json"), []byte(`{"worker":true}`), 0644)

	err := copyHostOAuthCredentials(workerHome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OAuth files should be copied
	for _, name := range []string{"oauth-token-abc123.json", "credentials.json"} {
		data, err := os.ReadFile(filepath.Join(workerClaudeDir, name))
		if err != nil {
			t.Errorf("expected %s to be copied, got error: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("expected %s to have content", name)
		}
	}

	// Managed files should NOT be copied (worker's settings.json preserved)
	data, _ := os.ReadFile(filepath.Join(workerClaudeDir, "settings.json"))
	if string(data) != `{"worker":true}` {
		t.Errorf("worker settings.json should not be overwritten, got: %s", string(data))
	}

	// .claude.json should NOT exist in worker dir (it's in managed list)
	if _, err := os.Stat(filepath.Join(workerClaudeDir, ".claude.json")); err == nil {
		t.Error(".claude.json should not be copied (managed file)")
	}

	// Non-JSON files should NOT be copied
	if _, err := os.Stat(filepath.Join(workerClaudeDir, "some-log.txt")); err == nil {
		t.Error("non-JSON files should not be copied")
	}
}

func TestCopyHostOAuthCredentials_OverwritesExistingFiles(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)

	hostClaudeDir := filepath.Join(hostHome, ".claude")
	os.MkdirAll(hostClaudeDir, 0755)
	os.WriteFile(filepath.Join(hostClaudeDir, "oauth-token.json"), []byte(`{"new":"token"}`), 0644)

	workerHome := t.TempDir()
	workerClaudeDir := filepath.Join(workerHome, ".claude")
	os.MkdirAll(workerClaudeDir, 0755)
	os.WriteFile(filepath.Join(workerClaudeDir, "oauth-token.json"), []byte(`{"existing":"token"}`), 0644)

	err := copyHostOAuthCredentials(workerHome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Credentials should always be copied fresh from host
	data, _ := os.ReadFile(filepath.Join(workerClaudeDir, "oauth-token.json"))
	if string(data) != `{"new":"token"}` {
		t.Errorf("OAuth file should be overwritten with host version, got: %s", string(data))
	}
}

func TestCopyHostOAuthCredentials_NoHostDir(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	// No .claude/ directory on host — should return nil (no error)

	workerHome := t.TempDir()
	err := copyHostOAuthCredentials(workerHome)
	if err != nil {
		t.Fatalf("expected no error when host .claude/ missing, got: %v", err)
	}
}

func TestStopSandboxWorker_CleansUpSrtConfig(t *testing.T) {
	// Create a fake workspace with worker home, PID file (non-existent process), and srt config
	tmp := t.TempDir()
	workerHome := filepath.Join(tmp, ".yak-boxes", "@home", "test-cleanup")
	workerDir := filepath.Join(workerHome, "scripts")
	os.MkdirAll(workerDir, 0755)

	// Create a fake srt config file
	srtConfig := filepath.Join(tmp, "srt-config-test.json")
	os.WriteFile(srtConfig, []byte("{}"), 0644)

	// Store the srt config path reference
	os.WriteFile(filepath.Join(workerDir, "srt-config-path"), []byte(srtConfig), 0644)

	// Create PID file with non-existent PID
	os.WriteFile(filepath.Join(workerDir, "worker.pid"), []byte("999999999"), 0644)

	oldWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(oldWd)

	// StopSandboxWorker will fail on the Zellij tab close (no Zellij running),
	// but it should still clean up the srt config
	_ = StopSandboxWorker("test-cleanup", 100)

	// Verify srt config was cleaned up
	if fileExists(srtConfig) {
		t.Error("srt config file should have been removed")
	}
	if fileExists(filepath.Join(workerDir, "srt-config-path")) {
		t.Error("srt config ref file should have been removed")
	}
}
