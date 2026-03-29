package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/runtime"
)

func noEnv(string) (string, bool) { return "", false }

func envWith(key, val string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		if k == key {
			return val, true
		}
		return "", false
	}
}

func TestDetectClaudeAuth_APIKey(t *testing.T) {
	d := runtime.DetectClaudeAuth("", envWith("_ANTHROPIC_API_KEY", "sk-ant-test"))
	if d.Mode != runtime.AuthModeAPIKey {
		t.Fatalf("expected AuthModeAPIKey, got %v", d.Mode)
	}
	if d.APIKey != "sk-ant-test" {
		t.Errorf("expected key 'sk-ant-test', got %q", d.APIKey)
	}
}

func TestDetectClaudeAuth_AnthropicAPIKeyFallback(t *testing.T) {
	d := runtime.DetectClaudeAuth("", envWith("ANTHROPIC_API_KEY", "sk-ant-fallback"))
	if d.Mode != runtime.AuthModeAPIKey {
		t.Fatalf("expected AuthModeAPIKey, got %v", d.Mode)
	}
}

func TestDetectClaudeAuth_OAuthCreds(t *testing.T) {
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "credentials.json"), []byte(`{"tok":"x"}`), 0600); err != nil {
		t.Fatal(err)
	}

	d := runtime.DetectClaudeAuth(tmpHome, noEnv)
	if d.Mode != runtime.AuthModeOAuth {
		t.Fatalf("expected AuthModeOAuth, got %v", d.Mode)
	}
}

func TestDetectClaudeAuth_EmptyJSONIgnored(t *testing.T) {
	// A zero-byte JSON file should not count as valid OAuth credentials.
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "credentials.json"), []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	d := runtime.DetectClaudeAuth(tmpHome, noEnv)
	if d.Mode != runtime.AuthModeNone {
		t.Fatalf("expected AuthModeNone for empty file, got %v", d.Mode)
	}
}

func TestDetectClaudeAuth_None(t *testing.T) {
	d := runtime.DetectClaudeAuth("", noEnv)
	if d.Mode != runtime.AuthModeNone {
		t.Fatalf("expected AuthModeNone, got %v", d.Mode)
	}
}

func TestDetectClaudeAuth_APIKeyTakesPriorityOverOAuth(t *testing.T) {
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "credentials.json"), []byte(`{"tok":"x"}`), 0600); err != nil {
		t.Fatal(err)
	}

	d := runtime.DetectClaudeAuth(tmpHome, envWith("_ANTHROPIC_API_KEY", "sk-ant-key"))
	if d.Mode != runtime.AuthModeAPIKey {
		t.Fatalf("API key should take priority over OAuth, got %v", d.Mode)
	}
}
