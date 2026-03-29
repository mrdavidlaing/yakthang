package runtime

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/devcontainer"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/types"
)

type MockCommander struct {
	calls []string
}

func (m *MockCommander) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	m.calls = append(m.calls, name+" "+args[0])
	return exec.CommandContext(ctx, "echo", "mock")
}

func TestWithWorker(t *testing.T) {
	cfg := &spawnConfig{}
	worker := &types.Worker{Name: "test"}
	opt := WithWorker(worker)
	if err := opt(cfg); err != nil {
		t.Errorf("WithWorker returned error: %v", err)
	}
	if cfg.worker != worker {
		t.Error("WithWorker failed to set worker")
	}
}


func TestWithPrompt(t *testing.T) {
	cfg := &spawnConfig{}
	prompt := "test prompt"
	opt := WithPrompt(prompt)
	if err := opt(cfg); err != nil {
		t.Errorf("WithPrompt returned error: %v", err)
	}
	if cfg.prompt != prompt {
		t.Error("WithPrompt failed to set prompt")
	}
}

func TestWithResourceProfile(t *testing.T) {
	cfg := &spawnConfig{}
	profile := types.ResourceProfile{Name: "test"}
	opt := WithResourceProfile(profile)
	if err := opt(cfg); err != nil {
		t.Errorf("WithResourceProfile returned error: %v", err)
	}
	if cfg.profile.Name != profile.Name {
		t.Error("WithResourceProfile failed to set profile")
	}
}

func TestWithHomeDir(t *testing.T) {
	cfg := &spawnConfig{}
	homeDir := "/test/home"
	opt := WithHomeDir(homeDir)
	if err := opt(cfg); err != nil {
		t.Errorf("WithHomeDir returned error: %v", err)
	}
	if cfg.homeDir != homeDir {
		t.Error("WithHomeDir failed to set homeDir")
	}
}

func TestWithDevConfig(t *testing.T) {
	cfg := &spawnConfig{}
	devConfig := &devcontainer.Config{Image: "test"}
	opt := WithDevConfig(devConfig)
	if err := opt(cfg); err != nil {
		t.Errorf("WithDevConfig returned error: %v", err)
	}
	if cfg.devConfig != devConfig {
		t.Error("WithDevConfig failed to set devConfig")
	}
}

func TestWithCommander(t *testing.T) {
	cfg := &spawnConfig{}
	cmdr := &MockCommander{}
	opt := WithCommander(cmdr)
	if err := opt(cfg); err != nil {
		t.Errorf("WithCommander returned error: %v", err)
	}
	if cfg.commander != cmdr {
		t.Error("WithCommander failed to set commander")
	}
}

func TestOptionError(t *testing.T) {
	errOption := func(c *spawnConfig) error {
		return errors.New("option failed")
	}

	err := SpawnDevcontainerWorker(context.Background(), errOption)
	if err == nil {
		t.Error("SpawnDevcontainerWorker expected error but got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "option error: option failed") {
		t.Errorf("Expected 'option error: option failed' in error, got '%v'", err)
	}
}
