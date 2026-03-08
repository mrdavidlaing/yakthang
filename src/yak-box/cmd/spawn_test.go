package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/errors"
	"github.com/wellmaintained/yakthang/src/yak-box/internal/pathutil"
)

func TestSpawnFlags(t *testing.T) {
	// Verify core flags are registered
	assert.NotNil(t, spawnCmd.Flags().Lookup("cwd"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yak-name"))

	// Verify optional flags are registered
	assert.NotNil(t, spawnCmd.Flags().Lookup("mode"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("resources"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yaks"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yak-path"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("runtime"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("model"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("shaver-name"))

	// Verify flag defaults
	mode, _ := spawnCmd.Flags().GetString("mode")
	assert.Equal(t, "build", mode)

	resources, _ := spawnCmd.Flags().GetString("resources")
	assert.Equal(t, "default", resources)

	yakPath, _ := spawnCmd.Flags().GetString("yak-path")
	assert.Equal(t, ".yaks", yakPath)

	runtime, _ := spawnCmd.Flags().GetString("runtime")
	assert.Equal(t, "auto", runtime)

	model, _ := spawnCmd.Flags().GetString("model")
	assert.Equal(t, "", model)
}

func TestFormatDisplayName(t *testing.T) {
	t.Run("uses shaver and worker names when different", func(t *testing.T) {
		displayName := formatDisplayName("Yakov", "cursor-test")
		assert.Equal(t, "Yakov 🪒🦬 cursor-test", displayName)
	})

	t.Run("uses shaver name once when names match", func(t *testing.T) {
		displayName := formatDisplayName("Yakira", "Yakira")
		assert.Equal(t, "Yakira", displayName)
	})

	t.Run("trims worker name", func(t *testing.T) {
		displayName := formatDisplayName("Yakov", "  cursor test  ")
		assert.Equal(t, "Yakov 🪒🦬 cursor test", displayName)
	})

	t.Run("falls back to shaver name when worker name empty", func(t *testing.T) {
		displayName := formatDisplayName("Yakov", "   ")
		assert.Equal(t, "Yakov", displayName)
	})
}

func TestResolveShaverName(t *testing.T) {
	t.Run("prefers YAK_SHAVER_NAME", func(t *testing.T) {
		t.Setenv("YAK_SHAVER_NAME", "Yakoff")
		t.Setenv("USER", "other")
		assert.Equal(t, "Yakoff", resolveShaverName())
	})
	t.Run("falls back to USER when YAK_SHAVER_NAME unset", func(t *testing.T) {
		t.Setenv("YAK_SHAVER_NAME", "")
		t.Setenv("USER", "alice")
		assert.Equal(t, "alice", resolveShaverName())
	})
	t.Run("falls back to yak-shaver when both unset", func(t *testing.T) {
		t.Setenv("YAK_SHAVER_NAME", "")
		t.Setenv("USER", "")
		assert.Equal(t, "yak-shaver", resolveShaverName())
	})
}

// resolveShaverNameForSpawn is the logic used in runSpawn: flag overrides env chain.
func resolveShaverNameForSpawn(flagValue string) string {
	s := strings.TrimSpace(flagValue)
	if s != "" {
		return s
	}
	return resolveShaverName()
}

func TestResolveShaverNameForSpawn(t *testing.T) {
	t.Run("uses flag when provided", func(t *testing.T) {
		t.Setenv("YAK_SHAVER_NAME", "EnvYak")
		t.Setenv("USER", "envuser")
		assert.Equal(t, "Yakoff", resolveShaverNameForSpawn("Yakoff"))
		assert.Equal(t, "Yakira", resolveShaverNameForSpawn("  Yakira  "))
	})
	t.Run("falls back to resolveShaverName when flag empty", func(t *testing.T) {
		t.Setenv("YAK_SHAVER_NAME", "EnvYak")
		t.Setenv("USER", "envuser")
		assert.Equal(t, "EnvYak", resolveShaverNameForSpawn(""))
		t.Setenv("YAK_SHAVER_NAME", "")
		assert.Equal(t, "envuser", resolveShaverNameForSpawn(""))
		t.Setenv("USER", "")
		assert.Equal(t, "yak-shaver", resolveShaverNameForSpawn(""))
	})
}

func TestSpawnValidation(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		spawnName string
		mode      string
		resources string
		runtime   string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "missing name",
			cwd:       "",
			spawnName: "",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--yak-name is required",
		},
		{
			name:      "invalid mode",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "invalid",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--mode must be 'plan' or 'build'",
		},
		{
			name:      "invalid resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "invalid",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--resources must be",
		},
		{
			name:      "invalid runtime",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "invalid",
			wantErr:   true,
			errMsg:    "--runtime must be",
		},
		{
			name:      "multiple validation errors batched",
			cwd:       "",
			spawnName: "",
			mode:      "invalid",
			resources: "invalid",
			runtime:   "invalid",
			wantErr:   true,
			errMsg:    "Validation errors",
		},
		{
			name:      "valid minimal config",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   false,
		},
		{
			name:      "valid with plan mode",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "plan",
			resources: "light",
			runtime:   "native",
			wantErr:   false,
		},
		{
			name:      "valid with heavy resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "heavy",
			runtime:   "sandboxed",
			wantErr:   false,
		},
		{
			name:      "valid with ram resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "ram",
			runtime:   "auto",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.PersistentFlags().AddFlagSet(spawnCmd.PersistentFlags())
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = tt.cwd
			spawnName = tt.spawnName
			spawnMode = tt.mode
			spawnResources = tt.resources
			spawnRuntime = tt.runtime

			err := spawnCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err, "expected error for test case: %s", tt.name)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
				}

				_, ok := err.(*errors.ValidationError)
				if ok {
					exitCode := errors.GetExitCode(err)
					assert.Equal(t, 2, exitCode, "ValidationError should return exit code 2")
				}
			} else {
				assert.NoError(t, err, "expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestSpawnToolOptions(t *testing.T) {
	validTools := []string{"claude", "opencode", "cursor"}

	for _, tool := range validTools {
		t.Run("valid_tool_"+tool, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = "default"
			spawnRuntime = "auto"
			spawnTool = tool

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}

	t.Run("invalid_tool", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().AddFlagSet(spawnCmd.Flags())

		spawnCWD = "/tmp/test"
		spawnName = "test-worker"
		spawnMode = "build"
		spawnResources = "default"
		spawnRuntime = "auto"
		spawnTool = "invalid"

		err := spawnCmd.PreRunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--tool must be")

		// Reset to avoid polluting subsequent tests that share this global
		spawnTool = "claude"
	})
}

func TestResolveSpawnModel(t *testing.T) {
	t.Run("respects explicit model override", func(t *testing.T) {
		assert.Equal(t, "haiku", resolveSpawnModel("claude", "haiku"))
	})

	t.Run("uses claude default model", func(t *testing.T) {
		assert.Equal(t, "default", resolveSpawnModel("claude", ""))
	})

	t.Run("uses cursor default model", func(t *testing.T) {
		assert.Equal(t, "auto", resolveSpawnModel("cursor", ""))
	})

	t.Run("uses no default for opencode", func(t *testing.T) {
		assert.Equal(t, "", resolveSpawnModel("opencode", ""))
	})
}

func TestResolveSpawnSkills(t *testing.T) {
	t.Run("returns nil when no skills given", func(t *testing.T) {
		got := resolveSpawnSkills(nil)
		assert.Empty(t, got)
	})

	t.Run("returns explicit skills only", func(t *testing.T) {
		root := t.TempDir()
		customSkill := filepath.Join(root, "skills", "custom")

		assert.NoError(t, os.MkdirAll(customSkill, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(customSkill, "SKILL.md"), []byte("name: custom"), 0644))

		got := resolveSpawnSkills([]string{customSkill})
		assert.Equal(t, []string{customSkill}, got)
	})

	t.Run("does not duplicate identical skill paths", func(t *testing.T) {
		root := t.TempDir()
		skill := filepath.Join(root, "skills", "yx-task-management")

		assert.NoError(t, os.MkdirAll(skill, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("name: yx-task-management"), 0644))

		got := resolveSpawnSkills([]string{skill, skill})
		assert.Equal(t, []string{skill}, got)
	})
}

func TestCopySkillsToHome(t *testing.T) {
	makeSkill := func(baseDir, name string) string {
		skillDir := filepath.Join(baseDir, name)
		assert.NoError(t, os.MkdirAll(skillDir, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("name: "+name), 0644))
		return skillDir
	}

	tests := []struct {
		name     string
		tool     string
		destBase string
	}{
		{
			name:     "claude destination",
			tool:     "claude",
			destBase: filepath.Join(".claude", "skills"),
		},
		{
			name:     "cursor destination",
			tool:     "cursor",
			destBase: filepath.Join(".claude", "skills"),
		},
		{
			name:     "opencode destination",
			tool:     "opencode",
			destBase: filepath.Join(".config", "opencode", "skills"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			srcDir := t.TempDir()
			skillDir := makeSkill(srcDir, "yx-task-management")

			assert.NoError(t, copySkillsToHome([]string{skillDir}, homeDir, tt.tool))

			destSkillFile := filepath.Join(homeDir, tt.destBase, "yx-task-management", "SKILL.md")
			info, err := os.Stat(destSkillFile)
			assert.NoError(t, err)
			assert.False(t, info.IsDir())
		})
	}

	t.Run("copies symlinked skill directory", func(t *testing.T) {
		homeDir := t.TempDir()
		srcDir := t.TempDir()

		// Create the real skill directory elsewhere.
		realSkillDir := filepath.Join(srcDir, "real-skills", "yak-brand")
		assert.NoError(t, os.MkdirAll(realSkillDir, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(realSkillDir, "SKILL.md"), []byte("name: yak-brand"), 0644))
		subDir := filepath.Join(realSkillDir, "subdir")
		assert.NoError(t, os.MkdirAll(subDir, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(subDir, "extra.md"), []byte("extra"), 0644))

		// Create a symlink that points to the real skill directory.
		symlinkDir := filepath.Join(srcDir, ".claude", "skills")
		assert.NoError(t, os.MkdirAll(symlinkDir, 0755))
		symlinkPath := filepath.Join(symlinkDir, "yak-brand")
		assert.NoError(t, os.Symlink(realSkillDir, symlinkPath))

		assert.NoError(t, copySkillsToHome([]string{symlinkPath}, homeDir, "claude"))

		// Verify files were copied.
		destSkillFile := filepath.Join(homeDir, ".claude", "skills", "yak-brand", "SKILL.md")
		content, err := os.ReadFile(destSkillFile)
		assert.NoError(t, err)
		assert.Equal(t, "name: yak-brand", string(content))

		// Verify subdirectory was copied too.
		destSubFile := filepath.Join(homeDir, ".claude", "skills", "yak-brand", "subdir", "extra.md")
		content, err = os.ReadFile(destSubFile)
		assert.NoError(t, err)
		assert.Equal(t, "extra", string(content))
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		homeDir := t.TempDir()
		srcDir := t.TempDir()
		skillDir := makeSkill(srcDir, "yx-task-management")

		err := copySkillsToHome([]string{skillDir}, homeDir, "unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported tool")
	})
}

func TestSpawnValidationBatching(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(spawnCmd.Flags())

	spawnCWD = ""
	spawnName = ""
	spawnMode = "invalid"
	spawnResources = "invalid"
	spawnRuntime = "invalid"

	err := spawnCmd.PreRunE(cmd, []string{})

	assert.Error(t, err)
	errMsg := err.Error()

	assert.Contains(t, errMsg, "Validation errors")
	assert.Contains(t, errMsg, "--yak-name is required")
	assert.Contains(t, errMsg, "--mode must be")
	assert.Contains(t, errMsg, "--resources must be")
	assert.Contains(t, errMsg, "--runtime must be")
}

func TestSpawnFlagTypes(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		want     interface{}
	}{
		{name: "cwd string flag", flagName: "cwd", want: ""},
		{name: "yak-name string flag", flagName: "yak-name", want: ""},
		{name: "session string flag", flagName: "session", want: "yak-box"},
		{name: "mode string flag", flagName: "mode", want: "build"},
		{name: "resources string flag", flagName: "resources", want: "default"},
		{name: "yak-path string flag", flagName: "yak-path", want: ".yaks"},
		{name: "runtime string flag", flagName: "runtime", want: "auto"},
		{name: "model string flag", flagName: "model", want: ""},
		{name: "shaver-name string flag", flagName: "shaver-name", want: ""},
		{name: "clean bool flag", flagName: "clean", want: false},
		{name: "auto-worktree bool flag", flagName: "auto-worktree", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := spawnCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestSpawnResourceOptions(t *testing.T) {
	validResources := []string{"light", "default", "heavy", "ram"}

	for _, resource := range validResources {
		t.Run("valid_resource_"+resource, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = resource
			spawnRuntime = "auto"

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestSpawnModeOptions(t *testing.T) {
	validModes := []string{"plan", "build"}

	for _, mode := range validModes {
		t.Run("valid_mode_"+mode, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = mode
			spawnResources = "default"
			spawnRuntime = "auto"

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestFindTaskDir(t *testing.T) {
	// Create a temp .yaks tree:
	// .yaks/
	//   release/
	//     yak-box/
	//       missing-tab-emoji/
	//   fixes/
	//     tab-emoji/
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "release", "yak-box", "missing-tab-emoji")
	os.MkdirAll(nested, 0755)
	other := filepath.Join(tmpDir, "fixes", "tab-emoji")
	os.MkdirAll(other, 0755)

	t.Run("finds nested task by leaf name", func(t *testing.T) {
		dir, err := findTaskDir(tmpDir, "missing-tab-emoji")
		assert.NoError(t, err)
		assert.Equal(t, nested, dir)
	})

	t.Run("finds task with direct full path", func(t *testing.T) {
		dir, err := findTaskDir(tmpDir, "release/yak-box/missing-tab-emoji")
		assert.NoError(t, err)
		assert.Equal(t, nested, dir)
	})

	t.Run("finds task in different subtree", func(t *testing.T) {
		dir, err := findTaskDir(tmpDir, "tab-emoji")
		assert.NoError(t, err)
		assert.Equal(t, other, dir)
	})

	t.Run("returns error for nonexistent task", func(t *testing.T) {
		_, err := findTaskDir(tmpDir, "nonexistent-task")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no directory matching")
	})
}

func TestResolveYakValue(t *testing.T) {
	// .yaks/
	//   parent/
	//     child/     <- has .id "my-yak-id-abc", .name "My Display Name"
	//   fixes/
	//     tab-emoji/ <- no .id (path-based only)
	//   native-worker-home-override-breaks-git-and-gh-auth/ <- lowercase yx-style dir for display-name input
	tmpDir := t.TempDir()
	childDir := filepath.Join(tmpDir, "parent", "child")
	os.MkdirAll(childDir, 0755)
	assert.NoError(t, os.WriteFile(filepath.Join(childDir, ".id"), []byte("my-yak-id-abc"), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(childDir, ".name"), []byte("My Display Name"), 0644))
	tabEmojiDir := filepath.Join(tmpDir, "fixes", "tab-emoji")
	os.MkdirAll(tabEmojiDir, 0755)
	displayNameDir := filepath.Join(tmpDir, "native-worker-home-override-breaks-git-and-gh-auth")
	os.MkdirAll(displayNameDir, 0755)
	assert.NoError(t, os.WriteFile(filepath.Join(displayNameDir, ".name"), []byte("native worker HOME override breaks git and gh auth"), 0644))

	t.Run("resolves by yak ID and uses .name as display path", func(t *testing.T) {
		taskDir, displayPath, err := resolveYakValue(tmpDir, "my-yak-id-abc")
		assert.NoError(t, err)
		assert.Equal(t, childDir, taskDir)
		assert.Equal(t, "My Display Name", displayPath)
	})

	t.Run("resolves by yak ID and uses id as display path when .name missing", func(t *testing.T) {
		noNameDir := filepath.Join(tmpDir, "parent", "no-name")
		os.MkdirAll(noNameDir, 0755)
		assert.NoError(t, os.WriteFile(filepath.Join(noNameDir, ".id"), []byte("no-name-id-xyz"), 0644))
		taskDir, displayPath, err := resolveYakValue(tmpDir, "no-name-id-xyz")
		assert.NoError(t, err)
		assert.Equal(t, noNameDir, taskDir)
		assert.Equal(t, "no-name-id-xyz", displayPath)
	})

	t.Run("falls back to slugify when no .id matches (display path)", func(t *testing.T) {
		taskDir, displayPath, err := resolveYakValue(tmpDir, "fixes/tab emoji")
		assert.NoError(t, err)
		assert.Equal(t, tabEmojiDir, taskDir)
		assert.Equal(t, "fixes/tab emoji", displayPath)
	})

	t.Run("resolves display name via lowercase-normalized slug path", func(t *testing.T) {
		taskDir, displayPath, err := resolveYakValue(tmpDir, "native worker HOME override breaks git and gh auth")
		assert.NoError(t, err)
		gotInfo, err := os.Stat(taskDir)
		assert.NoError(t, err)
		wantInfo, err := os.Stat(displayNameDir)
		assert.NoError(t, err)
		assert.True(t, os.SameFile(gotInfo, wantInfo))
		assert.Equal(t, "native worker HOME override breaks git and gh auth", displayPath)
	})

	t.Run("returns error for empty value", func(t *testing.T) {
		_, _, err := resolveYakValue(tmpDir, "   ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("returns error when neither ID nor path matches", func(t *testing.T) {
		_, _, err := resolveYakValue(tmpDir, "nonexistent-id-or-path")
		assert.Error(t, err)
	})
}

func TestYakRwDirsPathTraversalRejected(t *testing.T) {
	// Spawn validates each YakRwDirs entry with pathutil.ValidatePath(taskDir, absYakPath).
	// Reject any path that resolves outside .yaks so a crafted .id or symlink cannot mount arbitrary dirs :rw.
	yakPath := filepath.Join(t.TempDir(), ".yaks")
	assert.NoError(t, os.MkdirAll(yakPath, 0755))
	dirOutsideYaks := t.TempDir() // sibling dir, not under .yaks

	err := pathutil.ValidatePath(dirOutsideYaks, yakPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal", "path outside .yaks must be rejected for YakRwDirs")
}

func TestFindYakPath(t *testing.T) {
	// Create a nested dir structure:
	// tmpDir/
	//   .yaks/           <- the target
	//   sub/
	//     deep/          <- startDir for walk-up tests
	tmpDir := t.TempDir()
	yakDir := filepath.Join(tmpDir, ".yaks")
	os.MkdirAll(yakDir, 0755)
	deepDir := filepath.Join(tmpDir, "sub", "deep")
	os.MkdirAll(deepDir, 0755)

	t.Run("finds .yaks at start dir", func(t *testing.T) {
		got, err := findYakPath(tmpDir, ".yaks")
		assert.NoError(t, err)
		assert.Equal(t, yakDir, got)
	})

	t.Run("finds .yaks two levels up", func(t *testing.T) {
		got, err := findYakPath(deepDir, ".yaks")
		assert.NoError(t, err)
		assert.Equal(t, yakDir, got)
	})

	t.Run("returns error when no .yaks exists", func(t *testing.T) {
		noYakDir := t.TempDir()
		_, err := findYakPath(noYakDir, ".yaks")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no .yaks directory found above")
		assert.Contains(t, err.Error(), "--yak-path")
	})
}

func TestResolveInheritedWorktrees(t *testing.T) {
	workspace := t.TempDir()
	absYakPath := filepath.Join(workspace, ".yaks")
	taskDir := filepath.Join(absYakPath, "sc-12345", "child-task")
	repoA := filepath.Join(workspace, "repos", "releng", "release")
	repoB := filepath.Join(workspace, "repos", "releng", "monix")

	assert.NoError(t, os.MkdirAll(taskDir, 0755))
	assert.NoError(t, os.MkdirAll(repoA, 0755))
	assert.NoError(t, os.MkdirAll(repoB, 0755))
	assert.NoError(t, os.WriteFile(
		filepath.Join(absYakPath, "sc-12345", "worktrees"),
		[]byte("repos/releng/release,repos/releng/monix"),
		0644,
	))

	initRepo := func(path string) {
		cmd := exec.Command("git", "init")
		cmd.Dir = path
		assert.NoError(t, cmd.Run())
	}
	initRepo(repoA)
	initRepo(repoB)

	t.Run("inherits worktrees from ancestor and uses ancestor as branch", func(t *testing.T) {
		gotRepos, gotBranch, err := resolveInheritedWorktrees(absYakPath, "sc-12345/child-task")
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{repoA, repoB}, gotRepos)
		assert.Equal(t, "sc-12345", gotBranch)
	})

	t.Run("returns no worktrees when field is absent", func(t *testing.T) {
		emptyYakPath := filepath.Join(workspace, ".empty-yaks")
		emptyTaskDir := filepath.Join(emptyYakPath, "sc-54321", "child-task")
		assert.NoError(t, os.MkdirAll(emptyTaskDir, 0755))

		gotRepos, gotBranch, err := resolveInheritedWorktrees(emptyYakPath, "sc-54321/child-task")
		assert.NoError(t, err)
		assert.Empty(t, gotRepos)
		assert.Empty(t, gotBranch)
	})

	t.Run("returns an error for non-git worktree paths", func(t *testing.T) {
		badYakPath := filepath.Join(workspace, ".bad-yaks")
		badTaskDir := filepath.Join(badYakPath, "sc-99999", "child-task")
		notGitRepo := filepath.Join(workspace, "repos", "releng", "not-git")
		assert.NoError(t, os.MkdirAll(badTaskDir, 0755))
		assert.NoError(t, os.MkdirAll(notGitRepo, 0755))
		assert.NoError(t, os.WriteFile(
			filepath.Join(badYakPath, "sc-99999", "worktrees"),
			[]byte("repos/releng/not-git"),
			0644,
		))

		_, _, err := resolveInheritedWorktrees(badYakPath, "sc-99999/child-task")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not a git repository")
	})
}

func TestSpawnRuntimeOptions(t *testing.T) {
	validRuntimes := []string{"auto", "sandboxed", "native"}

	for _, runtime := range validRuntimes {
		t.Run("valid_runtime_"+runtime, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = "default"
			spawnRuntime = runtime

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}
