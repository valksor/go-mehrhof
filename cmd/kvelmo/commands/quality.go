package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var qualityJSON bool

// QualityCmd is the parent command for quality gate controls.
var QualityCmd = &cobra.Command{
	Use:   "quality",
	Short: "Quality gate controls",
	Long:  "Commands for interacting with the quality gate during task review.",
	RunE:  runQuality,
}

var qualityRespondCmd = &cobra.Command{
	Use:   "respond",
	Short: "Answer a pending quality gate prompt",
	Long: `Answer a pending quality gate prompt by providing a yes/no response.

The prompt ID is shown in 'kvelmo status' when a quality gate question is waiting.`,
	RunE: runQualityRespond,
}

var qualityAutofixStatusCmd = &cobra.Command{
	Use:   "autofix-status",
	Short: "Show the current auto-fix loop status",
	Long:  "Display the status of the quality gate auto-fix loop including attempt count and result.",
	RunE:  runQualityAutofixStatus,
}

var qualityFailclassCmd = &cobra.Command{
	Use:   "failclass",
	Short: "Show failure classification statistics",
	Long:  "Display statistics about failure pattern classification (flaky vs genuine) for quality gate findings.",
	RunE:  runQualityFailclass,
}

func init() {
	QualityCmd.Flags().BoolVar(&qualityJSON, "json", false, "Output as JSON")

	qualityRespondCmd.Flags().String("prompt-id", "", "Prompt ID to respond to (required)")
	qualityRespondCmd.Flags().Bool("yes", false, "Answer yes")
	qualityRespondCmd.Flags().Bool("no", false, "Answer no")
	_ = qualityRespondCmd.MarkFlagRequired("prompt-id")
	QualityCmd.AddCommand(qualityRespondCmd)
	QualityCmd.AddCommand(qualityAutofixStatusCmd)
	QualityCmd.AddCommand(qualityFailclassCmd)
}

func runQuality(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "autofix.status", nil)
	if err != nil {
		return fmt.Errorf("autofix.status: %w", err)
	}

	if qualityJSON {
		fmt.Println(string(resp.Result))

		return nil
	}

	return outputJSON(resp.Result)
}

func runQualityRespond(cmd *cobra.Command, args []string) error {
	promptID, _ := cmd.Flags().GetString("prompt-id")
	yes, _ := cmd.Flags().GetBool("yes")
	no, _ := cmd.Flags().GetBool("no")

	if !yes && !no {
		return errors.New("must specify --yes or --no")
	}
	if yes && no {
		return errors.New("cannot specify both --yes and --no")
	}

	_, err := callWorktree(context.Background(), "quality.respond", map[string]any{
		"prompt_id": promptID,
		"answer":    yes,
	})
	if err != nil {
		return fmt.Errorf("quality respond: %w", err)
	}

	if yes {
		fmt.Println("Answered: yes")
	} else {
		fmt.Println("Answered: no")
	}

	return nil
}

func runQualityAutofixStatus(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "autofix.status", nil)
	if err != nil {
		return fmt.Errorf("autofix.status: %w", err)
	}

	return outputJSON(resp.Result)
}

func runQualityFailclass(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "failclass.stats", nil)
	if err != nil {
		return fmt.Errorf("failclass.stats: %w", err)
	}

	return outputJSON(resp.Result)
}
