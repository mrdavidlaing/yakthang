package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/errors"
)

func TestMessageFlags(t *testing.T) {
	assert.NotNil(t, messageCmd.Flags().Lookup("format"))
	assert.NotNil(t, messageCmd.Flags().Lookup("session"))

	format, _ := messageCmd.Flags().GetString("format")
	assert.Equal(t, "", format)

	session, _ := messageCmd.Flags().GetString("session")
	assert.Equal(t, "", session)
}

func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		format  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid args",
			args:    []string{"my-worker", "hello world"},
			format:  "",
			wantErr: false,
		},
		{
			name:    "valid with json format",
			args:    []string{"my-worker", "hello"},
			format:  "json",
			wantErr: false,
		},
		{
			name:    "valid with default format",
			args:    []string{"my-worker", "hello"},
			format:  "default",
			wantErr: false,
		},
		{
			name:    "invalid format",
			args:    []string{"my-worker", "hello"},
			format:  "xml",
			wantErr: true,
			errMsg:  "--format must be 'default' or 'json'",
		},
		{
			name:    "empty worker name",
			args:    []string{"", "hello"},
			format:  "",
			wantErr: true,
			errMsg:  "worker name cannot be empty",
		},
		{
			name:    "whitespace-only worker name",
			args:    []string{"  ", "hello"},
			format:  "",
			wantErr: true,
			errMsg:  "worker name cannot be empty",
		},
		{
			name:    "empty message",
			args:    []string{"my-worker", ""},
			format:  "",
			wantErr: true,
			errMsg:  "message text cannot be empty",
		},
		{
			name:    "whitespace-only message",
			args:    []string{"my-worker", "   "},
			format:  "",
			wantErr: true,
			errMsg:  "message text cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(messageCmd.Flags())

			messageFormat = tt.format

			err := messageCmd.PreRunE(cmd, tt.args)

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

func TestMessageFlagTypes(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{name: "format string flag", flagName: "format"},
		{name: "session string flag", flagName: "session"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := messageCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestMessageArgsRequirement(t *testing.T) {
	assert.NotNil(t, messageCmd.Args)

	err := messageCmd.Args(messageCmd, []string{})
	assert.Error(t, err)

	err = messageCmd.Args(messageCmd, []string{"worker-only"})
	assert.Error(t, err)

	err = messageCmd.Args(messageCmd, []string{"worker", "message"})
	assert.NoError(t, err)

	err = messageCmd.Args(messageCmd, []string{"worker", "multi", "word", "message"})
	assert.NoError(t, err)
}

func TestMessageMultipleValidationErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(messageCmd.Flags())

	messageFormat = "invalid"

	err := messageCmd.PreRunE(cmd, []string{"  ", "  "})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker name cannot be empty")
	assert.Contains(t, err.Error(), "message text cannot be empty")
	assert.Contains(t, err.Error(), "--format must be 'default' or 'json'")
}
