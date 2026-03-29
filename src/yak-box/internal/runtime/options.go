// Package runtime provides devcontainer worker management for yak-box.
package runtime

import (
	"context"
	"os/exec"

	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/devcontainer"
	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/types"
)

type Commander interface {
	CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd
}

type defaultCommander struct{}

func (c *defaultCommander) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

type spawnConfig struct {
	worker    *types.Worker
	prompt    string
	profile   types.ResourceProfile
	homeDir   string
	devConfig *devcontainer.Config
	commander Commander
}

type SpawnOption func(*spawnConfig) error

func WithWorker(worker *types.Worker) SpawnOption {
	return func(c *spawnConfig) error {
		c.worker = worker
		return nil
	}
}

func WithPrompt(prompt string) SpawnOption {
	return func(c *spawnConfig) error {
		c.prompt = prompt
		return nil
	}
}

func WithResourceProfile(profile types.ResourceProfile) SpawnOption {
	return func(c *spawnConfig) error {
		c.profile = profile
		return nil
	}
}

func WithHomeDir(homeDir string) SpawnOption {
	return func(c *spawnConfig) error {
		c.homeDir = homeDir
		return nil
	}
}

func WithDevConfig(devConfig *devcontainer.Config) SpawnOption {
	return func(c *spawnConfig) error {
		c.devConfig = devConfig
		return nil
	}
}

func WithCommander(cmdr Commander) SpawnOption {
	return func(c *spawnConfig) error {
		c.commander = cmdr
		return nil
	}
}