package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	changelogFull bool
	changelogJSON bool
)

// ChangelogCmd generates a changelog between two git refs.
var ChangelogCmd = &cobra.Command{
	Use:   "changelog <source> <target> [note]",
	Short: "Generate changelog between two refs",
	Long: `Generate a changelog from commits between two git refs (tags, branches, SHAs).

Commits are grouped by category (Added, Changed, Fixed, Removed) based on
their subject line. Use --full to include commit body text.

An optional note is rendered as a blockquote at the top of the output.

Examples:
  kvelmo changelog v1.0.0 v1.1.0
  kvelmo changelog main release/2.0
  kvelmo changelog abc1234 def5678 --full
  kvelmo changelog dev main "only frontend changes"`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runChangelog,
}

func init() {
	ChangelogCmd.Flags().BoolVar(&changelogFull, "full", false, "Include commit body/description text")
	ChangelogCmd.Flags().BoolVar(&changelogJSON, "json", false, "Output raw JSON response")
}

func runChangelog(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params := map[string]any{
		"source": args[0],
		"target": args[1],
		"full":   changelogFull,
	}
	if len(args) > 2 {
		params["note"] = args[2]
	}

	resp, err := callWorktree(ctx, "changelog.generate", params)
	if err != nil {
		return fmt.Errorf("changelog: %w", err)
	}

	if changelogJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		fmt.Println(string(resp.Result))

		return nil
	}

	if result.Markdown == "" {
		fmt.Printf("No commits between %s and %s\n", args[0], args[1])
	} else {
		fmt.Print(result.Markdown)
	}

	return nil
}
