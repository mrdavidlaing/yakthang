package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/errors"
)

func TestDiffFlags(t *testing.T) {
	assert.NotNil(t, diffCmd.Flags().Lookup("name"))

	name, _ := diffCmd.Flags().GetString("name")
	assert.Equal(t, "", name)
}

func TestDiffFlagTypes(t *testing.T) {
	flag := diffCmd.Flags().Lookup("name")
	assert.NotNil(t, flag)
	assert.Equal(t, "string", flag.Value.Type())
}

func TestDiffValidation(t *testing.T) {
	tests := []struct {
		name     string
		diffName string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "missing name",
			diffName: "",
			wantErr:  true,
			errMsg:   "--name is required",
		},
		{
			name:     "name provided",
			diffName: "Yakira",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(diffCmd.Flags())

			diffName = tt.diffName

			err := diffCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				_, ok := err.(*errors.ValidationError)
				if ok {
					assert.Equal(t, 2, errors.GetExitCode(err))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiffRunMissingHome(t *testing.T) {
	diffName = "nonexistent-worker-xyz-" + t.Name()
	err := runDiff()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-worker-xyz-")
}

func initGitRepo(t *testing.T, path string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", "-b", "main", path},
		{"git", "-C", path, "config", "user.email", "test@test.com"},
		{"git", "-C", path, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "command failed: %v\n%s", args, out)
	}
	// create initial commit so branch exists
	readme := filepath.Join(path, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("test\n"), 0644))
	out, err := exec.Command("git", "-C", path, "add", ".").CombinedOutput()
	require.NoError(t, err, "%s", out)
	out, err = exec.Command("git", "-C", path, "commit", "-m", "init").CombinedOutput()
	require.NoError(t, err, "%s", out)
}

func TestDefaultBranch(t *testing.T) {
	t.Run("returns main when main branch exists", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		branch := defaultBranch(dir)
		assert.Equal(t, "main", branch)
	})

	t.Run("returns master when master branch exists", func(t *testing.T) {
		dir := t.TempDir()
		cmds := [][]string{
			{"git", "init", "-b", "master", dir},
			{"git", "-C", dir, "config", "user.email", "test@test.com"},
			{"git", "-C", dir, "config", "user.name", "Test"},
		}
		for _, args := range cmds {
			out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
			require.NoError(t, err, "%s", out)
		}
		readme := filepath.Join(dir, "README.md")
		require.NoError(t, os.WriteFile(readme, []byte("test\n"), 0644))
		out, err := exec.Command("git", "-C", dir, "add", ".").CombinedOutput()
		require.NoError(t, err, "%s", out)
		out, err = exec.Command("git", "-C", dir, "commit", "-m", "init").CombinedOutput()
		require.NoError(t, err, "%s", out)

		branch := defaultBranch(dir)
		assert.Equal(t, "master", branch)
	})

	t.Run("falls back to main for empty repo", func(t *testing.T) {
		dir := t.TempDir()
		out, err := exec.Command("git", "init", dir).CombinedOutput()
		require.NoError(t, err, "%s", out)
		branch := defaultBranch(dir)
		assert.Equal(t, "main", branch)
	})
}

func TestRunDiffWithRepos(t *testing.T) {
	// Build a fake home dir tree under a temp git root so sessions.GetHomeDir resolves it.
	// Structure: <tmpRoot>/.git, <tmpRoot>/.yak-boxes/@home/<worker>/<repo>/
	tmpRoot := t.TempDir()

	// Make it a real git repo (workspace.FindRoot uses git rev-parse).
	initGitRepo(t, tmpRoot)

	workerName := "test-worker"
	repoName := "my-repo"
	homeBase := filepath.Join(tmpRoot, ".yak-boxes", "@home", workerName, repoName)
	require.NoError(t, os.MkdirAll(homeBase, 0755))

	// Initialise the nested git repo
	initGitRepo(t, homeBase)

	// Add an uncommitted change so diff has something to show
	require.NoError(t, os.WriteFile(filepath.Join(homeBase, "new.txt"), []byte("hello\n"), 0644))
	out, err := exec.Command("git", "-C", homeBase, "add", ".").CombinedOutput()
	require.NoError(t, err, "%s", out)
	out, err = exec.Command("git", "-C", homeBase, "commit", "-m", "add new.txt").CombinedOutput()
	require.NoError(t, err, "%s", out)

	// Switch to tmpRoot so workspace.FindRoot() resolves this test repo root.
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpRoot))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	diffName = workerName
	err = runDiff()
	assert.NoError(t, err)
}

func TestRunDiffSkipsNonGitDirs(t *testing.T) {
	tmpRoot := t.TempDir()
	// Make it a real git repo (workspace.FindRoot uses git rev-parse).
	initGitRepo(t, tmpRoot)

	workerName := "test-worker-skip"
	// Create a plain (non-git) subdirectory
	plainDir := filepath.Join(tmpRoot, ".yak-boxes", "@home", workerName, "not-a-repo")
	require.NoError(t, os.MkdirAll(plainDir, 0755))

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpRoot))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	diffName = workerName
	err = runDiff()
	// Should succeed (just print "No git repos found")
	assert.NoError(t, err)
}
