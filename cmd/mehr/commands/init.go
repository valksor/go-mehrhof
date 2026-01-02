package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
)

var initInteractive bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the task workspace",
	Long: `Initialize the task workspace by creating the .mehrhof directory
and updating .gitignore.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Interactive setup for API key, provider, and agent")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	// Try to find git root, fall back to current directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	git, err := vcs.New(ctx, workDir)
	root := workDir
	if err == nil {
		root = git.Root()
	}

	ws, err := storage.OpenWorkspace(root, nil)
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

	// Run interactive setup if requested
	if initInteractive {
		if err := interactiveSetup(cmd, ws, envPath); err != nil {
			_, _ = fmt.Fprintf(errOut, "warning: interactive setup failed: %v\n", err)
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

// interactiveSetup guides the user through initial configuration.
func interactiveSetup(cmd *cobra.Command, ws *storage.Workspace, envPath string) error {
	out := cmd.OutOrStdout()
	in := bufio.NewReader(cmd.InOrStdin())

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Interactive Setup")
	_, _ = fmt.Fprintln(out, "-----------------")

	// Step 1: API Key
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "Enter your Anthropic API key (sk-ant-...): ")
	apiKey, _ := in.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey != "" {
		if !strings.HasPrefix(apiKey, "sk-ant-") {
			_, _ = fmt.Fprintln(out, "Warning: API key doesn't start with 'sk-ant-'. Did you enter it correctly?")
		}
		// Append to .env file
		f, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "\nANTHROPIC_API_KEY=%s\n", apiKey)
			if err := f.Close(); err != nil {
				_, _ = fmt.Fprintf(out, "warning: failed to close .env file: %v\n", err)
			}
			_, _ = fmt.Fprintln(out, "API key saved to .env")
		}
	}

	// Step 2: Default Provider
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Available providers: file, dir, github, jira, linear, notion, wrike, youtrack")
	_, _ = fmt.Fprintf(out, "Enter default provider [file]: ")
	provider, _ := in.ReadString('\n')
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "file"
	}

	// Step 3: Default Agent
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Available built-in agents: claude")
	_, _ = fmt.Fprintf(out, "Enter default agent [claude]: ")
	agent, _ := in.ReadString('\n')
	agent = strings.TrimSpace(agent)

	if agent == "" {
		agent = "claude"
	}

	if agent == "glm" {
		_, _ = fmt.Fprintln(out, "Note: 'glm' is not a built-in agent. You'll need to configure it as an alias in config.yaml.")
	}

	// Update config.yaml with user's choices
	cfg, err := ws.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Agent and Providers are value types in WorkspaceConfig, not pointers
	cfg.Providers.Default = provider
	cfg.Agent.Default = agent

	if err := ws.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Configuration saved!")

	return nil
}
