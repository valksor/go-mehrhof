package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var (
	implementWait   bool
	implementJSON   bool
	implementDryRun bool
)

var ImplementCmd = &cobra.Command{
	Use:     "implement",
	Aliases: []string{"impl"},
	Short:   "Start implementation phase for current task",
	Long:    "Submit an implementation job to the worker pool for the current task.",
	RunE:    runImplement,
}

func init() {
	ImplementCmd.Flags().BoolVarP(&implementWait, "wait", "w", false, "Wait for job to complete, streaming output")
	ImplementCmd.Flags().BoolVar(&implementJSON, "json", false, "Output result as JSON")
	ImplementCmd.Flags().BoolVar(&implementDryRun, "dry-run", false, "Simulate without executing agent")
}

func runImplement(cmd *cobra.Command, args []string) error {
	wtPath, err := ensureWorktreeSocket()
	if err != nil {
		return err
	}

	spinner := cli.NewSpinner("Submitting implementation job...")
	spinner.Start()

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		spinner.Fail("Connection failed")

		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := map[string]any{
		"dry_run": implementDryRun,
	}

	// Submit implement job
	resp, err := client.Call(ctx, "implement", params)
	if err != nil {
		spinner.Fail("Implementation submission failed")

		return fmt.Errorf("implement call: %w", err)
	}

	var result ImplementResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse result: %w", err)
	}

	if implementJSON {
		spinner.Stop()
		fmt.Println(string(resp.Result))

		return nil
	}

	spinner.Success("Implementation job submitted: " + result.JobID)

	if implementWait {
		return waitForJob(wtPath, result.JobID)
	}

	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}

type ImplementResult struct {
	JobID string `json:"job_id"`
}
