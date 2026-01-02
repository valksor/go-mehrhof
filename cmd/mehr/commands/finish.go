package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/display"
)

var (
	finishYes           bool
	finishMerge         bool
	finishDelete        bool
	finishPush          bool
	finishNoSquash      bool
	finishTargetBranch  string
	finishSkipQuality   bool
	finishQualityTarget string
	finishDeleteWork    bool
	// PR-related flags.
	finishDraftPR bool
	finishPRTitle string
	finishPRBody  string
)

var finishCmd = &cobra.Command{
	Use:     "finish",
	Aliases: []string{"fi", "done"},
	Short:   "Complete the task (creates PR by default for supported providers)",
	Long: `Complete the current task by creating a pull request or merging locally.

PROVIDER BEHAVIOR:
  github:     Creates a pull request automatically
  gitlab:     Creates a merge request automatically
  file:, dir: Prompts for action (merge locally / mark done / cancel)
  jira:       Prompts for action (merge locally / mark done / cancel)
  others:     Prompts for action (merge locally / mark done / cancel)

By default, this:
- Runs 'make quality' if available (code formatting, linting, etc.)
- For PR providers: creates a pull/merge request
- For other providers: prompts what to do
- Keeps the task branch (use --delete to remove it)
- Keeps the work directory (use --delete-work to remove it)
- Does NOT push after local merge (use --push to enable)

When using --merge, this performs a local merge instead of creating a PR:
- Performs a squash merge to keep the history clean
- Does NOT delete the task branch by default
- Does NOT push to remote by default

If quality checks modify files (e.g., auto-formatting), you'll be prompted
to confirm before proceeding.

FLAG COMBINATIONS:
  PR mode (default):
    --draft, --pr-title, --pr-body are allowed
    --merge is NOT allowed with these flags

  Merge mode (--merge):
    --delete, --push, --no-squash, --target are allowed
    --draft, --pr-title, --pr-body are NOT allowed

Examples:
  mehr finish                      # Create PR (github/gitlab) or prompt for action
  mehr finish --yes                # Skip confirmation prompt
  mehr finish --merge              # Force local merge instead of PR
  mehr finish --merge --delete     # Merge and delete task branch
  mehr finish --merge --push       # Merge and push to remote
  mehr finish --no-squash          # Regular merge instead of squash
  mehr finish --target develop     # Merge to specific branch
  mehr finish --skip-quality       # Skip quality checks
  mehr finish --quality-target lint # Use custom make target
  mehr finish --draft              # Create PR as draft
  mehr finish --pr-title "Fix bug" # Custom PR title
  mehr finish --delete-work        # Delete work directory after finishing`,
	RunE: runFinish,
}

func init() {
	rootCmd.AddCommand(finishCmd)

	finishCmd.Flags().BoolVarP(&finishYes, "yes", "y", false, "Skip confirmation prompt")
	finishCmd.Flags().BoolVar(&finishMerge, "merge", false, "Force local merge instead of creating PR")
	finishCmd.Flags().BoolVar(&finishDelete, "delete", false, "Delete branch after merge")
	finishCmd.Flags().BoolVar(&finishPush, "push", false, "Push to remote after local merge")
	finishCmd.Flags().BoolVar(&finishNoSquash, "no-squash", false, "Use regular merge instead of squash")
	finishCmd.Flags().StringVarP(&finishTargetBranch, "target", "t", "", "Target branch to merge into")
	finishCmd.Flags().BoolVar(&finishSkipQuality, "skip-quality", false, "Skip quality checks (make quality)")
	finishCmd.Flags().StringVar(&finishQualityTarget, "quality-target", "quality", "Make target for quality checks")
	finishCmd.Flags().BoolVar(&finishDeleteWork, "delete-work", false, "Delete work directory after finishing")

	// PR-related flags
	finishCmd.Flags().BoolVar(&finishDraftPR, "draft", false, "Create PR as draft")
	finishCmd.Flags().StringVar(&finishPRTitle, "pr-title", "", "Custom PR title")
	finishCmd.Flags().StringVar(&finishPRBody, "pr-body", "", "Custom PR body")

	// PR flags are mutually exclusive with merge mode
	finishCmd.MarkFlagsMutuallyExclusive("merge", "draft")
	finishCmd.MarkFlagsMutuallyExclusive("merge", "pr-title")
	finishCmd.MarkFlagsMutuallyExclusive("merge", "pr-body")
}

func runFinish(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(verbose))
	if err != nil {
		return err
	}

	// Check for active task
	activeTask := cond.GetActiveTask()
	if activeTask == nil {
		fmt.Print(display.NoActiveTaskError())
		return fmt.Errorf("no active task")
	}

	// Get status for display
	status, err := cond.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Build confirmation prompt
	promptLines := fmt.Sprintf("About to finish task: %s", status.TaskID)
	if status.Title != "" {
		promptLines += fmt.Sprintf("\n  Title: %s", status.Title)
	}
	if status.Branch != "" {
		promptLines += fmt.Sprintf("\n  Branch: %s", status.Branch)
	}
	promptLines += fmt.Sprintf("\n  State: %s", status.State)
	promptLines += fmt.Sprintf("\n  Specifications: %d", status.Specifications)

	if finishMerge {
		promptLines += "\n\nThis will perform a local merge"
		if finishDelete && status.Branch != "" {
			promptLines += " and delete the task branch"
		}
		if finishPush {
			promptLines += " and push to remote"
		}
		promptLines += "."
	} else {
		promptLines += "\n\nThis will create a pull request (if provider supports it)"
		if finishDraftPR {
			promptLines += " as draft"
		}
		promptLines += "."
	}

	// Confirmation prompt (unless --yes)
	confirmed, err := confirmAction(promptLines, finishYes)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled")
		return nil
	}

	// Run quality checks (unless skipped)
	if !finishSkipQuality {
		qualityOpts := conductor.QualityOptions{
			Target: finishQualityTarget,
		}

		result, err := cond.RunQuality(ctx, qualityOpts)
		if err != nil {
			return fmt.Errorf("quality check: %w", err)
		}

		if result.Ran {
			if result.UserAborted {
				fmt.Println("Finish cancelled by user")
				return nil
			}

			if result.Passed {
				fmt.Println(display.SuccessMsg("Quality checks passed"))
			}
		}
	}

	// Build finish options
	// Use tri-state for DeleteWork: nil=defer to config, true=delete, false=keep
	var deleteWork *bool
	if cmd.Flags().Changed("delete-work") {
		deleteWork = conductor.BoolPtr(finishDeleteWork)
	}

	opts := conductor.FinishOptions{
		SquashMerge:  !finishNoSquash,
		DeleteBranch: finishDelete,
		TargetBranch: finishTargetBranch,
		PushAfter:    finishPush,
		DeleteWork:   deleteWork,
		// PR options
		ForceMerge: finishMerge,
		DraftPR:    finishDraftPR,
		PRTitle:    finishPRTitle,
		PRBody:     finishPRBody,
	}

	// Perform finish
	if err := cond.Finish(ctx, opts); err != nil {
		return fmt.Errorf("finish: %w", err)
	}

	// Success message depends on what happened
	if finishMerge {
		fmt.Println(display.SuccessMsg("Task completed and merged"))
	} else {
		fmt.Println(display.SuccessMsg("Task completed"))
	}
	return nil
}
