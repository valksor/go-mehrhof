package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// ApproveCmd grants explicit approval for a workflow transition that requires it.
var ApproveCmd = &cobra.Command{
	Use:   "approve [event]",
	Short: "Approve a workflow transition or graph node",
	Long: `Explicitly approve a workflow transition or graph node that requires human approval.

When policy.approval_required is configured (e.g. submit: true), the transition
is blocked until a human runs this command.

Use --node to approve or reject a graph node's approval gate.

Examples:
  kvelmo approve submit               # Approve the submit transition
  kvelmo approve implement            # Approve the implement transition
  kvelmo approve --node review_sql    # Approve a graph node
  kvelmo approve --node review_sql --reject  # Reject a graph node`,
	Args: cobra.MaximumNArgs(1),
	RunE: runApprove,
}

// ChecklistCmd manages review checklist items.
var ChecklistCmd = &cobra.Command{
	Use:   "checklist",
	Short: "Manage review checklist",
	Long: `View, check, and uncheck review checklist items configured in policy.

Subcommands:
  kvelmo checklist             # Show checklist status
  kvelmo checklist --check X   # Mark item X as checked
  kvelmo checklist --uncheck X # Mark item X as unchecked`,
	RunE: runChecklist,
}

func init() {
	ApproveCmd.Flags().String("node", "", "Approve a graph node (by node ID)")
	ApproveCmd.Flags().Bool("reject", false, "Reject instead of approve (with --node)")
	ChecklistCmd.Flags().String("check", "", "Mark a checklist item as checked")
	ChecklistCmd.Flags().String("uncheck", "", "Mark a checklist item as unchecked")
}

func runApprove(cmd *cobra.Command, args []string) error {
	nodeID, _ := cmd.Flags().GetString("node")
	reject, _ := cmd.Flags().GetBool("reject")

	if reject && nodeID == "" {
		return errors.New("--reject can only be used with --node")
	}

	// Node-level approval
	if nodeID != "" {
		_, err := callWorktree(context.Background(), "approve.node", map[string]any{
			"node_id": nodeID,
			"reject":  reject,
		})
		if err != nil {
			return fmt.Errorf("approve node: %w", err)
		}

		if reject {
			fmt.Printf("Rejected node: %s\n", nodeID)
		} else {
			fmt.Printf("Approved node: %s\n", nodeID)
		}

		return nil
	}

	// Workflow transition approval (requires event argument)
	if len(args) == 0 {
		return errors.New("event argument required (or use --node)")
	}
	event := args[0]

	_, err := callWorktree(context.Background(), "approve", map[string]any{
		"event": event,
	})
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}

	fmt.Printf("Approved: %s\n", event)

	return nil
}

func runChecklist(cmd *cobra.Command, _ []string) error {
	client, err := worktreeClient(defaultTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Handle --check flag
	if checkItem, _ := cmd.Flags().GetString("check"); checkItem != "" {
		_, err = client.Call(ctx, "review.checklist.check", map[string]any{
			"item": checkItem,
		})
		if err != nil {
			return fmt.Errorf("check item: %w", err)
		}

		fmt.Printf("Checked: %s\n", checkItem)

		return nil
	}

	// Handle --uncheck flag
	if uncheckItem, _ := cmd.Flags().GetString("uncheck"); uncheckItem != "" {
		_, err = client.Call(ctx, "review.checklist.uncheck", map[string]any{
			"item": uncheckItem,
		})
		if err != nil {
			return fmt.Errorf("uncheck item: %w", err)
		}

		fmt.Printf("Unchecked: %s\n", uncheckItem)

		return nil
	}

	// Default: show checklist status
	resp, err := client.Call(ctx, "review.checklist.get", nil)
	if err != nil {
		return fmt.Errorf("get checklist: %w", err)
	}

	var result struct {
		Required []string `json:"required"`
		Checked  []string `json:"checked"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if len(result.Required) == 0 {
		fmt.Println("No review checklist configured.")

		return nil
	}

	fmt.Println("Review Checklist:")
	checkedSet := make(map[string]bool)
	for _, item := range result.Checked {
		checkedSet[item] = true
	}
	for _, item := range result.Required {
		mark := "[ ]"
		if checkedSet[item] {
			mark = "[x]"
		}
		fmt.Printf("  %s %s\n", mark, item)
	}

	return nil
}
