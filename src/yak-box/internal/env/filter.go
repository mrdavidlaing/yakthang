// Package env provides environment variable utilities for secure handling of sensitive data.
// This package includes filtering capabilities to prevent sensitive variables (passwords, secrets, tokens)
// from being leaked through logs, environment inspection, or other observability mechanisms.
package env

import (
	"fmt"
	"os"
	"strings"
)

// sensitivePatterns defines the patterns used to identify sensitive environment variables.
// Variables matching any of these patterns (case-insensitive) will be filtered out.
var sensitivePatterns = []string{
	"PASSWORD",
	"PASSWD",
	"PWD",
	"SECRET",
	"_TOKEN",
	"TOKEN_",
	"API_KEY",
	"APIKEY",
	"PRIVATE_KEY",
	"AWS_",
	"AMAZON",
	"_KEY",
	"KEY_",
	"CREDENTIAL",
	"_AUTH",
	"AUTH_",
	"AUTHORIZATION",
	"AUTHENTICATE",
}

// FilterSensitive removes environment variables matching sensitive patterns from the input map.
// It returns a new map containing only the non-sensitive variables.
// When sensitive variables are found, a warning is printed to stderr listing the filtered variable names.
//
// The filtering is case-insensitive: PASSWORD, password, PaSsWoRd will all be filtered.
// Patterns support partial matching: MY_SECRET_KEY matches the SECRET pattern.
func FilterSensitive(envVars map[string]string) map[string]string {
	filtered := make(map[string]string)
	var filteredKeys []string

	for key, value := range envVars {
		if isSensitive(key) {
			filteredKeys = append(filteredKeys, key)
		} else {
			filtered[key] = value
		}
	}

	// Print warning if any variables were filtered
	if len(filteredKeys) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: filtered sensitive variables: %v\n", filteredKeys)
	}

	return filtered
}

// isSensitive checks if an environment variable key matches any of the sensitive patterns.
// The check is case-insensitive and uses substring matching.
func isSensitive(key string) bool {
	upperKey := strings.ToUpper(key)

	for _, pattern := range sensitivePatterns {
		if strings.Contains(upperKey, pattern) {
			return true
		}
	}

	return false
}
