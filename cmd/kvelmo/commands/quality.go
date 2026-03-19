package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// QualityCmd is the parent command for quality gate controls.
var QualityCmd = &cobra.Command{
	Use:   "quality",
	Short: "Quality gate controls",
	Long:  "Commands for interacting with the quality gate during task review.",
}

var qualityRespondCmd = &cobra.Command{
	Use:   "respond",
	Short: "Answer a pending quality gate prompt",
	Long: `Answer a pending quality gate prompt by providing a yes/no response.

The prompt ID is shown in 'kvelmo status' when a quality gate question is waiting.`,
	RunE: runQualityRespond,
}

func init() {
	qualityRespondCmd.Flags().String("prompt-id", "", "Prompt ID to respond to (required)")
	qualityRespondCmd.Flags().Bool("yes", false, "Answer yes")
	qualityRespondCmd.Flags().Bool("no", false, "Answer no")
	_ = qualityRespondCmd.MarkFlagRequired("prompt-id")
	QualityCmd.AddCommand(qualityRespondCmd)
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
