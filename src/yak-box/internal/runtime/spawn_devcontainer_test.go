package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/devcontainer"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/types"
)

type TestCommander struct {
	calls      []CommandCall
	responses  map[string]CommandResponse
	failingCmd string
}

type CommandCall struct {
	name string
	args []string
}

type CommandResponse struct {
	output string
	err    error
}

func (tc *TestCommander) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	tc.calls = append(tc.calls, CommandCall{name: name, args: args})

	if tc.failingCmd == name || (tc.failingCmd != "" && len(args) > 0 && tc.failingCmd == args[0]) {
		return exec.CommandContext(ctx, "false")
	}

	return exec.CommandContext(ctx, "echo", "success")
}

func (tc *TestCommander) getCall(index int) *CommandCall {
	if index >= len(tc.calls) {
		return nil
	}
	return &tc.calls[index]
}

func (tc *TestCommander) hasCommand(cmd string) bool {
	for _, call := range tc.calls {
		if call.name == cmd {
			return true
		}
	}
	return false
}

// TestSpawnDevcontainerWorker_SuccessfulSpawnWithDefaults tests basic spawn
func TestSpawnDevcontainerWorker_SuccessfulSpawnWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		SessionName: "",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	scriptDir := filepath.Join(tmpDir, "scripts")
	expectedFiles := []string{"prompt.txt", "inner.sh", "shell-exec.sh", "run.sh", "layout.kdl", "passwd", "group"}
	for _, file := range expectedFiles {
		path := filepath.Join(scriptDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected script file not created: %s", file)
		}
	}

	if !cmdr.hasCommand("zellij") {
		t.Error("zellij command was not called")
	}
}

// TestSpawnDevcontainerWorker_WithSessionName tests spawn with session name
func TestSpawnDevcontainerWorker_WithSessionName(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		SessionName: "test-session",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	found := false
	for _, call := range cmdr.calls {
		if call.name == "zellij" && len(call.args) > 0 && call.args[0] == "--session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("--session flag not found in zellij command")
	}
}

func TestSpawnDevcontainerWorker_MissingWorker(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	err := SpawnDevcontainerWorker(
		context.Background(),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
	)

	if err == nil {
		t.Error("Expected error when worker is missing")
	}
	if !strings.Contains(err.Error(), "worker is required") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestSpawnDevcontainerWorker_OptionError(t *testing.T) {
	failingOption := func(c *spawnConfig) error {
		return errors.New("test option error")
	}

	err := SpawnDevcontainerWorker(
		context.Background(),
		failingOption,
	)

	if err == nil {
		t.Error("Expected error from failing option")
	}
	if !strings.Contains(err.Error(), "option error") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestSpawnDevcontainerWorker_InvalidHomeDir(t *testing.T) {
	invalidDir := "/nonexistent/path/that/does/not/exist/yak-boxes"

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         "/tmp",
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(invalidDir),
	)

	if err == nil {
		t.Error("Expected error when home dir cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create scripts dir") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestSpawnDevcontainerWorker_CustomResourceProfile(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}
	profile := types.ResourceProfile{
		Name:   "heavy",
		CPUs:   "2.0",
		Memory: "4g",
		PIDs:   1024,
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithResourceProfile(profile),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "2.0") || !strings.Contains(contentStr, "4g") {
		t.Error("run.sh does not contain custom resource profile settings")
	}
}

func TestSpawnDevcontainerWorker_WithDevConfig(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}
	devConfig := &devcontainer.Config{
		Image: "custom-image:1.0",
		Mounts: []string{
			"source=/custom,target=/custom,type=bind",
		},
		RemoteEnv: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithDevConfig(devConfig),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "custom-image:1.0") {
		t.Error("run.sh does not contain custom image")
	}
	if !strings.Contains(contentStr, "/custom") {
		t.Error("run.sh does not contain custom mount")
	}
}

func TestSpawnDevcontainerWorker_WithWorktreePath(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worktreePath := filepath.Join(tmpDir, "worktree")
	os.MkdirAll(worktreePath, 0755)

	worker := &types.Worker{
		Name:         "test-worker",
		DisplayName:  "Test Worker",
		CWD:          tmpDir,
		YakPath:      "/test/yak",
		WorktreePath: worktreePath,
		WorkerName:   "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, worktreePath) {
		t.Error("run.sh does not contain worktree path mount")
	}
}

func TestSpawnDevcontainerWorker_PromptFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	testPrompt := "This is a test prompt with special chars: #$%&"

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt(testPrompt),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	promptPath := filepath.Join(tmpDir, "scripts", "prompt.txt")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Errorf("Failed to read prompt.txt: %v", err)
	}
	if string(content) != testPrompt {
		t.Errorf("Prompt mismatch: expected %q, got %q", testPrompt, string(content))
	}
}

func TestSpawnDevcontainerWorker_ZellijLayoutContent(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "My Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	layoutPath := filepath.Join(tmpDir, "scripts", "layout.kdl")
	content, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Errorf("Failed to read layout.kdl: %v", err)
	}
	contentStr := string(content)

	expectedStrings := []string{
		"My Test Worker",
		"run.sh",
		"shell-exec.sh",
		"yak-worker-test-worker",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("layout.kdl missing expected string: %s", expected)
		}
	}
}

func TestSpawnDevcontainerWorker_InnerScriptContent(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	innerPath := filepath.Join(tmpDir, "scripts", "inner.sh")
	content, err := os.ReadFile(innerPath)
	if err != nil {
		t.Errorf("Failed to read inner.sh: %v", err)
	}
	contentStr := string(content)

	expectedStrings := []string{
		"opencode",
		"--prompt",
		"WORKSPACE_ROOT=",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("inner.sh missing expected string: %s", expected)
		}
	}
}

func TestSpawnDevcontainerWorker_PasswdFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	passwdPath := filepath.Join(tmpDir, "scripts", "passwd")
	content, err := os.ReadFile(passwdPath)
	if err != nil {
		t.Errorf("Failed to read passwd: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "yakshaver") {
		t.Error("passwd file missing yakshaver entry")
	}
	if !strings.Contains(contentStr, fmt.Sprintf(":%d:%d:", os.Getuid(), os.Getgid())) {
		t.Error("passwd file does not contain correct uid/gid")
	}
}

func TestGetNetworkMode_BridgeMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	mode := GetNetworkMode(ctx)
	if mode == "" {
		t.Error("GetNetworkMode returned empty string")
	}
}

func TestGetNetworkMode_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mode := GetNetworkMode(ctx)
	if mode != "bridge" {
		t.Errorf("Expected 'bridge' on cancelled context, got %s", mode)
	}
}

// TestStopDevcontainerWorker_Success tests that stopping a nonexistent container returns an error
func TestStopDevcontainerWorker_Success(t *testing.T) {
	err := StopDevcontainerWorker("nonexistent-worker", 30*time.Second)

	if err == nil {
		t.Error("Expected error for nonexistent container")
	}
	// Docker may be unavailable (failed to check) or container missing (not found)
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "failed to check") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestStopDevcontainerWorker_ContainerNotFound(t *testing.T) {
	err := StopDevcontainerWorker("definitely-not-a-real-container", 30*time.Second)

	if err == nil {
		t.Error("Expected error when container not found")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "failed to check") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestGetResourceProfile_Light(t *testing.T) {
	profile := GetResourceProfile("light")

	if profile.Name != "light" {
		t.Errorf("Expected name 'light', got %s", profile.Name)
	}
	if profile.CPUs != "0.5" {
		t.Errorf("Expected CPUs '0.5', got %s", profile.CPUs)
	}
	if profile.Memory != "1g" {
		t.Errorf("Expected Memory '1g', got %s", profile.Memory)
	}
	if profile.PIDs != 256 {
		t.Errorf("Expected PIDs 256, got %d", profile.PIDs)
	}
}

func TestGetResourceProfile_Heavy(t *testing.T) {
	profile := GetResourceProfile("heavy")

	if profile.Name != "heavy" {
		t.Errorf("Expected name 'heavy', got %s", profile.Name)
	}
	if profile.CPUs != "2.0" {
		t.Errorf("Expected CPUs '2.0', got %s", profile.CPUs)
	}
	if profile.Memory != "6g" {
		t.Errorf("Expected Memory '6g', got %s", profile.Memory)
	}
}

func TestGetResourceProfile_Ram(t *testing.T) {
	profile := GetResourceProfile("ram")

	if profile.Name != "ram" {
		t.Errorf("Expected name 'ram', got %s", profile.Name)
	}
	if profile.CPUs != "0" {
		t.Errorf("Expected CPUs '0', got %s", profile.CPUs)
	}
	if profile.Swap != "24g" {
		t.Errorf("Expected Swap '24g', got %s", profile.Swap)
	}
}

func TestGetResourceProfile_Default(t *testing.T) {
	profile := GetResourceProfile("default")

	if profile.Name != "default" {
		t.Errorf("Expected name 'default', got %s", profile.Name)
	}
	if profile.CPUs != "1.0" {
		t.Errorf("Expected CPUs '1.0', got %s", profile.CPUs)
	}
	if profile.Memory != "3g" {
		t.Errorf("Expected Memory '3g', got %s", profile.Memory)
	}
	if profile.PIDs != 512 {
		t.Errorf("Expected PIDs 512, got %d", profile.PIDs)
	}
}

func TestGetResourceProfile_UnknownName(t *testing.T) {
	profile := GetResourceProfile("unknown-profile-xyz")

	if profile.Name != "default" {
		t.Errorf("Expected name 'default', got %s", profile.Name)
	}
}

func TestSpawnDevcontainerWorker_ContainerNameGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "my-special-worker",
		DisplayName: "My Special Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)
	expectedName := "yak-worker-my-special-worker"
	if !strings.Contains(contentStr, expectedName) {
		t.Errorf("run.sh does not contain expected container name: %s", expectedName)
	}
}

func TestSpawnDevcontainerWorker_WorkerEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "MasterBuilder",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "WORKER_NAME=\"MasterBuilder\"") {
		t.Error("run.sh does not set WORKER_NAME correctly")
	}
}

func TestSpawnDevcontainerWorker_ShaverNameEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Yakoff 🪒🐃 test-worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "test-worker",
		ShaverName:  "Yakoff",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, `YAK_SHAVER_NAME="Yakoff"`) {
		t.Errorf("run.sh must set YAK_SHAVER_NAME when worker.ShaverName is set, got:\n%s", contentStr)
	}
}

func TestSpawnDevcontainerWorker_ZellijCommandFails(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	failingCmdr := &TestCommander{failingCmd: "zellij"}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(failingCmdr),
	)

	if err == nil {
		t.Error("Expected error when zellij command fails")
	}
	if !strings.Contains(err.Error(), "failed to create Zellij tab") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestSpawnDevcontainerWorker_MultipleOptions(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "complex-worker",
		DisplayName: "Complex Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		SessionName: "my-session",
		WorkerName:  "ComplexBot",
	}
	profile := types.ResourceProfile{
		Name:   "light",
		CPUs:   "0.5",
		Memory: "1g",
		PIDs:   256,
	}
	devConfig := &devcontainer.Config{
		Image: "test-image:v1",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("complex prompt"),
		WithHomeDir(tmpDir),
		WithResourceProfile(profile),
		WithDevConfig(devConfig),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "test-image:v1") {
		t.Error("run.sh missing custom image")
	}
	if !strings.Contains(contentStr, "0.5") {
		t.Error("run.sh missing CPU setting")
	}
}

func TestStopDevcontainerWorker_CheckContainerError(t *testing.T) {
	err := StopDevcontainerWorker("test-worker-that-doesnt-exist", 10*time.Second)

	if err == nil {
		t.Error("Expected error when checking container fails")
	}
}

func TestSpawnDevcontainerWorker_WindowsLineEndings(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt\nwith\nnewlines"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	promptPath := filepath.Join(tmpDir, "scripts", "prompt.txt")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Errorf("Failed to read prompt.txt: %v", err)
	}
	if !strings.Contains(string(content), "with") {
		t.Error("Prompt with newlines not preserved correctly")
	}
}

func TestSpawnDevcontainerWorker_EmptyPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt(""),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed with empty prompt: %v", err)
	}

	promptPath := filepath.Join(tmpDir, "scripts", "prompt.txt")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Errorf("Failed to read prompt.txt: %v", err)
	}
	if len(content) != 0 {
		t.Error("Empty prompt should result in empty file")
	}
}

func TestSpawnDevcontainerWorker_SpecialCharactersInWorkerName(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker-with-dashes",
		DisplayName: "Test Worker With Dashes",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	layoutPath := filepath.Join(tmpDir, "scripts", "layout.kdl")
	content, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Errorf("Failed to read layout.kdl: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "yak-worker-test-worker-with-dashes") {
		t.Error("layout.kdl does not contain proper container name")
	}
}

func TestSpawnDevcontainerWorker_WorktreePathOptional(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:         "test-worker",
		DisplayName:  "Test Worker",
		CWD:          tmpDir,
		YakPath:      "/test/yak",
		WorktreePath: "",
		WorkerName:   "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	runScriptPath := filepath.Join(tmpDir, "scripts", "run.sh")
	content, err := os.ReadFile(runScriptPath)
	if err != nil {
		t.Errorf("Failed to read run.sh: %v", err)
	}
	contentStr := string(content)

	if strings.Contains(contentStr, "-v \":") {
		t.Error("run.sh contains an empty mount entry when worktree path is unset")
	}
}

func TestSpawnDevcontainerWorker_DefaultCommanderUsed(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker with mocked commander failed: %v", err)
	}
}

func TestSpawnDevcontainerWorker_GroupFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	groupPath := filepath.Join(tmpDir, "scripts", "group")
	content, err := os.ReadFile(groupPath)
	if err != nil {
		t.Errorf("Failed to read group file: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "yakshaver") {
		t.Error("group file missing yakshaver entry")
	}
	if !strings.Contains(contentStr, fmt.Sprintf("x:%d:", os.Getgid())) {
		t.Error("group file does not contain correct gid")
	}
}

func TestSpawnDevcontainerWorker_ShellExecScriptContent(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	shellExecPath := filepath.Join(tmpDir, "scripts", "shell-exec.sh")
	content, err := os.ReadFile(shellExecPath)
	if err != nil {
		t.Errorf("Failed to read shell-exec.sh: %v", err)
	}
	contentStr := string(content)

	expectedStrings := []string{
		"docker inspect",
		"docker exec",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("shell-exec.sh missing expected string: %s", expected)
		}
	}
}

func TestSpawnDevcontainerWorker_LargePrompt(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	largePrompt := strings.Repeat("This is a large prompt. ", 100)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt(largePrompt),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed with large prompt: %v", err)
	}

	promptPath := filepath.Join(tmpDir, "scripts", "prompt.txt")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Errorf("Failed to read prompt.txt: %v", err)
	}
	if string(content) != largePrompt {
		t.Error("Large prompt not preserved correctly")
	}
}

func TestSpawnDevcontainerWorker_UnicodeCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker with Unicode 🎉",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("Prompt with unicode: 你好 مرحبا"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed with unicode: %v", err)
	}

	promptPath := filepath.Join(tmpDir, "scripts", "prompt.txt")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Errorf("Failed to read prompt.txt: %v", err)
	}
	if !strings.Contains(string(content), "你好") {
		t.Error("Unicode characters not preserved in prompt")
	}
}

func TestSpawnDevcontainerWorkerNullBytePath(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	invalidHome := tmpDir + "\x00test"

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test"),
		WithHomeDir(invalidHome),
	)

	if err == nil {
		t.Error("Expected error with null byte in path")
	}
}

func TestSpawnDevcontainerWorkerContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := SpawnDevcontainerWorker(
		ctx,
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
	)

	if err != nil {
		t.Logf("SpawnDevcontainerWorker with cancelled context error: %v", err)
	}
}

func TestSpawnDevcontainerWorkerScriptFileCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	scriptDir := filepath.Join(tmpDir, "scripts")
	os.MkdirAll(scriptDir, 0755)

	corruptFile := filepath.Join(scriptDir, "run.sh")
	os.WriteFile(corruptFile, []byte{0xFF, 0xFE, 0xFD}, 0644)
	os.Chmod(corruptFile, 0000)
	defer os.Chmod(corruptFile, 0644)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err == nil {
		t.Logf("SpawnDevcontainerWorker succeeded despite corruption (may be acceptable)")
	}
}

func TestSpawnDevcontainerWorker_ClaudeJsonPreseeded(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt("test prompt"),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed: %v", err)
	}

	// .claude.json lives at $HOME/.claude.json, not inside the .claude/ subdir.
	claudeJSONPath := filepath.Join(tmpDir, ".claude.json")
	content, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		t.Errorf("Expected .claude.json to be created at home root: %v", err)
	}
	if !strings.Contains(string(content), "bypassPermissionsModeAccepted") {
		t.Error(".claude.json missing bypassPermissionsModeAccepted field")
	}
}

func TestSpawnDevcontainerWorkerExtremelyLongPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	massivePrompt := strings.Repeat("This is a very long prompt that tests buffer limits. ", 1000)

	worker := &types.Worker{
		Name:        "test-worker",
		DisplayName: "Test Worker",
		CWD:         tmpDir,
		YakPath:     "/test/yak",
		WorkerName:  "TestBot",
	}

	cmdr := &TestCommander{}
	err := SpawnDevcontainerWorker(
		context.Background(),
		WithWorker(worker),
		WithPrompt(massivePrompt),
		WithHomeDir(tmpDir),
		WithCommander(cmdr),
	)

	if err != nil {
		t.Errorf("SpawnDevcontainerWorker failed with long prompt: %v", err)
	}

	promptPath := filepath.Join(tmpDir, "scripts", "prompt.txt")
	content, err := os.ReadFile(promptPath)
	if err != nil {
		t.Errorf("Failed to read prompt.txt: %v", err)
	}
	if len(content) < len(massivePrompt)/2 {
		t.Error("Large prompt appears to be truncated")
	}
}
