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

var (
	statusTimeout time.Duration
	statusVerbose bool
	statusJSON    bool
	statusAll     bool
	statusFull    bool
)

var StatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Show current task state",
	Long:    "Connect to the worktree socket and display the current task state.",
	RunE:    runStatus,
}

func init() {
	StatusCmd.Flags().DurationVarP(&statusTimeout, "timeout", "t", 5*time.Second, "Connection timeout")
	StatusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "Show socket paths")
	StatusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output raw JSON response")
	StatusCmd.Flags().BoolVarP(&statusAll, "all", "a", false, "Show status of all active projects")
	StatusCmd.Flags().BoolVar(&statusFull, "full", false, "Show extended status including checkpoints")
}

func runStatus(cmd *cobra.Command, args []string) error {
	if statusAll {
		return showAllStatus()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if statusVerbose {
		fmt.Printf("Socket: %s\n", wtPath)
	}

	if !socket.SocketExists(wtPath) {
		return fmt.Errorf("no worktree socket running for %s\nRun '"+meta.Name+" start' first", cwd)
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(statusTimeout))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	resp, err := client.Call(ctx, "status", nil)
	if err != nil {
		return fmt.Errorf("status call: %w", err)
	}

	// --json: output raw JSON
	if statusJSON {
		return outputJSON(resp.Result)
	}

	var result socket.StatusResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse status: %w", err)
	}

	fmt.Printf("Path:  %s\n", result.Path)
	fmt.Printf("State: Task: %s\n", capitalize(string(result.State)))

	if result.Task != nil {
		fmt.Printf("Task:  %s - %s\n", result.Task.ID, result.Task.Title)
		fmt.Printf("Source: %s\n", result.Task.Source)
	}

	if result.ActiveJobID != "" {
		fmt.Printf("Job:   %s\n", result.ActiveJobID)
	}

	if result.QueueDepth > 0 {
		fmt.Printf("Queue: %d tasks\n", result.QueueDepth)
	}

	if result.LastError != "" {
		fmt.Printf("Error: %s\n", result.LastError)
	}

	if result.PendingPromptID != "" {
		fmt.Printf("\n! Quality gate waiting for your input.\n")
		fmt.Printf("  Run: kvelmo quality respond --prompt-id %s [--yes|--no]\n", result.PendingPromptID)
	}

	if statusFull {
		cpResp, cpErr := client.Call(ctx, "checkpoints", nil)
		if cpErr == nil {
			var cpResult struct {
				Checkpoints []json.RawMessage `json:"checkpoints"`
			}
			if json.Unmarshal(cpResp.Result, &cpResult) == nil {
				fmt.Printf("Checkpoints: %d\n", len(cpResult.Checkpoints))
			}
		}
	}

	return nil
}

func showAllStatus() error {
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' or '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(statusTimeout))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	resp, err := client.Call(ctx, "tasks.list", nil)
	if err != nil {
		return fmt.Errorf("tasks.list: %w", err)
	}

	// --json: output raw JSON
	if statusJSON {
		return outputJSON(resp.Result)
	}

	var result socket.TasksListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse tasks list: %w", err)
	}

	// Filter to only active tasks (state != "none") unless --verbose
	active := make([]socket.TaskListSummary, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		if statusVerbose || (t.State != "" && t.State != "none") {
			active = append(active, t)
		}
	}

	if len(active) == 0 {
		fmt.Println("No active tasks across projects")

		return nil
	}

	fmt.Printf("%-40s  %-14s  %s\n", "PROJECT", "STATE", "TASK")
	fmt.Printf("%-40s  %-14s  %s\n", "----------------------------------------", "--------------", "----")
	for _, t := range active {
		taskDisplay := t.TaskTitle
		if taskDisplay == "" {
			taskDisplay = t.TaskID
		}
		if taskDisplay == "" {
			taskDisplay = "\u2014"
		}

		source := ""
		if t.Source != "" {
			source = " (" + t.Source + ")"
		}

		path := t.Path
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}

		fmt.Printf("%-40s  %-14s  %s%s\n", path, t.State, taskDisplay, source)
	}

	return nil
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}

	return s
}
