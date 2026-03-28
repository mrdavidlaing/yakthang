package devcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LockedFeature represents a pinned feature version in devcontainer-lock.json
type LockedFeature struct {
	Version  string `json:"version"`  // Semantic version of the feature
	Resolved string `json:"resolved"` // Full OCI ref with digest or version
}

// LockFile represents devcontainer-lock.json which pins feature versions
type LockFile struct {
	Features map[string]LockedFeature `json:"features"`
}

// PortAttributes represents attributes for a specific port
type PortAttributes struct {
	Label            string `json:"label,omitempty"`            // User-visible label for the port
	Protocol         string `json:"protocol,omitempty"`         // http or https
	OnAutoForward    string `json:"onAutoForward,omitempty"`    // notify, openBrowser, openBrowserOnce, openPreview, silent, ignore
	RequireLocalPort *bool  `json:"requireLocalPort,omitempty"` // Require this specific local port (fail if unavailable)
	ElevateIfNeeded  *bool  `json:"elevateIfNeeded,omitempty"`  // Elevate permissions if needed to bind port
}

// HostRequirements represents minimum host system requirements (advisory only)
type HostRequirements struct {
	Cpus    *int    `json:"cpus,omitempty"`    // Minimum CPU cores
	Memory  *string `json:"memory,omitempty"`  // Minimum RAM (e.g., "8gb")
	Storage *string `json:"storage,omitempty"` // Minimum disk space (e.g., "32gb")
	Gpu     *bool   `json:"gpu,omitempty"`     // Requires GPU
}

// Config represents a parsed devcontainer.json.
// The full spec is accepted for compatibility; the yak-box runtime currently honors only:
// Image, Mounts, ContainerEnv, and RemoteEnv (via GetResolvedEnvironment).
// Other fields (RunArgs, Privileged, CapAdd, SecurityOpt, ContainerUser, WorkspaceMount,
// lifecycle commands, etc.) are parsed but not applied when generating the worker run script.
type Config struct {
	// Basic container configuration
	Image                       string                    `json:"image"`
	DockerFile                  string                    `json:"dockerFile"`
	Build                       *BuildConfig              `json:"build,omitempty"`
	Name                        string                    `json:"name,omitempty"`                // Display name for the dev container
	ContainerUser               string                    `json:"containerUser,omitempty"`       // User for container operations (docker run --user)
	RemoteUser                  string                    `json:"remoteUser"`                    // User for remote operations (docker exec --user)
	UpdateRemoteUserUID         bool                      `json:"updateRemoteUserUID,omitempty"` // Sync container user UID/GID to match host (Linux only)
	UserEnvProbe                string                    `json:"userEnvProbe,omitempty"`        // Shell type for environment probing: none, loginShell, interactiveShell, loginInteractiveShell
	ContainerEnv                map[string]string         `json:"containerEnv,omitempty"`
	RemoteEnv                   map[string]string         `json:"remoteEnv,omitempty"`
	ForwardPorts                []interface{}             `json:"forwardPorts,omitempty"`         // int or string
	PortsAttributes             map[string]PortAttributes `json:"portsAttributes,omitempty"`      // Port-specific metadata
	OtherPortsAttributes        PortAttributes            `json:"otherPortsAttributes,omitempty"` // Default attributes for ports not in portsAttributes
	Mounts                      []string                  `json:"mounts,omitempty"`               // Docker mount syntax
	RunArgs                     []string                  `json:"runArgs,omitempty"`              // Additional docker run arguments
	Features                    map[string]interface{}    `json:"features,omitempty"`
	OverrideFeatureInstallOrder []string                  `json:"overrideFeatureInstallOrder,omitempty"` // Manual feature installation order (overrides dependency resolution)

	// Security properties - can be set directly in devcontainer.json or via features
	Privileged  *bool    `json:"privileged,omitempty"`  // Run container in privileged mode
	Init        *bool    `json:"init,omitempty"`        // Add tiny init process to reap zombie processes
	CapAdd      []string `json:"capAdd,omitempty"`      // Linux capabilities to add
	SecurityOpt []string `json:"securityOpt,omitempty"` // Security options (e.g., seccomp, apparmor)
	Entrypoint  []string `json:"-"`                     // Container entrypoint (custom unmarshaling handles string or array)

	// Docker Compose orchestration (alternative to image/dockerfile)
	DockerComposeFile interface{} `json:"dockerComposeFile,omitempty"` // string or []string - path(s) to compose file(s)
	Service           string      `json:"service,omitempty"`           // Service name to connect to
	RunServices       []string    `json:"runServices,omitempty"`       // Services to start (empty = all services)

	// Workspace configuration - CRITICAL for proper workspace setup
	WorkspaceFolder string `json:"workspaceFolder,omitempty"` // Path inside container where workspace should be
	WorkspaceMount  string `json:"workspaceMount,omitempty"`  // Custom mount string for workspace

	// Lifecycle commands - complete Microsoft specification
	InitializeCommand    *LifecycleCommand `json:"initializeCommand,omitempty"` // Runs on host before container creation
	OnCreateCommand      *LifecycleCommand `json:"onCreateCommand,omitempty"`
	UpdateContentCommand *LifecycleCommand `json:"updateContentCommand,omitempty"`
	PostCreateCommand    *LifecycleCommand `json:"postCreateCommand,omitempty"`
	PostStartCommand     *LifecycleCommand `json:"postStartCommand,omitempty"`
	PostAttachCommand    *LifecycleCommand `json:"postAttachCommand,omitempty"` // Runs every time IDE attaches

	// Lifecycle control
	WaitFor         string `json:"waitFor,omitempty"`         // Which lifecycle command to wait for before setup is complete
	OverrideCommand *bool  `json:"overrideCommand,omitempty"` // Whether to override container CMD with user command (default: true)
	ShutdownAction  string `json:"shutdownAction,omitempty"`  // What to do on exit: none (default), stopContainer, stopCompose

	// Host requirements (advisory validation only)
	HostRequirements *HostRequirements `json:"hostRequirements,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to handle entrypoint which can be string or array
func (c *Config) UnmarshalJSON(data []byte) error {
	// Use Alias + embedding so all fields except Entrypoint are populated directly on c
	type Alias Config
	aux := &struct {
		EntrypointRaw json.RawMessage `json:"entrypoint,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Handle entrypoint field specially - it can be string or array
	if len(aux.EntrypointRaw) > 0 {
		var entrypointStr string
		if err := json.Unmarshal(aux.EntrypointRaw, &entrypointStr); err == nil {
			c.Entrypoint = []string{entrypointStr}
		} else {
			var entrypointArr []string
			if err := json.Unmarshal(aux.EntrypointRaw, &entrypointArr); err != nil {
				return fmt.Errorf("entrypoint must be either a string or an array of strings: %w", err)
			}
			c.Entrypoint = entrypointArr
		}
	}

	return nil
}

// LoadConfig loads and parses .devcontainer/devcontainer.json if it exists.
// It returns the config, any security warnings (caller decides how to present them), and an error.
// Parsing and validation are separate from UI; callers may log or display warnings as needed.
func LoadConfig(projectPath string) (*Config, []SecurityWarning, error) {
	configPath := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, err
	}

	if config.RemoteUser == "" {
		config.RemoteUser = "root"
	}

	warnings := ValidateSecurityConfig(&config)
	return &config, warnings, nil
}

// GetDefaultConfig returns the default devcontainer config
// If defaultImage is empty, uses "yak-worker:latest"
func GetDefaultConfig(defaultImage string) *Config {
	if defaultImage == "" {
		defaultImage = "yak-worker:latest"
	}

	return &Config{
		Image:      defaultImage,
		RemoteUser: "root",
	}
}

// GetDockerfile returns the dockerfile path from either DockerFile field or Build.Dockerfile
func (c *Config) GetDockerfile() string {
	if c.Build != nil && c.Build.Dockerfile != "" {
		return c.Build.Dockerfile
	}
	return c.DockerFile
}

// HasDockerfile returns true if a dockerfile is specified
func (c *Config) HasDockerfile() bool {
	return c.GetDockerfile() != ""
}

// GetDockerComposeFiles returns dockerComposeFile as a string slice
// Handles both string and []string JSON values
func (c *Config) GetDockerComposeFiles() []string {
	if c.DockerComposeFile == nil {
		return nil
	}

	switch v := c.DockerComposeFile.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

// ShouldOverrideCommand returns whether to override the container's CMD with user command
// Returns true by default (when OverrideCommand is nil or true)
func (c *Config) ShouldOverrideCommand() bool {
	if c.OverrideCommand == nil {
		return true // default behavior
	}
	return *c.OverrideCommand
}

// GetResolvedEnvironment applies variable substitution and returns resolved environment variables
// First applies containerEnv, then remoteEnv (which can reference containerEnv)
func (c *Config) GetResolvedEnvironment(ctx *SubstituteContext) map[string]string {
	result := make(map[string]string)

	// First pass: containerEnv
	for k, v := range c.ContainerEnv {
		resolved := substituteString(ctx, v)
		result[k] = resolved
		// Add to context for containerEnv: references
		ctx.ContainerEnv[k] = resolved
	}

	// Second pass: remoteEnv (can reference containerEnv)
	for k, v := range c.RemoteEnv {
		if v == "" {
			// Empty string/null removes variable
			delete(result, k)
		} else {
			result[k] = substituteString(ctx, v)
		}
	}

	return result
}

// LoadLockFile loads and parses .devcontainer/devcontainer-lock.json if it exists
// Returns nil if the lockfile doesn't exist (not an error)
func LoadLockFile(projectPath string) (*LockFile, error) {
	lockPath := filepath.Join(projectPath, ".devcontainer", "devcontainer-lock.json")

	// Check if file exists
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		return nil, nil // No lockfile is not an error
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}

	var lockfile LockFile
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return nil, err
	}

	return &lockfile, nil
}

// GetPortAttributes returns the port attributes for a given port
// If the port is explicitly defined in portsAttributes, returns those attributes
// Otherwise, returns otherPortsAttributes (which may be empty)
func (c *Config) GetPortAttributes(port string) PortAttributes {
	// Check if this port has explicit attributes
	if attrs, exists := c.PortsAttributes[port]; exists {
		return attrs
	}

	// Return otherPortsAttributes as default
	return c.OtherPortsAttributes
}
