package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// ForkCmd is the top-level fork command with subcommands.
var ForkCmd = &cobra.Command{
	Use:   "fork",
	Short: "Manage conversation forks for parallel approaches",
	Long:  "Create, list, compare, and select parallel forks of the current task to try different implementation approaches.",
}

var forkCreateCmd = &cobra.Command{
	Use:   "create <label>",
	Short: "Create a new fork from the current checkpoint",
	Args:  cobra.ExactArgs(1),
	RunE:  runForkCreate,
}

var forkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active forks",
	RunE:  runForkList,
}

var forkCompareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare all active forks",
	RunE:  runForkCompare,
}

var forkSelectCmd = &cobra.Command{
	Use:   "select <fork-id>",
	Short: "Select the winning fork and merge it back",
	Args:  cobra.ExactArgs(1),
	RunE:  runForkSelect,
}

var forkListJSON bool
var forkCompareJSON bool

func init() {
	ForkCmd.AddCommand(forkCreateCmd)
	ForkCmd.AddCommand(forkListCmd)
	ForkCmd.AddCommand(forkCompareCmd)
	ForkCmd.AddCommand(forkSelectCmd)

	forkListCmd.Flags().BoolVar(&forkListJSON, "json", false, "Output raw JSON response")
	forkCompareCmd.Flags().BoolVar(&forkCompareJSON, "json", false, "Output raw JSON response")
}

func runForkCreate(_ *cobra.Command, args []string) error {
	label := args[0]

	resp, err := callWorktree(context.Background(), "fork.create", map[string]any{
		"label": label,
	})
	if err != nil {
		return fmt.Errorf("fork.create: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("fork.create: %s", resp.Error.Message)
	}

	var result struct {
		ID            string `json:"id"`
		Label         string `json:"label"`
		Branch        string `json:"branch"`
		WorktreeDir   string `json:"worktree_dir"`
		CheckpointSHA string `json:"checkpoint_sha"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Fork created: %s\n", result.Label)
	fmt.Printf("  ID:        %s\n", result.ID)
	fmt.Printf("  Branch:    %s\n", result.Branch)
	fmt.Printf("  Worktree:  %s\n", result.WorktreeDir)
	fmt.Printf("  Base SHA:  %s\n", shortSHA(result.CheckpointSHA))

	return nil
}

func runForkList(_ *cobra.Command, _ []string) error {
	resp, err := callWorktree(context.Background(), "fork.list", nil)
	if err != nil {
		return fmt.Errorf("fork.list: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("fork.list: %s", resp.Error.Message)
	}

	if forkListJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Forks []struct {
			ID            string    `json:"id"`
			Label         string    `json:"label"`
			Branch        string    `json:"branch"`
			State         string    `json:"state"`
			CheckpointSHA string    `json:"checkpoint_sha"`
			TokensUsed    int64     `json:"tokens_used"`
			CreatedAt     time.Time `json:"created_at"`
		} `json:"forks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Forks) == 0 {
		fmt.Println("No active forks.")

		return nil
	}

	fmt.Printf("Active forks (%d):\n", len(result.Forks))
	for _, f := range result.Forks {
		fmt.Printf("  %s  %-20s  [%s]  base: %s\n",
			f.ID, f.Label, f.State, shortSHA(f.CheckpointSHA))
	}

	return nil
}

func runForkCompare(_ *cobra.Command, _ []string) error {
	resp, err := callWorktree(context.Background(), "fork.compare", nil)
	if err != nil {
		return fmt.Errorf("fork.compare: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("fork.compare: %s", resp.Error.Message)
	}

	if forkCompareJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Forks []struct {
			ID            string `json:"id"`
			Label         string `json:"label"`
			State         string `json:"state"`
			FilesAdded    int    `json:"files_added"`
			FilesModified int    `json:"files_modified"`
			LinesAdded    int    `json:"lines_added"`
			LinesRemoved  int    `json:"lines_removed"`
			DiffStats     struct {
				Files   int `json:"files"`
				Added   int `json:"added"`
				Removed int `json:"removed"`
			} `json:"diff_stats"`
		} `json:"forks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Forks) == 0 {
		fmt.Println("No active forks to compare.")

		return nil
	}

	fmt.Println("Fork comparison:")
	for _, f := range result.Forks {
		fmt.Printf("  %s  %-20s  files: %d  +%d/-%d lines\n",
			f.ID, f.Label, f.DiffStats.Files, f.LinesAdded, f.LinesRemoved)
	}

	return nil
}

func runForkSelect(_ *cobra.Command, args []string) error {
	forkID := args[0]

	resp, err := callWorktree(context.Background(), "fork.select", map[string]any{
		"fork_id": forkID,
	})
	if err != nil {
		return fmt.Errorf("fork.select: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("fork.select: %s", resp.Error.Message)
	}

	fmt.Printf("Fork %s selected and merged successfully.\n", forkID)

	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}

	return sha
}
