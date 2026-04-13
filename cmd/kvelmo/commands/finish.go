package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var FinishCmd = &cobra.Command{
	Use:     "finish",
	Aliases: []string{"fin"},
	Short:   "Clean up after PR merge",
	Long: `Finish cleans up after a pull request has been merged.

This command:
  1. Switches to the base branch (main/master)
  2. Pulls the latest changes
  3. Deletes the local feature branch
  4. Optionally deletes the remote feature branch
  5. Clears the task state

Use --force to finish even if the PR is not merged.`,
	RunE: runFinish,
}

var RefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Check PR status and update local state",
	Long: `Refresh checks the current PR status and provides guidance.

If the PR is merged, it suggests running 'kvelmo finish'.
If the PR is still open, it checks if the branch needs rebasing.`,
	RunE: runRefresh,
}

func init() {
	FinishCmd.Flags().Bool("delete-remote", false, "Delete the remote feature branch")
	FinishCmd.Flags().Bool("force", false, "Finish even if PR is not merged")
}

func runFinish(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(socketPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(socketPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	deleteRemote, _ := cmd.Flags().GetBool("delete-remote")
	force, _ := cmd.Flags().GetBool("force")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "task.finish", map[string]any{
		"delete_remote": deleteRemote,
		"force":         force,
	})
	if err != nil {
		return fmt.Errorf("finish: %w", err)
	}

	var result struct {
		PreviousBranch      string `json:"previous_branch"`
		CurrentBranch       string `json:"current_branch"`
		BranchDeleted       bool   `json:"branch_deleted"`
		RemoteBranchDeleted bool   `json:"remote_branch_deleted"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Task finished!\n")
	fmt.Printf("  Switched to: %s\n", result.CurrentBranch)
	if result.BranchDeleted {
		fmt.Printf("  Deleted local branch: %s\n", result.PreviousBranch)
	}
	if result.RemoteBranchDeleted {
		fmt.Printf("  Deleted remote branch: %s\n", result.PreviousBranch)
	}
	fmt.Printf("\nReady for next task. Run 'kvelmo start' to begin.\n")

	return nil
}

func runRefresh(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(socketPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(socketPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "task.refresh", nil)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	var result struct {
		TaskID            string `json:"task_id"`
		Branch            string `json:"branch"`
		PRStatus          string `json:"pr_status"`
		PRMerged          bool   `json:"pr_merged"`
		PRURL             string `json:"pr_url"`
		CommitsBehindBase int    `json:"commits_behind_base"`
		Action            string `json:"action"`
		Message           string `json:"message"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	// Display status
	fmt.Printf("Task: %s\n", result.TaskID)
	fmt.Printf("Branch: %s\n", result.Branch)

	if result.PRURL != "" {
		fmt.Printf("PR: %s (%s)\n", result.PRURL, result.PRStatus)
	}

	if result.CommitsBehindBase > 0 {
		fmt.Printf("Status: %d commits behind base branch\n", result.CommitsBehindBase)
	}

	fmt.Printf("\n%s\n", result.Message)

	// Suggest next action
	switch result.Action {
	case "merged":
		fmt.Printf("\nRun: kvelmo finish\n")
	case "closed":
		fmt.Printf("\nRun: kvelmo finish --force\n")
	}

	return nil
}
