package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/cli"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var (
	planWait   bool
	planJSON   bool
	planDryRun bool
)

var PlanCmd = &cobra.Command{
	Use:     "plan",
	Aliases: []string{"pl"},
	Short:   "Start planning phase for current task",
	Long:    "Submit a planning job to the worker pool for the current task.",
	RunE:    runPlan,
}

func init() {
	PlanCmd.Flags().BoolVarP(&planWait, "wait", "w", false, "Wait for job to complete, streaming output")
	PlanCmd.Flags().BoolVar(&planJSON, "json", false, "Output result as JSON")
	PlanCmd.Flags().BoolVar(&planDryRun, "dry-run", false, "Simulate without executing agent")
}

func runPlan(cmd *cobra.Command, args []string) error {
	wtPath, err := ensureWorktreeSocket()
	if err != nil {
		return err
	}

	spinner := cli.NewSpinner("Submitting plan job...")
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
	if planDryRun {
		params = map[string]any{"dry_run": true}
	}

	// Submit plan job
	resp, err := client.Call(ctx, "plan", params)
	if err != nil {
		spinner.Fail("Plan submission failed")

		return fmt.Errorf("plan call: %w", err)
	}

	var result PlanResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse result: %w", err)
	}

	if planJSON {
		spinner.Stop()
		fmt.Println(string(resp.Result))

		return nil
	}

	spinner.Success("Planning job submitted: " + result.JobID)

	if planWait {
		return waitForJob(wtPath, result.JobID)
	}

	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}

type PlanResult struct {
	JobID string `json:"job_id"`
}
