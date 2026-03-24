package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var AbortCmd = &cobra.Command{
	Use:   "abort",
	Short: "Abort the current task",
	Long: `Abort the current task, stopping any running jobs.
The task state will be preserved and can be resumed later.

Use --force to skip the confirmation prompt.`,
	RunE: runAbort,
}

func init() {
	AbortCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

func runAbort(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Println("This will stop any running jobs and abort the current task.")
		fmt.Print("Are you sure? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")

			return nil
		}
	}

	resp, err := callWorktree(context.Background(), "abort", nil)
	if err != nil {
		return fmt.Errorf("abort call: %w", err)
	}

	var result struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Task aborted (state: %s)\n", result.State)

	return nil
}
