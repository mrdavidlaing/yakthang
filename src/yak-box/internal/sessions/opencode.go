package sessions

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mrdavidlaing/yakthang/src/yak-box/pkg/types"
)

// OpenCodeSession represents a session returned by `opencode session list --format json`.
type OpenCodeSession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Created   int64  `json:"created"`
	ProjectID string `json:"projectId"`
	Directory string `json:"directory"`
}

// MessageResult holds the result of sending a message to a worker.
type MessageResult struct {
	Output   string
	ExitCode int
}

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// ExecRunner is the real command runner using os/exec.
type ExecRunner struct{}

// Run executes a command and returns combined output.
func (r *ExecRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// DiscoverOpenCodeSessions finds OpenCode sessions for a worker.
// For Docker workers, it exec's into the container.
// For native workers, it runs opencode locally with the worker's CWD.
func DiscoverOpenCodeSessions(runner CommandRunner, session *Session) ([]OpenCodeSession, error) {
	var output []byte
	var err error

	if types.Runtime(session.Runtime) == types.RuntimeDevcontainer {
		output, err = runner.Run("docker", "exec", session.Container, "opencode", "session", "list", "--format", "json")
	} else {
		output, err = runner.Run("opencode", "session", "list", "--format", "json", "--dir", session.CWD)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list opencode sessions: %w\nOutput: %s", err, string(output))
	}

	return ParseOpenCodeSessions(output)
}

// ParseOpenCodeSessions parses the JSON output from `opencode session list --format json`.
func ParseOpenCodeSessions(data []byte) ([]OpenCodeSession, error) {
	// Trim any non-JSON prefix (e.g., RTK plugin messages)
	trimmed := strings.TrimSpace(string(data))
	startIdx := strings.Index(trimmed, "[")
	if startIdx == -1 {
		return nil, fmt.Errorf("no JSON array found in output: %s", trimmed)
	}
	trimmed = trimmed[startIdx:]

	var sessions []OpenCodeSession
	if err := json.Unmarshal([]byte(trimmed), &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse opencode sessions: %w", err)
	}

	return sessions, nil
}

// FindMostRecentSession returns the most recently updated session from a list.
// Returns nil if the list is empty.
func FindMostRecentSession(sessions []OpenCodeSession) *OpenCodeSession {
	if len(sessions) == 0 {
		return nil
	}

	most := &sessions[0]
	for i := 1; i < len(sessions); i++ {
		if sessions[i].Updated > most.Updated {
			most = &sessions[i]
		}
	}
	return most
}

// SendMessage sends a message to a worker's OpenCode session.
// For Docker workers, it exec's into the container.
// For native workers, it runs opencode locally with --dir pointing to the worker's CWD.
func SendMessage(runner CommandRunner, session *Session, openCodeSessionID string, message string, format string) (*MessageResult, error) {
	baseCmd := "opencode"
	var args []string

	if types.Runtime(session.Runtime) == types.RuntimeDevcontainer {
		baseCmd = "docker"
		args = append(args, "exec", session.Container, "opencode")
	}

	args = append(args, "run", "--session", openCodeSessionID)
	if types.Runtime(session.Runtime) != types.RuntimeDevcontainer && session.CWD != "" {
		args = append(args, "--dir", session.CWD)
	}
	if format != "" && format != "default" {
		args = append(args, "--format", format)
	}
	args = append(args, message)

	output, err := runner.Run(baseCmd, args...)

	result := &MessageResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("failed to send message: %w", err)
		}
	}

	return result, nil
}

// ListWorkers returns a list of registered worker session names.
func ListWorkers() ([]string, error) {
	sessions, err := Load()
	if err != nil {
		return nil, err
	}

	var names []string
	for name := range sessions {
		names = append(names, name)
	}
	return names, nil
}
