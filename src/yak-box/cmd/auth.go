package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/runtime"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/sessions"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage shaver authentication",
	Long:  "Commands for inspecting and setting up Claude authentication for shavers.",
}

// ---------- auth-status ----------

var authStatusShaver string

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Claude auth status for a shaver or the host",
	Long: `Show which auth mode is active for a named shaver or for the host environment.

Without --shaver, reports host-level auth (API key env var or host ~/.claude/).
With --shaver, checks the shaver's persistent home directory for credentials.`,
	Example: `  yak-box auth status
  yak-box auth status --shaver yakira`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthStatus(authStatusShaver)
	},
}

func runAuthStatus(shaverName string) error {
	if shaverName == "" {
		// Host-level check
		hostHome := os.Getenv("HOME")
		detection := runtime.DetectClaudeAuth(hostHome, os.LookupEnv)
		fmt.Println("Auth status (host)")
		printDetection(detection, hostHome)
		return nil
	}

	homeDir, err := sessions.GetHomeDir(shaverName)
	if err != nil {
		return fmt.Errorf("failed to resolve home dir for shaver %q: %w", shaverName, err)
	}

	detection := runtime.DetectClaudeAuth(homeDir, os.LookupEnv)
	fmt.Printf("Auth status for shaver: %s\n", shaverName)
	fmt.Printf("  Home: %s\n", homeDir)
	printDetection(detection, homeDir)

	if detection.Mode == runtime.AuthModeNone {
		fmt.Printf("\n  Action required: yak-box auth login --shaver %s\n", shaverName)
	}
	return nil
}

func printDetection(d runtime.AuthDetection, homeDir string) {
	switch d.Mode {
	case runtime.AuthModeAPIKey:
		fmt.Println("  Mode: api-key")
		masked := d.APIKey
		if len(masked) > 8 {
			masked = masked[:4] + "..." + masked[len(masked)-4:]
		}
		fmt.Printf("  Key:  %s\n", masked)
	case runtime.AuthModeOAuth:
		fmt.Println("  Mode: oauth")
		fmt.Printf("  Credentials: present (%s/.claude/)\n", homeDir)
	default:
		fmt.Println("  Mode: none")
		fmt.Println("  Credentials: not found")
	}
}

// ---------- auth-login ----------

var authLoginShaver string

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log a shaver in to Claude via OAuth device flow",
	Long: `Run 'claude login' inside a sandboxed container for the named shaver.

The device-flow URL is printed to stdout. Open it in a browser to authorize.
OAuth credentials are stored in the shaver's persistent home directory and
are reused automatically on all subsequent spawns.`,
	Example: `  yak-box auth login --shaver yakira`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuthLogin(authLoginShaver)
	},
}

func runAuthLogin(shaverName string) error {
	if shaverName == "" {
		return fmt.Errorf("--shaver is required")
	}

	homeDir, err := sessions.EnsureHomeDir(shaverName)
	if err != nil {
		return fmt.Errorf("failed to ensure home dir for shaver %q: %w", shaverName, err)
	}

	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required for auth login: %w", err)
	}

	uid := os.Getuid()
	gid := os.Getgid()

	// Write a temporary passwd/group so the container has a proper user identity.
	tmpDir, err := os.MkdirTemp("", "yak-auth-login-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	passwdContent := fmt.Sprintf("root:x:0:0:root:/root:/bin/bash\nyakshaver:x:%d:%d:Yak Shaver:/home/yak-shaver:/bin/bash\n", uid, gid)
	groupContent := fmt.Sprintf("root:x:0:\nyakshaver:x:%d:\n", gid)
	passwdFile := filepath.Join(tmpDir, "passwd")
	groupFile := filepath.Join(tmpDir, "group")
	if err := os.WriteFile(passwdFile, []byte(passwdContent), 0644); err != nil {
		return fmt.Errorf("failed to write passwd: %w", err)
	}
	if err := os.WriteFile(groupFile, []byte(groupContent), 0644); err != nil {
		return fmt.Errorf("failed to write group: %w", err)
	}

	containerName := fmt.Sprintf("yak-auth-login-%s", shaverName)

	fmt.Printf("Starting auth login for shaver %q...\n", shaverName)
	fmt.Println("A device-flow URL will appear below. Open it in your browser to authorize.")
	fmt.Println()

	dockerArgs := []string{
		"run", "--rm", "-it",
		"--name", containerName,
		fmt.Sprintf("--user=%d:%d", uid, gid),
		"--network=none", // no outbound network needed beyond what claude login requires
		"-v", fmt.Sprintf("%s:/home/yak-shaver:rw", homeDir),
		"-v", fmt.Sprintf("%s:/etc/passwd:ro", passwdFile),
		"-v", fmt.Sprintf("%s:/etc/group:ro", groupFile),
		"-e", "HOME=/home/yak-shaver",
		"-e", "IS_DEMO=true",
		"yak-worker:latest",
		"bash", "-c", "claude login",
	}

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("auth login failed: %w\n\nSuggestion: Ensure the yak-worker:latest image is built ('yak-box spawn' builds it automatically)", err)
	}

	fmt.Printf("\nLogin complete. Credentials stored in %s/.claude/\n", homeDir)
	fmt.Printf("You can now spawn %q without an API key.\n", shaverName)
	return nil
}

func init() {
	authStatusCmd.Flags().StringVar(&authStatusShaver, "shaver", "", "Shaver name to check (default: host)")
	authLoginCmd.Flags().StringVar(&authLoginShaver, "shaver", "", "Shaver name to log in (required)")
	_ = authLoginCmd.MarkFlagRequired("shaver")

	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLoginCmd)
}
