package devcontainer

import (
	"path/filepath"
	"strings"

	"github.com/mrdavidlaing/yakthang/src/yak-box/internal/pathutil"
)

// Severity represents the severity level of a security warning.
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// SecurityWarning represents a security-related warning about devcontainer configuration
type SecurityWarning struct {
	Severity Severity
	Message  string
}

// Dangerous Linux capabilities that could bypass container isolation
var dangerousCapabilities = map[string]bool{
	"SYS_ADMIN":    true, // Can perform many privileged operations
	"SYS_PTRACE":   true, // Can trace processes, attach debuggers
	"SYS_MODULE":   true, // Can load/unload kernel modules
	"SYS_RAWIO":    true, // Can perform raw I/O operations
	"NET_ADMIN":    true, // Can administer the network
	"SYS_TIME":     true, // Can set system time
	"SYS_BOOT":     true, // Can use reboot and kexec
	"SYS_RESOURCE": true, // Can override resource limits
}

// Dangerous security options that weaken container isolation
var dangerousSecurityOpts = []string{
	"seccomp=unconfined",  // Disables seccomp filtering
	"apparmor=unconfined", // Disables AppArmor confinement
	"label=disable",       // Disables SELinux labels
}

// ValidateSecurityConfig checks the devcontainer configuration for security issues
// and returns a list of warnings (non-blocking - configuration can proceed)
func ValidateSecurityConfig(cfg *Config) []SecurityWarning {
	var warnings []SecurityWarning

	if cfg == nil {
		return warnings
	}

	if cfg.Privileged != nil && *cfg.Privileged {
		warnings = append(warnings, SecurityWarning{
			Severity: SeverityCritical,
			Message:  "Container is running in privileged mode, which disables most security isolation",
		})
	}

	for _, cap := range cfg.CapAdd {
		if dangerousCapabilities[cap] {
			warnings = append(warnings, SecurityWarning{
				Severity: SeverityCritical,
				Message:  "Dangerous capability requested: " + cap + " - this can bypass container isolation",
			})
		}
	}

	for _, opt := range cfg.SecurityOpt {
		for _, dangerous := range dangerousSecurityOpts {
			if opt == dangerous {
				warnings = append(warnings, SecurityWarning{
					Severity: SeverityCritical,
					Message:  "Dangerous security option: " + opt + " - this weakens container isolation",
				})
				break
			}
		}
	}

	validateMountPaths(cfg, &warnings)

	return warnings
}

func validateMountPaths(cfg *Config, warnings *[]SecurityWarning) {
	if len(cfg.Mounts) == 0 {
		return
	}

	for _, mount := range cfg.Mounts {
		source := extractMountSource(mount)
		if source == "" {
			continue
		}

		if !filepath.IsAbs(source) {
			continue
		}

		if err := pathutil.ValidatePath(source, "/"); err != nil {
			continue
		}
	}
}

func extractMountSource(mount string) string {
	parts := strings.Split(mount, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, "source=") {
			return strings.TrimPrefix(part, "source=")
		}
	}
	return ""
}
