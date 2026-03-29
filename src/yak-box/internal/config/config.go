// Package config provides configuration management for yak-box.
package config

import (
	"os"
	"path/filepath"

	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/workspace"
)

type Config struct {
	WorkspaceRoot string
	YakPath       string
	MetadataDir   string
}

// LoadConfig loads the yak-box configuration from the workspace.
func LoadConfig() (*Config, error) {
	root, err := workspace.FindRoot()
	if err != nil {
		return nil, err
	}

	yakPath := os.Getenv("YAK_PATH")
	if yakPath == "" {
		yakPath = filepath.Join(root, ".yaks")
	}

	return &Config{
		WorkspaceRoot: root,
		YakPath:       yakPath,
		MetadataDir:   filepath.Join(root, ".yak-boxes"),
	}, nil
}
