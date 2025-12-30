package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/template"
)

var (
	startAgent         string
	startNoBranch      bool
	startWorktree      bool
	startKey           string // External key override (e.g., "FEATURE-123")
	startTitle         string // Title override for the task
	startSlug          string // Slug override for branch naming
	startCommitPrefix  string // Commit prefix template override
	startBranchPattern string // Branch pattern template override
	startTemplate      string // Template to apply

	// Per-step agent overrides
	startAgentPlanning     string
	startAgentImplementing string
	startAgentReviewing    string
)

var startCmd = &cobra.Command{
	Use:   "start <reference>",
	Short: "Start a new task from a file, directory, or provider",
	Long: `Start a new task from a file, directory, or external provider.

This command reads the source, creates a git branch, and sets up the task
as active. It does NOT run planning - use 'mehr plan' for that.

USAGE:
  mehr start <reference>

PROVIDERS:
  file:task.md              Markdown file (default, can omit 'file:')
  dir:./tasks/              Directory of markdown files
  github:123                GitHub issue (requires configuration)
  notion:abc123 / nt:       Notion page by ID or URL
  jira:PROJ-123             Jira issue (requires configuration)
  linear:ABC-123            Linear issue (requires configuration)
  wrike:abc123              Wrike task (requires configuration)
  youtrack:PROJ-123         YouTrack issue (requires configuration)

AGENT SELECTION (highest to lowest priority):
  1. CLI flag: --agent or --agent-plan/--agent-implement/--agent-review
  2. Task frontmatter: agent: or agent_steps: in markdown
  3. Workspace config: agent.default or agent.steps in .mehrhof/config.yaml
  4. Auto-detect: uses default claude agent

GIT OPTIONS:
  --no-branch               Do not create a git branch
  --worktree                Create isolated git worktree (allows parallel tasks)

PER-STEP AGENT FLAGS:
  --agent-plan              Agent for planning step (overrides default)
  --agent-implement         Agent for implementation step (overrides default)
  --agent-review            Agent for review step (overrides default)

TEMPLATES:
  --template bug-fix        Apply bug-fix template
  --template feature        Apply feature template
  --template refactor       Apply refactor template
  mehr templates            List all available templates

EXAMPLES:
  mehr start file:task.md         # Start from a markdown file
  mehr start dir:./tasks/         # Start from a directory
  mehr start --no-branch task.md  # Start without creating a branch
  mehr start --worktree task.md   # Start with a separate worktree
  mehr start --template bug-fix file:task.md  # Apply bug-fix template

See also:
  mehr plan                 - Create implementation specifications
  mehr status               - Show active task status
  mehr templates            - List available task templates`,
	Args: cobra.ExactArgs(1),
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&startAgent, "agent", "A", "", "Agent to use (default: auto-detect)")
	startCmd.Flags().BoolVar(&startNoBranch, "no-branch", false, "Do not create a git branch")
	startCmd.Flags().BoolVarP(&startWorktree, "worktree", "w", false, "Create a separate git worktree for this task")

	// Naming override flags
	startCmd.Flags().StringVarP(&startKey, "key", "k", "", "External key for branch/commit naming (e.g., FEATURE-123)")
	startCmd.Flags().StringVar(&startTitle, "title", "", "Task title override")
	startCmd.Flags().StringVar(&startSlug, "slug", "", "Branch slug override (e.g., custom-slug)")
	startCmd.Flags().StringVar(&startCommitPrefix, "commit-prefix", "", "Commit prefix template (e.g., [{key}])")
	startCmd.Flags().StringVar(&startBranchPattern, "branch-pattern", "", "Branch pattern template (e.g., {type}/{key}--{slug})")
	startCmd.Flags().StringVar(&startTemplate, "template", "", "Template to apply (bug-fix, feature, refactor, docs, test, chore)")

	// Per-step agent overrides
	startCmd.Flags().StringVar(&startAgentPlanning, "agent-plan", "", "Agent for planning step")
	startCmd.Flags().StringVar(&startAgentImplementing, "agent-implement", "", "Agent for implementation step")
	startCmd.Flags().StringVar(&startAgentReviewing, "agent-review", "", "Agent for review step")
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	reference := args[0]

	// Apply template if specified (only works for file: provider)
	if startTemplate != "" {
		if !strings.HasPrefix(reference, "file:") {
			return fmt.Errorf("--template only works with file: provider")
		}

		filePath := strings.TrimPrefix(reference, "file:")
		tpl, err := template.LoadBuiltIn(startTemplate)
		if err != nil {
			return fmt.Errorf("load template: %w", err)
		}

		// Read existing content
		var content string
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		content = string(data)

		// Apply template and write back
		newContent := tpl.ApplyToContent(content)
		if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
			return fmt.Errorf("write file with template: %w", err)
		}

		fmt.Printf("Applied template '%s' to %s\n", startTemplate, filePath)
	}

	// Determine branch behavior
	// Branch creation is default, --no-branch disables it
	// --worktree forces branch creation (even with --no-branch)
	createBranch := !startNoBranch || startWorktree

	// Build conductor options
	opts := []conductor.Option{
		conductor.WithVerbose(verbose),
		conductor.WithCreateBranch(createBranch),
		conductor.WithUseWorktree(startWorktree),
		conductor.WithAutoInit(true),
	}

	if startAgent != "" {
		opts = append(opts, conductor.WithAgent(startAgent))
	}

	// Per-step agent options
	if startAgentPlanning != "" {
		opts = append(opts, conductor.WithStepAgent("planning", startAgentPlanning))
	}
	if startAgentImplementing != "" {
		opts = append(opts, conductor.WithStepAgent("implementing", startAgentImplementing))
	}
	if startAgentReviewing != "" {
		opts = append(opts, conductor.WithStepAgent("reviewing", startAgentReviewing))
	}

	// Pass default provider from workspace config
	if wd, err := os.Getwd(); err == nil {
		if ws, err := storage.OpenWorkspace(wd); err == nil {
			if wsCfg, err := ws.LoadConfig(); err == nil && wsCfg.Providers.Default != "" {
				opts = append(opts, conductor.WithDefaultProvider(wsCfg.Providers.Default))
			}
		}
	}

	// Naming override options
	if startKey != "" {
		opts = append(opts, conductor.WithExternalKey(startKey))
	}
	if startTitle != "" {
		opts = append(opts, conductor.WithTitleOverride(startTitle))
	}
	if startSlug != "" {
		opts = append(opts, conductor.WithSlugOverride(startSlug))
	}
	if startCommitPrefix != "" {
		opts = append(opts, conductor.WithCommitPrefixTemplate(startCommitPrefix))
	}
	if startBranchPattern != "" {
		opts = append(opts, conductor.WithBranchPatternTemplate(startBranchPattern))
	}

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, opts...)
	if err != nil {
		return err
	}

	// Check for existing task
	if cond.GetActiveTask() != nil {
		return fmt.Errorf("task already active: %s\nUse 'mehr status' to check or 'mehr finish' to complete it", cond.GetActiveTask().ID)
	}

	// Start (register) task
	if err := cond.Start(ctx, reference); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	// Get status
	status, err := cond.Status()
	if err != nil {
		return err
	}

	fmt.Printf("Task started: %s\n", status.TaskID)
	fmt.Printf("  Title: %s\n", status.Title)
	if status.ExternalKey != "" {
		fmt.Printf("  Key: %s\n", status.ExternalKey)
	}
	fmt.Printf("  Source: %s\n", status.Ref)
	fmt.Printf("  State: %s\n", status.State)
	if status.Branch != "" {
		fmt.Printf("  Branch: %s\n", status.Branch)
	}
	if status.WorktreePath != "" {
		fmt.Printf("  Worktree: %s\n", status.WorktreePath)
	}
	fmt.Printf("\nNext steps:\n")
	if status.WorktreePath != "" {
		fmt.Printf("  cd %s           - Switch to the worktree\n", status.WorktreePath)
	}
	fmt.Printf("  mehr plan      - Create implementation specifications\n")
	fmt.Printf("  mehr note      - Add notes to the task\n")

	return nil
}
