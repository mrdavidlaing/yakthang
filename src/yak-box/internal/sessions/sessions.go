// Package sessions manages yak-box sessions, storing and retrieving workspace metadata.
package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	yakBoxesDir  = ".yak-boxes"
	sessionsFile = "sessions.json"
	homeDir      = "@home"
)

var (
	ErrSessionNotFound = fmt.Errorf("session not found")
)

// Session represents an active worker session
type Session struct {
	Worker        string    `json:"worker"`
	Task          string    `json:"task,omitempty"`
	Container     string    `json:"container,omitempty"`
	SpawnedAt     time.Time `json:"spawned_at"`
	Runtime       string    `json:"runtime"`
	CWD           string    `json:"cwd"`
	WorkerName    string    `json:"worker_name,omitempty"`
	DisplayName   string    `json:"display_name"`
	ZellijSession string    `json:"zellij_session,omitempty"`
	PidFile       string    `json:"pid_file,omitempty"`
}

// Sessions is the map of active sessions keyed by session ID
type Sessions map[string]Session

func getRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func ensureYakBoxesDir() error {
	root, err := getRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, yakBoxesDir)
	return os.MkdirAll(dir, 0755)
}

func ensureHomeDir(workerName string) error {
	root, err := getRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, yakBoxesDir, homeDir, workerName)
	return os.MkdirAll(dir, 0755)
}

func getSessionsPath() (string, error) {
	root, err := getRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, yakBoxesDir, sessionsFile), nil
}

// Load loads sessions from sessions.json
func Load() (Sessions, error) {
	path, err := getSessionsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(Sessions), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions file: %w", err)
	}

	var sessions Sessions
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sessions: %w", err)
	}

	return sessions, nil
}

// Save saves sessions to sessions.json
func Save(sessions Sessions) error {
	if err := ensureYakBoxesDir(); err != nil {
		return fmt.Errorf("failed to ensure yak-boxes dir: %w", err)
	}

	path, err := getSessionsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write sessions file: %w", err)
	}

	return nil
}

// Register adds a new session to sessions.json
func Register(sessionID string, session Session) error {
	sessions, err := Load()
	if err != nil {
		return err
	}

	sessions[sessionID] = session
	return Save(sessions)
}

// Unregister removes a session from sessions.json
func Unregister(sessionID string) error {
	sessions, err := Load()
	if err != nil {
		return err
	}

	delete(sessions, sessionID)
	return Save(sessions)
}

// Get returns a session by ID
func Get(sessionID string) (*Session, error) {
	sessions, err := Load()
	if err != nil {
		return nil, err
	}

	session, ok := sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}

	return &session, nil
}

// GetByContainer returns a session by container name
func GetByContainer(containerName string) (*Session, error) {
	sessions, err := Load()
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.Container == containerName {
			return &session, nil
		}
	}

	return nil, ErrSessionNotFound
}

// List returns all active sessions
func List() (Sessions, error) {
	return Load()
}

// GetHomeDir returns the path to a worker's persistent home directory
func GetHomeDir(workerName string) (string, error) {
	root, err := getRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, yakBoxesDir, homeDir, workerName), nil
}

// EnsureHomeDir creates a worker's persistent home directory
func EnsureHomeDir(workerName string) (string, error) {
	if err := ensureHomeDir(workerName); err != nil {
		return "", err
	}

	// Pre-create .local directory structure with correct permissions
	// to prevent Docker from creating it as root
	homePath, err := GetHomeDir(workerName)
	if err != nil {
		return "", err
	}

	localDirs := []string{
		filepath.Join(homePath, ".local"),
		filepath.Join(homePath, ".local", "share"),
		filepath.Join(homePath, ".local", "share", "opencode"),
		filepath.Join(homePath, ".local", "state"),
		filepath.Join(homePath, ".config"),
		filepath.Join(homePath, ".cache"),
	}

	for _, dir := range localDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return homePath, nil
}

// CleanHome removes a worker's persistent home directory
func CleanHome(workerName string) error {
	root, err := getRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, yakBoxesDir, homeDir, workerName)
	return os.RemoveAll(dir)
}

// ListHomes returns all worker home directories
func ListHomes() ([]string, error) {
	root, err := getRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, yakBoxesDir, homeDir)

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var homes []string
	for _, entry := range entries {
		if entry.IsDir() {
			homes = append(homes, entry.Name())
		}
	}

	return homes, nil
}
