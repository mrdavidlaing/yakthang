package runtime

import (
	"strings"
	"testing"

	"github.com/yakthang/yakbox/pkg/devcontainer"
	"github.com/yakthang/yakbox/pkg/types"
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
		},
		persona: &types.Persona{
			Name:  "TestPersona",
			Emoji: "🧪",
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
		"-w \"/test/cwd\"",
		"WORKER_NAME=\"TestPersona\"",
	}

	for _, exp := range expected {
		if !strings.Contains(script, exp) {
			t.Errorf("Run script missing expected string: %s", exp)
		}
	}
}

func TestGenerateRunScript_WithDevConfig(t *testing.T) {
	cfg := &spawnConfig{
		worker: &types.Worker{
			Name:        "test-worker",
			CWD:         "/test/cwd",
			YakPath:     "/test/yak",
			DisplayName: "Test Worker",
		},
		persona: &types.Persona{
			Name:  "TestPersona",
			Emoji: "🧪",
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

func TestCreateZellijLayout(t *testing.T) {
	layout := createZellijLayout("Test Worker", "/run.sh", "/wait.sh", "yak-worker-test")

	expected := []string{
		"tab name=\"Test Worker\"",
		"args \"/run.sh\"",
		"args \"/wait.sh\" \"yak-worker-test\"",
	}

	for _, exp := range expected {
		if !strings.Contains(layout, exp) {
			t.Errorf("Layout missing expected string: %s", exp)
		}
	}
}
