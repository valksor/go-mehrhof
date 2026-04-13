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

var SimplifyCmd = &cobra.Command{
	Use:     "simplify",
	Aliases: []string{"simp"},
	Short:   "Run optional simplification pass on implemented code",
	Long: `Run an optional simplification pass on the implemented code.

This command is only available after implementation is complete.
It submits a simplification job to the worker pool that will:
- Remove unnecessary complexity and abstractions
- Simplify control flow where possible
- Remove dead code and unused variables
- Consolidate duplicate logic
- Use clearer, more descriptive names
- Break down overly long functions
- Prefer standard library solutions over custom implementations

Focus is on making code easier to understand and maintain.
You can run simplify multiple times before proceeding to review.`,
	RunE: runSimplify,
}

var (
	simplifyWait   bool
	simplifyDryRun bool
)

func init() {
	SimplifyCmd.Flags().BoolVarP(&simplifyWait, "wait", "w", false, "Wait for job to complete, streaming output")
	SimplifyCmd.Flags().BoolVar(&simplifyDryRun, "dry-run", false, "Simulate without executing agent")
}

func runSimplify(cmd *cobra.Command, args []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var params map[string]any
	if simplifyDryRun {
		params = map[string]any{"dry_run": true}
	}

	resp, err := client.Call(ctx, "simplify", params)
	if err != nil {
		return fmt.Errorf("simplify call: %w", err)
	}

	var result SimplifyResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Simplification job submitted: %s\n", result.JobID)

	if simplifyWait {
		return waitForJob(wtPath, result.JobID)
	}

	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}

type SimplifyResult struct {
	JobID string `json:"job_id"`
}
