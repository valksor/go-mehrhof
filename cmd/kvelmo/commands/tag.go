package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var TagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage task tags",
	Long:  "Add, remove, or list tags on the current task for categorization and filtering.",
}

var tagAddCmd = &cobra.Command{
	Use:   "add <tag> [tag...]",
	Short: "Add tags to the current task",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTagAdd,
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <tag>",
	Short: "Remove a tag from the current task",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return completeExistingTags()
	},
	RunE: runTagRemove,
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags on the current task",
	RunE:  runTagList,
}

var tagListJSON bool

func init() {
	TagCmd.AddCommand(tagAddCmd)
	TagCmd.AddCommand(tagRemoveCmd)
	TagCmd.AddCommand(tagListCmd)

	tagListCmd.Flags().BoolVar(&tagListJSON, "json", false, "Output raw JSON response")
}

func runTagAdd(_ *cobra.Command, args []string) error {
	resp, err := callWorktree(context.Background(), "task.tag", map[string]any{
		"action": "add",
		"tags":   args,
	})
	if err != nil {
		return fmt.Errorf("task.tag: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("task.tag: %s", resp.Error.Message)
	}

	fmt.Printf("Added tags: %s\n", strings.Join(args, ", "))

	return nil
}

func runTagRemove(_ *cobra.Command, args []string) error {
	resp, err := callWorktree(context.Background(), "task.tag", map[string]any{
		"action": "remove",
		"tags":   []string{args[0]},
	})
	if err != nil {
		return fmt.Errorf("task.tag: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("task.tag: %s", resp.Error.Message)
	}

	fmt.Printf("Removed tag: %s\n", args[0])

	return nil
}

func runTagList(_ *cobra.Command, _ []string) error {
	resp, err := callWorktree(context.Background(), "task.tag", map[string]any{
		"action": "list",
	})
	if err != nil {
		return fmt.Errorf("task.tag: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("task.tag: %s", resp.Error.Message)
	}

	if tagListJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Tags) == 0 {
		fmt.Println("No tags.")
	} else {
		for _, tag := range result.Tags {
			fmt.Printf("  %s\n", tag)
		}
	}

	return nil
}

// completeExistingTags queries the worktree socket for current task tags to provide
// shell completion suggestions. Falls back gracefully if the socket is unavailable.
func completeExistingTags() ([]string, cobra.ShellCompDirective) {
	client, cleanup, err := connectWorktree()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "task.tag", map[string]any{
		"action": "list",
	})
	if err != nil || resp.Error != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return result.Tags, cobra.ShellCompDirectiveNoFileComp
}

func connectWorktree() (*socket.Client, func(), error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return nil, nil, errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return nil, nil, fmt.Errorf("connect to worktree socket: %w", err)
	}

	return client, func() { _ = client.Close() }, nil
}
