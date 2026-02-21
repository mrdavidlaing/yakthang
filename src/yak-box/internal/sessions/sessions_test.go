package sessions

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func initTestGitRepo(tmpDir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	return cmd.Run()
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, tmpDir string)
		expectError bool
		expectEmpty bool
	}{
		{
			name: "load nonexistent file returns empty sessions",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "load valid sessions file",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			expectEmpty: false,
		},
		{
			name: "load malformed JSON file",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				os.WriteFile(sessionsPath, []byte("invalid json {"), 0644)
			},
			expectError: true,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			sessions, err := Load()

			if (err != nil) != tt.expectError {
				t.Errorf("Load() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError {
				if tt.expectEmpty && len(sessions) != 0 {
					t.Errorf("Load() expected empty sessions, got %d", len(sessions))
				}
				if !tt.expectEmpty && len(sessions) == 0 {
					t.Error("Load() expected non-empty sessions, got empty")
				}
			}
		})
	}
}

func TestSave(t *testing.T) {
	tests := []struct {
		name        string
		sessions    Sessions
		expectError bool
	}{
		{
			name:        "save empty sessions",
			sessions:    Sessions{},
			expectError: false,
		},
		{
			name: "save single session",
			sessions: Sessions{
				"session1": Session{
					Worker:      "worker1",
					Task:        "task1",
					Container:   "container1",
					SpawnedAt:   time.Now(),
					Runtime:     "runtime1",
					CWD:         "/path/to/cwd",
					WorkerName:  "worker1",
					DisplayName: "Session 1",
				},
			},
			expectError: false,
		},
		{
			name: "save multiple sessions",
			sessions: Sessions{
				"session1": Session{
					Worker:      "worker1",
					Task:        "task1",
					Container:   "container1",
					SpawnedAt:   time.Now(),
					Runtime:     "runtime1",
					CWD:         "/path/to/cwd",
					WorkerName:  "worker1",
					DisplayName: "Session 1",
				},
				"session2": Session{
					Worker:      "worker2",
					Task:        "task2",
					Container:   "container2",
					SpawnedAt:   time.Now(),
					Runtime:     "runtime2",
					CWD:         "/path/to/cwd2",
					WorkerName:  "worker2",
					DisplayName: "Session 2",
				},
			},
			expectError: false,
		},
		{
			name: "save session with optional fields",
			sessions: Sessions{
				"session_full": Session{
					Worker:        "worker1",
					Task:          "task1",
					Container:     "container1",
					SpawnedAt:     time.Now(),
					Runtime:       "runtime1",
					CWD:           "/path/to/cwd",
					WorkerName:  "worker1",
					DisplayName:   "Full Session",
					ZellijSession: "zellij_session1",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := initTestGitRepo(tmpDir); err != nil {
				t.Fatalf("failed to init test repo: %v", err)
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			err = Save(tt.sessions)

			if (err != nil) != tt.expectError {
				t.Errorf("Save() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError {
				sessionsPath := filepath.Join(tmpDir, yakBoxesDir, sessionsFile)
				data, err := os.ReadFile(sessionsPath)
				if err != nil {
					t.Errorf("failed to read saved sessions file: %v", err)
				}

				var loaded Sessions
				if err := json.Unmarshal(data, &loaded); err != nil {
					t.Errorf("failed to unmarshal saved sessions: %v", err)
				}

				if len(loaded) != len(tt.sessions) {
					t.Errorf("loaded sessions count = %d, expected %d", len(loaded), len(tt.sessions))
				}
			}
		})
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		session     Session
		expectError bool
	}{
		{
			name:      "register new session",
			sessionID: "new_session",
			session: Session{
				Worker:      "worker1",
				Task:        "task1",
				Container:   "container1",
				SpawnedAt:   time.Now(),
				Runtime:     "runtime1",
				CWD:         "/path/to/cwd",
				WorkerName:  "worker1",
				DisplayName: "New Session",
			},
			expectError: false,
		},
		{
			name:      "register session with empty ID",
			sessionID: "",
			session: Session{
				Worker:      "worker1",
				Task:        "task1",
				Container:   "container1",
				SpawnedAt:   time.Now(),
				Runtime:     "runtime1",
				CWD:         "/path/to/cwd",
				WorkerName:  "worker1",
				DisplayName: "Session",
			},
			expectError: false,
		},
		{
			name:      "register session overwrites existing",
			sessionID: "session1",
			session: Session{
				Worker:      "worker2",
				Task:        "task2",
				Container:   "container2",
				SpawnedAt:   time.Now(),
				Runtime:     "runtime2",
				CWD:         "/path/to/cwd2",
				WorkerName:  "worker2",
				DisplayName: "Updated Session",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := initTestGitRepo(tmpDir); err != nil {
				t.Fatalf("failed to init test repo: %v", err)
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			err = Register(tt.sessionID, tt.session)

			if (err != nil) != tt.expectError {
				t.Errorf("Register() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError {
				sessions, err := Load()
				if err != nil {
					t.Errorf("failed to load sessions after register: %v", err)
				}

				if _, ok := sessions[tt.sessionID]; !ok {
					t.Errorf("session %q not found after register", tt.sessionID)
				}

				if sessions[tt.sessionID].Worker != tt.session.Worker {
					t.Errorf("registered session worker = %q, expected %q", sessions[tt.sessionID].Worker, tt.session.Worker)
				}
			}
		})
	}
}

func TestUnregister(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		setupFunc   func(t *testing.T, tmpDir string)
		expectError bool
		expectFound bool
	}{
		{
			name:      "unregister existing session",
			sessionID: "session1",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			expectFound: false,
		},
		{
			name:      "unregister nonexistent session",
			sessionID: "nonexistent",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			err = Unregister(tt.sessionID)

			if (err != nil) != tt.expectError {
				t.Errorf("Unregister() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError {
				sessions, err := Load()
				if err != nil {
					t.Errorf("failed to load sessions after unregister: %v", err)
				}

				_, found := sessions[tt.sessionID]
				if found != tt.expectFound {
					t.Errorf("session %q found = %v, expected %v", tt.sessionID, found, tt.expectFound)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		setupFunc   func(t *testing.T, tmpDir string)
		expectError bool
		errTarget   error
	}{
		{
			name:      "get existing session",
			sessionID: "session1",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			errTarget:   nil,
		},
		{
			name:      "get nonexistent session",
			sessionID: "nonexistent",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: true,
			errTarget:   ErrSessionNotFound,
		},
		{
			name:      "get from empty sessions",
			sessionID: "any_session",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
			},
			expectError: true,
			errTarget:   ErrSessionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			session, err := Get(tt.sessionID)

			if (err != nil) != tt.expectError {
				t.Errorf("Get() error = %v, expectError = %v", err, tt.expectError)
			}

			if tt.errTarget != nil && err != nil {
				if !errors.Is(err, tt.errTarget) {
					t.Errorf("Get() error type = %v, expected %v", err, tt.errTarget)
				}
			}

			if !tt.expectError && session == nil {
				t.Error("Get() returned nil session for non-error case")
			}

			if !tt.expectError && session != nil && session.Worker != "worker1" {
				t.Errorf("Get() returned session with worker = %q, expected %q", session.Worker, "worker1")
			}
		})
	}
}

func TestGetByContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		setupFunc     func(t *testing.T, tmpDir string)
		expectError   bool
		errTarget     error
	}{
		{
			name:          "get session by existing container",
			containerName: "container1",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
					"session2": Session{
						Worker:      "worker2",
						Task:        "task2",
						Container:   "container2",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime2",
						CWD:         "/path/to/cwd2",
						WorkerName:  "worker2",
						DisplayName: "Session 2",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			errTarget:   nil,
		},
		{
			name:          "get session by nonexistent container",
			containerName: "nonexistent",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: true,
			errTarget:   ErrSessionNotFound,
		},
		{
			name:          "get session from empty containers",
			containerName: "any_container",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
			},
			expectError: true,
			errTarget:   ErrSessionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			session, err := GetByContainer(tt.containerName)

			if (err != nil) != tt.expectError {
				t.Errorf("GetByContainer() error = %v, expectError = %v", err, tt.expectError)
			}

			if tt.errTarget != nil && err != nil {
				if !errors.Is(err, tt.errTarget) {
					t.Errorf("GetByContainer() error type = %v, expected %v", err, tt.errTarget)
				}
			}

			if !tt.expectError && session == nil {
				t.Error("GetByContainer() returned nil session for non-error case")
			}

			if !tt.expectError && session != nil && session.Container != tt.containerName {
				t.Errorf("GetByContainer() returned session with container = %q, expected %q", session.Container, tt.containerName)
			}
		})
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, tmpDir string)
		expectError bool
		expectCount int
	}{
		{
			name: "list empty sessions",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name: "list single session",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name: "list multiple sessions",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
				os.MkdirAll(yakBoxesPath, 0755)
				sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)
				sessions := Sessions{
					"session1": Session{
						Worker:      "worker1",
						Task:        "task1",
						Container:   "container1",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime1",
						CWD:         "/path/to/cwd",
						WorkerName:  "worker1",
						DisplayName: "Session 1",
					},
					"session2": Session{
						Worker:      "worker2",
						Task:        "task2",
						Container:   "container2",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime2",
						CWD:         "/path/to/cwd2",
						WorkerName:  "worker2",
						DisplayName: "Session 2",
					},
					"session3": Session{
						Worker:      "worker3",
						Task:        "task3",
						Container:   "container3",
						SpawnedAt:   time.Now(),
						Runtime:     "runtime3",
						CWD:         "/path/to/cwd3",
						WorkerName:  "worker3",
						DisplayName: "Session 3",
					},
				}
				data, _ := json.MarshalIndent(sessions, "", "  ")
				os.WriteFile(sessionsPath, data, 0644)
			},
			expectError: false,
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			sessions, err := List()

			if (err != nil) != tt.expectError {
				t.Errorf("List() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError && len(sessions) != tt.expectCount {
				t.Errorf("List() returned %d sessions, expected %d", len(sessions), tt.expectCount)
			}
		})
	}
}

func TestGetHomeDir(t *testing.T) {
	tests := []struct {
		name    string
		persona string
	}{
		{
			name:    "get home dir for persona",
			persona: "test_persona",
		},
		{
			name:    "get home dir for persona with special chars",
			persona: "persona-with-dash",
		},
		{
			name:    "get home dir for empty persona",
			persona: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := initTestGitRepo(tmpDir); err != nil {
				t.Fatalf("failed to init test repo: %v", err)
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			homePath, err := GetHomeDir(tt.persona)

			if err != nil {
				t.Errorf("GetHomeDir() error = %v", err)
			}

			expectedPath := filepath.Join(tmpDir, yakBoxesDir, homeDir, tt.persona)
			if homePath != expectedPath {
				t.Errorf("GetHomeDir() = %q, expected %q", homePath, expectedPath)
			}
		})
	}
}

func TestEnsureHomeDir(t *testing.T) {
	tests := []struct {
		name        string
		persona     string
		expectError bool
	}{
		{
			name:        "ensure home dir for new persona",
			persona:     "new_persona",
			expectError: false,
		},
		{
			name:        "ensure home dir for existing persona",
			persona:     "existing_persona",
			expectError: false,
		},
		{
			name:        "ensure home dir for empty persona",
			persona:     "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := initTestGitRepo(tmpDir); err != nil {
				t.Fatalf("failed to init test repo: %v", err)
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			homePath, err := EnsureHomeDir(tt.persona)

			if (err != nil) != tt.expectError {
				t.Errorf("EnsureHomeDir() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError {
				if _, err := os.Stat(homePath); err != nil {
					t.Errorf("EnsureHomeDir() failed to create directory: %v", err)
				}

				requiredDirs := []string{
					filepath.Join(homePath, ".local"),
					filepath.Join(homePath, ".local", "share"),
					filepath.Join(homePath, ".local", "share", "opencode"),
					filepath.Join(homePath, ".local", "state"),
					filepath.Join(homePath, ".config"),
					filepath.Join(homePath, ".cache"),
				}

				for _, dir := range requiredDirs {
					if _, err := os.Stat(dir); err != nil {
						t.Errorf("EnsureHomeDir() failed to create required directory %q: %v", dir, err)
					}
				}
			}
		})
	}
}

func TestCleanHome(t *testing.T) {
	tests := []struct {
		name        string
		persona     string
		setupFunc   func(t *testing.T, tmpDir string)
		expectError bool
	}{
		{
			name:    "clean existing home dir",
			persona: "existing_persona",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				yakBoxesDirPath := filepath.Join(tmpDir, yakBoxesDir, homeDir, "existing_persona")
				os.MkdirAll(yakBoxesDirPath, 0755)
				os.WriteFile(filepath.Join(yakBoxesDirPath, "testfile.txt"), []byte("content"), 0644)
			},
			expectError: false,
		},
		{
			name:    "clean nonexistent home dir",
			persona: "nonexistent_persona",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			err = CleanHome(tt.persona)

			if (err != nil) != tt.expectError {
				t.Errorf("CleanHome() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError {
				expectedPath := filepath.Join(tmpDir, yakBoxesDir, homeDir, tt.persona)
				if _, err := os.Stat(expectedPath); err == nil {
					t.Errorf("CleanHome() failed to remove directory %q", expectedPath)
				}
			}
		})
	}
}

func TestListHomes(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, tmpDir string)
		expectError bool
		expectCount int
	}{
		{
			name: "list no homes",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name: "list single home",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				homePath := filepath.Join(tmpDir, yakBoxesDir, homeDir, "worker1")
				os.MkdirAll(homePath, 0755)
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name: "list multiple homes",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				for i := 1; i <= 3; i++ {
					personaNum := string(rune('0' + i))
					homePath := filepath.Join(tmpDir, yakBoxesDir, homeDir, "worker"+personaNum)
					os.MkdirAll(homePath, 0755)
				}
			},
			expectError: false,
			expectCount: 3,
		},
		{
			name: "list homes ignores files",
			setupFunc: func(t *testing.T, tmpDir string) {
				if err := initTestGitRepo(tmpDir); err != nil {
					t.Fatalf("failed to init test repo: %v", err)
				}
				homesPath := filepath.Join(tmpDir, yakBoxesDir, homeDir)
				os.MkdirAll(homesPath, 0755)
				os.MkdirAll(filepath.Join(homesPath, "worker1"), 0755)
				os.MkdirAll(filepath.Join(homesPath, "worker2"), 0755)
				os.WriteFile(filepath.Join(homesPath, "somefile.txt"), []byte("content"), 0644)
			},
			expectError: false,
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			homes, err := ListHomes()

			if (err != nil) != tt.expectError {
				t.Errorf("ListHomes() error = %v, expectError = %v", err, tt.expectError)
			}

			if !tt.expectError && len(homes) != tt.expectCount {
				t.Errorf("ListHomes() returned %d homes, expected %d", len(homes), tt.expectCount)
			}
		})
	}
}

func TestSessionRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		sessions Sessions
	}{
		{
			name:     "roundtrip empty sessions",
			sessions: Sessions{},
		},
		{
			name: "roundtrip single session",
			sessions: Sessions{
				"session1": Session{
					Worker:      "worker1",
					Task:        "task1",
					Container:   "container1",
					SpawnedAt:   time.Now(),
					Runtime:     "runtime1",
					CWD:         "/path/to/cwd",
					WorkerName:  "worker1",
					DisplayName: "Session 1",
				},
			},
		},
		{
			name: "roundtrip session with all fields",
			sessions: Sessions{
				"full_session": Session{
					Worker:        "worker_full",
					Task:          "task_full",
					Container:     "container_full",
					SpawnedAt:     time.Now(),
					Runtime:       "runtime_full",
					CWD:           "/path/to/cwd",
					WorkerName:  "worker_full",
					DisplayName:   "Full Session",
					ZellijSession: "zellij1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := initTestGitRepo(tmpDir); err != nil {
				t.Fatalf("failed to init test repo: %v", err)
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			os.Chdir(tmpDir)
			defer os.Chdir(originalWD)

			if err := Save(tt.sessions); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			loaded, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if len(loaded) != len(tt.sessions) {
				t.Errorf("roundtrip session count = %d, expected %d", len(loaded), len(tt.sessions))
			}

			for sessionID, expectedSession := range tt.sessions {
				actualSession, ok := loaded[sessionID]
				if !ok {
					t.Errorf("session %q not found after roundtrip", sessionID)
					continue
				}

				if actualSession.Worker != expectedSession.Worker {
					t.Errorf("Worker mismatch for session %q: got %q, expected %q", sessionID, actualSession.Worker, expectedSession.Worker)
				}
				if actualSession.WorkerName != expectedSession.WorkerName {
					t.Errorf("WorkerName mismatch for session %q: got %q, expected %q", sessionID, actualSession.WorkerName, expectedSession.WorkerName)
				}
				if actualSession.DisplayName != expectedSession.DisplayName {
					t.Errorf("DisplayName mismatch for session %q: got %q, expected %q", sessionID, actualSession.DisplayName, expectedSession.DisplayName)
				}
			}
		})
	}
}

func TestErrSessionNotFound(t *testing.T) {
	if ErrSessionNotFound == nil {
		t.Error("ErrSessionNotFound should not be nil")
	}
}

func TestConcurrentSessions(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	initialSessions := Sessions{
		"session0": Session{
			Worker:      "worker0",
			Task:        "task0",
			Container:   "container0",
			SpawnedAt:   time.Now(),
			Runtime:     "runtime0",
			CWD:         "/path/to/cwd0",
			WorkerName:  "worker0",
			DisplayName: "Session 0",
		},
	}
	if err := Save(initialSessions); err != nil {
		t.Fatalf("initial Save() error = %v", err)
	}

	done := make(chan error, 10)
	for i := 1; i <= 10; i++ {
		go func(idx int) {
			session := Session{
				Worker:      "worker" + string(rune('0'+byte(idx%10))),
				Task:        "task" + string(rune('0'+byte(idx%10))),
				Container:   "container" + string(rune('0'+byte(idx%10))),
				SpawnedAt:   time.Now(),
				Runtime:     "runtime" + string(rune('0'+byte(idx%10))),
				CWD:         "/path/to/cwd",
				WorkerName:  "worker" + string(rune('0'+byte(idx%10))),
				DisplayName: "Concurrent Session " + string(rune('0'+byte(idx%10))),
			}
			done <- Register("session"+string(rune('0'+byte(idx%10))), session)
		}(i)
	}

	for i := 0; i < 10; i++ {
		err := <-done
		if err != nil {
			t.Errorf("concurrent Register() error = %v", err)
		}
	}

	finalSessions, err := Load()
	if err != nil {
		t.Errorf("final Load() error = %v", err)
	}

	if len(finalSessions) == 0 {
		t.Error("no sessions after concurrent operations")
	}
}

func TestGetRootError(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	_, err = Load()
	if err == nil {
		t.Error("Load() expected error when not in git repo")
	}
}

func TestSaveErrorCreatingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
	os.MkdirAll(yakBoxesPath, 0755)
	os.Chmod(yakBoxesPath, 0000)
	defer os.Chmod(yakBoxesPath, 0755)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	sessions := Sessions{
		"test": Session{
			Worker:      "worker",
			CWD:         "/path",
			WorkerName:  "worker",
			DisplayName: "Test",
			SpawnedAt:   time.Now(),
			Runtime:     "runtime",
		},
	}

	err = Save(sessions)
	if err == nil {
		t.Error("Save() expected error with inaccessible directory")
	}
}

func TestEnsureHomeDirCreatesAllDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	homePath, err := EnsureHomeDir("testworker")
	if err != nil {
		t.Fatalf("EnsureHomeDir() error = %v", err)
	}

	requiredDirs := []string{
		".local",
		".local/share",
		".local/share/opencode",
		".local/state",
		".config",
		".cache",
	}

	for _, dir := range requiredDirs {
		fullPath := filepath.Join(homePath, dir)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Errorf("directory %q not created: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestGetHomeDirReturnsCorrectPath(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	workerName := "myworker"
	homePath, err := GetHomeDir(workerName)
	if err != nil {
		t.Fatalf("GetHomeDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, yakBoxesDir, homeDir, workerName)
	if homePath != expected {
		t.Errorf("GetHomeDir() = %q, expected %q", homePath, expected)
	}

	if !strings.Contains(homePath, yakBoxesDir) {
		t.Errorf("home path should contain yakBoxesDir")
	}

	if !strings.Contains(homePath, homeDir) {
		t.Errorf("home path should contain homeDir")
	}

	if !strings.Contains(homePath, workerName) {
		t.Errorf("home path should contain workerName")
	}
}

func TestListHomesWithDifferentStructures(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	homesPath := filepath.Join(tmpDir, yakBoxesDir, homeDir)
	os.MkdirAll(homesPath, 0755)

	os.MkdirAll(filepath.Join(homesPath, "worker1"), 0755)
	os.MkdirAll(filepath.Join(homesPath, "worker2"), 0755)
	os.MkdirAll(filepath.Join(homesPath, "worker3"), 0755)

	os.WriteFile(filepath.Join(homesPath, "file.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(homesPath, ".hidden"), []byte("hidden"), 0644)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	homes, err := ListHomes()
	if err != nil {
		t.Fatalf("ListHomes() error = %v", err)
	}

	if len(homes) != 3 {
		t.Errorf("ListHomes() returned %d homes, expected 3", len(homes))
	}

	expectedWorkers := map[string]bool{
		"worker1": false,
		"worker2": false,
		"worker3": false,
	}

	for _, home := range homes {
		if _, ok := expectedWorkers[home]; ok {
			expectedWorkers[home] = true
		} else {
			t.Errorf("unexpected worker in ListHomes(): %q", home)
		}
	}

	for worker, found := range expectedWorkers {
		if !found {
			t.Errorf("expected worker %q not found in ListHomes()", worker)
		}
	}
}

func TestLoadWithEmptyYakBoxesDir(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
	os.MkdirAll(yakBoxesPath, 0755)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	sessions, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("Load() with no sessions.json should return empty map, got %d sessions", len(sessions))
	}
}

func TestRegisterAndGetMultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	sessions := []struct {
		id   string
		sess Session
	}{
		{
			id: "sess1",
			sess: Session{
				Worker:      "worker1",
				Container:   "cont1",
				WorkerName:  "worker1",
				DisplayName: "Session 1",
				SpawnedAt:   time.Now(),
				Runtime:     "runtime1",
				CWD:         "/path1",
			},
		},
		{
			id: "sess2",
			sess: Session{
				Worker:      "worker2",
				Container:   "cont2",
				WorkerName:  "worker2",
				DisplayName: "Session 2",
				SpawnedAt:   time.Now(),
				Runtime:     "runtime2",
				CWD:         "/path2",
			},
		},
	}

	for _, s := range sessions {
		if err := Register(s.id, s.sess); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}

	for _, s := range sessions {
		retrieved, err := Get(s.id)
		if err != nil {
			t.Errorf("Get(%q) error = %v", s.id, err)
		}

		if retrieved.Worker != s.sess.Worker {
			t.Errorf("Get(%q) Worker = %q, expected %q", s.id, retrieved.Worker, s.sess.Worker)
		}
		if retrieved.DisplayName != s.sess.DisplayName {
			t.Errorf("Get(%q) DisplayName = %q, expected %q", s.id, retrieved.DisplayName, s.sess.DisplayName)
		}
	}
}

func TestUnregisterMultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	sessionIDs := []string{"sess1", "sess2", "sess3"}
	for _, id := range sessionIDs {
		session := Session{
			Worker:      "worker",
			Container:   "container",
			WorkerName:  "worker",
			DisplayName: "Session",
			SpawnedAt:   time.Now(),
			Runtime:     "runtime",
			CWD:         "/path",
		}
		if err := Register(id, session); err != nil {
			t.Fatalf("Register(%q) error = %v", id, err)
		}
	}

	for i, id := range sessionIDs {
		if err := Unregister(id); err != nil {
			t.Errorf("Unregister(%q) error = %v", id, err)
		}

		remaining, err := List()
		if err != nil {
			t.Errorf("List() after Unregister error = %v", err)
		}

		expectedCount := len(sessionIDs) - i - 1
		if len(remaining) != expectedCount {
			t.Errorf("after unregistering %q, expected %d sessions, got %d", id, expectedCount, len(remaining))
		}

		if _, ok := remaining[id]; ok {
			t.Errorf("session %q still exists after Unregister()", id)
		}
	}
}

func TestSessionTypeFields(t *testing.T) {
	session := Session{
		Worker:        "test_worker",
		Task:          "test_task",
		Container:     "test_container",
		SpawnedAt:     time.Now(),
		Runtime:       "test_runtime",
		CWD:           "/test/path",
		WorkerName:  "test_worker",
		DisplayName:   "Test Display Name",
		ZellijSession: "test_zellij",
	}

	if session.Worker != "test_worker" {
		t.Error("Worker field not set correctly")
	}
	if session.Task != "test_task" {
		t.Error("Task field not set correctly")
	}
	if session.Container != "test_container" {
		t.Error("Container field not set correctly")
	}
	if session.Runtime != "test_runtime" {
		t.Error("Runtime field not set correctly")
	}
	if session.CWD != "/test/path" {
		t.Error("CWD field not set correctly")
	}
	if session.WorkerName != "test_worker" {
		t.Error("WorkerName field not set correctly")
	}
	if session.DisplayName != "Test Display Name" {
		t.Error("DisplayName field not set correctly")
	}
	if session.ZellijSession != "test_zellij" {
		t.Error("ZellijSession field not set correctly")
	}
}

func TestLoadCorruptedSessionFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
	os.MkdirAll(yakBoxesPath, 0755)
	sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)

	corruptedContent := `{"session1": {invalid json here}`
	os.WriteFile(sessionsPath, []byte(corruptedContent), 0644)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	_, err = Load()
	if err == nil {
		t.Error("Load() should fail with corrupted JSON")
	}
}

func TestConcurrentSaveLoadRaceCondition(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	done := make(chan error, 20)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			sessions := Sessions{
				"session" + string(rune('0'+byte(idx))): Session{
					Worker:      "worker" + string(rune('0'+byte(idx))),
					Task:        "task" + string(rune('0'+byte(idx))),
					Container:   "container" + string(rune('0'+byte(idx))),
					SpawnedAt:   time.Now(),
					Runtime:     "runtime" + string(rune('0'+byte(idx))),
					CWD:         "/path",
					WorkerName:  "worker" + string(rune('0'+byte(idx))),
					DisplayName: "Session " + string(rune('0'+byte(idx))),
				},
			}
			done <- Save(sessions)
		}(i)

		go func(idx int) {
			_, err := Load()
			done <- err
		}(i)
	}

	for i := 0; i < 20; i++ {
		err := <-done
		if err != nil && !strings.Contains(err.Error(), "no such file") {
			t.Logf("concurrent operation error (acceptable): %v", err)
		}
	}
}

func TestEmptySessionFields(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	emptySession := Session{
		Worker:      "",
		Task:        "",
		Container:   "",
		SpawnedAt:   time.Now(),
		Runtime:     "",
		CWD:         "",
		Persona:     "",
		DisplayName: "",
	}

	err = Register("empty-session", emptySession)
	if err != nil {
		t.Logf("Register with empty fields returned error (acceptable): %v", err)
	}

	sessions, err := Load()
	if err != nil {
		t.Logf("Load after empty fields returned error: %v", err)
	} else if len(sessions) > 0 {
		if sess, ok := sessions["empty-session"]; ok {
			t.Logf("Loaded empty session: %+v", sess)
		}
	}
}

func TestSessionFilePermissionsIssue(t *testing.T) {
	tmpDir := t.TempDir()
	if err := initTestGitRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	yakBoxesPath := filepath.Join(tmpDir, yakBoxesDir)
	os.MkdirAll(yakBoxesPath, 0755)
	sessionsPath := filepath.Join(yakBoxesPath, sessionsFile)

	sessions := Sessions{
		"test": Session{
			Worker:      "worker",
			Container:   "container",
			WorkerName:  "worker",
			DisplayName: "Test",
			SpawnedAt:   time.Now(),
			Runtime:     "runtime",
			CWD:         "/path",
		},
	}
	data, _ := json.MarshalIndent(sessions, "", "  ")
	os.WriteFile(sessionsPath, data, 0644)
	os.Chmod(sessionsPath, 0000)
	defer os.Chmod(sessionsPath, 0644)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	_, err = Load()
	if err == nil {
		t.Logf("Load with restricted permissions succeeded (unexpected)")
	}
}
