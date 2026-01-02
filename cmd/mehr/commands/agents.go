package commands

import (
	"context"
	"fmt"
	"os"
	"slices"
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

var agentsExplainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain agent configuration priority and resolution",
	Long: `Show how agents are resolved based on configuration sources.

Agent selection follows this priority (highest to lowest):
  1. CLI step-specific flag: --agent-plan, --agent-implement, --agent-review
  2. CLI global flag: --agent
  3. Task frontmatter step-specific: agent_steps.planning.agent
  4. Task frontmatter default: agent
  5. Workspace config step-specific: agent.steps.*.name
  6. Workspace config default: agent.default
  7. Auto-detection (first available agent)

This command explains each priority level with examples.`,
	RunE: runAgentsExplain,
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsExplainCmd)
}

// agentInfo holds display information about an agent.
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
	slices.Sort(agentNames)

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

func runAgentsExplain(cmd *cobra.Command, args []string) error {
	fmt.Print(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                    AGENT CONFIGURATION PRIORITY & RESOLUTION                    ║
╚══════════════════════════════════════════════════════════════════════════════╝

When you run a mehr command, the agent is selected by checking these sources
in order (highest priority first):

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 1. CLI STEP-SPECIFIC FLAG (Highest Priority)                                   │
├──────────────────────────────────────────────────────────────────────────────────┤
│    mehr plan --agent-plan opus                                                 │
│    mehr implement --agent-implement sonnet                                      │
│    mehr review --agent-review haiku                                             │
│                                                                                  │
│    Overrides: Everything else                                                  │
│    Scope: Single workflow step only                                            │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 2. CLI GLOBAL FLAG                                                              │
├──────────────────────────────────────────────────────────────────────────────────┤
│    mehr start --agent opus file:task.md                                        │
│    mehr auto --agent sonnet file:task.md                                       │
│                                                                                  │
│    Overrides: Task frontmatter, workspace config, auto-detection               │
│    Scope: Entire workflow (all steps)                                          │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 3. TASK FRONTMATTER STEP-SPECIFIC                                               │
├──────────────────────────────────────────────────────────────────────────────────┤
│    ---                                                                           │
│    agent_steps:                                                                 │
│      planning:                                                                 │
│        agent: opus                                                             │
│      implementing:                                                             │
│        agent: sonnet                                                            │
│    ---                                                                           │
│                                                                                  │
│    Overrides: Task default agent, workspace config, auto-detection             │
│    Scope: Single workflow step only                                            │
│    Location: In your task.md file                                               │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 4. TASK FRONTMATTER DEFAULT                                                      │
├──────────────────────────────────────────────────────────────────────────────────┤
│    ---                                                                           │
│    agent: sonnet                                                                 │
│    agent_args: ["--max-turns", "10"]                                            │
│    ---                                                                           │
│                                                                                  │
│    Overrides: Workspace config, auto-detection                                 │
│    Scope: Entire workflow (all steps)                                          │
│    Location: In your task.md file                                               │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 5. WORKSPACE CONFIG STEP-SPECIFIC                                                │
├──────────────────────────────────────────────────────────────────────────────────┤
│    # .mehrhof/config.yaml                                                        │
│    agent:                                                                        │
│      steps:                                                                      │
│        planning:                                                                 │
│          name: opus                                                             │
│        implementing:                                                             │
│          name: sonnet                                                            │
│        reviewing:                                                                │
│          name: claude                                                           │
│                                                                                  │
│    Overrides: Workspace default, auto-detection                                │
│    Scope: Single workflow step only                                            │
│    Location: .mehrhof/config.yaml                                               │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 6. WORKSPACE CONFIG DEFAULT                                                       │
├──────────────────────────────────────────────────────────────────────────────────┤
│    # .mehrhof/config.yaml                                                        │
│    agent:                                                                        │
│      default: claude                                                             │
│                                                                                  │
│    Overrides: Auto-detection                                                   │
│    Scope: Entire workflow (all steps)                                          │
│    Location: .mehrhof/config.yaml                                               │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ 7. AUTO-DETECTION (Lowest Priority)                                              │
├──────────────────────────────────────────────────────────────────────────────────┤
│    Scans for available agents in this order:                                    │
│      1. Checks common CLI binaries (claude, etc.)                               │
│      2. Selects first available agent                                           │
│                                                                                  │
│    Used only when no other configuration specifies an agent                     │
└──────────────────────────────────────────────────────────────────────────────────┘

╔══════════════════════════════════════════════════════════════════════════════╗
║                          EXAMPLE SCENARIOS                                     ║
╚══════════════════════════════════════════════════════════════════════════════╝

Scenario 1: Override workspace default for one task
─────────────────────────────────────────────────────
  # .mehrhof/config.yaml
  agent:
    default: claude        # Default for all tasks

  # task.md
  ---
  agent: sonnet            # This task uses sonnet instead
  ---

Scenario 2: Different agents for planning vs implementation
──────────────────────────────────────────────────────────
  # task.md
  ---
  agent_steps:
    planning:
      agent: opus          # Use Opus for planning (careful)
    implementing:
      agent: sonnet        # Use Sonnet for implementation (fast)
  ---

Scenario 3: One-off override with CLI flag
───────────────────────────────────────────
  mehr plan --agent-plan opus        # Use Opus just for this planning
  mehr implement --agent sonnet      # Use Sonnet just for this implementation

Scenario 4: Per-step workspace configuration
───────────────────────────────────────────
  # .mehrhof/config.yaml
  agent:
    default: claude
    steps:
      planning:
        name: opus           # Always use Opus for planning
      implementing:
        name: sonnet         # Always use Sonnet for implementation
      reviewing:
        name: claude         # Use Claude for review

Scenario 5: Agent aliases with custom settings
────────────────────────────────────────────
  # .mehrhof/config.yaml
  agents:
    opus:
      extends: claude
      description: "Claude Opus for planning"
      args: ["--model", "claude-opus-4-20250514"]
    sonnet-fast:
      extends: claude
      description: "Sonnet with limited turns"
      args: ["--model", "claude-sonnet-4-20250514", "--max-turns", "5"]

  agent:
    default: opus            # Uses the alias defined above

╔══════════════════════════════════════════════════════════════════════════════╗
║                        RESOLUTION SUMMARY                                     ║
╚══════════════════════════════════════════════════════════════════════════════╝

For a given workflow step (e.g., planning):

  1. Is --agent-plan set? → Use that
  2. Is --agent set? → Use that
  3. Is agent_steps.planning.agent in frontmatter? → Use that
  4. Is agent in frontmatter? → Use that
  5. Is agent.steps.planning.name in workspace config? → Use that
  6. Is agent.default in workspace config? → Use that
  7. Auto-detect → Use first available agent

TIP: Run 'mehr agents list' to see all available agents and aliases.
`)

	return nil
}
