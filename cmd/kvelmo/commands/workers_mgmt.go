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

var workersAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new worker to the pool",
	Long:  `Add a new worker with the specified agent.`,
	RunE:  runWorkersAdd,
}

var workersRemoveCmd = &cobra.Command{
	Use:   "remove [id]",
	Short: "Remove a worker from the pool",
	Long:  `Remove a worker by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkersRemove,
}

var workersStatsCmd = &cobra.Command{
	Use:   subStats,
	Short: "Show worker pool statistics",
	Long:  `Show detailed statistics about the worker pool.`,
	RunE:  runWorkersStats,
}

func init() {
	WorkersCmd.AddCommand(workersAddCmd)
	WorkersCmd.AddCommand(workersRemoveCmd)
	WorkersCmd.AddCommand(workersStatsCmd)

	workersAddCmd.Flags().StringP("agent", "a", "claude", "Agent type (claude, codex, custom)")
}

func runWorkersAdd(cmd *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	agent, _ := cmd.Flags().GetString("agent")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "workers.add", map[string]any{
		"agent": agent,
	})
	if err != nil {
		return fmt.Errorf("workers.add call: %w", err)
	}

	var result struct {
		ID        string `json:"id"`
		AgentName string `json:"agent_name"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Worker added:\n")
	fmt.Printf("  ID:    %s\n", result.ID)
	fmt.Printf("  Agent: %s\n", result.AgentName)
	fmt.Printf("  Status: %s\n", result.Status)

	return nil
}

func runWorkersRemove(cmd *cobra.Command, args []string) error {
	id := args[0]

	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "workers.remove", map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("workers.remove call: %w", err)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.OK {
		fmt.Printf("Worker %s removed\n", id)
	} else {
		fmt.Printf("Failed to remove worker %s\n", id)
	}

	return nil
}

func runWorkersStats(cmd *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "workers.stats", nil)
	if err != nil {
		return fmt.Errorf("workers.stats call: %w", err)
	}

	var result struct {
		TotalWorkers     int `json:"total_workers"`
		AvailableWorkers int `json:"available_workers"`
		WorkingWorkers   int `json:"working_workers"`
		QueuedJobs       int `json:"queued_jobs"`
		InProgressJobs   int `json:"in_progress_jobs"`
		CompletedJobs    int `json:"completed_jobs"`
		FailedJobs       int `json:"failed_jobs"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Println("Worker Pool Statistics:")
	fmt.Printf("  Workers:     %d total, %d available, %d working\n",
		result.TotalWorkers, result.AvailableWorkers, result.WorkingWorkers)
	fmt.Printf("  Jobs:        %d queued, %d in progress\n",
		result.QueuedJobs, result.InProgressJobs)
	fmt.Printf("  Completed:   %d successful, %d failed\n",
		result.CompletedJobs, result.FailedJobs)

	return nil
}
