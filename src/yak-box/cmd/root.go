// Package cmd defines command-line interface commands for yak-box.
package cmd

import (
	"github.com/spf13/cobra"
)

var version string

var rootCmd = &cobra.Command{
	Use:   "yak-box",
	Short: "Docker-based worker orchestration CLI",
	Long:  "yak-box is a CLI tool for managing sandboxed and native workers",
}

// Execute runs the root CLI command.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string for the CLI.
func SetVersion(v string) {
	version = v
	rootCmd.Version = version
}

func init() {
	rootCmd.AddCommand(spawnCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(checkCmd)
}
