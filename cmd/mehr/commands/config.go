package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
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

var (
	validateStrict bool
	validateFormat string
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configValidateCmd)

	configValidateCmd.Flags().BoolVar(&validateStrict, "strict", false,
		"Treat warnings as errors (exit code 1 if warnings present)")
	configValidateCmd.Flags().StringVar(&validateFormat, "format", "text",
		"Output format: text, json")
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

	// Exit code based on result
	if !result.Valid {
		os.Exit(1)
	}
	if validateStrict && result.Warnings > 0 {
		os.Exit(1)
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
