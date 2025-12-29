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
	startBranch        bool
	startWorktree      bool
	startKey           string // External key override (e.g., "FEATURE-123")
	startCommitPrefix  string // Commit prefix template override
	startBranchPattern string // Branch pattern template override
	startTemplate      string // Template to apply

	// Per-step agent overrides
	startAgentPlanning     string
	startAgentImplementing string
	startAgentReviewing    string
	startAgentDialogue     string
)

var startCmd = &cobra.Command{
	Use:   "start <scheme:reference>",
	Short: "Register a new task from a file or directory",
	Long: `Register a new task from a file, directory, or external provider.

IMPORTANT: You must specify a provider scheme prefix (e.g., file:, dir:).

This command reads the source, creates a git branch, and registers
the task as active. It does NOT run planning - use 'mehr plan' for that.

The source is snapshot and stored read-only. All work happens in
.mehrhof/work/<id>/ directory on the task branch.

With --worktree, a separate git worktree is created for the task,
allowing you to work in isolation without switching branches in your
main repository.

Examples:
  mehr start file:task.md              # Start from a markdown file
  mehr start dir:./tasks/              # Start from a directory
  mehr start --branch=false file:task.md  # Start without creating a branch
  mehr start --worktree file:task.md   # Start with a separate worktree

Or configure a default provider in .mehrhof/config.yaml:
  providers:
      default: file

Then bare references will use that provider:
  mehr start task.md              # Uses file: provider`,
	Args: cobra.ExactArgs(1),
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&startAgent, "agent", "a", "", "Agent to use (default: auto-detect)")
	startCmd.Flags().BoolVarP(&startBranch, "branch", "b", true, "Create a git branch for this task (use --branch=false to disable)")
	startCmd.Flags().BoolVarP(&startWorktree, "worktree", "w", false, "Create a separate git worktree for this task")

	// Naming override flags
	startCmd.Flags().StringVarP(&startKey, "key", "k", "", "External key for branch/commit naming (e.g., FEATURE-123)")
	startCmd.Flags().StringVar(&startCommitPrefix, "commit-prefix", "", "Commit prefix template (e.g., [{key}])")
	startCmd.Flags().StringVar(&startBranchPattern, "branch-pattern", "", "Branch pattern template (e.g., {type}/{key}--{slug})")
	startCmd.Flags().StringVarP(&startTemplate, "template", "t", "", "Template to apply (bug-fix, feature, refactor, docs, test, chore)")

	// Per-step agent overrides
	startCmd.Flags().StringVar(&startAgentPlanning, "agent-plan", "", "Agent for planning step")
	startCmd.Flags().StringVar(&startAgentImplementing, "agent-implement", "", "Agent for implementation step")
	startCmd.Flags().StringVar(&startAgentReviewing, "agent-review", "", "Agent for review step")
	startCmd.Flags().StringVar(&startAgentDialogue, "agent-chat", "", "Agent for dialogue/chat step")
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
	// Worktree implies branch creation
	createBranch := startBranch
	if startWorktree {
		createBranch = true
	}

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
	if startAgentDialogue != "" {
		opts = append(opts, conductor.WithStepAgent("dialogue", startAgentDialogue))
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

	fmt.Printf("Task registered: %s\n", status.TaskID)
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
	fmt.Printf("  mehr chat      - Add notes or discuss the task\n")

	return nil
}
