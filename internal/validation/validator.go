package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/valksor/go-mehrhof/internal/config"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// Options configures validation behavior
type Options struct {
	Strict        bool // Treat warnings as errors
	AppOnly       bool // Only validate app config
	WorkspaceOnly bool // Only validate workspace config
}

// Validator validates configuration files
type Validator struct {
	workspacePath string
	opts          Options
	builtInAgents []string
}

// New creates a new configuration validator
func New(workspacePath string, opts Options) *Validator {
	return &Validator{
		workspacePath: workspacePath,
		opts:          opts,
		builtInAgents: []string{"claude"}, // Default built-in agents
	}
}

// SetBuiltInAgents sets the list of known built-in agent names
func (v *Validator) SetBuiltInAgents(names []string) {
	v.builtInAgents = names
}

// Validate runs all configured validations and returns the combined result
func (v *Validator) Validate(ctx context.Context) (*Result, error) {
	result := NewResult()

	// Validate workspace config unless app-only mode
	if !v.opts.AppOnly {
		wsResult, err := v.validateWorkspace()
		if err != nil {
			return nil, fmt.Errorf("workspace validation: %w", err)
		}
		result.Merge(wsResult)
	}

	// Validate app config unless workspace-only mode
	if !v.opts.WorkspaceOnly {
		appResult, err := v.validateApp(ctx)
		if err != nil {
			return nil, fmt.Errorf("app validation: %w", err)
		}
		result.Merge(appResult)
	}

	// Cross-validation (only if both configs are being validated)
	if !v.opts.AppOnly && !v.opts.WorkspaceOnly {
		crossResult, err := v.validateCross(ctx)
		if err != nil {
			return nil, fmt.Errorf("cross validation: %w", err)
		}
		result.Merge(crossResult)
	}

	// In strict mode, warnings make the config invalid
	if v.opts.Strict && result.Warnings > 0 {
		result.Valid = false
	}

	return result, nil
}

// validateWorkspace validates the workspace configuration
func (v *Validator) validateWorkspace() (*Result, error) {
	result := NewResult()

	ws, err := storage.OpenWorkspace(v.workspacePath)
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

// validateApp validates the application configuration (.env files)
func (v *Validator) validateApp(ctx context.Context) (*Result, error) {
	result := NewResult()

	// Load app config
	cfg, err := config.Load(ctx)
	if err != nil {
		// Try to identify which .env file caused the error
		result.AddError("ENV_LOAD_ERROR", fmt.Sprintf("Failed to load app config: %s", err), "", ".env")
		return result, nil
	}

	// Run app-specific validations
	validateAppConfig(cfg, result)

	return result, nil
}

// validateCross performs cross-config validation
func (v *Validator) validateCross(ctx context.Context) (*Result, error) {
	result := NewResult()

	ws, err := storage.OpenWorkspace(v.workspacePath)
	if err != nil {
		return result, nil // Skip cross-validation if workspace can't be opened
	}

	wsConfig, err := ws.LoadConfig()
	if err != nil {
		return result, nil // Skip if workspace config invalid
	}

	appConfig, err := config.Load(ctx)
	if err != nil {
		return result, nil // Skip if app config invalid
	}

	// Run cross-validation
	validateCrossConfig(appConfig, wsConfig, v.builtInAgents, result)

	return result, nil
}

// WorkspaceConfigPath returns the expected workspace config file path
func (v *Validator) WorkspaceConfigPath() string {
	return filepath.Join(v.workspacePath, ".mehrhof", "config.yaml")
}
