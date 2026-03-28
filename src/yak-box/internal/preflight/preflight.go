package preflight

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Dep describes a host dependency to verify before a command runs.
type Dep struct {
	Name     string // binary name looked up via PATH
	Required bool   // false → warn only; true → fatal
	Hint     string // shown when the dep is missing
}

// Result holds the outcome of a preflight check.
type Result struct {
	Missing  []Dep    // required deps that were not found
	Warnings []string // messages for optional deps that were not found
}

// Check verifies that all deps in the list are available in PATH.
// Required deps that are missing populate Result.Missing.
// Optional deps that are missing generate a warning message.
func Check(deps []Dep) *Result {
	result := &Result{}
	for _, dep := range deps {
		if _, err := exec.LookPath(dep.Name); err != nil {
			if dep.Required {
				result.Missing = append(result.Missing, dep)
			} else {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s not found — %s", dep.Name, dep.Hint))
			}
		}
	}
	return result
}

// Run checks deps, prints warnings to w, and returns an error if any required
// deps are missing.  Call this at command entry before doing real work.
func Run(deps []Dep, w io.Writer) error {
	result := Check(deps)
	for _, warn := range result.Warnings {
		fmt.Fprintf(w, "Warning: %s\n", warn)
	}
	if len(result.Missing) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("preflight check failed — missing required tools:\n")
	for _, dep := range result.Missing {
		fmt.Fprintf(&sb, "  - %s: %s\n", dep.Name, dep.Hint)
	}
	return fmt.Errorf("%s", sb.String())
}

// Standard dependency definitions used across commands.
var (
	Zellij = Dep{
		Name:     "zellij",
		Required: true,
		Hint:     "install with: brew install zellij (or see https://zellij.dev/documentation/installation)",
	}
	Docker = Dep{
		Name:     "docker",
		Required: true,
		Hint:     "install Docker Desktop from https://docs.docker.com/get-docker/",
	}
	DockerOptional = Dep{
		Name:     "docker",
		Required: false,
		Hint:     "Docker not available — container status will not be shown",
	}
	Yx = Dep{
		Name:     "yx",
		Required: true,
		Hint:     "yx is part of the yak-box toolchain — ensure it is in your PATH",
	}
	Claude = Dep{
		Name:     "claude",
		Required: true,
		Hint:     "install with: npm install -g @anthropic-ai/claude-code",
	}
	// CursorAgent is the Cursor agent CLI binary (invoked as "agent").
	CursorAgent = Dep{
		Name:     "agent",
		Required: true,
		Hint:     "install Cursor with agent CLI support and ensure 'agent' is in your PATH",
	}
	Opencode = Dep{
		Name:     "opencode",
		Required: true,
		Hint:     "install with: npm install -g opencode-ai",
	}
	// Goccc is optional — cost tracking is disabled when it is absent.
	Goccc = Dep{
		Name:     "goccc",
		Required: false,
		Hint:     "cost tracking will be disabled (install with: go install github.com/backstabslash/goccc@latest)",
	}
)

// SpawnNativeDeps returns the deps required to spawn a native worker for the
// given tool.
func SpawnNativeDeps(tool string) []Dep {
	deps := []Dep{Zellij, Yx}
	switch tool {
	case "claude":
		deps = append(deps, Claude, Goccc)
	case "cursor":
		deps = append(deps, CursorAgent)
	case "opencode":
		deps = append(deps, Opencode)
	}
	return deps
}

// SpawnDevcontainerDeps returns the deps required to spawn a devcontainer worker.
func SpawnDevcontainerDeps() []Dep {
	return []Dep{Docker, Zellij, Yx}
}

// StopDeps returns the deps checked before stopping a worker.
func StopDeps() []Dep {
	return []Dep{Goccc}
}

// CheckDeps returns the deps checked before running the check command.
func CheckDeps() []Dep {
	return []Dep{DockerOptional, Goccc}
}

// EnsureClaudeAuthEnv verifies that Claude authentication is available when spawning
// a worker. Native workers inherit the host's OAuth session (~/.claude/) so the API
// key is not required. For devcontainer workers, either an API key env var OR OAuth
// credentials already present in shaverHomeDir/.claude/ are acceptable.
// Pass shaverHomeDir="" to skip the OAuth credential check (e.g. before homeDir is created).
func EnsureClaudeAuthEnv(tool, runtime, shaverHomeDir string, lookupEnv func(string) (string, bool)) error {
	if tool != "claude" {
		return nil
	}
	// Native runtime always passes: it inherits OAuth credentials from the host's
	// ~/.claude/ via the HOME override set in the native wrapper script.
	if runtime == "native" {
		return nil
	}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	// Check API key first (fast path).
	if key, ok := lookupEnv("_ANTHROPIC_API_KEY"); ok && strings.TrimSpace(key) != "" {
		return nil
	}
	if key, ok := lookupEnv("ANTHROPIC_API_KEY"); ok && strings.TrimSpace(key) != "" {
		return nil
	}
	// Check for OAuth credentials in the shaver's persistent home dir.
	if shaverHomeDir != "" && hasOAuthCreds(shaverHomeDir) {
		return nil
	}
	hint := ""
	if shaverHomeDir != "" {
		hint = fmt.Sprintf("\n  Option 1 (OAuth): run 'yak-box auth-login --shaver <name>' to log in via device flow\n  Option 2 (API key): export _ANTHROPIC_API_KEY=your-key")
	} else {
		hint = "\n  Option 1 (OAuth): run 'yak-box auth-login --shaver <name>' to log in via device flow\n  Option 2 (API key): export _ANTHROPIC_API_KEY=your-key"
	}
	return fmt.Errorf("preflight check failed — no Claude auth configured for devcontainer runtime.%s", hint)
}

// hasOAuthCreds returns true when homeDir/.claude/ contains at least one non-empty JSON file.
func hasOAuthCreds(homeDir string) bool {
	claudeDir := homeDir + "/.claude"
	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err == nil && info.Size() > 0 {
			return true
		}
	}
	return false
}
