package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var (
	reportFormat string
	reportSince  string
)

// ReportCmd generates structured compliance reports.
var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate compliance report",
	Long:  "Generate a structured compliance report with task summaries, AI usage, and activity metrics.",
	RunE:  runReport,
}

func init() {
	ReportCmd.Flags().StringVar(&reportFormat, "format", "md", "Output format (md, json)")
	ReportCmd.Flags().StringVar(&reportSince, "since", "30d", "Time range (e.g., 7d, 30d, 90d)")
}

func runReport(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "report.generate", map[string]any{
		"format": reportFormat,
		"since":  reportSince,
	})
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}

	var result struct {
		Markdown string          `json:"markdown"`
		Report   json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		fmt.Println(string(resp.Result))

		return nil
	}

	if reportFormat == "json" {
		out, err := json.MarshalIndent(result.Report, "", "  ")
		if err != nil {
			fmt.Println(string(result.Report))
		} else {
			fmt.Println(string(out))
		}
	} else {
		fmt.Print(result.Markdown)
	}

	return nil
}
