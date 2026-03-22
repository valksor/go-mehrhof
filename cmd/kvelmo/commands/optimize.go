package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/cli"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var (
	optimizeWait   bool
	optimizeDryRun bool
)

var OptimizeCmd = &cobra.Command{
	Use:     "optimize",
	Aliases: []string{"opt"},
	Short:   "Run optional optimization pass on implemented code",
	Long: `Run an optional optimization pass on the implemented code.

This command is only available after implementation is complete.
It submits an optimization job to the worker pool that will:
- Improve code quality and readability
- Add missing error handling
- Optimize performance where applicable
- Ensure proper documentation/comments
- Check for edge cases
- Ensure tests are comprehensive

You can run optimize multiple times before proceeding to review.`,
	RunE: runOptimize,
}

func init() {
	OptimizeCmd.Flags().BoolVarP(&optimizeWait, "wait", "w", false, "Wait for job to complete, streaming output")
	OptimizeCmd.Flags().BoolVar(&optimizeDryRun, "dry-run", false, "Simulate without executing agent")
}

func runOptimize(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	spinner := cli.NewSpinner("Submitting optimization job...")
	spinner.Start()

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		spinner.Fail("Connection failed")

		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var params map[string]any
	if optimizeDryRun {
		params = map[string]any{"dry_run": true}
	}

	resp, err := client.Call(ctx, "optimize", params)
	if err != nil {
		spinner.Fail("Optimization submission failed")

		return fmt.Errorf("optimize call: %w", err)
	}

	var result OptimizeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse result: %w", err)
	}

	spinner.Success("Optimization job submitted: " + result.JobID)

	if optimizeWait {
		return waitForJob(wtPath, result.JobID)
	}

	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}

type OptimizeResult struct {
	JobID string `json:"job_id"`
}
