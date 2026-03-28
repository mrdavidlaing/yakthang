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
