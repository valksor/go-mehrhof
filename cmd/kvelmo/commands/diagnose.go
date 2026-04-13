package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
	"github.com/valksor/kvelmo/settings"
)

var (
	diagnoseJSON   bool
	diagnoseHealth bool
)

var DiagnoseCmd = &cobra.Command{
	Use:     "diagnose",
	Aliases: []string{"diag"},
	Short:   "Check system setup and configuration",
	Long: `Diagnose checks that kvelmo is properly configured.

It verifies:
  - Git is installed
  - AI agent CLIs are available (claude, codex)
  - Global socket is running
  - Provider tokens are configured

Run this command to troubleshoot setup issues.`,
	RunE: runDiagnose,
}

func init() {
	DiagnoseCmd.Flags().BoolVar(&diagnoseJSON, "json", false, "Output raw JSON response")
	DiagnoseCmd.Flags().BoolVar(&diagnoseHealth, "health", false, "Show worktree socket health status")
}

type diagnoseCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

type diagnoseProvider struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
}

type diagnoseResult struct {
	Checks       []diagnoseCheck    `json:"checks"`
	GlobalSocket string             `json:"global_socket"`
	Providers    []diagnoseProvider `json:"providers"`
	Issues       []string           `json:"issues,omitempty"`
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	if diagnoseHealth {
		return runDiagnoseHealth()
	}

	// Try server-side diagnose first (consistent with web UI).
	if handled, err := runDiagnoseViaRPC(); handled {
		return err
	}

	// Fall back to offline diagnostics when server is not available.
	var issues []string

	// Run preflight checks for git and agent CLIs
	preflight := agent.RunPreflight()

	var jsonChecks []diagnoseCheck

	for _, c := range preflight.Checks {
		jc := diagnoseCheck{
			Name:   c.Name,
			Status: string(c.Status),
			Detail: c.Detail,
			Fix:    c.Fix,
		}
		jsonChecks = append(jsonChecks, jc)
		if c.Fix != "" {
			issues = append(issues, c.Fix)
		}
	}

	// Check global socket
	globalPath := socket.GlobalSocketPath()
	socketStatus := "not_running"
	if socket.SocketExists(globalPath) {
		client, err := socket.NewClient(globalPath, socket.WithTimeout(500*time.Millisecond))
		if err == nil {
			_ = client.Close()
			socketStatus = "running"
		} else {
			socketStatus = "stale"
			issues = append(issues, "Remove stale socket: rm "+globalPath)
		}
	} else {
		issues = append(issues, fmt.Sprintf("Start server: %s serve", meta.Name))
	}

	// Check provider tokens
	providerChecks := []struct {
		name   string
		envVar string
	}{
		{"GitHub", "GITHUB_TOKEN"},
		{"GitLab", "GITLAB_TOKEN"},
		{"Linear", "LINEAR_TOKEN"},
		{"Wrike", "WRIKE_TOKEN"},
	}

	var jsonProviders []diagnoseProvider
	for _, p := range providerChecks {
		configured := detectExistingToken(p.envVar, settings.ScopeGlobal, "") != nil
		jsonProviders = append(jsonProviders, diagnoseProvider{
			Name:       p.name,
			Configured: configured,
		})
	}

	if diagnoseJSON {
		result := diagnoseResult{
			Checks:       jsonChecks,
			GlobalSocket: socketStatus,
			Providers:    jsonProviders,
			Issues:       issues,
		}
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(out))

		return nil
	}

	// Formatted output
	fmt.Println()
	fmt.Printf("  %s Diagnostics\n", meta.Name)
	fmt.Println("  ─────────────────────────────────────")
	fmt.Println()

	for _, c := range preflight.Checks {
		symbol := "✓"
		label := "installed"

		switch c.Status {
		case agent.CheckPassed:
			// default values are fine
		case agent.CheckFailed:
			symbol = "✗"
			label = "not found"
		case agent.CheckWarning:
			symbol = "⚠"
			label = "not found"
		}
		detail := ""
		if c.Status == agent.CheckPassed && c.Detail != "" {
			detail = " (" + c.Detail + ")"
		}
		displayName := c.Name
		switch c.Name {
		case "claude":
			displayName = "Claude CLI"
		case "codex":
			displayName = "Codex CLI"
		case "git":
			displayName = "Git"
		}
		fmt.Printf("  %-14s %s %s%s\n", displayName+":", symbol, label, detail)
	}

	switch socketStatus {
	case "running":
		fmt.Printf("  Global socket: ✓ running\n")
	case "stale":
		fmt.Printf("  Global socket: ⚠ stale (not responding)\n")
	default:
		fmt.Printf("  Global socket: ✗ not running\n")
	}

	fmt.Println()
	fmt.Println("  Providers:")

	for _, p := range providerChecks {
		if token := detectExistingToken(p.envVar, settings.ScopeGlobal, ""); token != nil {
			masked := "****"
			if len(token.Value) > 4 {
				masked = "****" + token.Value[len(token.Value)-4:]
			}
			fmt.Printf("    %-8s ✓ configured (%s)\n", p.name+":", masked)
		} else {
			fmt.Printf("    %-8s ✗ not configured\n", p.name+":")
		}
	}

	fmt.Println()

	if len(issues) > 0 {
		fmt.Println("  Next steps:")
		for _, issue := range issues {
			fmt.Printf("    • %s\n", issue)
		}
		fmt.Println()
	} else {
		fmt.Println("  ✓ All checks passed!")
		fmt.Println()
	}

	return nil
}

// runDiagnoseViaRPC attempts to call system.diagnose on the global socket.
// Returns (true, nil) on success, (true, err) on display error, (false, nil) if unavailable.
func runDiagnoseViaRPC() (bool, error) {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return false, nil
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(2*time.Second))
	if err != nil {
		return false, nil
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "system.diagnose", nil)
	if err != nil {
		return false, nil
	}

	if diagnoseJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return true, nil
	}

	// Parse and display the server-side diagnose result using the same format.
	var result diagnoseResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false, nil
	}

	fmt.Println()
	fmt.Printf("  %s Diagnostics (via server)\n", meta.Name)
	fmt.Println("  ─────────────────────────────────────")
	fmt.Println()

	for _, c := range result.Checks {
		symbol := "✓"
		label := "installed"

		switch c.Status {
		case "fail", "failed":
			symbol = "✗"
			label = "not found"
		case "warning", "warn":
			symbol = "⚠"
			label = "not found"
		}

		detail := ""
		if (c.Status == "ok" || c.Status == "pass") && c.Detail != "" {
			detail = " (" + c.Detail + ")"
		}

		displayName := c.Name
		switch c.Name {
		case "claude":
			displayName = "Claude CLI"
		case "codex":
			displayName = "Codex CLI"
		case "git":
			displayName = "Git"
		}

		fmt.Printf("  %-14s %s %s%s\n", displayName+":", symbol, label, detail)
	}

	fmt.Printf("  Global socket: ✓ %s\n", result.GlobalSocket)
	fmt.Println()
	fmt.Println("  Providers:")

	for _, p := range result.Providers {
		if p.Configured {
			fmt.Printf("    %-8s ✓ configured\n", p.Name+":")
		} else {
			fmt.Printf("    %-8s ✗ not configured\n", p.Name+":")
		}
	}

	fmt.Println()

	if len(result.Issues) > 0 {
		fmt.Println("  Next steps:")
		for _, issue := range result.Issues {
			fmt.Printf("    • %s\n", issue)
		}
		fmt.Println()
	} else {
		fmt.Println("  ✓ All checks passed!")
		fmt.Println()
	}

	return true, nil
}

func runDiagnoseHealth() error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "system.health", nil)
	if err != nil {
		return fmt.Errorf("system.health: %w", err)
	}

	if diagnoseJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var result struct {
		Worktrees []struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			State    string `json:"state"`
			Healthy  *bool  `json:"healthy"`
			LastPing string `json:"last_ping"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Worktrees) == 0 {
		fmt.Println("No worktrees registered.")

		return nil
	}

	fmt.Printf("Worktree health (%d registered):\n", len(result.Worktrees))
	for _, wt := range result.Worktrees {
		status := "unknown"
		if wt.Healthy != nil {
			if *wt.Healthy {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
		}
		fmt.Printf("  %-40s  %-12s  %s\n", wt.Path, wt.State, status)
	}

	return nil
}
