package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/cli"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
	"github.com/valksor/kvelmo/pkg/storage"
)

var (
	reviewWait     bool
	reviewListJSON bool
)

var ReviewCmd = &cobra.Command{
	Use:     "review",
	Aliases: []string{"rev"},
	Short:   "Review current implementation and approve for submission",
	Long: `Moves the current task to reviewing state. This is the human approval
gate before submission. After reviewing, use 'submit' to push changes.

Subcommands:
  review list        List all reviews for current task
  review view <N>    View review number N`,
	RunE: runReview,
}

func init() {
	ReviewCmd.Flags().Bool("approve", false, "Immediately approve (skip interactive review)")
	ReviewCmd.Flags().Bool("reject", false, "Reject and return to planning state")
	ReviewCmd.Flags().StringP("message", "m", "", "Review message/notes")
	ReviewCmd.Flags().Bool("fix", false, "Auto-fix issues after entering review state")
	ReviewCmd.Flags().Bool("force", false, "Re-run review even if already reviewed")
	ReviewCmd.Flags().Bool("adversarial", false, "Run adversarial review (stress-test the implementation)")
	ReviewCmd.Flags().BoolVarP(&reviewWait, "wait", "w", false, "Wait for job to complete, streaming output")

	reviewListCmd.Flags().BoolVar(&reviewListJSON, "json", false, "Output as JSON")

	ReviewCmd.AddCommand(reviewListCmd)
	ReviewCmd.AddCommand(reviewViewCmd)
	ReviewCmd.AddCommand(reviewAdversarialResultsCmd)
	ReviewCmd.AddCommand(reviewRiskCmd)
	ReviewCmd.AddCommand(reviewRiskHistoryCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)

	spinner := cli.NewSpinner("Submitting review job...")
	spinner.Start()

	client, err := socket.NewClient(socketPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		spinner.Fail("Connection failed")

		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	approve, _ := cmd.Flags().GetBool("approve")
	reject, _ := cmd.Flags().GetBool("reject")
	message, _ := cmd.Flags().GetString("message")
	fix, _ := cmd.Flags().GetBool("fix")
	force, _ := cmd.Flags().GetBool("force")
	adversarial, _ := cmd.Flags().GetBool("adversarial")

	// If --adversarial, run adversarial review via separate RPC
	if adversarial {
		ctx := context.Background()
		resp, advErr := client.Call(ctx, "adversarial.run", nil)
		if advErr != nil {
			spinner.Fail("Adversarial review failed")

			return fmt.Errorf("adversarial.run: %w", advErr)
		}
		spinner.Success("Adversarial review started")

		return outputJSON(resp.Result)
	}

	params := map[string]any{
		"approve": approve,
		"reject":  reject,
		"message": message,
		"fix":     fix,
		"force":   force,
	}

	ctx := context.Background()
	resp, err := client.Call(ctx, "review", params)
	if err != nil {
		spinner.Fail("Review submission failed")

		return fmt.Errorf("review: %w", err)
	}

	var result struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse result: %w", err)
	}

	spinner.Success("Review: " + result.Status)
	if fix {
		fmt.Println("Fix mode enabled: agent is reviewing and fixing issues")
	}

	if reviewWait && result.JobID != "" {
		return waitForJob(socketPath, result.JobID)
	}

	if !reviewWait {
		fmt.Println("Use '" + meta.Name + " status' to check progress")
	}

	return nil
}

// reviewListCmd lists all reviews for the current task.
var reviewListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all reviews for current task",
	RunE:  runReviewList,
}

func runReviewList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)

	client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	resp, err := client.Call(ctx, "review.list", nil)
	if err != nil {
		return fmt.Errorf("review.list: %w", err)
	}

	if reviewListJSON {
		fmt.Println(string(resp.Result))

		return nil
	}

	var result struct {
		Reviews []storage.Review `json:"reviews"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Reviews) == 0 {
		fmt.Println("No reviews yet")

		return nil
	}

	fmt.Printf("%-4s  %-20s  %-8s  %s\n", "#", "Timestamp", "Decision", "Title")
	fmt.Printf("%-4s  %-20s  %-8s  %s\n", "---", "-------------------", "--------", "-------")
	for _, r := range result.Reviews {
		decision := "Rejected"
		if r.Status == storage.ReviewStatusApproved {
			decision = "Approved"
		}
		ts := r.CreatedAt
		if ts.IsZero() {
			ts = r.UpdatedAt
		}
		fmt.Printf("%-4d  %-20s  %-8s  %s\n",
			r.Number,
			ts.Format("2006-01-02 15:04:05"),
			decision,
			r.Title,
		)
	}

	return nil
}

// reviewViewCmd shows a specific review.
var reviewViewCmd = &cobra.Command{
	Use:   "view <N>",
	Short: "View review number N",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewView,
}

func runReviewView(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)

	client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	var number int
	if _, scanErr := fmt.Sscanf(args[0], "%d", &number); scanErr != nil {
		return fmt.Errorf("invalid review number: %s", args[0])
	}

	params := map[string]any{"number": number}

	ctx := context.Background()
	resp, err := client.Call(ctx, "review.view", params)
	if err != nil {
		return fmt.Errorf("review.view: %w", err)
	}

	var review storage.Review
	if err := json.Unmarshal(resp.Result, &review); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	decision := "Rejected"
	if review.Status == storage.ReviewStatusApproved {
		decision = "Approved"
	}

	ts := review.CreatedAt
	if ts.IsZero() {
		ts = review.UpdatedAt
	}

	fmt.Printf("Review #%d\n", review.Number)
	fmt.Printf("Time:     %s\n", ts.Format("2006-01-02 15:04:05"))
	fmt.Printf("Decision: %s\n", decision)
	if review.Title != "" {
		fmt.Printf("Title:    %s\n", review.Title)
	}
	if review.Content != "" {
		fmt.Printf("\n%s\n", review.Content)
	}

	return nil
}

// reviewAdversarialResultsCmd shows results of the last adversarial review.
var reviewAdversarialResultsCmd = &cobra.Command{
	Use:   "adversarial-results",
	Short: "Show results of the last adversarial review",
	RunE:  runReviewAdversarialResults,
}

func runReviewAdversarialResults(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "adversarial.results", nil)
	if err != nil {
		return fmt.Errorf("adversarial.results: %w", err)
	}

	return outputJSON(resp.Result)
}

// reviewRiskCmd evaluates the risk score for the current task.
var reviewRiskCmd = &cobra.Command{
	Use:   "risk",
	Short: "Evaluate risk score for the current task",
	RunE:  runReviewRisk,
}

func runReviewRisk(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "risk.evaluate", nil)
	if err != nil {
		return fmt.Errorf("risk.evaluate: %w", err)
	}

	return outputJSON(resp.Result)
}

// reviewRiskHistoryCmd shows the risk evaluation history.
var reviewRiskHistoryCmd = &cobra.Command{
	Use:   "risk-history",
	Short: "Show risk evaluation history",
	RunE:  runReviewRiskHistory,
}

func runReviewRiskHistory(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "risk.history", nil)
	if err != nil {
		return fmt.Errorf("risk.history: %w", err)
	}

	return outputJSON(resp.Result)
}
