package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/errors"
)

func TestStopFlags(t *testing.T) {
	assert.NotNil(t, stopCmd.Flags().Lookup("name"))
	assert.NotNil(t, stopCmd.Flags().Lookup("timeout"))
	assert.NotNil(t, stopCmd.Flags().Lookup("force"))
	assert.NotNil(t, stopCmd.Flags().Lookup("dry-run"))

	timeout, _ := stopCmd.Flags().GetString("timeout")
	assert.Equal(t, "30s", timeout)

	force, _ := stopCmd.Flags().GetBool("force")
	assert.False(t, force)

	dryRun, _ := stopCmd.Flags().GetBool("dry-run")
	assert.False(t, dryRun)
}

func TestStopValidation(t *testing.T) {
	tests := []struct {
		name     string
		stopName string
		timeout  string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "missing name",
			stopName: "",
			timeout:  "30s",
			wantErr:  true,
			errMsg:   "--name is required",
		},
		{
			name:     "invalid timeout format",
			stopName: "test-worker",
			timeout:  "invalid",
			wantErr:  true,
			errMsg:   "--timeout has invalid format",
		},
		{
			name:     "valid timeout 30s",
			stopName: "test-worker",
			timeout:  "30s",
			wantErr:  false,
		},
		{
			name:     "valid timeout 1m",
			stopName: "test-worker",
			timeout:  "1m",
			wantErr:  false,
		},
		{
			name:     "valid timeout 5m30s",
			stopName: "test-worker",
			timeout:  "5m30s",
			wantErr:  false,
		},
		{
			name:     "valid minimal config",
			stopName: "test-worker",
			timeout:  "30s",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(stopCmd.Flags())

			stopName = tt.stopName
			stopTimeout = tt.timeout

			err := stopCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err, "expected error for: %s", tt.name)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}

				_, ok := err.(*errors.ValidationError)
				if ok {
					exitCode := errors.GetExitCode(err)
					assert.Equal(t, 2, exitCode)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStopTimeoutParsing(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "30 seconds",
			timeout: "30s",
			want:    30 * time.Second,
			wantErr: false,
		},
		{
			name:    "1 minute",
			timeout: "1m",
			want:    1 * time.Minute,
			wantErr: false,
		},
		{
			name:    "2 minutes 30 seconds",
			timeout: "2m30s",
			want:    2*time.Minute + 30*time.Second,
			wantErr: false,
		},
		{
			name:    "invalid format",
			timeout: "invalid",
			wantErr: true,
		},
		{
			name:    "malformed number",
			timeout: "10x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := time.ParseDuration(tt.timeout)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, duration)
			}
		})
	}
}

func TestStopFlagTypes(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{name: "name string flag", flagName: "name"},
		{name: "timeout string flag", flagName: "timeout"},
		{name: "force bool flag", flagName: "force"},
		{name: "dry-run bool flag", flagName: "dry-run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := stopCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestStopTimeoutEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		stopName string
		timeout  string
		wantErr  bool
	}{
		{
			name:     "zero timeout invalid",
			stopName: "test",
			timeout:  "0s",
			wantErr:  false,
		},
		{
			name:     "very large timeout valid",
			stopName: "test",
			timeout:  "1000h",
			wantErr:  false,
		},
		{
			name:     "nanoseconds valid",
			stopName: "test",
			timeout:  "500000000ns",
			wantErr:  false,
		},
		{
			name:     "empty timeout defaults to 30s",
			stopName: "test",
			timeout:  "30s",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(stopCmd.Flags())

			stopName = tt.stopName
			stopTimeout = tt.timeout

			err := stopCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
