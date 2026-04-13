package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/internal/socket"
)

var PromptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Output task status for shell prompt integration",
	Long: `Output a short status string suitable for shell prompt (PS1) integration.

Outputs nothing if no task is active. Output format: [kvelmo:STATE]

Example PS1 setup (bash):
  PS1='$(kvelmo prompt 2>/dev/null)\$ '

Example for fish:
  function fish_prompt
    set -l kv (kvelmo prompt 2>/dev/null)
    echo -n "$kv \$ "
  end`,
	SilenceUsage: true,
	RunE:         runPrompt,
}

func runPrompt(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return nil // Silent failure for prompt integration
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return nil // No socket, no output
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(2*time.Second))
	if err != nil {
		return nil // Silent failure
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "status", nil)
	if err != nil {
		return nil // Silent failure
	}

	var result struct {
		State string `json:"state"`
	}
	if json.Unmarshal(resp.Result, &result) != nil {
		return nil
	}

	if result.State == "" || result.State == "none" {
		return nil
	}

	fmt.Printf("[kvelmo:%s]", result.State)

	return nil
}
