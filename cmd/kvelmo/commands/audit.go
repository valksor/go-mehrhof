package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	auditSince  string
	auditMethod string
	auditJSON   bool
)

var AuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View compliance audit trail",
	Long:  "Show a compliance-focused view of all kvelmo actions with user identity and timestamps.",
	RunE:  runAudit,
}

func init() {
	AuditCmd.Flags().StringVar(&auditSince, "since", "7d", "Time range (e.g., 24h, 7d, 30d)")
	AuditCmd.Flags().StringVar(&auditMethod, "method", "", "Filter by method pattern (pipe-separated)")
	AuditCmd.Flags().BoolVar(&auditJSON, "json", false, "Output as JSON")
}

func runAudit(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callGlobal(ctx, "export", map[string]any{
		"format":  "json",
		"since":   auditSince,
		"include": "tasks,metrics,activity",
	})
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	if auditJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var result struct {
		Tasks []struct {
			ID    string `json:"id"`
			Path  string `json:"path"`
			State string `json:"state"`
		} `json:"tasks"`
		Activity []struct {
			Timestamp  string `json:"timestamp"`
			Method     string `json:"method"`
			DurationMs int64  `json:"duration_ms"`
			Error      string `json:"error"`
			UserID     string `json:"user_id"`
			TaskID     string `json:"task_id"`
			AgentModel string `json:"agent_model"`
		} `json:"activity"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Compliance Audit Report (since %s)\n", auditSince)
	fmt.Println(strings.Repeat("=", 59))

	if len(result.Tasks) > 0 {
		fmt.Printf("\nActive Tasks (%d)\n", len(result.Tasks))
		for _, t := range result.Tasks {
			fmt.Printf("  %-40s  %s\n", t.ID, t.State)
		}
	}

	if len(result.Activity) > 0 {
		fmt.Printf("\nActivity Log (%d entries)\n", len(result.Activity))
		for _, a := range result.Activity {
			t, _ := time.Parse(time.RFC3339Nano, a.Timestamp)
			user := a.UserID
			if user == "" {
				user = "-"
			}
			fmt.Printf("  %s  %-12s  %-30s  %4dms", t.Format("2006-01-02 15:04:05"), user, a.Method, a.DurationMs)
			if a.Error != "" {
				fmt.Printf("  ERR")
			}
			fmt.Println()
		}
	}

	return nil
}
