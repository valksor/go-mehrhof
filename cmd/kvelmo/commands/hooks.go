package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var hooksJSON bool

// HooksCmd lists configured workflow transition hooks.
var HooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "List configured workflow hooks",
	Long:  "Display all configured pre-transition hooks that run before workflow state changes.",
	RunE:  runHooks,
}

func init() {
	HooksCmd.Flags().BoolVar(&hooksJSON, "json", false, "Output as JSON")
}

func runHooks(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "hooks.list", nil)
	if err != nil {
		return fmt.Errorf("hooks.list: %w", err)
	}

	if hooksJSON {
		out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(string(out))
		}

		return nil
	}

	var hooks map[string][]struct {
		Command     string `json:"command"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	}
	if err := json.Unmarshal(resp.Result, &hooks); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if len(hooks) == 0 {
		fmt.Println("No workflow hooks configured.")

		return nil
	}

	for event, eventHooks := range hooks {
		fmt.Printf("\n%s:\n", event)
		for _, h := range eventHooks {
			req := ""
			if h.Required {
				req = " [required]"
			}
			desc := h.Description
			if desc == "" {
				desc = h.Command
			}
			fmt.Printf("  %s%s\n", desc, req)
		}
	}

	return nil
}
