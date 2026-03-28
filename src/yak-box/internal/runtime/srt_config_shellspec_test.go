package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestSrtConfigForShellspec is a Go test helper invoked by shellspec tests.
// It generates an srt config via GenerateSrtConfig (the same codepath used by
// yak-box spawn --runtime sandbox) and prints the path and validation results
// to stdout so shellspec can consume them.
//
// Set SANDBOX_CWD to the working directory that should appear in allowWrite.
// The caller is responsible for cleaning up the generated config file.
func TestSrtConfigForShellspec(t *testing.T) {
	cwd := os.Getenv("SANDBOX_CWD")
	if cwd == "" {
		t.Skip("SANDBOX_CWD not set — only runs from shellspec")
	}

	configPath, err := GenerateSrtConfig(cwd)
	if err != nil {
		t.Fatalf("GenerateSrtConfig failed: %v", err)
	}
	fmt.Printf("SRT_CONFIG_PATH=%s\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var cfg SrtConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify CWD is in allowWrite
	cwdFound := false
	for _, w := range cfg.Filesystem.AllowWrite {
		if w == cwd {
			cwdFound = true
			break
		}
	}
	if !cwdFound {
		t.Errorf("CWD %q not in allowWrite: %v", cwd, cfg.Filesystem.AllowWrite)
	}

	// Verify /tmp is in allowWrite
	tmpFound := false
	for _, w := range cfg.Filesystem.AllowWrite {
		if w == "/tmp" {
			tmpFound = true
			break
		}
	}
	if !tmpFound {
		t.Errorf("/tmp not in allowWrite: %v", cfg.Filesystem.AllowWrite)
	}

	// Verify allowedDomains has expected entries
	anthropicFound := false
	githubFound := false
	for _, d := range cfg.Network.AllowedDomains {
		if d == "api.anthropic.com" {
			anthropicFound = true
		}
		if d == "github.com" {
			githubFound = true
		}
	}
	if !anthropicFound {
		t.Error("api.anthropic.com not in allowedDomains")
	}
	if !githubFound {
		t.Error("github.com not in allowedDomains")
	}

	if !cfg.Network.AllowLocalBinding {
		t.Error("allowLocalBinding should be true")
	}

	fmt.Println("CONFIG_VALID=true")
}
