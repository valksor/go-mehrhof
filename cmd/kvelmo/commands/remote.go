package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/socket"
)

var (
	remoteApproveJSON bool
	remoteMergeJSON   bool
)

// RemoteCmd is the parent command for remote provider operations.
var RemoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Remote provider operations (approve, merge PR/MR)",
	Long:  `Commands for interacting with the remote provider (GitHub, GitLab) after submitting a PR/MR.`,
}

var RemoteApproveCmd = &cobra.Command{
	Use:   "approve",
	Short: "Approve the PR/MR for the current task",
	Long:  `Approves the pull request or merge request associated with the current task.`,
	RunE:  runRemoteApprove,
}

var RemoteMergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge the PR/MR for the current task",
	Long: `Merges the pull request or merge request associated with the current task.
Supports merge methods: merge, squash, rebase (default: rebase).`,
	RunE: runRemoteMerge,
}

func init() {
	// Approve flags
	RemoteApproveCmd.Flags().StringP("comment", "c", "", "Comment to include with the approval")
	RemoteApproveCmd.Flags().BoolVar(&remoteApproveJSON, "json", false, "Output as JSON")

	// Merge flags
	RemoteMergeCmd.Flags().StringP("method", "m", "rebase", "Merge method: merge, squash, rebase")
	RemoteMergeCmd.Flags().BoolVar(&remoteMergeJSON, "json", false, "Output as JSON")

	// Register subcommands
	RemoteCmd.AddCommand(RemoteApproveCmd)
	RemoteCmd.AddCommand(RemoteMergeCmd)
}

func runRemoteApprove(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)

	client, err := socket.NewClient(socketPath, socket.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	comment, _ := cmd.Flags().GetString("comment")

	params := map[string]any{
		"comment": comment,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := client.Call(ctx, "remote.approve", params)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}

	if remoteApproveJSON {
		fmt.Println(string(result.Result))

		return nil
	}

	fmt.Printf("PR approved: %v\n", result)

	return nil
}

func runRemoteMerge(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)

	client, err := socket.NewClient(socketPath, socket.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	method, _ := cmd.Flags().GetString("method")

	params := map[string]any{
		"method": method,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := client.Call(ctx, "remote.merge", params)
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}

	if remoteMergeJSON {
		fmt.Println(string(result.Result))

		return nil
	}

	fmt.Printf("PR merged: %v\n", result)

	return nil
}
