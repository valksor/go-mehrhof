package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var retryPhase string

var RetryCmd = &cobra.Command{
	Use:   "retry",
	Short: "Re-run the last failed phase",
	Long: `Retry the last failed phase by resetting from failed state and re-running the appropriate command.

When a task is in 'failed' state, retry will:
  1. Query status to confirm the task is in failed state
  2. Reset the task back to the previous stable state
  3. Re-run the phase that failed (plan, implement, simplify, optimize)

The failed phase is inferred from the task's progress:
  - No specifications → retry plan
  - Has specifications → retry implement

Use --phase to override the inferred phase:
  kvelmo retry --phase plan
  kvelmo retry --phase implement
  kvelmo retry --phase simplify
  kvelmo retry --phase optimize`,
	RunE: runRetry,
}

func init() {
	RetryCmd.Flags().StringVar(&retryPhase, "phase", "", "Override which phase to retry (plan, implement, simplify, optimize)")
}

func runRetry(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Step 1: Get current status to confirm failed state and determine phase.
	spinner := cli.NewSpinner("Checking task status...")
	spinner.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, subStatus, nil)
	if err != nil {
		spinner.Fail("Status check failed")

		return fmt.Errorf("status call: %w", err)
	}

	var status socket.StatusResult
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		spinner.Fail("Invalid status response")

		return fmt.Errorf("parse status: %w", err)
	}

	if status.State != socket.StateFailed {
		spinner.Fail("Task is not in failed state")

		return fmt.Errorf("retry requires task in 'failed' state, currently '%s'", status.State)
	}

	// Determine which phase to retry.
	phase := retryPhase
	if phase == "" {
		phase = inferFailedPhase(status.LastError)
	}

	if err := validatePhase(phase); err != nil {
		spinner.Fail("Invalid phase")

		return err
	}

	spinner.Success("Task is failed, will retry: " + phase)

	// Step 2: Reset the task from failed state.
	spinner = cli.NewSpinner("Resetting task...")
	spinner.Start()

	resetCtx, resetCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resetCancel()

	resetResp, err := client.Call(resetCtx, "reset", nil)
	if err != nil {
		spinner.Fail("Reset failed")

		return fmt.Errorf("reset call: %w", err)
	}

	var resetResult struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(resetResp.Result, &resetResult); err != nil {
		spinner.Fail("Invalid reset response")

		return fmt.Errorf("parse reset result: %w", err)
	}

	spinner.Success(fmt.Sprintf("Task reset (state: %s)", resetResult.State))

	// Step 3: Re-run the failed phase.
	spinner = cli.NewSpinner(fmt.Sprintf("Submitting %s job...", phase))
	spinner.Start()

	phaseCtx, phaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer phaseCancel()

	phaseResp, err := client.Call(phaseCtx, phase, nil)
	if err != nil {
		spinner.Fail(capitalize(phase) + " submission failed")

		return fmt.Errorf("%s call: %w", phase, err)
	}

	var phaseResult struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(phaseResp.Result, &phaseResult); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse %s result: %w", phase, err)
	}

	spinner.Success(fmt.Sprintf("%s job submitted: %s", capitalize(phase), phaseResult.JobID))
	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}

// inferFailedPhase determines which phase to retry based on the last error message.
// This is a heuristic; users can override with --phase.
func inferFailedPhase(lastError string) string {
	// Check if the last error hints at which phase failed.
	for _, keyword := range []struct {
		phase   string
		pattern string
	}{
		{phaseImplement, phaseImplement},
		{phaseSimplify, phaseSimplify},
		{phaseOptimize, phaseOptimize},
		{phasePlan, phasePlan},
	} {
		if strings.Contains(strings.ToLower(lastError), keyword.pattern) {
			return keyword.phase
		}
	}

	// Default to plan for ambiguous failures
	return phasePlan
}

func validatePhase(phase string) error {
	switch phase {
	case phasePlan, phaseImplement, phaseSimplify, phaseOptimize:
		return nil
	default:
		return fmt.Errorf("invalid phase %q: must be one of plan, implement, simplify, optimize", phase)
	}
}
