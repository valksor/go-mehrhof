package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

// QueueCmd is the root command for task queue management.
var QueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage task queue",
	Long:  "Add, remove, list, and reorder tasks in the worktree task queue.",
}

var queueAddCmd = &cobra.Command{
	Use:   "add <source>",
	Short: "Add a task to the queue",
	Long:  "Add a new task to the queue by specifying its source (e.g. GitHub issue URL, file path).",
	Args:  cobra.ExactArgs(1),
	RunE:  runQueueAdd,
}

var queueRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a task from the queue",
	Long:  "Remove a queued task by its ID.",
	Args:  cobra.ExactArgs(1),
	RunE:  runQueueRemove,
}

var queueListCmd = &cobra.Command{
	Use:   subList,
	Short: "List queued tasks",
	Long:  "List all tasks currently in the queue.",
	RunE:  runQueueList,
}

var queueReorderCmd = &cobra.Command{
	Use:   "reorder <id> <position>",
	Short: "Reorder a queued task",
	Long:  "Move a queued task to a new position in the queue.",
	Args:  cobra.ExactArgs(2),
	RunE:  runQueueReorder,
}

var (
	queueAddTitle string
	queueListJSON bool
)

func init() {
	queueAddCmd.Flags().StringVar(&queueAddTitle, "title", "", "Optional title for the queued task")
	queueListCmd.Flags().BoolVar(&queueListJSON, "json", false, "Output raw JSON response")

	QueueCmd.AddCommand(queueAddCmd)
	QueueCmd.AddCommand(queueRemoveCmd)
	QueueCmd.AddCommand(queueListCmd)
	QueueCmd.AddCommand(queueReorderCmd)
}

func runQueueAdd(cmd *cobra.Command, args []string) error {
	source := args[0]

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := map[string]any{
		paramSource: source,
	}
	if queueAddTitle != "" {
		params["title"] = queueAddTitle
	}

	resp, err := client.Call(ctx, "queue.add", params)
	if err != nil {
		return fmt.Errorf("queue.add call: %w", err)
	}

	var result struct {
		ID     string `json:"id"`
		Source string `json:"source"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Added to queue: %s\n", result.ID)
	if result.Title != "" {
		fmt.Printf("  Title:  %s\n", result.Title)
	}
	fmt.Printf("  Source: %s\n", result.Source)

	return nil
}

func runQueueRemove(cmd *cobra.Command, args []string) error {
	id := args[0]

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "queue.remove", map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("queue.remove call: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Success {
		fmt.Printf("Removed %s from queue\n", id)
	} else {
		fmt.Printf("Failed to remove %s from queue\n", id)
	}

	return nil
}

func runQueueList(cmd *cobra.Command, args []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "queue.list", nil)
	if err != nil {
		return fmt.Errorf("queue.list call: %w", err)
	}

	if queueListJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Queue []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Title  string `json:"title"`
		} `json:"queue"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("No tasks in queue")

		return nil
	}

	fmt.Printf("Task queue (%d):\n\n", result.Count)
	for i, task := range result.Queue {
		fmt.Printf("  %d. %s\n", i+1, task.ID)
		if task.Title != "" {
			fmt.Printf("     Title:  %s\n", task.Title)
		}
		fmt.Printf("     Source: %s\n", task.Source)
		fmt.Println()
	}

	return nil
}

func runQueueReorder(cmd *cobra.Command, args []string) error {
	id := args[0]
	position, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid position %q: must be an integer", args[1])
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "queue.reorder", map[string]any{
		"id":       id,
		"position": position,
	})
	if err != nil {
		return fmt.Errorf("queue.reorder call: %w", err)
	}

	var result struct {
		Queue []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Title  string `json:"title"`
		} `json:"queue"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Moved %s to position %d\n", id, position)
	fmt.Printf("Queue now has %d task(s)\n", result.Count)

	return nil
}
