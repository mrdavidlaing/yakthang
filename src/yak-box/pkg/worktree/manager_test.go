package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func initRepoWithCommit(t *testing.T, repoPath string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(repoPath, 0755))

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	assert.NoError(t, cmd.Run())

	assert.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("test\n"), 0644))

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	assert.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=yak-box-test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=yak-box-test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	assert.NoError(t, cmd.Run())
}

func TestEnsureWorktreeAtPath(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	destPath := filepath.Join(tmpDir, "worker-home", "repo")

	initRepoWithCommit(t, repoPath)

	wtPath, err := EnsureWorktreeAtPath(repoPath, destPath, "sc-12345", false)
	assert.NoError(t, err)
	assert.Equal(t, destPath, wtPath)
	assert.True(t, IsGitRepo(destPath))

	branch, err := GetCurrentBranch(destPath)
	assert.NoError(t, err)
	assert.Equal(t, "sc-12345", branch)

	wtPath, err = EnsureWorktreeAtPath(repoPath, destPath, "sc-12345", false)
	assert.NoError(t, err)
	assert.Equal(t, destPath, wtPath)
}

func TestDetermineWorktreePath(t *testing.T) {
	tests := []struct {
		name         string
		projectPath  string
		taskPath     string
		wantContains []string
	}{
		{
			name:         "basic worktree path",
			projectPath:  "/home/user/myproject",
			taskPath:     "auth/api",
			wantContains: []string{"yak-box/worktrees", "myproject", "auth-api"},
		},
		{
			name:         "single level task",
			projectPath:  "/home/user/myproject",
			taskPath:     "bugfix",
			wantContains: []string{"yak-box/worktrees", "myproject", "bugfix"},
		},
		{
			name:         "deep task path",
			projectPath:  "/home/user/myproject",
			taskPath:     "feature/auth/api/oauth",
			wantContains: []string{"yak-box/worktrees", "myproject", "feature-auth-api-oauth"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineWorktreePath(tt.projectPath, tt.taskPath)

			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("DetermineWorktreePath() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}

func TestSanitizeTaskPath(t *testing.T) {
	tests := []struct {
		name     string
		taskPath string
		want     string
	}{
		{
			name:     "simple path",
			taskPath: "auth",
			want:     "auth",
		},
		{
			name:     "path with slashes",
			taskPath: "auth/api/login",
			want:     "auth-api-login",
		},
		{
			name:     "path with colons",
			taskPath: "auth:api",
			want:     "auth-api",
		},
		{
			name:     "path with spaces",
			taskPath: "auth api login",
			want:     "auth-api-login",
		},
		{
			name:     "mixed characters",
			taskPath: "feature/auth: api login",
			want:     "feature-auth--api-login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTaskPath(tt.taskPath)
			if got != tt.want {
				t.Errorf("sanitizeTaskPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "my-branch",
			want:  "my-branch",
		},
		{
			name:  "spaces to hyphens",
			input: "SC-102103 add UnixODBC manager to imogen-workstation",
			want:  "sc-102103-add-unixodbc-manager-to-imogen-workstation",
		},
		{
			name:  "uppercase to lowercase",
			input: "Feature/Auth",
			want:  "feature/auth",
		},
		{
			name:  "colons replaced",
			input: "auth:api:login",
			want:  "auth-api-login",
		},
		{
			name:  "consecutive hyphens collapsed",
			input: "auth -- api",
			want:  "auth-api",
		},
		{
			name:  "leading/trailing hyphens trimmed",
			input: " -my branch- ",
			want:  "my-branch",
		},
		{
			name:  "dots preserved internally",
			input: "v1.2.3-release",
			want:  "v1.2.3-release",
		},
		{
			name:  "leading dots trimmed",
			input: ".hidden-branch",
			want:  "hidden-branch",
		},
		{
			name:  "slashes preserved",
			input: "feature/my task",
			want:  "feature/my-task",
		},
		{
			name:  "empty string fallback",
			input: "   ",
			want:  "worktree",
		},
		{
			name:  "special chars stripped",
			input: "fix: handle ~tilde & ampersand [brackets]",
			want:  "fix-handle-tilde-ampersand-brackets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBranchName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
