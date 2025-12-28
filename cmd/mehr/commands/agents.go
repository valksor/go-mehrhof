package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/conductor"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agents",
	Long: `Manage AI agents including built-in agents and user-defined aliases.

Aliases can be configured in .mehrhof/config.yaml:

  agents:
    glm:
      extends: claude
      description: "Claude with GLM-specific API key"
      env:
        ANTHROPIC_API_KEY: "${GLM_API_KEY}"`,
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agents",
	Long: `List all registered agents including built-in agents and user-defined aliases.

The output shows:
  - NAME: Agent identifier used with --agent flag
  - TYPE: Whether it's a built-in agent or a user-defined alias
  - EXTENDS: For aliases, which agent they extend
  - DESCRIPTION: Human-readable description

Examples:
  mehr agents list`,
	RunE: runAgentsList,
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsListCmd)
}

// agentInfo holds display information about an agent
type agentInfo struct {
	name        string
	agentType   string // "built-in" or "alias"
	extends     string // base agent name for aliases
	description string
	available   bool
}

func runAgentsList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize conductor to get registered agents
	cond, err := initializeConductor(ctx, conductor.WithAutoInit(false))
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	registry := cond.GetAgentRegistry()
	agentNames := registry.List()

	if len(agentNames) == 0 {
		fmt.Println("No agents registered.")
		return nil
	}

	// Sort agent names for consistent output
	sort.Strings(agentNames)

	// Gather info about each agent
	agents := make([]agentInfo, 0, len(agentNames))
	for _, name := range agentNames {
		a, err := registry.Get(name)
		if err != nil {
			continue
		}

		info := agentInfo{
			name:      name,
			available: a.Available() == nil,
		}

		// Check if it's an alias
		if alias, ok := a.(*agent.AliasAgent); ok {
			info.agentType = "alias"
			info.description = alias.Description()
			// Get the base agent name
			if base := alias.BaseAgent(); base != nil {
				info.extends = base.Name()
			}
		} else {
			info.agentType = "built-in"
			// Try to get description from MetadataProvider interface
			if mp, ok := a.(agent.MetadataProvider); ok {
				meta := mp.Metadata()
				info.description = meta.Description
			}
		}

		agents = append(agents, info)
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tTYPE\tEXTENDS\tAVAILABLE\tDESCRIPTION"); err != nil {
		return fmt.Errorf("print header: %w", err)
	}

	for _, a := range agents {
		extends := "-"
		if a.extends != "" {
			extends = a.extends
		}

		availableStr := "no"
		if a.available {
			availableStr = "yes"
		}

		desc := a.description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		if desc == "" {
			desc = "-"
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			a.name,
			a.agentType,
			extends,
			availableStr,
			desc); err != nil {
			return fmt.Errorf("print row: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush table: %w", err)
	}

	// Show hint for configuring aliases
	fmt.Println()
	fmt.Println("Configure aliases in .mehrhof/config.yaml under 'agents:'")

	return nil
}
