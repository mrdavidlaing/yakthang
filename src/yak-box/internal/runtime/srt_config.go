package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SrtFilesystemConfig defines filesystem access rules for the srt sandbox.
type SrtFilesystemConfig struct {
	AllowWrite []string `json:"allowWrite"`
	DenyWrite  []string `json:"denyWrite"`
	DenyRead   []string `json:"denyRead"`
	AllowRead  []string `json:"allowRead"`
}

// SrtNetworkConfig defines network access rules for the srt sandbox.
type SrtNetworkConfig struct {
	AllowedDomains    []string `json:"allowedDomains"`
	DeniedDomains     []string `json:"deniedDomains"`
	AllowLocalBinding bool     `json:"allowLocalBinding"`
}

// SrtConfig is the top-level srt sandbox configuration.
type SrtConfig struct {
	Filesystem SrtFilesystemConfig `json:"filesystem"`
	Network    SrtNetworkConfig    `json:"network"`
}

// expandHome replaces a leading ~ with the actual home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// toolchainCacheDirs returns common toolchain cache directories that sandbox
// workers need write access to. Uses environment variables where available,
// falling back to conventional paths under ~.
func toolchainCacheDirs() []string {
	dirs := []string{
		envOrDefault("GOPATH", "~/go"),
		envOrDefault("CARGO_HOME", "~/.cargo"),
		"~/.cache",
		"~/.local/share/mise",
		envOrDefault("RUSTUP_HOME", "~/.rustup"),
		"~/.bun",
	}
	expanded := make([]string, len(dirs))
	for i, d := range dirs {
		expanded[i] = expandHome(d)
	}
	return expanded
}

// envOrDefault returns the value of the named environment variable, or the
// fallback if the variable is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GenerateSrtConfig builds a hardcoded JSON config for srt and writes it to a
// temp file. The caller is responsible for removing the file when done.
// Returns the path to the temp file.
func GenerateSrtConfig(cwd string) (string, error) {
	allowWrite := append([]string{cwd, "/tmp"}, toolchainCacheDirs()...)
	cfg := SrtConfig{
		Filesystem: SrtFilesystemConfig{
			AllowWrite: allowWrite,
			DenyWrite:  []string{},
			DenyRead:   []string{expandHome("~/.ssh"), expandHome("~/.aws/credentials")},
			AllowRead:  []string{},
		},
		Network: SrtNetworkConfig{
			AllowedDomains: []string{
				"github.com", "*.github.com",
				"api.anthropic.com", "*.anthropic.com",
				"anthropic.com", "console.anthropic.com",
				"claude.ai", "*.claude.ai",
				"registry.npmjs.org",
				"crates.io", "static.crates.io",
				"proxy.golang.org", "sum.golang.org",
			},
			DeniedDomains:     []string{},
			AllowLocalBinding: true,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal srt config: %w", err)
	}

	f, err := os.CreateTemp("", "srt-config-*.json")
	if err != nil {
		return "", fmt.Errorf("create srt config temp file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write srt config: %w", err)
	}

	return f.Name(), nil
}
