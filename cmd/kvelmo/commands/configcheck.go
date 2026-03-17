package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/configcheck"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var configCheckJSON bool

var configCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for config drift between global and project settings",
	Long:  "Compare project-level settings against global settings and report differences.",
	RunE:  runConfigCheck,
}

func init() {
	configCheckCmd.Flags().BoolVar(&configCheckJSON, "json", false, "Output as JSON")
	ConfigCmd.AddCommand(configCheckCmd)
}

func runConfigCheck(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "config.check", nil)
	if err != nil {
		return fmt.Errorf("config.check: %w", err)
	}

	var result struct {
		Drifts []configcheck.Drift `json:"drifts"`
		Count  int                 `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if configCheckJSON {
		out, jsonErr := json.MarshalIndent(map[string]any{
			"drifts": result.Drifts,
			"count":  result.Count,
		}, "", "  ")
		if jsonErr != nil {
			return fmt.Errorf("format: %w", jsonErr)
		}
		fmt.Println(string(out))

		return nil
	}

	if len(result.Drifts) == 0 {
		fmt.Println("No config drift detected. Project and global settings are aligned.")

		return nil
	}

	fmt.Printf("Config drift (%d difference(s)):\n\n", len(result.Drifts))
	for _, d := range result.Drifts {
		fmt.Printf("  %-40s\n", d.Path)
		fmt.Printf("    global:  %v\n", d.Expected)
		fmt.Printf("    project: %v\n", d.Actual)
	}

	return nil
}
