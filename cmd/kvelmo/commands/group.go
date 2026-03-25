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
	"github.com/valksor/kvelmo/pkg/taskgroup"
)

// GroupCmd is the top-level command for cross-repo task group management.
var GroupCmd = &cobra.Command{
	Use:   "group",
	Short: "Cross-repo task group management",
	Long:  "Manage task groups that link tasks across repositories for synchronized lifecycle operations.",
}

var groupCreateCmd = &cobra.Command{
	Use:   "create <label>",
	Short: "Create a new task group",
	Args:  cobra.ExactArgs(1),
	RunE:  runGroupCreate,
}

var groupAddCmd = &cobra.Command{
	Use:   "add <group-id> <task-id>",
	Short: "Add a task to a group",
	Args:  cobra.ExactArgs(2),
	RunE:  runGroupAdd,
}

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all task groups",
	RunE:  runGroupList,
}

var groupStatusCmd = &cobra.Command{
	Use:   "status <group-id>",
	Short: "Show group status and member tasks",
	Args:  cobra.ExactArgs(1),
	RunE:  runGroupStatus,
}

var groupSubmitCmd = &cobra.Command{
	Use:   "submit <group-id>",
	Short: "Mark group as submitted",
	Long:  "Mark group as submitted. Does not create PRs — use 'kvelmo submit' in each project.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGroupSubmit,
}

var groupRemoveCmd = &cobra.Command{
	Use:   "remove <group-id>",
	Short: "Remove a task group",
	Args:  cobra.ExactArgs(1),
	RunE:  runGroupRemove,
}

var groupAddState string

func init() {
	GroupCmd.AddCommand(groupCreateCmd)
	GroupCmd.AddCommand(groupAddCmd)
	GroupCmd.AddCommand(groupListCmd)
	GroupCmd.AddCommand(groupStatusCmd)
	GroupCmd.AddCommand(groupSubmitCmd)
	GroupCmd.AddCommand(groupRemoveCmd)

	groupAddCmd.Flags().StringVar(&groupAddState, "state", "loaded", "Initial state to record for the task")
}

func groupClient() (*socket.Client, error) {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return nil, errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	return socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
}

func runGroupCreate(_ *cobra.Command, args []string) error {
	client, err := groupClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "taskgroup.create", map[string]any{"label": args[0]})
	if err != nil {
		return fmt.Errorf("taskgroup.create: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("taskgroup.create: %s", resp.Error.Message)
	}

	var g taskgroup.Group
	if err := json.Unmarshal(resp.Result, &g); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	fmt.Printf("Created group %s (%s)\n", g.ID, g.Label)

	return nil
}

func runGroupAdd(_ *cobra.Command, args []string) error {
	client, err := groupClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "taskgroup.add", map[string]any{
		"id":          args[0],
		"task_id":     args[1],
		"project_dir": cwd,
		"state":       groupAddState,
	})
	if err != nil {
		return fmt.Errorf("taskgroup.add: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("taskgroup.add: %s", resp.Error.Message)
	}
	fmt.Printf("Added task %s to group %s\n", args[1], args[0])
	fmt.Printf("Note: task state recorded as %q. Use 'kvelmo group status' to verify.\n", groupAddState)

	return nil
}

func runGroupList(_ *cobra.Command, _ []string) error {
	client, err := groupClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "taskgroup.list", nil)
	if err != nil {
		return fmt.Errorf("taskgroup.list: %w", err)
	}

	var result struct {
		Groups []taskgroup.Group `json:"groups"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if len(result.Groups) == 0 {
		fmt.Println("No task groups.")

		return nil
	}

	for _, g := range result.Groups {
		fmt.Printf("  %-18s %-8s %s (%d tasks)\n", g.ID, g.Status, g.Label, len(g.Tasks))
	}

	return nil
}

func runGroupStatus(_ *cobra.Command, args []string) error {
	client, err := groupClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "taskgroup.status", map[string]any{"id": args[0]})
	if err != nil {
		return fmt.Errorf("taskgroup.status: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("taskgroup.status: %s", resp.Error.Message)
	}

	var g taskgroup.Group
	if err := json.Unmarshal(resp.Result, &g); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	fmt.Printf("Group:  %s\n", g.ID)
	fmt.Printf("Label:  %s\n", g.Label)
	fmt.Printf("Status: %s\n", g.Status)
	fmt.Printf("Tasks:  %d\n", len(g.Tasks))
	for _, t := range g.Tasks {
		fmt.Printf("  %-20s %-14s %s\n", t.TaskID, t.State, t.ProjectDir)
	}

	return nil
}

func runGroupSubmit(_ *cobra.Command, args []string) error {
	client, err := groupClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "taskgroup.submit", map[string]any{"id": args[0]})
	if err != nil {
		return fmt.Errorf("taskgroup.submit: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("taskgroup.submit: %s", resp.Error.Message)
	}
	fmt.Printf("Group %s submitted\n", args[0])

	return nil
}

func runGroupRemove(_ *cobra.Command, args []string) error {
	client, err := groupClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "taskgroup.remove", map[string]any{"id": args[0]})
	if err != nil {
		return fmt.Errorf("taskgroup.remove: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("taskgroup.remove: %s", resp.Error.Message)
	}
	fmt.Printf("Group %s removed\n", args[0])

	return nil
}
