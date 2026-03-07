// Package runtime provides sandboxed worker management for yak-box.
package runtime

import (
	"context"
	"os/exec"

	"github.com/wellmaintained/yakthang/src/yak-box/pkg/devcontainer"
	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

// Commander abstracts command execution for testing
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

// SpawnOption configures the spawn process
type SpawnOption func(*spawnConfig) error

// WithWorker sets the worker configuration
func WithWorker(worker *types.Worker) SpawnOption {
	return func(c *spawnConfig) error {
		c.worker = worker
		return nil
	}
}

// WithPrompt sets the prompt string
func WithPrompt(prompt string) SpawnOption {
	return func(c *spawnConfig) error {
		c.prompt = prompt
		return nil
	}
}

// WithResourceProfile sets the resource profile
func WithResourceProfile(profile types.ResourceProfile) SpawnOption {
	return func(c *spawnConfig) error {
		c.profile = profile
		return nil
	}
}

// WithHomeDir sets the home directory
func WithHomeDir(homeDir string) SpawnOption {
	return func(c *spawnConfig) error {
		c.homeDir = homeDir
		return nil
	}
}

// WithDevConfig sets the devcontainer configuration
func WithDevConfig(devConfig *devcontainer.Config) SpawnOption {
	return func(c *spawnConfig) error {
		c.devConfig = devConfig
		return nil
	}
}

// WithCommander sets a custom commander for testing
func WithCommander(cmdr Commander) SpawnOption {
	return func(c *spawnConfig) error {
		c.commander = cmdr
		return nil
	}
}