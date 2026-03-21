package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/security"
)

// SecurityCmd is the root command for security subcommands.
var SecurityCmd = &cobra.Command{
	Use:   "security",
	Short: "Security scanning",
	Long:  "Scan project directories for hardcoded secrets, vulnerable dependencies, and other security issues.",
}

var securityScanCmd = &cobra.Command{
	Use:   "scan [dir]",
	Short: "Scan a directory for security issues",
	Long:  "Run all security scanners against the specified directory (defaults to current working directory).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSecurityScan,
}

var securityScanJSON bool

func init() {
	SecurityCmd.AddCommand(securityScanCmd)

	securityScanCmd.Flags().BoolVar(&securityScanJSON, "json", false, "Output raw JSON response")
}

func runSecurityScan(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if len(args) > 0 {
		dir = args[0]
	}

	client, err := globalClient(60 * time.Second)
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "security.scan", map[string]any{
		"dir": dir,
	})
	if err != nil {
		return fmt.Errorf("security.scan: %w", err)
	}

	if securityScanJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Findings []security.Finding `json:"findings"`
		Count    int                `json:"count"`
		Scanners []string           `json:"scanners"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Security scan complete (%s)\n", formatScanners(result.Scanners))

	if result.Count == 0 {
		fmt.Println("No issues found.")

		return nil
	}

	fmt.Printf("Found %d issue(s):\n\n", result.Count)

	// Group findings by severity for display.
	severityOrder := []security.Severity{
		security.SeverityCritical,
		security.SeverityHigh,
		security.SeverityMedium,
		security.SeverityLow,
		security.SeverityInfo,
	}

	grouped := make(map[security.Severity][]security.Finding)
	for _, f := range result.Findings {
		grouped[f.Severity] = append(grouped[f.Severity], f)
	}

	for _, sev := range severityOrder {
		findings := grouped[sev]
		if len(findings) == 0 {
			continue
		}

		label := severityLabel(sev)
		fmt.Printf("  %s (%d)\n", label, len(findings))
		for _, f := range findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			fmt.Printf("    %s  %s\n", loc, f.Message)
			if f.Suggestion != "" {
				fmt.Printf("      -> %s\n", f.Suggestion)
			}
		}
		fmt.Println()
	}

	return nil
}

func severityLabel(s security.Severity) string {
	switch s {
	case security.SeverityCritical:
		return "CRITICAL"
	case security.SeverityHigh:
		return "HIGH"
	case security.SeverityMedium:
		return "MEDIUM"
	case security.SeverityLow:
		return "LOW"
	case security.SeverityInfo:
		return "INFO"
	default:
		return string(s)
	}
}

func formatScanners(scanners []string) string {
	if len(scanners) == 0 {
		return "no scanners"
	}
	if len(scanners) == 1 {
		return scanners[0]
	}

	result := scanners[0]
	var resultSb171 strings.Builder
	for _, s := range scanners[1:] {
		resultSb171.WriteString(", " + s)
	}
	result += resultSb171.String()

	return result
}
