package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/config"
	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/log"
)

var (
	settings *config.Settings

	// Global flags
	verbose bool
	noColor bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "mehr",
	Short: "AI-powered task automation",
	Long: `mehrhof is a CLI tool for AI-assisted task automation.

It orchestrates AI agents to perform planning, implementation, and code review
workflows. Tasks can be sourced from files, directories, or external providers.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load .env file FIRST, before anything else
		// This ensures env vars are available for all subsequent operations
		if err := config.LoadDotEnvFromCwd(); err != nil {
			// Log warning but don't fail - .env parsing errors should be reported
			// but shouldn't prevent the command from running
			fmt.Fprintf(os.Stderr, "warning: failed to load .mehrhof/.env: %v\n", err)
		}

		// Configure logging from CLI flag
		log.Configure(log.Options{
			Verbose: verbose,
		})

		// Initialize color output from CLI flag (also respects NO_COLOR env)
		display.InitColors(noColor)

		// Load settings (user preferences)
		var err error
		settings, err = config.LoadSettings()
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}

		log.Debug("initialized", "verbose", verbose)

		// Async update check (non-blocking, doesn't slow startup)
		// Skip for the 'update' command itself to avoid redundant checks
		if cmd.Name() != "update" && shouldCheckForUpdates(settings) {
			go checkForUpdatesInBackground(cmd.Context())
		}

		return nil
	},
}

// Execute runs the root command with signal handling
func Execute() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")

	// Add command groups for better help organization
	rootCmd.AddGroup(&cobra.Group{
		ID:    "workflow",
		Title: "Workflow Commands:",
	}, &cobra.Group{
		ID:    "task",
		Title: "Task Commands:",
	}, &cobra.Group{
		ID:    "info",
		Title: "Information Commands:",
	}, &cobra.Group{
		ID:    "config",
		Title: "Configuration Commands:",
	})

	// Register shell completions for common flags
	registerCompletions()
}

// registerCompletions sets up custom completion functions for flags
func registerCompletions() {
	// Agent flag completions
	agentFlags := []string{"agent", "agent-plan", "agent-implement", "agent-review", "agent-chat"}
	for _, flag := range agentFlags {
		if err := rootCmd.RegisterFlagCompletionFunc(flag, completeAgent); err != nil {
			// Some flags might not exist in all commands, ignore errors
			continue
		}
	}

	// Workflow step completions - may not exist in all commands
	// nolint:staticcheck // Flag may not exist in all commands, ignore error
	_ = rootCmd.RegisterFlagCompletionFunc("step", completeWorkflow)
}

// GetSettings returns the loaded settings
func GetSettings() *config.Settings {
	return settings
}
