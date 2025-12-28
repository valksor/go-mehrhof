package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/config"
	"github.com/valksor/go-mehrhof/internal/log"
)

var (
	cfg      *config.Config
	settings *config.Settings

	// Global flags
	verbose bool
	noColor bool
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
		ctx := cmd.Context()

		// Configure logging
		log.Configure(log.Options{
			Verbose: verbose,
		})

		// Load configuration
		var err error
		cfg, err = config.Load(ctx)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Apply flag overrides
		if verbose {
			cfg.UI.Verbose = true
		}
		if noColor {
			cfg.UI.Color = false
		}

		// Load settings
		settings, err = config.LoadSettings()
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}

		log.Debug("configuration loaded", "verbose", verbose)
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
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
}

// GetConfig returns the loaded configuration
func GetConfig() *config.Config {
	return cfg
}

// GetSettings returns the loaded settings
func GetSettings() *config.Settings {
	return settings
}
