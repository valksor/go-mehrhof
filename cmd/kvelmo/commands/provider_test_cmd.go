package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

// ProviderTestCmd tests a provider connection.
var ProviderTestCmd = &cobra.Command{
	Use:   "test-provider <provider>",
	Short: "Test a provider connection",
	Long:  "Verify that a provider (github, gitlab, linear, wrike, jira) is configured and reachable.",
	Args:  cobra.ExactArgs(1),
	RunE:  runProviderTest,
}

func runProviderTest(_ *cobra.Command, args []string) error {
	provider := args[0]

	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "providers.test", map[string]any{
		"provider": provider,
	})
	if err != nil {
		return fmt.Errorf("providers.test: %w", err)
	}

	var result struct {
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.OK {
		fmt.Printf("%s: connected\n", provider)
		if result.Detail != "" {
			fmt.Printf("  %s\n", result.Detail)
		}
	} else {
		msg := result.Detail
		if msg == "" {
			msg = result.Error
		}
		fmt.Printf("%s: failed\n", provider)
		if msg != "" {
			fmt.Printf("  %s\n", msg)
		}
	}

	return nil
}
