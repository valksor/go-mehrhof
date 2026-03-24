package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var AbandonCmd = &cobra.Command{
	Use:     "abandon",
	Aliases: []string{"abn"},
	Short:   "Abandon the current task",
	Long: `Stop any running jobs, delete the git branch (unless --keep-branch), and reset state.
If worktree isolation was used, the worktree is also removed.`,
	RunE: runAbandon,
}

func init() {
	AbandonCmd.Flags().Bool("keep-branch", false, "Keep the git branch after abandoning")
}

func runAbandon(cmd *cobra.Command, args []string) error {
	keepBranch, _ := cmd.Flags().GetBool("keep-branch")

	base := cmd.Context()
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 30*time.Second)
	defer cancel()

	_, err := callWorktree(ctx, "abandon", map[string]any{
		"keep_branch": keepBranch,
	})
	if err != nil {
		return fmt.Errorf("abandon: %w", err)
	}

	fmt.Println("Task abandoned")
	if !keepBranch {
		fmt.Println("Branch deleted")
	}

	return nil
}
