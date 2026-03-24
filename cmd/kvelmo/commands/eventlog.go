package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	eventlogType  string
	eventlogPhase string
	eventlogSince string
	eventlogJSON  bool
)

var EventlogCmd = &cobra.Command{
	Use:   "eventlog",
	Short: "View task lifecycle event log",
	Long: `Query the lifecycle event log for the current task.

Events include phase starts, completions, failures, checkpoint creation,
finding detection, and other lifecycle transitions.

Event types: phase_started, phase_completed, phase_failed, checkpoint_created,
finding_detected, router_decision, spec_changed, guardrail_checked,
task_loaded, task_finished`,
	RunE: runEventlog,
}

func init() {
	EventlogCmd.Flags().StringVar(&eventlogType, "type", "", "Filter by event type (e.g., phase_started)")
	EventlogCmd.Flags().StringVar(&eventlogPhase, "phase", "", "Filter by phase (e.g., plan, implement)")
	EventlogCmd.Flags().StringVar(&eventlogSince, "since", "", "Time range (e.g., 1h, 30m)")
	EventlogCmd.Flags().BoolVar(&eventlogJSON, "json", false, "Output as JSON")
}

func runEventlog(_ *cobra.Command, _ []string) error {
	params := map[string]any{}
	if eventlogType != "" {
		params["type"] = eventlogType
	}
	if eventlogPhase != "" {
		params["phase"] = eventlogPhase
	}
	if eventlogSince != "" {
		params["since"] = eventlogSince
	}

	resp, err := callWorktree(context.Background(), "eventlog.query", params)
	if err != nil {
		return fmt.Errorf("eventlog.query: %w", err)
	}

	if eventlogJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Entries []struct {
			Timestamp time.Time      `json:"timestamp"`
			Type      string         `json:"type"`
			Phase     string         `json:"phase"`
			Message   string         `json:"message"`
			Data      map[string]any `json:"data"`
		} `json:"entries"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Total == 0 {
		fmt.Println("No lifecycle events found.")

		return nil
	}

	// Use date+time format if events span multiple days, time-only otherwise.
	timeFormat := "15:04:05"
	if len(result.Entries) > 1 {
		first := result.Entries[0].Timestamp
		last := result.Entries[len(result.Entries)-1].Timestamp
		if first.YearDay() != last.YearDay() || first.Year() != last.Year() {
			timeFormat = "Jan 02 15:04:05"
		}
	}

	fmt.Printf("Lifecycle events (%d total)\n\n", result.Total)
	for _, e := range result.Entries {
		phase := ""
		if e.Phase != "" {
			phase = " [" + e.Phase + "]"
		}
		msg := e.Message
		if msg == "" {
			msg = e.Type
		}
		fmt.Printf("  %s  %-24s%s  %s\n", e.Timestamp.Format(timeFormat), e.Type, phase, msg)
	}

	return nil
}
