package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSrtConfig(t *testing.T) {
	cwd := "/home/user/project"

	path, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := filepath.Dir(path.Name())
	path.Close()
	os.Remove(path.Name())

	configPath, err := GenerateSrtConfig(cwd)
	if err != nil {
		t.Fatalf("GenerateSrtConfig(%q) returned error: %v", cwd, err)
	}
	defer os.Remove(configPath)

	// Verify temp file is in the system temp dir
	if filepath.Dir(configPath) != tmpDir {
		t.Errorf("config file not in temp dir: got %s, want dir %s", configPath, tmpDir)
	}

	// Read and parse the config
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var cfg SrtConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON in config: %v", err)
	}

	// Verify filesystem config
	if len(cfg.Filesystem.AllowWrite) != 2 {
		t.Fatalf("expected 2 allowWrite entries, got %d", len(cfg.Filesystem.AllowWrite))
	}
	if cfg.Filesystem.AllowWrite[0] != cwd {
		t.Errorf("allowWrite[0] = %q, want %q", cfg.Filesystem.AllowWrite[0], cwd)
	}
	if cfg.Filesystem.AllowWrite[1] != "/tmp" {
		t.Errorf("allowWrite[1] = %q, want %q", cfg.Filesystem.AllowWrite[1], "/tmp")
	}

	// Verify denyRead paths are expanded (no ~ prefix)
	for i, path := range cfg.Filesystem.DenyRead {
		if path[0] == '~' {
			t.Errorf("denyRead[%d] = %q still has ~ prefix", i, path)
		}
	}
	if len(cfg.Filesystem.DenyRead) != 2 {
		t.Fatalf("expected 2 denyRead entries, got %d", len(cfg.Filesystem.DenyRead))
	}

	// Verify network config
	if len(cfg.Network.AllowedDomains) != 13 {
		t.Errorf("expected 13 allowedDomains, got %d", len(cfg.Network.AllowedDomains))
	}
	if cfg.Network.AllowedDomains[0] != "github.com" {
		t.Errorf("allowedDomains[0] = %q, want %q", cfg.Network.AllowedDomains[0], "github.com")
	}

	// Verify OAuth-related domains are present
	wantDomains := []string{"anthropic.com", "console.anthropic.com", "claude.ai", "*.claude.ai"}
	for _, want := range wantDomains {
		found := false
		for _, d := range cfg.Network.AllowedDomains {
			if d == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("allowedDomains missing %q", want)
		}
	}

	// Verify AllowLocalBinding is enabled for OAuth callback
	if !cfg.Network.AllowLocalBinding {
		t.Error("AllowLocalBinding should be true for OAuth callback support")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/.ssh", filepath.Join(home, ".ssh")},
		{"~/.aws/credentials", filepath.Join(home, ".aws/credentials")},
		{"/tmp", "/tmp"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
