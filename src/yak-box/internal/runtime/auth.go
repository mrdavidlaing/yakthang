package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// AuthMode describes how a shaver authenticates with Claude.
type AuthMode int

const (
	AuthModeNone   AuthMode = iota // no auth configured
	AuthModeAPIKey                 // _ANTHROPIC_API_KEY / ANTHROPIC_API_KEY env var
	AuthModeOAuth                  // claude login credentials in ~/.claude/
)

// AuthDetection holds the result of DetectClaudeAuth.
type AuthDetection struct {
	Mode   AuthMode
	APIKey string // non-empty only when Mode == AuthModeAPIKey
}

// DetectClaudeAuth determines which auth mode is available.
// Priority: API key env var > OAuth creds in shaverHomeDir/.claude/ > none.
// Pass shaverHomeDir="" to skip the OAuth credential check.
func DetectClaudeAuth(shaverHomeDir string, lookupEnv func(string) (string, bool)) AuthDetection {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	for _, envKey := range []string{"_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY"} {
		if key, ok := lookupEnv(envKey); ok && strings.TrimSpace(key) != "" {
			return AuthDetection{Mode: AuthModeAPIKey, APIKey: strings.TrimSpace(key)}
		}
	}
	if shaverHomeDir != "" && hasOAuthCredentials(shaverHomeDir) {
		return AuthDetection{Mode: AuthModeOAuth}
	}
	return AuthDetection{Mode: AuthModeNone}
}

// hasOAuthCredentials returns true when shaverHomeDir/.claude/ contains at least
// one non-empty JSON file (the shape Claude Code uses for OAuth token storage).
func hasOAuthCredentials(shaverHomeDir string) bool {
	claudeDir := filepath.Join(shaverHomeDir, ".claude")
	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err == nil && info.Size() > 0 {
			return true
		}
	}
	return false
}
