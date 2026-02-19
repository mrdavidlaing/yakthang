package devcontainer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `{
		"image": "mcr.microsoft.com/devcontainers/go:1.21",
		"containerEnv": {
			"TEST_VAR": "test_value"
		},
		"mounts": [
			"source=/tmp,target=/tmp,type=bind"
		]
	}`

	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if config.Image != "mcr.microsoft.com/devcontainers/go:1.21" {
		t.Errorf("Expected image mcr.microsoft.com/devcontainers/go:1.21, got %s", config.Image)
	}

	if config.ContainerEnv["TEST_VAR"] != "test_value" {
		t.Errorf("Expected TEST_VAR=test_value, got %s", config.ContainerEnv["TEST_VAR"])
	}

	if len(config.Mounts) != 1 {
		t.Errorf("Expected 1 mount, got %d", len(config.Mounts))
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config != nil {
		t.Error("Expected config to be nil for non-existent devcontainer.json")
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig("")
	if config.Image != "yak-shaver:latest" {
		t.Errorf("Expected default image yak-shaver:latest, got %s", config.Image)
	}

	if config.RemoteUser != "root" {
		t.Errorf("Expected default remote user root, got %s", config.RemoteUser)
	}
}

func TestGetResolvedEnvironment(t *testing.T) {
	config := &Config{
		ContainerEnv: map[string]string{
			"VAR1": "value1",
			"VAR2": "${localEnv:HOME}/test",
		},
		RemoteEnv: map[string]string{
			"VAR3": "${containerEnv:VAR1}",
		},
	}

	ctx := &SubstituteContext{
		LocalWorkspaceFolder:     "/workspace",
		ContainerWorkspaceFolder: "/workspace",
		LocalEnv: map[string]string{
			"HOME": "/home/user",
		},
		ContainerEnv: make(map[string]string),
	}

	resolved := config.GetResolvedEnvironment(ctx)

	if resolved["VAR1"] != "value1" {
		t.Errorf("Expected VAR1=value1, got %s", resolved["VAR1"])
	}

	if resolved["VAR2"] != "/home/user/test" {
		t.Errorf("Expected VAR2=/home/user/test, got %s", resolved["VAR2"])
	}

	if resolved["VAR3"] != "value1" {
		t.Errorf("Expected VAR3=value1, got %s", resolved["VAR3"])
	}
}

func TestLoadConfigCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	corruptContent := `{"image": "test", "mounts": [invalid]}`
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(corruptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err == nil {
		t.Fatalf("LoadConfig should fail with corrupt JSON, got nil error")
	}
	if config != nil {
		t.Error("Expected config to be nil on corrupt JSON")
	}
}

func TestLoadConfigMissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	minimalContent := `{}`
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(minimalContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config == nil {
		t.Error("Expected config to be non-nil for minimal JSON")
	}
}

func TestLoadConfigInvalidTypes(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidTypeContent := `{"image": 123, "containerEnv": "not-a-map"}`
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(invalidTypeContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err == nil {
		t.Fatalf("LoadConfig should fail with invalid types, got nil error")
	}
	if config != nil {
		t.Error("Expected config to be nil on invalid types")
	}
}

func TestLoadConfigMalformedMounts(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	malformedContent := `{
		"image": "test",
		"mounts": ["not-enough-fields", "source=/tmp,target=/tmp"]
	}`
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(malformedContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Logf("LoadConfig with malformed mounts returned error (acceptable): %v", err)
	}
	if config != nil && len(config.Mounts) > 0 {
		t.Logf("Config loaded with malformed mounts: %v", config.Mounts)
	}
}

func TestLoadConfigNullValues(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	nullContent := `{"image": null, "containerEnv": null}`
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(nullContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Logf("LoadConfig with null values returned error: %v", err)
	}
	if config != nil {
		t.Logf("Config loaded with null values: image=%q", config.Image)
	}
}
