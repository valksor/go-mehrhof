package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var ciJSON bool

var CICmd = &cobra.Command{
	Use:   "ci",
	Short: "CI pipeline operations",
}

var ciStatusCmd = &cobra.Command{
	Use:   subStatus,
	Short: "Show CI status for the current task's PR",
	RunE:  runCIStatus,
}

func init() {
	CICmd.AddCommand(ciStatusCmd)
	ciStatusCmd.Flags().BoolVar(&ciJSON, "json", false, "Output as JSON")
}

func runCIStatus(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "ci.status", nil)
	if err != nil {
		return fmt.Errorf("ci.status: %w", err)
	}

	if ciJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var result struct {
		State  string `json:"state"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			URL    string `json:"url"`
		} `json:"checks"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if result.Message != "" {
		fmt.Println(result.Message)

		return nil
	}

	fmt.Printf("CI Status: %s\n\n", result.State)
	for _, c := range result.Checks {
		icon := "?"
		switch c.Status {
		case "success":
			icon = "+"
		case "failure":
			icon = "-"
		case "pending":
			icon = "~"
		}
		fmt.Printf("  [%s] %s\n", icon, c.Name)
	}

	return nil
}
