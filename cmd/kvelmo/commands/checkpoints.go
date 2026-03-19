package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var CheckpointsCmd = &cobra.Command{
	Use:     "checkpoints",
	Aliases: []string{"cp"},
	Short:   "List checkpoint history",
	Long: `List all checkpoints for the current task.
Checkpoints are created after each agent operation (plan, implement, etc.)
and can be navigated with 'undo' and 'redo' commands.`,
	RunE: runCheckpoints,
}

func runCheckpoints(cmd *cobra.Command, args []string) error {
	resp, err := callWorktree(context.Background(), "checkpoints", nil)
	if err != nil {
		return fmt.Errorf("checkpoints call: %w", err)
	}

	if checkpointsJSON {
		return outputJSON(resp.Result)
	}

	// CheckpointInfo matches the socket response structure
	type CheckpointInfo struct {
		SHA       string `json:"sha"`
		Message   string `json:"message"`
		Author    string `json:"author"`
		Timestamp string `json:"timestamp"`
	}

	var result struct {
		Checkpoints []CheckpointInfo `json:"checkpoints"`
		RedoStack   []CheckpointInfo `json:"redo_stack"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Checkpoints) == 0 {
		fmt.Println("No checkpoints")

		return nil
	}

	fmt.Println("Checkpoints (oldest to newest):")
	for i, cp := range result.Checkpoints {
		marker := "  "
		if i == len(result.Checkpoints)-1 {
			marker = "* " // Current position
		}
		shortSHA := cp.SHA
		if len(shortSHA) > 8 {
			shortSHA = shortSHA[:8]
		}
		if cp.Message != "" {
			fmt.Printf("%s%d. %s - %s\n", marker, i+1, shortSHA, cp.Message)
		} else {
			fmt.Printf("%s%d. %s\n", marker, i+1, shortSHA)
		}
	}

	if len(result.RedoStack) > 0 {
		fmt.Printf("\nRedo stack: %d checkpoint(s) available\n", len(result.RedoStack))
	}

	return nil
}

var checkpointsGotoCmd = &cobra.Command{
	Use:   "goto <sha>",
	Short: "Jump to a specific checkpoint SHA",
	Args:  cobra.ExactArgs(1),
	RunE:  runCheckpointsGoto,
}

var checkpointsJSON bool

func init() {
	CheckpointsCmd.Flags().BoolVar(&checkpointsJSON, "json", false, "Output raw JSON response")
	CheckpointsCmd.AddCommand(checkpointsGotoCmd)
}

func runCheckpointsGoto(cmd *cobra.Command, args []string) error {
	sha := args[0]

	resp, err := callWorktree(context.Background(), "checkpoint.goto", map[string]any{"sha": sha})
	if err != nil {
		return fmt.Errorf("checkpoint.goto call: %w", err)
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Moved to checkpoint %s\n", result.SHA[:8])

	return nil
}
