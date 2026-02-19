package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/yakthang/yakbox/internal/errors"
)

func TestSpawnFlags(t *testing.T) {
	// Verify required flags are registered
	assert.NotNil(t, spawnCmd.Flags().Lookup("cwd"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("name"))

	// Verify optional flags are registered
	assert.NotNil(t, spawnCmd.Flags().Lookup("mode"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("resources"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yaks"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yak-path"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("runtime"))

	// Verify flag defaults
	mode, _ := spawnCmd.Flags().GetString("mode")
	assert.Equal(t, "build", mode)

	resources, _ := spawnCmd.Flags().GetString("resources")
	assert.Equal(t, "default", resources)

	yakPath, _ := spawnCmd.Flags().GetString("yak-path")
	assert.Equal(t, ".yaks", yakPath)

	runtime, _ := spawnCmd.Flags().GetString("runtime")
	assert.Equal(t, "auto", runtime)
}

func TestSpawnValidation(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		spawnName string
		mode      string
		resources string
		runtime   string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "missing both cwd and name",
			cwd:       "",
			spawnName: "",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "Validation errors",
		},
		{
			name:      "missing cwd",
			cwd:       "",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--cwd is required",
		},
		{
			name:      "missing name",
			cwd:       "/tmp/test",
			spawnName: "",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--name is required",
		},
		{
			name:      "invalid mode",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "invalid",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--mode must be 'plan' or 'build'",
		},
		{
			name:      "invalid resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "invalid",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--resources must be",
		},
		{
			name:      "invalid runtime",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "invalid",
			wantErr:   true,
			errMsg:    "--runtime must be",
		},
		{
			name:      "multiple validation errors batched",
			cwd:       "",
			spawnName: "",
			mode:      "invalid",
			resources: "invalid",
			runtime:   "invalid",
			wantErr:   true,
			errMsg:    "Validation errors",
		},
		{
			name:      "valid minimal config",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   false,
		},
		{
			name:      "valid with plan mode",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "plan",
			resources: "light",
			runtime:   "native",
			wantErr:   false,
		},
		{
			name:      "valid with heavy resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "heavy",
			runtime:   "sandboxed",
			wantErr:   false,
		},
		{
			name:      "valid with ram resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "ram",
			runtime:   "auto",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.PersistentFlags().AddFlagSet(spawnCmd.PersistentFlags())
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = tt.cwd
			spawnName = tt.spawnName
			spawnMode = tt.mode
			spawnResources = tt.resources
			spawnRuntime = tt.runtime

			err := spawnCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err, "expected error for test case: %s", tt.name)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
				}

				_, ok := err.(*errors.ValidationError)
				if ok {
					exitCode := errors.GetExitCode(err)
					assert.Equal(t, 2, exitCode, "ValidationError should return exit code 2")
				}
			} else {
				assert.NoError(t, err, "expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestSpawnValidationBatching(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(spawnCmd.Flags())

	spawnCWD = ""
	spawnName = ""
	spawnMode = "invalid"
	spawnResources = "invalid"
	spawnRuntime = "invalid"

	err := spawnCmd.PreRunE(cmd, []string{})

	assert.Error(t, err)
	errMsg := err.Error()

	assert.Contains(t, errMsg, "Validation errors")
	assert.Contains(t, errMsg, "--cwd is required")
	assert.Contains(t, errMsg, "--name is required")
	assert.Contains(t, errMsg, "--mode must be")
	assert.Contains(t, errMsg, "--resources must be")
	assert.Contains(t, errMsg, "--runtime must be")
}

func TestSpawnFlagTypes(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		want     interface{}
	}{
		{name: "cwd string flag", flagName: "cwd", want: ""},
		{name: "name string flag", flagName: "name", want: ""},
		{name: "session string flag", flagName: "session", want: "yakthang"},
		{name: "mode string flag", flagName: "mode", want: "build"},
		{name: "resources string flag", flagName: "resources", want: "default"},
		{name: "yak-path string flag", flagName: "yak-path", want: ".yaks"},
		{name: "runtime string flag", flagName: "runtime", want: "auto"},
		{name: "clean bool flag", flagName: "clean", want: false},
		{name: "auto-worktree bool flag", flagName: "auto-worktree", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := spawnCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestSpawnResourceOptions(t *testing.T) {
	validResources := []string{"light", "default", "heavy", "ram"}

	for _, resource := range validResources {
		t.Run("valid_resource_"+resource, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = resource
			spawnRuntime = "auto"

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestSpawnModeOptions(t *testing.T) {
	validModes := []string{"plan", "build"}

	for _, mode := range validModes {
		t.Run("valid_mode_"+mode, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = mode
			spawnResources = "default"
			spawnRuntime = "auto"

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestSpawnRuntimeOptions(t *testing.T) {
	validRuntimes := []string{"auto", "sandboxed", "native"}

	for _, runtime := range validRuntimes {
		t.Run("valid_runtime_"+runtime, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = "default"
			spawnRuntime = runtime

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}
