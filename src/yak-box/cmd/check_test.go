package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/errors"
)

func TestCheckFlags(t *testing.T) {
	assert.NotNil(t, checkCmd.Flags().Lookup("blocked"))
	assert.NotNil(t, checkCmd.Flags().Lookup("wip"))
	assert.NotNil(t, checkCmd.Flags().Lookup("prefix"))

	blocked, _ := checkCmd.Flags().GetBool("blocked")
	assert.False(t, blocked)

	wip, _ := checkCmd.Flags().GetBool("wip")
	assert.False(t, wip)

	prefix, _ := checkCmd.Flags().GetString("prefix")
	assert.Equal(t, "", prefix)
}

func TestCheckValidation(t *testing.T) {
	tests := []struct {
		name    string
		blocked bool
		wip     bool
		prefix  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no filters",
			blocked: false,
			wip:     false,
			prefix:  "",
			wantErr: false,
		},
		{
			name:    "blocked filter only",
			blocked: true,
			wip:     false,
			prefix:  "",
			wantErr: false,
		},
		{
			name:    "wip filter only",
			blocked: false,
			wip:     true,
			prefix:  "",
			wantErr: false,
		},
		{
			name:    "blocked and wip exclusive",
			blocked: true,
			wip:     true,
			prefix:  "",
			wantErr: true,
			errMsg:  "--blocked and --wip are mutually exclusive",
		},
		{
			name:    "prefix with blocked",
			blocked: true,
			wip:     false,
			prefix:  "auth/api",
			wantErr: false,
		},
		{
			name:    "prefix with wip",
			blocked: false,
			wip:     true,
			prefix:  "backend",
			wantErr: false,
		},
		{
			name:    "prefix only",
			blocked: false,
			wip:     false,
			prefix:  "auth/api",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(checkCmd.Flags())

			checkBlocked = tt.blocked
			checkWIP = tt.wip
			checkPrefix = tt.prefix

			err := checkCmd.PreRunE(cmd, []string{})

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

func TestCheckValidationBatching(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(checkCmd.Flags())

	checkBlocked = true
	checkWIP = true

	err := checkCmd.PreRunE(cmd, []string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--blocked and --wip are mutually exclusive")
}

func TestCheckFlagTypes(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{name: "blocked bool flag", flagName: "blocked"},
		{name: "wip bool flag", flagName: "wip"},
		{name: "prefix string flag", flagName: "prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := checkCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestCheckPrefixFiltering(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "simple prefix",
			prefix: "auth",
		},
		{
			name:   "nested prefix",
			prefix: "auth/api",
		},
		{
			name:   "deep nested prefix",
			prefix: "backend/services/auth",
		},
		{
			name:   "empty prefix",
			prefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(checkCmd.Flags())

			checkBlocked = false
			checkWIP = false
			checkPrefix = tt.prefix

			err := checkCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestCheckFilterCombinations(t *testing.T) {
	tests := []struct {
		name    string
		blocked bool
		wip     bool
		prefix  string
		wantErr bool
	}{
		{
			name:    "all filters disabled",
			blocked: false,
			wip:     false,
			prefix:  "",
			wantErr: false,
		},
		{
			name:    "only blocked",
			blocked: true,
			wip:     false,
			prefix:  "",
			wantErr: false,
		},
		{
			name:    "only wip",
			blocked: false,
			wip:     true,
			prefix:  "",
			wantErr: false,
		},
		{
			name:    "only prefix",
			blocked: false,
			wip:     false,
			prefix:  "auth/api",
			wantErr: false,
		},
		{
			name:    "blocked with prefix",
			blocked: true,
			wip:     false,
			prefix:  "auth",
			wantErr: false,
		},
		{
			name:    "wip with prefix",
			blocked: false,
			wip:     true,
			prefix:  "backend",
			wantErr: false,
		},
		{
			name:    "blocked and wip conflict",
			blocked: true,
			wip:     true,
			prefix:  "",
			wantErr: true,
		},
		{
			name:    "blocked wip and prefix conflict",
			blocked: true,
			wip:     true,
			prefix:  "auth",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(checkCmd.Flags())

			checkBlocked = tt.blocked
			checkWIP = tt.wip
			checkPrefix = tt.prefix

			err := checkCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err)
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
