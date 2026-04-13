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

var NotifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Notification management",
	Long:  "Send test notifications or manage webhook configuration.",
}

var notifyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test notification to all configured webhooks",
	RunE:  runNotifyTest,
}

func init() {
	NotifyCmd.AddCommand(notifyTestCmd)
}

func runNotifyTest(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "notify.test", nil)
	if err != nil {
		return fmt.Errorf("notify.test: %w", err)
	}

	var result struct {
		Sent    int    `json:"sent"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Test notification sent to %d endpoint(s)\n", result.Sent)
	if result.Message != "" {
		fmt.Println(result.Message)
	}

	return nil
}
