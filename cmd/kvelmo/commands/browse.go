package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var browseFiles bool

var BrowseCmd = &cobra.Command{
	Use:   "browse [path]",
	Short: "Browse project directory structure",
	Long:  `Browse the directory structure of the current project. Uses the worktree socket if available, otherwise falls back to the global socket.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBrowse,
}

func init() {
	BrowseCmd.Flags().BoolVar(&browseFiles, "files", false, "Include files (not just directories)")
}

func runBrowse(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	params := map[string]any{
		"files": browseFiles,
	}
	if len(args) > 0 {
		params["path"] = args[0]
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if socket.SocketExists(wtPath) {
		return browseViaSocket(wtPath, params)
	}

	gPath := socket.GlobalSocketPath()
	if !socket.SocketExists(gPath) {
		return errors.New(meta.Name + " server not running\nRun '" + meta.Name + " serve' first")
	}

	return browseViaSocket(gPath, params)
}

func browseViaSocket(socketPath string, params map[string]any) error {
	client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "browse", params)
	if err != nil {
		return fmt.Errorf("browse call: %w", err)
	}

	var result struct {
		Entries []string `json:"entries"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Entries) == 0 {
		fmt.Println("No entries found")

		return nil
	}

	for _, entry := range result.Entries {
		fmt.Println(entry)
	}

	return nil
}
