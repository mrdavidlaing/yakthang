package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "yak-box", rootCmd.Use)
}

func TestRootCommandHasSubcommands(t *testing.T) {
	assert.NotNil(t, rootCmd)

	cmd := rootCmd
	var names []string
	for _, subcmd := range cmd.Commands() {
		names = append(names, subcmd.Name())
	}

	assert.Contains(t, names, "spawn")
	assert.Contains(t, names, "stop")
	assert.Contains(t, names, "check")
}

func TestSetVersion(t *testing.T) {
	testVersion := "v1.2.3"
	SetVersion(testVersion)

	assert.Equal(t, testVersion, rootCmd.Version)
}

func TestRootCommandExecution(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args shows help",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: false,
		},
		{
			name:    "invalid command",
			args:    []string{"invalid-command"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := rootCmd
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
