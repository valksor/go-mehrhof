// Package plugin provides runtime extensibility for providers, agents, and workflows.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PluginType identifies the category of plugin.
type PluginType string

const (
	PluginTypeProvider PluginType = "provider"
	PluginTypeAgent    PluginType = "agent"
	PluginTypeWorkflow PluginType = "workflow"
)

// Manifest describes a plugin's metadata and configuration.
type Manifest struct {
	// Core metadata
	Version     string     `yaml:"version"`
	Name        string     `yaml:"name"`
	Type        PluginType `yaml:"type"`
	Description string     `yaml:"description"`
	Author      string     `yaml:"author,omitempty"`
	Homepage    string     `yaml:"homepage,omitempty"`

	// Version requirement for Mehrhof
	Requires string `yaml:"requires,omitempty"`

	// Protocol version for JSON-RPC communication
	Protocol string `yaml:"protocol"`

	// Executable configuration
	Executable ExecutableConfig `yaml:"executable"`

	// Type-specific configuration (only one should be set)
	Provider *ProviderConfig `yaml:"provider,omitempty"`
	Agent    *AgentConfig    `yaml:"agent,omitempty"`
	Workflow *WorkflowConfig `yaml:"workflow,omitempty"`

	// Environment variables the plugin expects (documentation)
	Env map[string]EnvVarSpec `yaml:"env,omitempty"`

	// Runtime fields (not from YAML)
	Dir   string `yaml:"-"` // Directory containing the plugin
	Scope string `yaml:"-"` // "global" or "project"
}

// ExecutableConfig describes how to run the plugin.
type ExecutableConfig struct {
	// Path to executable (relative to plugin dir or absolute)
	Path string `yaml:"path,omitempty"`

	// Command with arguments (alternative to Path)
	Command []string `yaml:"command,omitempty"`
}

// ProviderConfig contains provider-specific configuration.
type ProviderConfig struct {
	Name         string   `yaml:"name"`
	Schemes      []string `yaml:"schemes"`
	Priority     int      `yaml:"priority"`
	Capabilities []string `yaml:"capabilities"`
}

// AgentConfig contains agent-specific configuration.
type AgentConfig struct {
	Name         string   `yaml:"name"`
	Streaming    bool     `yaml:"streaming"`
	Capabilities []string `yaml:"capabilities,omitempty"`
}

// WorkflowConfig contains workflow extension configuration.
type WorkflowConfig struct {
	Phases  []PhaseConfig  `yaml:"phases,omitempty"`
	Guards  []GuardConfig  `yaml:"guards,omitempty"`
	Effects []EffectConfig `yaml:"effects,omitempty"`
}

// PhaseConfig describes a custom workflow phase.
type PhaseConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	After       string `yaml:"after,omitempty"`  // Insert after this phase
	Before      string `yaml:"before,omitempty"` // Insert before this phase
}

// GuardConfig describes a custom workflow guard.
type GuardConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// EffectConfig describes a custom workflow effect.
type EffectConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Critical    bool   `yaml:"critical,omitempty"` // If true, effect failure blocks transition
}

// EnvVarSpec documents an expected environment variable.
type EnvVarSpec struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
}

// LoadManifest reads and parses a plugin manifest from the given path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	m.Dir = filepath.Dir(path)
	return &m, nil
}

// Validate checks that the manifest has required fields and is internally consistent.
func (m *Manifest) Validate() error {
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Type == "" {
		return fmt.Errorf("type is required")
	}
	if m.Protocol == "" {
		return fmt.Errorf("protocol is required")
	}

	// Validate executable configuration
	if m.Executable.Path == "" && len(m.Executable.Command) == 0 {
		return fmt.Errorf("executable.path or executable.command is required")
	}

	// Validate type matches configuration
	switch m.Type {
	case PluginTypeProvider:
		if m.Provider == nil {
			return fmt.Errorf("provider configuration required for provider plugin")
		}
		if m.Provider.Name == "" {
			return fmt.Errorf("provider.name is required")
		}
		if len(m.Provider.Schemes) == 0 {
			return fmt.Errorf("provider.schemes is required")
		}
	case PluginTypeAgent:
		if m.Agent == nil {
			return fmt.Errorf("agent configuration required for agent plugin")
		}
		if m.Agent.Name == "" {
			return fmt.Errorf("agent.name is required")
		}
	case PluginTypeWorkflow:
		if m.Workflow == nil {
			return fmt.Errorf("workflow configuration required for workflow plugin")
		}
	default:
		return fmt.Errorf("invalid plugin type: %s", m.Type)
	}

	return nil
}

// ExecutablePath returns the resolved path to the plugin executable.
func (m *Manifest) ExecutablePath() string {
	if m.Executable.Path == "" {
		return ""
	}
	if filepath.IsAbs(m.Executable.Path) {
		return m.Executable.Path
	}
	return filepath.Join(m.Dir, m.Executable.Path)
}

// ExecutableCommand returns the command and arguments to run the plugin.
func (m *Manifest) ExecutableCommand() []string {
	if len(m.Executable.Command) > 0 {
		// Resolve relative paths in command
		cmd := make([]string, len(m.Executable.Command))
		copy(cmd, m.Executable.Command)
		if len(cmd) > 1 && !filepath.IsAbs(cmd[1]) {
			// If second arg looks like a path, make it relative to plugin dir
			if _, err := os.Stat(filepath.Join(m.Dir, cmd[1])); err == nil {
				cmd[1] = filepath.Join(m.Dir, cmd[1])
			}
		}
		return cmd
	}
	return []string{m.ExecutablePath()}
}

// HasCapability checks if a provider plugin has the specified capability.
func (m *Manifest) HasCapability(cap string) bool {
	if m.Provider == nil {
		return false
	}
	for _, c := range m.Provider.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}
