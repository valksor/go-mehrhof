package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/internal/socket"
)

var (
	batchStateFilter string
	batchTagFilter   string
	batchMatchFilter string
	batchJSON        bool
)

var BatchCmd = &cobra.Command{
	Use:   "batch <action>",
	Short: "Run an action across all active projects",
	Long: `Execute an action on all active projects matching the optional filters.

Actions:
  plan      Plan all matching tasks
  implement Implement all matching tasks
  review    Review all matching tasks
  submit    Submit all matching tasks (creates PRs)
  abort     Abort all matching tasks
  reset     Reset all matching tasks
  stop      Stop all matching tasks

Filters:
  --state   Filter by task state (e.g. reviewing, failed)
  --tag     Filter by task tag
  --match   Filter by project path substring

Examples:
  kvelmo batch submit --state reviewing   Submit all reviewed tasks
  kvelmo batch stop                       Stop all active tasks
  kvelmo batch abort --state failed       Abort all failed tasks
  kvelmo batch stop --tag backend         Stop all tasks tagged 'backend'
  kvelmo batch submit --match myproject   Submit tasks in paths containing 'myproject'`,
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return []string{
			"plan\tPlan all matching tasks",
			"implement\tImplement all matching tasks",
			"review\tReview all matching tasks",
			"submit\tSubmit all matching tasks (creates PRs)",
			"abort\tAbort all matching tasks",
			"reset\tReset all matching tasks",
			"stop\tStop all matching tasks",
		}, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: runBatch,
}

func init() {
	BatchCmd.Flags().StringVar(&batchStateFilter, "state", "", "Only act on tasks in this state")
	BatchCmd.Flags().StringVar(&batchTagFilter, "tag", "", "Only act on tasks with this tag")
	BatchCmd.Flags().StringVar(&batchMatchFilter, "match", "", "Only act on projects whose path contains this substring")
	BatchCmd.Flags().BoolVar(&batchJSON, "json", false, "Output raw JSON response")
}

func runBatch(_ *cobra.Command, args []string) error {
	action := args[0]

	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running (run 'kvelmo serve' first)")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	params := map[string]any{
		paramAction: action,
	}
	filter := map[string]string{}
	if batchStateFilter != "" {
		filter["state"] = batchStateFilter
	}
	if batchTagFilter != "" {
		filter["tag"] = batchTagFilter
	}
	if batchMatchFilter != "" {
		filter["match"] = batchMatchFilter
	}
	if len(filter) > 0 {
		params["filter"] = filter
	}

	spinner := cli.NewSpinner(fmt.Sprintf("Running batch %s...", action))
	spinner.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "tasks.batch", params)
	if err != nil {
		spinner.Fail("Batch operation failed")

		return fmt.Errorf("batch call: %w", err)
	}

	if batchJSON {
		spinner.Stop()

		return outputJSON(resp.Result)
	}

	var result struct {
		Action  string `json:"action"`
		Total   int    `json:"total"`
		Results []struct {
			Path    string `json:"path"`
			State   string `json:"state"`
			Success bool   `json:"success"`
			Error   string `json:"error,omitempty"`
		} `json:"results"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse result: %w", err)
	}

	if result.Total == 0 {
		spinner.Success("No matching tasks found")

		return nil
	}

	succeeded := 0
	for _, r := range result.Results {
		if r.Success {
			succeeded++
		}
	}

	spinner.Success(fmt.Sprintf("Batch %s: %d/%d succeeded", action, succeeded, result.Total))

	// Print details
	for _, r := range result.Results {
		status := cli.Green.Sprint("ok")
		if !r.Success {
			status = cli.Red.Sprint("fail")
		}
		detail := r.State
		if r.Error != "" {
			detail = r.Error
		}
		fmt.Printf("  [%s] %s (%s)\n", status, r.Path, detail)
	}

	return nil
}
