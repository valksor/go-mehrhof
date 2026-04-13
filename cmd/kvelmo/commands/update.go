package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
)

var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Re-fetch task from provider and generate delta specification if changed",
	Long: `Re-fetches the current task from its original provider and checks for changes.
If the task has changed, a delta specification file is generated describing what changed.`,
	RunE: runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	resp, err := callWorktree(context.Background(), "update", nil)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	var result socket.UpdateResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if !result.Changed {
		fmt.Println("Task unchanged")

		return nil
	}

	fmt.Println("Task updated")
	if result.NewSpecification != "" {
		fmt.Printf("Delta specification: %s\n", result.NewSpecification)
	}

	return nil
}
