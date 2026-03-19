package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var DiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show what the agent changed",
	Long: `Show the diff against the last checkpoint, highlighting what the AI agent changed.
Falls back to regular git diff if no checkpoints exist.`,
	RunE: runDiff,
}

var diffJSON bool

func init() {
	DiffCmd.Flags().Bool("stat", false, "Show only file summary")
	DiffCmd.Flags().BoolVar(&diffJSON, "json", false, "Output raw JSON response")
}

func runDiff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	stat, _ := cmd.Flags().GetBool("stat")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get checkpoints to find the pre-action checkpoint
	resp, err := client.Call(ctx, "checkpoints", nil)
	if err != nil {
		return fmt.Errorf("checkpoints call: %w", err)
	}

	var checkpointsResult struct {
		Checkpoints []struct {
			SHA string `json:"sha"`
		} `json:"checkpoints"`
	}
	if err := json.Unmarshal(resp.Result, &checkpointsResult); err != nil {
		return fmt.Errorf("parse checkpoints: %w", err)
	}

	// If we have at least 2 checkpoints, diff against the second-to-last (pre-action).
	// If we have exactly 1, diff against that one.
	// Otherwise fall back to regular git diff.
	if len(checkpointsResult.Checkpoints) >= 2 {
		ref := checkpointsResult.Checkpoints[len(checkpointsResult.Checkpoints)-2].SHA

		return showDiffAgainst(ctx, client, ref, stat)
	}

	if len(checkpointsResult.Checkpoints) == 1 {
		ref := checkpointsResult.Checkpoints[0].SHA

		return showDiffAgainst(ctx, client, ref, stat)
	}

	// No checkpoints — fall back to regular git diff
	return showRegularDiff(ctx, client, stat)
}

func showDiffAgainst(ctx context.Context, client *socket.Client, ref string, stat bool) error {
	resp, err := client.Call(ctx, "git.diff_against", map[string]any{
		"ref":  ref,
		"stat": stat,
	})
	if err != nil {
		return fmt.Errorf("git.diff_against call: %w", err)
	}

	if diffJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Diff == "" {
		fmt.Println("No changes since last checkpoint")
	} else {
		fmt.Print(result.Diff)
	}

	return nil
}

func showRegularDiff(ctx context.Context, client *socket.Client, stat bool) error {
	if stat {
		// Use diff_against with HEAD for stat-only view
		resp, err := client.Call(ctx, "git.diff_against", map[string]any{
			"ref":  "HEAD",
			"stat": true,
		})
		if err != nil {
			return fmt.Errorf("git.diff_against call: %w", err)
		}

		if diffJSON {
			return outputJSON(resp.Result)
		}

		var result struct {
			Diff string `json:"diff"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return fmt.Errorf("parse result: %w", err)
		}

		if result.Diff == "" {
			fmt.Println("No changes")
		} else {
			fmt.Print(result.Diff)
		}

		return nil
	}

	resp, err := client.Call(ctx, "git.diff", map[string]any{"cached": false})
	if err != nil {
		return fmt.Errorf("git.diff call: %w", err)
	}

	if diffJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Diff == "" {
		fmt.Println("No changes")
	} else {
		fmt.Print(result.Diff)
	}

	return nil
}
