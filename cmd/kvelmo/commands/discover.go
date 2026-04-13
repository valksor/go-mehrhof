package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var discoveryJSON bool

var DiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "List available project commands",
	Long: `Scan the project for available commands from Makefile targets,
package.json scripts, Taskfile tasks, and executable binaries in bin/.`,
	RunE: runDiscover,
}

func init() {
	DiscoverCmd.Flags().BoolVar(&discoveryJSON, "json", false, "Output as JSON")
}

func runDiscover(_ *cobra.Command, _ []string) error {
	resp, err := callWorktree(context.Background(), "discovery.scan", nil)
	if err != nil {
		return fmt.Errorf("discovery.scan: %w", err)
	}

	if discoveryJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Commands []string `json:"commands"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("No project commands discovered.")

		return nil
	}

	fmt.Printf("Discovered commands (%d)\n\n", result.Count)
	for _, cmd := range result.Commands {
		fmt.Printf("  %s\n", cmd)
	}

	return nil
}
