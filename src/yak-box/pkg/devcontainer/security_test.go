package devcontainer

import (
	"testing"
)

func TestValidateSecurityConfigSafe(t *testing.T) {
	cfg := &Config{
		Image:      "ubuntu:22.04",
		RemoteUser: "user",
	}

	warnings := ValidateSecurityConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for safe config, got %d warnings", len(warnings))
	}
}

func TestValidateSecurityConfigPrivileged(t *testing.T) {
	privilegedTrue := true
	cfg := &Config{
		Image:      "ubuntu:22.04",
		Privileged: &privilegedTrue,
	}

	warnings := ValidateSecurityConfig(cfg)
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 warning for privileged mode, got %d warnings", len(warnings))
	}

	if warnings[0].Severity != "critical" {
		t.Errorf("Expected critical severity, got %s", warnings[0].Severity)
	}

	if len(warnings[0].Message) == 0 {
		t.Error("Expected warning message to be non-empty")
	}
}

func TestValidateSecurityConfigPrivilegedFalse(t *testing.T) {
	privilegedFalse := false
	cfg := &Config{
		Image:      "ubuntu:22.04",
		Privileged: &privilegedFalse,
	}

	warnings := ValidateSecurityConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings when privileged=false, got %d warnings", len(warnings))
	}
}

func TestValidateSecurityConfigDangerousCapability(t *testing.T) {
	tests := []struct {
		name   string
		capAdd []string
		want   int
	}{
		{
			name:   "SYS_ADMIN",
			capAdd: []string{"SYS_ADMIN"},
			want:   1,
		},
		{
			name:   "SYS_PTRACE",
			capAdd: []string{"SYS_PTRACE"},
			want:   1,
		},
		{
			name:   "SYS_MODULE",
			capAdd: []string{"SYS_MODULE"},
			want:   1,
		},
		{
			name:   "SYS_RAWIO",
			capAdd: []string{"SYS_RAWIO"},
			want:   1,
		},
		{
			name:   "NET_ADMIN",
			capAdd: []string{"NET_ADMIN"},
			want:   1,
		},
		{
			name:   "SYS_TIME",
			capAdd: []string{"SYS_TIME"},
			want:   1,
		},
		{
			name:   "SYS_BOOT",
			capAdd: []string{"SYS_BOOT"},
			want:   1,
		},
		{
			name:   "SYS_RESOURCE",
			capAdd: []string{"SYS_RESOURCE"},
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Image:  "ubuntu:22.04",
				CapAdd: tt.capAdd,
			}

			warnings := ValidateSecurityConfig(cfg)
			if len(warnings) != tt.want {
				t.Errorf("Expected %d warning, got %d", tt.want, len(warnings))
			}

			if len(warnings) > 0 && warnings[0].Severity != "critical" {
				t.Errorf("Expected critical severity, got %s", warnings[0].Severity)
			}
		})
	}
}

func TestValidateSecurityConfigSafeCapability(t *testing.T) {
	cfg := &Config{
		Image:  "ubuntu:22.04",
		CapAdd: []string{"NET_BIND_SERVICE", "CHOWN"},
	}

	warnings := ValidateSecurityConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for safe capabilities, got %d warnings", len(warnings))
	}
}

func TestValidateSecurityConfigDangerousSecurityOpt(t *testing.T) {
	tests := []struct {
		name        string
		securityOpt []string
		want        int
	}{
		{
			name:        "seccomp=unconfined",
			securityOpt: []string{"seccomp=unconfined"},
			want:        1,
		},
		{
			name:        "apparmor=unconfined",
			securityOpt: []string{"apparmor=unconfined"},
			want:        1,
		},
		{
			name:        "label=disable",
			securityOpt: []string{"label=disable"},
			want:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Image:       "ubuntu:22.04",
				SecurityOpt: tt.securityOpt,
			}

			warnings := ValidateSecurityConfig(cfg)
			if len(warnings) != tt.want {
				t.Errorf("Expected %d warning, got %d", tt.want, len(warnings))
			}

			if len(warnings) > 0 && warnings[0].Severity != "critical" {
				t.Errorf("Expected critical severity, got %s", warnings[0].Severity)
			}
		})
	}
}

func TestValidateSecurityConfigSafeSecurityOpt(t *testing.T) {
	cfg := &Config{
		Image:       "ubuntu:22.04",
		SecurityOpt: []string{"no-new-privileges=true"},
	}

	warnings := ValidateSecurityConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for safe security options, got %d warnings", len(warnings))
	}
}

func TestValidateSecurityConfigMultipleWarnings(t *testing.T) {
	privilegedTrue := true
	cfg := &Config{
		Image:       "ubuntu:22.04",
		Privileged:  &privilegedTrue,
		CapAdd:      []string{"SYS_ADMIN", "NET_ADMIN"},
		SecurityOpt: []string{"seccomp=unconfined"},
	}

	warnings := ValidateSecurityConfig(cfg)

	if len(warnings) != 4 {
		t.Fatalf("Expected 4 warnings, got %d", len(warnings))
	}

	for _, warning := range warnings {
		if warning.Severity != "critical" {
			t.Errorf("Expected all warnings to be critical, got %s", warning.Severity)
		}
		if len(warning.Message) == 0 {
			t.Error("Expected warning message to be non-empty")
		}
	}
}

func TestValidateSecurityConfigNilConfig(t *testing.T) {
	warnings := ValidateSecurityConfig(nil)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for nil config, got %d warnings", len(warnings))
	}
}

func TestValidateSecurityConfigEmptyConfig(t *testing.T) {
	cfg := &Config{}
	warnings := ValidateSecurityConfig(cfg)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for empty config, got %d warnings", len(warnings))
	}
}
