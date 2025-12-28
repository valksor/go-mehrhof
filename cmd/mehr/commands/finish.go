package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

var (
	finishYes           bool
	finishNoPush        bool
	finishNoDelete      bool
	finishNoSquash      bool
	finishTargetBranch  string
	finishSkipQuality   bool
	finishQualityTarget string
	// PR-related flags
	finishCreatePR bool
	finishDraftPR  bool
	finishPRTitle  string
	finishPRBody   string
)

var finishCmd = &cobra.Command{
	Use:   "finish",
	Short: "Complete the task and merge (or create PR)",
	Long: `Complete the current task and merge changes to the target branch, or create a pull request.

By default, this:
- Runs 'make quality' if available (code formatting, linting, etc.)
- Performs a squash merge to keep the history clean
- Deletes the task branch
- Does NOT push to remote (use --push to enable)

When using --pr, this creates a pull request instead of merging locally:
- Pushes the branch to origin
- Creates a PR via the GitHub API (for github: tasks)
- Does NOT delete the branch or merge locally
- Optionally posts a comment to the original issue

If quality checks modify files (e.g., auto-formatting), you'll be prompted
to confirm before proceeding.

Examples:
  mehr finish                      # Complete and merge (with confirmation)
  mehr finish --yes                # Skip confirmation prompt
  mehr finish --no-push            # Merge but don't push
  mehr finish --no-delete          # Keep task branch after merge
  mehr finish --no-squash          # Regular merge instead of squash
  mehr finish --target develop     # Merge to specific branch
  mehr finish --skip-quality       # Skip quality checks
  mehr finish --quality-target lint # Use custom make target
  mehr finish --pr                 # Create PR instead of merging
  mehr finish --pr --draft         # Create PR as draft
  mehr finish --pr --pr-title "Fix bug" # Custom PR title`,
	RunE: runFinish,
}

func init() {
	rootCmd.AddCommand(finishCmd)

	finishCmd.Flags().BoolVarP(&finishYes, "yes", "y", false, "Skip confirmation prompt")
	finishCmd.Flags().BoolVar(&finishNoPush, "no-push", false, "Don't push after merge")
	finishCmd.Flags().BoolVar(&finishNoDelete, "no-delete", false, "Don't delete task branch")
	finishCmd.Flags().BoolVar(&finishNoSquash, "no-squash", false, "Use regular merge instead of squash")
	finishCmd.Flags().StringVarP(&finishTargetBranch, "target", "t", "", "Target branch to merge into")
	finishCmd.Flags().BoolVar(&finishSkipQuality, "skip-quality", false, "Skip quality checks (make quality)")
	finishCmd.Flags().StringVar(&finishQualityTarget, "quality-target", "quality", "Make target for quality checks")

	// PR-related flags
	finishCmd.Flags().BoolVar(&finishCreatePR, "pr", false, "Create pull request instead of merging locally")
	finishCmd.Flags().BoolVar(&finishDraftPR, "draft", false, "Create PR as draft (requires --pr)")
	finishCmd.Flags().StringVar(&finishPRTitle, "pr-title", "", "Custom PR title (requires --pr)")
	finishCmd.Flags().StringVar(&finishPRBody, "pr-body", "", "Custom PR body (requires --pr)")
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
		return fmt.Errorf("no active task to finish\nUse 'mehr start <reference>' to register a task first")
	}

	// Get status for display
	status, err := cond.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Validate PR flags
	if !finishCreatePR && (finishDraftPR || finishPRTitle != "" || finishPRBody != "") {
		return fmt.Errorf("--draft, --pr-title, and --pr-body require --pr flag")
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

	if finishCreatePR {
		promptLines += "\n\nThis will create a pull request"
		if finishDraftPR {
			promptLines += " (as draft)"
		}
		promptLines += "."
	} else {
		promptLines += "\n\nThis will merge changes"
		if !finishNoDelete && status.Branch != "" {
			promptLines += " and delete the task branch"
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
				fmt.Println("Quality checks passed")
			}
		}
	}

	// Build finish options
	opts := conductor.FinishOptions{
		SquashMerge:  !finishNoSquash,
		DeleteBranch: !finishNoDelete,
		TargetBranch: finishTargetBranch,
		PushAfter:    !finishNoPush,
		// PR options
		CreatePR: finishCreatePR,
		DraftPR:  finishDraftPR,
		PRTitle:  finishPRTitle,
		PRBody:   finishPRBody,
	}

	// Perform finish
	if err := cond.Finish(ctx, opts); err != nil {
		return fmt.Errorf("finish: %w", err)
	}

	if finishCreatePR {
		fmt.Println("Pull request created successfully")
	} else {
		fmt.Println("Task completed and merged successfully")
	}
	return nil
}
