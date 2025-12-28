package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the task workspace",
	Long: `Initialize the task workspace by creating the .mehrhof directory
and updating .gitignore.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Try to find git root, fall back to current directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	git, err := vcs.New(workDir)
	root := workDir
	if err == nil {
		root = git.Root()
	}

	ws, err := storage.OpenWorkspace(root)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	if err := ws.EnsureInitialized(); err != nil {
		return fmt.Errorf("initialize workspace: %w", err)
	}

	// Update .gitignore with standard entries
	if err := ws.UpdateGitignore(); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	// Create config file with defaults if it doesn't exist
	if !ws.HasConfig() {
		cfg := storage.NewDefaultWorkspaceConfig()
		if err := ws.SaveConfig(cfg); err != nil {
			return fmt.Errorf("create config file: %w", err)
		}
		fmt.Printf("Created config file: %s\n", ws.ConfigPath())
	} else {
		fmt.Printf("Config file already exists: %s\n", ws.ConfigPath())
	}

	fmt.Printf("Workspace initialized in %s\n", root)
	return nil
}
