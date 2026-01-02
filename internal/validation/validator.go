package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// Options configures validation behavior.
type Options struct {
	Strict bool // Treat warnings as errors
}

// Validator validates configuration files.
type Validator struct {
	workspacePath string
	builtInAgents []string
	opts          Options
}

// New creates a new configuration validator.
func New(workspacePath string, opts Options) *Validator {
	return &Validator{
		workspacePath: workspacePath,
		opts:          opts,
		builtInAgents: []string{"claude"}, // Default built-in agents
	}
}

// SetBuiltInAgents sets the list of known built-in agent names.
func (v *Validator) SetBuiltInAgents(names []string) {
	v.builtInAgents = names
}

// Validate runs workspace validation and returns the result.
func (v *Validator) Validate(ctx context.Context) (*Result, error) {
	result := NewResult()

	// Validate workspace config
	wsResult, err := v.validateWorkspace()
	if err != nil {
		return nil, fmt.Errorf("workspace validation: %w", err)
	}
	result.Merge(wsResult)

	// In strict mode, warnings make the config invalid
	if v.opts.Strict && result.Warnings > 0 {
		result.Valid = false
	}

	return result, nil
}

// validateWorkspace validates the workspace configuration.
func (v *Validator) validateWorkspace() (*Result, error) {
	result := NewResult()

	ws, err := storage.OpenWorkspace(v.workspacePath, nil)
	if err != nil {
		return nil, fmt.Errorf("open workspace: %w", err)
	}

	configPath := ws.ConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No config file is valid - defaults are used
		result.AddInfo("CONFIG_NOT_FOUND", "No workspace config found, using defaults", "", configPath)

		return result, nil
	}

	// Load and validate workspace config
	cfg, err := ws.LoadConfig()
	if err != nil {
		result.AddError("YAML_SYNTAX", fmt.Sprintf("Failed to parse config: %s", err), "", configPath)

		return result, nil
	}

	// Run workspace-specific validations
	validateWorkspaceConfig(cfg, configPath, v.builtInAgents, result)

	return result, nil
}

// WorkspaceConfigPath returns the expected workspace config file path.
func (v *Validator) WorkspaceConfigPath() string {
	return filepath.Join(v.workspacePath, ".mehrhof", "config.yaml")
}
