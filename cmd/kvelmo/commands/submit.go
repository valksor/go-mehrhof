package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/socket"
)

var SubmitCmd = &cobra.Command{
	Use:     "submit",
	Aliases: []string{"sub"},
	Short:   "Submit the current task (push changes, create PR)",
	Long: `Pushes the current branch and creates a pull request (or equivalent)
on the provider. Requires the task to be in 'reviewed' state.`,
	RunE: runSubmit,
}

func init() {
	SubmitCmd.Flags().StringP("title", "t", "", "PR/MR title (defaults to task title)")
	SubmitCmd.Flags().StringP("body", "b", "", "PR/MR body (defaults to task description)")
	SubmitCmd.Flags().Bool("draft", false, "Create as draft PR")
	SubmitCmd.Flags().StringSlice("reviewers", nil, "Assign reviewers")
	SubmitCmd.Flags().StringSlice("labels", nil, "Add labels")
	SubmitCmd.Flags().Bool("delete-branch", false, "Delete local branch after successful submission")
	SubmitCmd.Flags().Bool("skip-review", false, "Skip review gate and submit directly")
	SubmitCmd.Flags().Bool("dry-run", false, "Preview the PR without creating it")
	SubmitCmd.Flags().StringSlice("section", nil, "Add custom PR section (format: \"Header=Content\")")
	SubmitCmd.Flags().Bool("json", false, "Output result as JSON")
}

func runSubmit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	socketPath := socket.WorktreeSocketPath(cwd)

	client, err := socket.NewClient(socketPath, socket.WithTimeout(120*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	title, _ := cmd.Flags().GetString("title")
	body, _ := cmd.Flags().GetString("body")
	draft, _ := cmd.Flags().GetBool("draft")
	reviewers, _ := cmd.Flags().GetStringSlice("reviewers")
	labels, _ := cmd.Flags().GetStringSlice("labels")
	deleteBranch, _ := cmd.Flags().GetBool("delete-branch")
	skipReview, _ := cmd.Flags().GetBool("skip-review")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	sectionFlags, _ := cmd.Flags().GetStringSlice("section")

	// Parse --section flags into map
	sections := make(map[string]string)
	for _, s := range sectionFlags {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) == 2 {
			sections[parts[0]] = parts[1]
		} else {
			fmt.Fprintf(os.Stderr, "Warning: ignoring malformed --section %q (expected format: Header=Content)\n", s)
		}
	}

	params := map[string]any{
		"title":         title,
		"body":          body,
		"draft":         draft,
		"reviewers":     reviewers,
		"labels":        labels,
		"delete_branch": deleteBranch,
		"skip_review":   skipReview,
		"dry_run":       dryRun,
	}
	if len(sections) > 0 {
		params["sections"] = sections
	}

	// Use 2 minute timeout for submit since it involves git push + PR creation
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "submit", params)
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		fmt.Println(string(resp.Result))

		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		fmt.Printf("Submitted: %s\n", string(resp.Result))

		return nil
	}

	// Handle dry-run preview output
	if dryRun {
		if preview, ok := result["preview"].(map[string]any); ok {
			fmt.Println("=== PR Preview (dry-run) ===")
			fmt.Println()
			if t, ok := preview["title"].(string); ok {
				fmt.Printf("Title: %s\n", t)
			}
			if branch, ok := preview["branch"].(string); ok {
				fmt.Printf("Branch: %s\n", branch)
			}
			if base, ok := preview["base_branch"].(string); ok && base != "" {
				fmt.Printf("Base: %s\n", base)
			}
			fmt.Println()
			if b, ok := preview["body"].(string); ok {
				fmt.Println(b)
			}
		} else {
			fmt.Println("Dry-run complete (no preview available)")
		}

		return nil
	}

	if url, ok := result["url"].(string); ok && url != "" {
		prTitle, _ := result["title"].(string)
		fmt.Printf("PR created: %s\n", url)
		if prTitle != "" {
			fmt.Printf("Title: %s\n", prTitle)
		}
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Wait for CI checks to pass")
		fmt.Println("  2. Merge the PR when ready")
		fmt.Println("  3. Run 'kvelmo finish' to clean up the worktree")

		return nil
	}

	// Fallback: print raw response
	fmt.Printf("Submitted: %s\n", string(resp.Result))

	return nil
}
