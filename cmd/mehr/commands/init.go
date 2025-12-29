package commands

import (
	"fmt"
	"os"
	"path/filepath"

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
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

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
		_, _ = fmt.Fprintf(out, "Created config file: %s\n", ws.ConfigPath())
	} else {
		_, _ = fmt.Fprintf(out, "Config file already exists: %s\n", ws.ConfigPath())
	}

	// Create .env template if it doesn't exist
	envPath := filepath.Join(ws.TaskRoot(), ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if err := createEnvTemplate(envPath); err != nil {
			_, _ = fmt.Fprintf(errOut, "warning: failed to create .env template: %v\n", err)
		} else {
			_, _ = fmt.Fprintf(out, "Created .env template: %s\n", envPath)
		}
	}

	_, _ = fmt.Fprintf(out, "Workspace initialized in %s\n", root)

	// Show welcome message and next steps
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Welcome to Mehrhof!")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Quick start:")
	_, _ = fmt.Fprintf(out, "  1. Set your API key in %s:\n", envPath)
	_, _ = fmt.Fprintf(out, "     echo 'ANTHROPIC_API_KEY=sk-ant-...' >> %s\n", envPath)
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "  2. Start your first task:\n")
	_, _ = fmt.Fprintf(out, "     mehr start file:task.md\n")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "  3. Create specifications:\n")
	_, _ = fmt.Fprintf(out, "     mehr plan\n")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "  4. Implement the specifications:\n")
	_, _ = fmt.Fprintf(out, "     mehr implement\n")
	_, _ = fmt.Fprintln(out)

	return nil
}

func createEnvTemplate(path string) error {
	template := `# Mehrhof environment variables
# This file is gitignored - store secrets here safely.
# System environment variables take priority over values defined here.

# Example: API keys for agents
# ANTHROPIC_API_KEY=sk-ant-...
# GLM_API_KEY=your-key-here

# Example: GitHub token
# GITHUB_TOKEN=ghp_...
`
	return os.WriteFile(path, []byte(template), 0o600) // 0600 for secrets
}
