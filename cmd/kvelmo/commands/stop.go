package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current operation",
	Long: `Stop the current operation (planning, implementing, etc.) and return to the previous stable state.

Unlike 'abort' which transitions to Failed state, 'stop' returns to a recoverable state:
  - Planning → Loaded (can re-plan)
  - Implementing → Planned (can re-implement)
  - Simplifying → Implemented (can re-simplify)
  - Optimizing → Implemented (can re-optimize)

This allows you to interrupt a long-running operation and continue from a known good state.`,
	RunE: runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	resp, err := callWorktree(context.Background(), "stop", nil)
	if err != nil {
		return fmt.Errorf("stop call: %w", err)
	}

	var result struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Operation stopped (state: %s)\n", result.State)

	return nil
}
