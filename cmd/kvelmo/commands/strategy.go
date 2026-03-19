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

var strategyJSON bool

// StrategyCmd manages agent reasoning strategies.
var StrategyCmd = &cobra.Command{
	Use:   "strategy",
	Short: "Agent reasoning strategies",
}

var strategyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agent reasoning strategies",
	RunE:  runStrategyList,
}

func init() {
	strategyListCmd.Flags().BoolVar(&strategyJSON, "json", false, "Output as JSON")
	StrategyCmd.AddCommand(strategyListCmd)
}

func runStrategyList(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "strategy.list", nil)
	if err != nil {
		return fmt.Errorf("strategy.list: %w", err)
	}

	if strategyJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var names []string
	if err := json.Unmarshal(resp.Result, &names); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(names) == 0 {
		fmt.Println("No strategies registered.")

		return nil
	}

	fmt.Println("Available strategies:")
	for _, name := range names {
		fmt.Printf("  • %s\n", name)
	}

	return nil
}
