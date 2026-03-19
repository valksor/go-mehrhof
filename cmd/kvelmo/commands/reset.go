package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var ResetCmd = &cobra.Command{
	Use:     "reset",
	Aliases: []string{"rst"},
	Short:   "Reset the current task to initial state",
	Long: `Reset the current task, clearing all progress and returning to 'none' state.
This will discard any uncommitted changes.`,
	RunE: runReset,
}

func runReset(cmd *cobra.Command, args []string) error {
	resp, err := callWorktree(context.Background(), "reset", nil)
	if err != nil {
		return fmt.Errorf("reset call: %w", err)
	}

	var result struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Task reset (state: %s)\n", result.State)

	return nil
}
