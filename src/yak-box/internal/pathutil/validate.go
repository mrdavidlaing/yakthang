// Package pathutil provides path utilities and security validation for file operations.
// This package prevents path traversal vulnerabilities (CWE-22) by implementing
// strict boundary checking on file system paths, ensuring operations remain within
// designated directories.
package pathutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathTraversal is returned when a path attempts to escape the designated boundary directory
var ErrPathTraversal = errors.New("path traversal attack detected: path attempts to escape boundary")

// ValidatePath validates that a given path is contained within a base directory
// and prevents path traversal attacks (CWE-22). The validation ensures:
// - Cleans path with filepath.Clean() to normalize
// - Converts relative paths to absolute by joining with baseDir
// - Resolves symbolic links with filepath.EvalSymlinks()
// - Verifies resolved path starts with baseDir boundary (with separator)
//
// Returns nil if path is valid, ErrPathTraversal if escaping boundary, or other errors on resolution failure.
func ValidatePath(path, baseDir string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if baseDir == "" {
		return fmt.Errorf("baseDir cannot be empty")
	}

	cleanPath := filepath.Clean(path)

	absBaseDir := filepath.Clean(baseDir)
	if !filepath.IsAbs(absBaseDir) {
		var err error
		absBaseDir, err = filepath.Abs(absBaseDir)
		if err != nil {
			return fmt.Errorf("cannot resolve baseDir to absolute path: %w", err)
		}
	}

	var absPath string
	if !filepath.IsAbs(cleanPath) {
		absPath = filepath.Join(absBaseDir, cleanPath)
	} else {
		absPath = cleanPath
	}

	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolvedPath = absPath
	}

	if baseInfo, err := os.Stat(absBaseDir); err == nil && (baseInfo.Mode()&os.ModeSymlink) != 0 {
		if resolved, err := filepath.EvalSymlinks(absBaseDir); err == nil {
			absBaseDir = resolved
		}
	}

	absBaseDir = filepath.Clean(absBaseDir)
	resolvedPath = filepath.Clean(resolvedPath)

	if resolvedPath == absBaseDir {
		return nil
	}

	if !strings.HasPrefix(resolvedPath, absBaseDir+string(filepath.Separator)) {
		return fmt.Errorf("%w: %s is not within %s", ErrPathTraversal, path, baseDir)
	}

	return nil
}
