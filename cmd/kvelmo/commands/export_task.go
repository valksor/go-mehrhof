package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var exportTaskFormat string

var exportTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Export full context for the current task",
	Long: `Export everything about the active task: metadata, spec, plan, chat history,
file changes, and checkpoints. Useful for handoffs, debugging, or "what did I do?"

Examples:
  kvelmo export task                  # JSON to stdout
  kvelmo export task --format md      # Markdown summary
  kvelmo export task > task-dump.json # Save to file`,
	RunE: runExportTask,
}

func init() {
	exportTaskCmd.Flags().StringVar(&exportTaskFormat, "format", "json", "Output format (json, md)")
	ExportCmd.AddCommand(exportTaskCmd)
}

func runExportTask(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return fmt.Errorf("no worktree socket running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "task.export", map[string]any{
		"format": exportTaskFormat,
	})
	if err != nil {
		return fmt.Errorf("task.export: %w", err)
	}

	if exportTaskFormat == "md" {
		var result struct {
			Markdown string `json:"markdown"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return fmt.Errorf("parse result: %w", err)
		}
		fmt.Println(result.Markdown)

		return nil
	}

	out, err := json.MarshalIndent(resp.Result, "", "  ")
	if err != nil {
		return fmt.Errorf("format result: %w", err)
	}
	fmt.Println(string(out))

	return nil
}
