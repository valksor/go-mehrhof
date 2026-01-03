package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/validation"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Commands for managing and validating mehrhof configuration.`,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	Long: `Validate workspace configuration (.mehrhof/config.yaml).

Performs the following checks:
  - YAML syntax validity
  - Required fields and valid enum values
  - Agent alias circular dependencies
  - Undefined agent references
  - Git pattern template validity
  - Plugin configuration

Examples:
  mehr config validate                    # Validate workspace config
  mehr config validate --strict           # Treat warnings as errors
  mehr config validate --format json      # JSON output for CI`,
	RunE: runConfigValidate,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new .mehrhof/config.yaml file",
	Long: `Create a new workspace configuration file with sensible defaults.

This command creates .mehrhof/config.yaml in the current directory with
default settings for git, agent, workflow, and plugins.

If a config file already exists, you'll be prompted to overwrite it
(unless --force is used).

The generated config includes helpful comments showing all available
options and examples for customization.

Examples:
  mehr config init              # Create config with interactive prompt
  mehr config init --force      # Overwrite existing config without prompt`,
	RunE: runConfigInit,
}

var (
	validateStrict    bool
	validateFormat    string
	configInitForce   bool
	configInitProject string // Project type hint (go, node, python, php)
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configInitCmd)

	configValidateCmd.Flags().BoolVar(&validateStrict, "strict", false,
		"Treat warnings as errors (exit code 1 if warnings present)")
	configValidateCmd.Flags().StringVar(&validateFormat, "format", "text",
		"Output format: text, json")

	configInitCmd.Flags().BoolVarP(&configInitForce, "force", "f", false,
		"Overwrite existing config file without prompting")
	configInitCmd.Flags().StringVar(&configInitProject, "project", "",
		"Project type for intelligent defaults (go, node, python, php)")
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Get built-in agent names by initializing conductor
	// We use WithAutoInit(false) to avoid requiring a task context
	builtInAgents := getBuiltInAgents(ctx)

	// Create validator
	validator := validation.New(wd, validation.Options{
		Strict: validateStrict,
	})
	validator.SetBuiltInAgents(builtInAgents)

	// Print header
	if validateFormat == "text" {
		fmt.Println("Validating workspace configuration...")
		fmt.Println()
	}

	// Run validation
	result, err := validator.Validate(ctx)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Output results
	fmt.Print(result.Format(validateFormat))

	// Return error for exit code handling
	if !result.Valid {
		return errors.New("validation failed")
	}
	if validateStrict && result.Warnings > 0 {
		return fmt.Errorf("validation failed: %d warning(s) in strict mode", result.Warnings)
	}

	return nil
}

// getBuiltInAgents returns the list of built-in agent names.
// It attempts to initialize the conductor to get the full registry,
// falling back to hardcoded defaults if initialization fails.
func getBuiltInAgents(ctx context.Context) []string {
	// Try to get from conductor registry
	cond, err := initializeConductor(ctx, conductor.WithAutoInit(false))
	if err == nil {
		return cond.GetAgentRegistry().List()
	}

	// Fall back to known built-in agents
	return []string{"claude"}
}

// runConfigInit creates a new workspace configuration file.
func runConfigInit(cmd *cobra.Command, args []string) error {
	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Open workspace to find config path
	ws, err := storage.OpenWorkspace(wd, nil)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	configPath := ws.ConfigPath()

	// Check if config already exists
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}

	if configExists {
		fmt.Printf(display.Warning("WARNING: Config file already exists: %s\n"), configPath)
		fmt.Println()
		fmt.Println("The 'mehr config init' command is for creating NEW configurations.")
		fmt.Println("Your existing config contains custom settings that would be lost.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s - View your current config\n", display.Cyan("cat .mehrhof/config.yaml"))
		fmt.Printf("  %s - Validate your current config\n", display.Cyan("mehr config validate"))
		fmt.Printf("  %s - Edit your current config\n", display.Cyan("vim .mehrhof/config.yaml"))
		fmt.Println()

		if !configInitForce {
			return nil // Safe default: do nothing
		}

		// With --force, require extra explicit confirmation
		fmt.Println(display.ErrorMsg("--force flag: This will DELETE your custom configuration!"))
		confirmed, err := confirmAction("Type 'yes' to confirm deletion and overwrite", false)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled - your config is safe")

			return nil
		}
		fmt.Println("Proceeding with overwrite...")
	} else {
		fmt.Printf("Creating new configuration: %s\n", configPath)
	}

	// Detect project type if not specified
	projectType := configInitProject
	if projectType == "" {
		projectType = detectProjectType(wd)
		if projectType != "" {
			fmt.Printf("Detected project type: %s\n", display.Cyan(projectType))
		}
	}

	// Create default config
	cfg := storage.NewDefaultWorkspaceConfig()

	// Apply project-specific customizations
	applyProjectCustomizations(cfg, projectType)

	// Ensure .mehrhof directory exists
	if err := os.MkdirAll(ws.TaskRoot(), 0o755); err != nil {
		return fmt.Errorf("create .mehrhof directory: %w", err)
	}

	// Save config
	if err := ws.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println(display.SuccessMsg("Configuration created successfully"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  %s - Edit the configuration\n", display.Cyan("vim .mehrhof/config.yaml"))
	fmt.Printf("  %s - Validate the configuration\n", display.Cyan("mehr config validate"))
	fmt.Printf("  %s - Start your first task\n", display.Cyan("mehr start task.md"))

	return nil
}

// detectProjectType attempts to detect the project type from common files.
func detectProjectType(dir string) string {
	// Check for Go projects
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}

	// Check for Node.js projects (package.json)
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "node"
	}

	// Check for Python projects (pyproject.toml, requirements.txt, setup.py)
	for _, name := range []string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return "python"
		}
	}

	// Check for PHP projects (composer.json)
	if _, err := os.Stat(filepath.Join(dir, "composer.json")); err == nil {
		return "php"
	}

	return ""
}

// applyProjectCustomizations customizes the config based on project type.
func applyProjectCustomizations(cfg *storage.WorkspaceConfig, projectType string) {
	switch projectType {
	case "go":
		// Go projects: use claude as default, enable go-specific agents
		cfg.Agent.Default = "claude"
		cfg.Git.CommitPrefix = "[{key}]"
		cfg.Git.BranchPattern = "{type}/{key}--{slug}"

	case "node":
		// Node.js projects: optimize for JS/TS workflows
		cfg.Agent.Default = "claude"
		cfg.Git.CommitPrefix = "feat({key}):"
		cfg.Git.BranchPattern = "{type}/{key}--{slug}"

	case "python":
		// Python projects: conventional commits style
		cfg.Agent.Default = "claude"
		cfg.Git.CommitPrefix = "[{key}]"
		cfg.Git.BranchPattern = "{type}/{key}--{slug}"

	case "php":
		// PHP projects: typical PHP conventions
		cfg.Agent.Default = "claude"
		cfg.Git.CommitPrefix = "[{key}]"
		cfg.Git.BranchPattern = "{type}/{key}--{slug}"
	}
}
