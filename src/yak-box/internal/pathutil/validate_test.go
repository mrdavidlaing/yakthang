package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create temp subdirectory: %v", err)
	}

	testFile := filepath.Join(subDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	symlinkPath := filepath.Join(subDir, "link")
	if err := os.Symlink(testFile, symlinkPath); err != nil {
		t.Logf("skipping symlink tests: %v", err)
	}

	escapeLinkPath := filepath.Join(tmpDir, "escape_link")
	if err := os.Symlink("/tmp", escapeLinkPath); err != nil {
		t.Logf("skipping escape symlink test: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		baseDir   string
		wantErr   bool
		errTarget error
	}{
		{
			name:    "valid relative path",
			path:    "file.txt",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "valid nested relative path",
			path:    "subdir/file.txt",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "valid absolute path within boundary",
			path:    testFile,
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "valid path at exact boundary",
			path:    ".",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:      "traversal with ../",
			path:      "../../../etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:      "absolute path outside boundary",
			path:      "/etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:    "empty filename is valid (directory exists)",
			path:    "",
			baseDir: tmpDir,
			wantErr: true,
		},
		{
			name:    "empty baseDir",
			path:    "file.txt",
			baseDir: "",
			wantErr: true,
		},
		{
			name:    "symlink to file within boundary",
			path:    "subdir/link",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "nonexistent file in valid location",
			path:    "subdir/nonexistent.txt",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:      "path with multiple traversal attempts",
			path:      "subdir/../../../../../../etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:    "path with . and .. that stays in boundary",
			path:    "subdir/../subdir/file.txt",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:      "path trying to escape via symlink",
			path:      "escape_link",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.baseDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q, %q) error = %v, wantErr %v", tt.path, tt.baseDir, err, tt.wantErr)
			}

			if tt.wantErr && tt.errTarget != nil {
				if !errors.Is(err, tt.errTarget) {
					t.Errorf("ValidatePath(%q, %q) error type mismatch. Got %v, want %v", tt.path, tt.baseDir, err, tt.errTarget)
				}
			}
		})
	}
}

func TestValidatePathEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		path      string
		baseDir   string
		wantErr   bool
		errTarget error
	}{
		{
			name:    "double dot at start",
			path:    "..",
			baseDir: tmpDir,
			wantErr: true,
		},
		{
			name:    "path with only dots",
			path:    "...",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "deeply nested valid path",
			path:    "a/b/c/d/e/f/g/h/i/j/file.txt",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:      "similar prefix attack (foo vs foobar)",
			path:      "../foo",
			baseDir:   filepath.Join(tmpDir, "foobar"),
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:    "slash in path",
			path:    "subdir/file.txt",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "relative baseDir",
			path:    "file.txt",
			baseDir: ".",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.baseDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q, %q) error = %v, wantErr %v", tt.path, tt.baseDir, err, tt.wantErr)
			}

			if tt.wantErr && tt.errTarget != nil {
				if !errors.Is(err, tt.errTarget) {
					t.Errorf("ValidatePath(%q, %q) error type mismatch. Got %v, want %v", tt.path, tt.baseDir, err, tt.errTarget)
				}
			}
		})
	}
}

func TestValidatePathAdvancedEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		path      string
		baseDir   string
		wantErr   bool
		errTarget error
	}{
		{
			name:      "path with null byte injection",
			path:      "/tmp/evil\x00.sh",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:      "path with encoded traversal %2e%2e",
			path:      "%2e%2e/etc/passwd",
			baseDir:   tmpDir,
			wantErr:   false,
		},
		{
			name:      "Windows backslash traversal attempt",
			path:      "..\\..\\..\\windows\\system32",
			baseDir:   tmpDir,
			wantErr:   false,
		},
		{
			name:      "path with double slash",
			path:      "//etc//passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:      "path with excessive dots",
			path:      "....//....//....//etc/passwd",
			baseDir:   tmpDir,
			wantErr:   false,
		},
		{
			name:      "path with alternate path separator",
			path:      "subdir:file.txt",
			baseDir:   tmpDir,
			wantErr:   false,
		},
		{
			name:      "mixed traversal with valid paths",
			path:      "subdir/../../../etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:    "path ending with double dot",
			path:    "validdir/..",
			baseDir: tmpDir,
			wantErr: false,
		},
		{
			name:      "absolute path with additional traversal",
			path:      "/../../etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:      "path with unicode normalization bypass",
			path:      "café/../../../etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
		{
			name:      "path with space and traversal",
			path:      "my docs/../../../etc/passwd",
			baseDir:   tmpDir,
			wantErr:   true,
			errTarget: ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.baseDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q, %q) error = %v, wantErr %v", tt.path, tt.baseDir, err, tt.wantErr)
			}

			if tt.wantErr && tt.errTarget != nil {
				if !errors.Is(err, tt.errTarget) {
					t.Errorf("ValidatePath(%q, %q) error type mismatch. Got %v, want %v", tt.path, tt.baseDir, err, tt.errTarget)
				}
			}
		})
	}
}

func BenchmarkValidatePath(b *testing.B) {
	tmpDir := b.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	testFile := filepath.Join(subDir, "file.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	testCases := []struct {
		name    string
		path    string
		baseDir string
	}{
		{"simple", "file.txt", tmpDir},
		{"nested", "subdir/file.txt", tmpDir},
		{"absolute", testFile, tmpDir},
		{"traversal", "../../../etc/passwd", tmpDir},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ValidatePath(tc.path, tc.baseDir)
			}
		})
	}
}
