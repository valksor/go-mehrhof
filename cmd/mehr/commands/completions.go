package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ─────────────────────────────────────────────────────────────────────────────
// Shell Completion Functions for Cobra
// ─────────────────────────────────────────────────────────────────────────────

// completionsCmd generates shell completion scripts
var completionsCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `To load completions:

Bash:
  $ source <(mehr completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ mehr completion bash > /etc/bash_completion.d/mehr
  # macOS:
  $ mehr completion bash > /usr/local/etc/bash_completion.d/mehr

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ mehr completion zsh > "${fpath[1]}/_mehr"

  # You will need to start a new shell for this setup to take effect.

fish:
  $ mehr completion fish | source

  # To load completions for each session, execute once:
  $ mehr completion fish > ~/.config/fish/completions/mehr.fish

PowerShell:
  PS> mehr completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> mehr completion powershell > mehr.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	RunE:                  runCompletions,
}

func init() {
	rootCmd.AddCommand(completionsCmd)
}

func runCompletions(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("shell type not specified. Choose from: bash, zsh, fish, powershell")
	}

	shell := args[0]
	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		return cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
	default:
		return fmt.Errorf("unsupported shell type: %s", shell)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Custom Completion Functions
// ─────────────────────────────────────────────────────────────────────────────

// completeAgent provides completion for agent names and aliases
func completeAgent(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Get list of registered agents from conductor (this would need to be initialized)
	// For now, return common built-in agents and suggest workspace aliases
	agents := []string{
		"claude",
		"claude-opus",
		"claude-haiku",
	}

	// Filter by prefix
	var filtered []string
	for _, agent := range agents {
		if strings.HasPrefix(agent, toComplete) {
			filtered = append(filtered, agent)
		}
	}

	return filtered, cobra.ShellCompDirectiveNoFileComp
}

// completeWorkflow provides completion for workflow steps
func completeWorkflow(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	steps := []string{
		"planning",
		"implementing",
		"reviewing",
		"checkpointing",
	}

	var filtered []string
	for _, step := range steps {
		if strings.HasPrefix(step, toComplete) {
			filtered = append(filtered, step)
		}
	}

	return filtered, cobra.ShellCompDirectiveNoFileComp
}
