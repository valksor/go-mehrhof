package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/socket"
)

var RecapCmd = &cobra.Command{
	Use:   "recap",
	Short: "Summarize current task state for resuming work",
	Long: `Show a concise recap of the current task: where you are, what changed,
and what to do next. Useful when returning to a task after a break.

Use --json for machine-readable output.`,
	RunE: runRecap,
}

func init() {
	RecapCmd.Flags().Bool("json", false, "Output raw JSON response")
}

func runRecap(cmd *cobra.Command, _ []string) error {
	resp, err := callWorktree(context.Background(), "recap", nil)
	if err != nil {
		return fmt.Errorf("recap: %w", err)
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return outputJSON(resp.Result)
	}

	var result socket.RecapResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse recap: %w", err)
	}

	printRecap(result)

	return nil
}

func printRecap(r socket.RecapResult) {
	// State
	fmt.Printf("State: %s\n", capitalize(string(r.State)))

	// Task info
	if r.Task != nil {
		fmt.Printf("Task:  %s\n", r.Task.Title)
		if r.Task.Source != "" {
			fmt.Printf("Source: %s\n", r.Task.Source)
		}
		if r.Task.Branch != "" {
			fmt.Printf("Branch: %s\n", r.Task.Branch)
		}
	}

	// Tags
	if len(r.Tags) > 0 {
		fmt.Printf("Tags:  %s\n", strings.Join(r.Tags, ", "))
	}

	// Last activity
	if r.LastActivity != "" {
		fmt.Printf("Last activity: %s\n", r.LastActivity)
	}

	// Checkpoints
	if r.LastCheckpoint != nil {
		sha := r.LastCheckpoint.SHA
		if len(sha) > 8 {
			sha = sha[:8]
		}
		fmt.Printf("Checkpoints: %d (latest: %s", r.CheckpointCount, sha)
		if r.LastCheckpoint.Message != "" {
			fmt.Printf(" — %s", r.LastCheckpoint.Message)
		}
		fmt.Println(")")
	}

	// Files changed
	if len(r.FilesChanged) > 0 {
		fmt.Printf("Files changed: %d\n", len(r.FilesChanged))
		limit := min(len(r.FilesChanged), 10)
		for _, f := range r.FilesChanged[:limit] {
			fmt.Printf("  %s %s\n", fileStatusLabel(f.Status), f.Path)
		}
		if len(r.FilesChanged) > 10 {
			fmt.Printf("  ... and %d more\n", len(r.FilesChanged)-10)
		}
	}

	// Phase metrics summary
	if len(r.PhaseMetrics) > 0 {
		fmt.Print("Phases completed:")
		phases := []string{"plan", "implement", "simplify", "optimize", "review"}
		var completed []string
		for _, p := range phases {
			if _, ok := r.PhaseMetrics[p]; ok {
				completed = append(completed, p)
			}
		}
		if len(completed) > 0 {
			fmt.Printf(" %s\n", strings.Join(completed, ", "))
		} else {
			fmt.Println(" none")
		}
	}

	// Error
	if r.LastError != "" {
		fmt.Printf("Error: %s\n", r.LastError)
	}

	// Next action
	fmt.Printf("\nNext: %s\n", r.NextAction)
}

func fileStatusLabel(status string) string {
	switch status {
	case "added":
		return "A"
	case "modified":
		return "M"
	case "deleted":
		return "D"
	case "renamed":
		return "R"
	default:
		return status
	}
}
