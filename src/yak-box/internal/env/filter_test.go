package env

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFilterSensitive(t *testing.T) {
	tests := []struct {
		name             string
		input            map[string]string
		expectedKept     []string
		expectedFiltered []string
	}{
		{
			name: "safe variables pass through",
			input: map[string]string{
				"HOME":  "/home/user",
				"PATH":  "/usr/bin:/bin",
				"USER":  "testuser",
				"SHELL": "/bin/bash",
				"TERM":  "xterm",
				"LANG":  "en_US.UTF-8",
			},
			expectedKept:     []string{"HOME", "PATH", "USER", "SHELL", "TERM", "LANG"},
			expectedFiltered: []string{},
		},
		{
			name: "PASSWORD filtered",
			input: map[string]string{
				"DATABASE_PASSWORD": "secret123",
				"HOME":              "/home/user",
			},
			expectedKept:     []string{"HOME"},
			expectedFiltered: []string{"DATABASE_PASSWORD"},
		},
		{
			name: "API_TOKEN filtered",
			input: map[string]string{
				"API_TOKEN":    "token123",
				"API_ENDPOINT": "https://api.example.com",
			},
			expectedKept:     []string{"API_ENDPOINT"},
			expectedFiltered: []string{"API_TOKEN"},
		},
		{
			name: "AWS_ACCESS_KEY_ID filtered",
			input: map[string]string{
				"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
				"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"SOME_OTHER_VAR":        "value",
			},
			expectedKept:     []string{"SOME_OTHER_VAR"},
			expectedFiltered: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		},
		{
			name: "case-insensitive password matching",
			input: map[string]string{
				"password":        "lowercase",
				"PASSWORD":        "uppercase",
				"PaSsWoRd":        "mixedcase",
				"DATABASE_PASSWD": "partial",
			},
			expectedKept:     []string{},
			expectedFiltered: []string{"password", "PASSWORD", "PaSsWoRd", "DATABASE_PASSWD"},
		},
		{
			name: "partial matches for SECRET",
			input: map[string]string{
				"MY_SECRET_KEY":    "value1",
				"SECRET_TOKEN":     "value2",
				"VERY_SECRET_HASH": "value3",
				"SECRET":           "value4",
			},
			expectedKept:     []string{},
			expectedFiltered: []string{"MY_SECRET_KEY", "SECRET_TOKEN", "VERY_SECRET_HASH", "SECRET"},
		},
		{
			name: "TOKEN filtering",
			input: map[string]string{
				"GITHUB_TOKEN":     "ghp_abc123",
				"REFRESH_TOKEN":    "refresh123",
				"TOKEN_EXPIRES_AT": "2024-12-31",
				"TOKENIZED":        "nltk",
			},
			expectedKept:     []string{"TOKENIZED"},
			expectedFiltered: []string{"GITHUB_TOKEN", "REFRESH_TOKEN", "TOKEN_EXPIRES_AT"},
		},
		{
			name: "KEY filtering",
			input: map[string]string{
				"PRIVATE_KEY":    "-----BEGIN PRIVATE KEY-----",
				"PUBLIC_KEY":     "ssh-rsa AAAA",
				"ENCRYPTION_KEY": "key123",
				"KEYBOARD":       "qwerty",
				"KEYSTONE":       "value",
			},
			expectedKept:     []string{"KEYBOARD", "KEYSTONE"},
			expectedFiltered: []string{"PRIVATE_KEY", "PUBLIC_KEY", "ENCRYPTION_KEY"},
		},
		{
			name: "CREDENTIAL filtering",
			input: map[string]string{
				"AWS_CREDENTIAL": "cred123",
				"CREDENTIALS":    "user:pass",
			},
			expectedKept:     []string{},
			expectedFiltered: []string{"AWS_CREDENTIAL", "CREDENTIALS"},
		},
		{
			name: "AUTH filtering",
			input: map[string]string{
				"AUTH_TOKEN":    "token123",
				"AUTHORIZATION": "Bearer token",
				"AUTHENTICATE":  "yes",
				"AUTHOR":        "John Doe",
			},
			expectedKept:     []string{"AUTHOR"},
			expectedFiltered: []string{"AUTH_TOKEN", "AUTHORIZATION", "AUTHENTICATE"},
		},
		{
			name: "mixed safe and sensitive",
			input: map[string]string{
				"HOME":         "/home/user",
				"PATH":         "/usr/bin",
				"DB_PASSWORD":  "pass123",
				"SHELL":        "/bin/bash",
				"API_SECRET":   "secret123",
				"LANG":         "en_US",
				"GITHUB_TOKEN": "ghp_abc",
				"USER":         "testuser",
			},
			expectedKept:     []string{"HOME", "PATH", "SHELL", "LANG", "USER"},
			expectedFiltered: []string{"DB_PASSWORD", "API_SECRET", "GITHUB_TOKEN"},
		},
		{
			name:             "empty input",
			input:            map[string]string{},
			expectedKept:     []string{},
			expectedFiltered: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			result := FilterSensitive(tt.input)

			w.Close()
			os.Stderr = oldStderr

			var stderrContent bytes.Buffer
			io.Copy(&stderrContent, r)
			stderrStr := stderrContent.String()

			for _, key := range tt.expectedKept {
				if _, exists := result[key]; !exists {
					t.Errorf("expected key %q to be kept, but it was filtered", key)
				}
			}

			for _, key := range tt.expectedFiltered {
				if _, exists := result[key]; exists {
					t.Errorf("expected key %q to be filtered, but it was kept", key)
				}
			}

			if len(tt.expectedFiltered) > 0 {
				if stderrStr == "" {
					t.Error("expected warning message for filtered variables, but got none")
				}
				if !bytes.Contains([]byte(stderrStr), []byte("Warning: filtered sensitive variables:")) {
					t.Errorf("expected warning message format, got: %q", stderrStr)
				}
			} else {
				if stderrStr != "" {
					t.Errorf("expected no warning for empty filtered set, got: %q", stderrStr)
				}
			}
		})
	}
}

func TestIsSensitive(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"PASSWORD", true},
		{"password", true},
		{"PaSsWoRd", true},
		{"MY_PASSWORD", true},
		{"PASSWORD_HASH", true},
		{"SECRET", true},
		{"API_SECRET", true},
		{"GITHUB_TOKEN", true},
		{"REFRESH_TOKEN", true},
		{"TOKEN_EXPIRES_AT", true},
		{"PRIVATE_KEY", true},
		{"PUBLIC_KEY", true},
		{"ENCRYPTION_KEY", true},
		{"AWS_ACCESS_KEY_ID", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"API_KEY", true},
		{"APIKEY", true},
		{"CREDENTIAL", true},
		{"AUTH_TOKEN", true},
		{"AUTHORIZATION", true},
		{"AUTHENTICATE", true},
		{"HOME", false},
		{"PATH", false},
		{"USER", false},
		{"SHELL", false},
		{"TERM", false},
		{"LANG", false},
		{"KEYBOARD", false},
		{"KEYSTONE", false},
		{"AUTHOR", false},
		{"TOKENIZED", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isSensitive(tt.key)
			if result != tt.expected {
				t.Errorf("isSensitive(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestFilterSensitiveEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		input            map[string]string
		expectedKept     []string
		expectedFiltered []string
	}{
		{
			name: "emoji in variable value",
			input: map[string]string{
				"EMOJI_VAR":   "🎉🎊🎈",
				"SAFE_EMOJI":  "test🚀",
				"NORMAL_SAFE": "value",
			},
			expectedKept:     []string{"EMOJI_VAR", "SAFE_EMOJI", "NORMAL_SAFE"},
			expectedFiltered: []string{},
		},
		{
			name: "extremely long variable value",
			input: map[string]string{
				"LONG_VAR":  strings.Repeat("a", 10000),
				"SHORT_VAR": "short",
			},
			expectedKept:     []string{"LONG_VAR", "SHORT_VAR"},
			expectedFiltered: []string{},
		},
		{
			name: "sql injection attempt in value",
			input: map[string]string{
				"DB_URL":   "postgres://user'; DROP TABLE users;--@localhost",
				"SAFE_VAR": "normal_value",
			},
			expectedKept:     []string{"DB_URL", "SAFE_VAR"},
			expectedFiltered: []string{},
		},
		{
			name: "command injection attempt in value",
			input: map[string]string{
				"SHELL_CMD":  "echo test; rm -rf /",
				"NORMAL_VAR": "value",
			},
			expectedKept:     []string{"SHELL_CMD", "NORMAL_VAR"},
			expectedFiltered: []string{},
		},
		{
			name: "null bytes and special characters in value",
			input: map[string]string{
				"SPECIAL_VAR": "test\x00null\x01byte",
				"NORMAL_VAR":  "value",
			},
			expectedKept:     []string{"SPECIAL_VAR", "NORMAL_VAR"},
			expectedFiltered: []string{},
		},
		{
			name: "malicious env var name",
			input: map[string]string{
				"LD_LIBRARY_PATH": "/usr/lib",
				"PATH":            "/usr/bin",
				"SAFE_PRELOAD":    "safe_value",
			},
			expectedKept:     []string{"LD_LIBRARY_PATH", "PATH", "SAFE_PRELOAD"},
			expectedFiltered: []string{},
		},
		{
			name: "mixed case token variations",
			input: map[string]string{
				"GITHUB_TOKEN": "token123",
				"TOKENIZED":    "value",
				"normal_var":   "value",
			},
			expectedKept:     []string{"TOKENIZED", "normal_var"},
			expectedFiltered: []string{"GITHUB_TOKEN"},
		},
		{
			name: "base64 encoded sensitive data",
			input: map[string]string{
				"ENCODED_TOKEN": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				"API_PASSWORD":  "base64encodedvalue",
				"SAFE_VAR":      "normal",
			},
			expectedKept:     []string{"SAFE_VAR"},
			expectedFiltered: []string{"ENCODED_TOKEN", "API_PASSWORD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stderr = w

			result := FilterSensitive(tt.input)

			w.Close()
			os.Stderr = oldStderr

			for _, key := range tt.expectedKept {
				if _, exists := result[key]; !exists {
					t.Errorf("expected key %q to be kept, but it was filtered", key)
				}
			}

			for _, key := range tt.expectedFiltered {
				if _, exists := result[key]; exists {
					t.Errorf("expected key %q to be filtered, but it was kept", key)
				}
			}
		})
	}
}

func TestFilterSensitiveWithPathTraversal(t *testing.T) {
	tests := []struct {
		name             string
		input            map[string]string
		expectedFiltered []string
	}{
		{
			name: "path traversal in value (safe)",
			input: map[string]string{
				"CONFIG_PATH": "../../../etc/passwd",
				"NORMAL":      "value",
			},
			expectedFiltered: []string{},
		},
		{
			name: "home directory path",
			input: map[string]string{
				"HOME":    "/home/user",
				"USERDIR": "/home/user/.config",
			},
			expectedFiltered: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stderr = w

			result := FilterSensitive(tt.input)

			w.Close()
			os.Stderr = oldStderr

			for _, key := range tt.expectedFiltered {
				if _, exists := result[key]; exists {
					t.Errorf("expected key %q to be filtered, but it was kept", key)
				}
			}
		})
	}
}
