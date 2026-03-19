package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	activitySince      string
	activityMethod     string
	activityErrorsOnly bool
	activityLimit      int
	activityJSON       bool
)

var ActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: "View RPC activity log",
	Long:  "Query the activity log for recent RPC calls with optional filtering by method, errors, and time range.",
	RunE:  runActivity,
}

func init() {
	ActivityCmd.Flags().StringVar(&activitySince, "since", "1h", "Time range (e.g., 1h, 30m, 24h)")
	ActivityCmd.Flags().StringVar(&activityMethod, "method", "", "Filter by method pattern (pipe-separated, e.g., \"start|plan|implement\")")
	ActivityCmd.Flags().BoolVar(&activityErrorsOnly, "errors-only", false, "Show only failed requests")
	ActivityCmd.Flags().IntVar(&activityLimit, "limit", 50, "Maximum entries to return")
	ActivityCmd.Flags().BoolVar(&activityJSON, "json", false, "Output as JSON")
}

func runActivity(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callGlobal(ctx, "activity.query", map[string]any{
		"since":          activitySince,
		"method_pattern": activityMethod,
		"errors_only":    activityErrorsOnly,
		"limit":          activityLimit,
	})
	if err != nil {
		return fmt.Errorf("activity.query: %w", err)
	}

	if activityJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var result struct {
		Entries []struct {
			Timestamp     string `json:"timestamp"`
			Method        string `json:"method"`
			CorrelationID string `json:"correlation_id"`
			DurationMs    int64  `json:"duration_ms"`
			Error         string `json:"error"`
			ParamsSize    int    `json:"params_size"`
			UserID        string `json:"user_id"`
			TaskID        string `json:"task_id"`
			AgentModel    string `json:"agent_model"`
		} `json:"entries"`
		Count   int  `json:"count"`
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if !result.Enabled {
		fmt.Println("Activity log is not enabled.")
		fmt.Println("Enable it with: kvelmo config set storage.activity_log.enabled true")

		return nil
	}

	if result.Count == 0 {
		fmt.Println("No activity entries found.")

		return nil
	}

	fmt.Printf("Activity log (%d entries)\n\n", result.Count)
	for _, e := range result.Entries {
		t, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
		status := "OK"
		if e.Error != "" {
			status = "ERR"
		}
		userInfo := ""
		if e.UserID != "" {
			userInfo = "  [" + e.UserID + "]"
		}
		fmt.Printf("  %s  %-30s  %4dms  %s%s\n", t.Format("15:04:05"), e.Method, e.DurationMs, status, userInfo)
		if e.Error != "" {
			fmt.Printf("           %s\n", e.Error)
		}
	}

	return nil
}
