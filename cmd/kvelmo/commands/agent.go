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

var agentStatusJSON bool

// AgentCmd manages agent operations.
var AgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management",
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check agent availability and health",
	RunE:  runAgentStatus,
}

func init() {
	agentStatusCmd.Flags().BoolVar(&agentStatusJSON, "json", false, "Output as JSON")
	AgentCmd.AddCommand(agentStatusCmd)
}

func runAgentStatus(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "agent.status", nil)
	if err != nil {
		return fmt.Errorf("agent.status: %w", err)
	}

	if agentStatusJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var result struct {
		AgentAvailable bool `json:"agent_available"`
		SimulationMode bool `json:"simulation_mode"`
		Checks         []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.AgentAvailable {
		fmt.Println("Agent: available")
	} else if result.SimulationMode {
		fmt.Println("Agent: simulation mode (no real agent connected)")
	} else {
		fmt.Println("Agent: not available")
	}

	for _, c := range result.Checks {
		status := "OK"
		if c.Status != "ok" && c.Status != "pass" && c.Status != "passed" {
			status = c.Status
		}
		fmt.Printf("  %-20s %s", c.Name, status)
		if c.Detail != "" {
			fmt.Printf("  (%s)", c.Detail)
		}
		fmt.Println()
	}

	return nil
}
