package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var policyJSON bool

var PolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Workflow policy management",
}

var policyCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check current task against workflow policies",
	Long:  "Evaluate the current task state against configured workflow policies and report any violations.",
	RunE:  runPolicyCheck,
}

func init() {
	PolicyCmd.AddCommand(policyCheckCmd)
	policyCheckCmd.Flags().BoolVar(&policyJSON, "json", false, "Output as JSON")
}

func runPolicyCheck(_ *cobra.Command, _ []string) error {
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

	resp, err := client.Call(ctx, "policy.check", nil)
	if err != nil {
		return fmt.Errorf("policy.check: %w", err)
	}

	if policyJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var result struct {
		Violations []struct {
			Severity string `json:"severity"`
			Rule     string `json:"rule"`
			Message  string `json:"message"`
		} `json:"violations"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Violations) == 0 {
		fmt.Println("No policy violations.")

		return nil
	}

	fmt.Printf("Policy violations (%d):\n\n", len(result.Violations))
	for _, v := range result.Violations {
		icon := "⚠"
		if v.Severity == "error" {
			icon = "✗"
		}
		fmt.Printf("  %s [%s] %s: %s\n", icon, v.Severity, v.Rule, v.Message)
	}

	return nil
}
