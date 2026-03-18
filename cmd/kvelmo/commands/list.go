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
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
	"github.com/valksor/kvelmo/pkg/storage"
)

var ListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all registered projects and their current task state",
	Long: `Queries the global socket for all registered worktrees with their state and task info.

Subcommands:
  history   Show completed/archived task history for the current project
  search    Search archived tasks by keyword

Legacy flag --history is still supported:
  kvelmo list --history --search "auth" --state finished --since 2026-03-01`,
	RunE: runList,
}

var listHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show completed/archived task history for the current project",
	Long: `Display all completed and archived tasks from the current project's worktree.

Use --search / -s to filter by keyword (uses task.search RPC instead of task.history).

Examples:
  kvelmo list history                          # All archived tasks
  kvelmo list history --json                   # Output as JSON
  kvelmo list history -s "auth"                # Search by keyword
  kvelmo list history -s "login" --state finished --since 2026-03-01
  kvelmo list history --file "auth/"           # Tasks that touched auth files`,
	RunE: runListHistoryCmd,
}

var listSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search archived tasks by keyword",
	Long: `Search completed/archived tasks by keyword, tag, state, or date range.

Examples:
  kvelmo list search "auth"                           # Search by keyword
  kvelmo list search "login" --state finished          # Filter by state
  kvelmo list search "" --tag backend --since 2026-03-01  # Filter by tag and date
  kvelmo list search "" --file "auth/login.go"        # Tasks that touched a file`,
	Args: cobra.ExactArgs(1),
	RunE: runListSearchCmd,
}

var (
	listHistoryJSON bool
	listSearchJSON  bool
)

func init() {
	// Legacy flags on parent command (backward compat)
	ListCmd.Flags().Bool("history", false, "Search archived task history for the current project")
	ListCmd.Flags().String("search", "", "Filter history by keyword (matches title, branch, source)")
	ListCmd.Flags().String("tag", "", "Filter history by tag")
	ListCmd.Flags().String("since", "", "Show tasks completed after this date (RFC3339 or YYYY-MM-DD)")
	ListCmd.Flags().String("until", "", "Show tasks completed before this date (RFC3339 or YYYY-MM-DD)")
	ListCmd.Flags().String("state", "", "Filter by final state (e.g., finished, abandoned)")
	ListCmd.Flags().Int("limit", 0, "Maximum number of results (0 = unlimited)")
	ListCmd.Flags().String("file", "", "Filter by file path touched (substring match)")

	// history subcommand flags
	listHistoryCmd.Flags().BoolVar(&listHistoryJSON, "json", false, "Output as JSON")
	listHistoryCmd.Flags().StringP("search", "s", "", "Filter by keyword (uses task.search RPC)")
	listHistoryCmd.Flags().String("tag", "", "Filter by tag")
	listHistoryCmd.Flags().String("since", "", "Show tasks completed after this date (RFC3339 or YYYY-MM-DD)")
	listHistoryCmd.Flags().String("until", "", "Show tasks completed before this date (RFC3339 or YYYY-MM-DD)")
	listHistoryCmd.Flags().String("state", "", "Filter by final state (e.g., finished, abandoned)")
	listHistoryCmd.Flags().Int("limit", 0, "Maximum number of results (0 = unlimited)")
	listHistoryCmd.Flags().String("file", "", "Filter by file path touched (substring match)")

	// search subcommand flags
	listSearchCmd.Flags().BoolVar(&listSearchJSON, "json", false, "Output as JSON")
	listSearchCmd.Flags().String("tag", "", "Filter by tag")
	listSearchCmd.Flags().String("since", "", "Show tasks completed after this date (RFC3339 or YYYY-MM-DD)")
	listSearchCmd.Flags().String("until", "", "Show tasks completed before this date (RFC3339 or YYYY-MM-DD)")
	listSearchCmd.Flags().String("state", "", "Filter by final state (e.g., finished, abandoned)")
	listSearchCmd.Flags().Int("limit", 0, "Maximum number of results (0 = unlimited)")
	listSearchCmd.Flags().String("file", "", "Filter by file path touched (substring match)")

	ListCmd.AddCommand(listHistoryCmd)
	ListCmd.AddCommand(listSearchCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	history, _ := cmd.Flags().GetBool("history")
	if history {
		return runListHistoryLegacy(cmd)
	}

	return runListProjects()
}

func runListProjects() error {
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' or '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "tasks.list", nil)
	if err != nil {
		return fmt.Errorf("tasks.list: %w", err)
	}

	var result socket.TasksListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Tasks) == 0 {
		fmt.Println("No registered projects")

		return nil
	}

	fmt.Printf("%-16s  %-14s  %-30s  %s\n", "ID", "State", "Task", "Path")
	fmt.Printf("%-16s  %-14s  %-30s  %s\n", "----------------", "--------------", "------------------------------", "----")
	for _, t := range result.Tasks {
		taskDisplay := t.TaskTitle
		if taskDisplay == "" {
			taskDisplay = t.TaskID
		}
		if taskDisplay == "" {
			taskDisplay = "(no task)"
		}
		if len(taskDisplay) > 30 {
			taskDisplay = taskDisplay[:27] + "..."
		}

		fmt.Printf("%-16s  %-14s  %-30s  %s\n",
			t.ID,
			t.State,
			taskDisplay,
			t.Path,
		)
	}

	return nil
}

// runListHistoryCmd handles the "list history" subcommand.
// Without --search it calls task.history; with --search it calls task.search.
func runListHistoryCmd(cmd *cobra.Command, _ []string) error {
	search, _ := cmd.Flags().GetString("search")
	if search != "" {
		return runTaskSearch(cmd, search, listHistoryJSON)
	}

	// Check if any filter flags are set -- if so, use task.search for filtering
	tag, _ := cmd.Flags().GetString("tag")
	sinceStr, _ := cmd.Flags().GetString("since")
	untilStr, _ := cmd.Flags().GetString("until")
	state, _ := cmd.Flags().GetString("state")
	limit, _ := cmd.Flags().GetInt("limit")

	fileFilter, _ := cmd.Flags().GetString("file")

	if tag != "" || sinceStr != "" || untilStr != "" || state != "" || limit > 0 || fileFilter != "" {
		return runTaskSearch(cmd, "", listHistoryJSON)
	}

	return runTaskHistory(listHistoryJSON)
}

// runListSearchCmd handles the "list search <query>" subcommand.
func runListSearchCmd(cmd *cobra.Command, args []string) error {
	return runTaskSearch(cmd, args[0], listSearchJSON)
}

// runTaskHistory calls task.history RPC to get all archived tasks.
func runTaskHistory(outputJSON bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return errors.New("worktree socket not running in current directory\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "task.history", nil)
	if err != nil {
		return fmt.Errorf("task.history: %w", err)
	}

	var result struct {
		Tasks []storage.ArchivedTask `json:"tasks"`
		Count int                    `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	return printArchivedTasks(result.Tasks, result.Count, outputJSON)
}

// runTaskSearch calls task.search RPC with filter parameters.
func runTaskSearch(cmd *cobra.Command, query string, outputJSON bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return errors.New("worktree socket not running in current directory\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	tag, _ := cmd.Flags().GetString("tag")
	sinceStr, _ := cmd.Flags().GetString("since")
	untilStr, _ := cmd.Flags().GetString("until")
	state, _ := cmd.Flags().GetString("state")
	limit, _ := cmd.Flags().GetInt("limit")

	params := map[string]any{}
	if query != "" {
		params["query"] = query
	}
	if tag != "" {
		params["tag"] = tag
	}
	if sinceStr != "" {
		t, err := parseFlexibleTime(sinceStr)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
		params["since"] = t.Format(time.RFC3339)
	}
	if untilStr != "" {
		t, err := parseFlexibleTime(untilStr)
		if err != nil {
			return fmt.Errorf("parse --until: %w", err)
		}
		params["until"] = t.Format(time.RFC3339)
	}
	if state != "" {
		params["state"] = state
	}
	if limit > 0 {
		params["limit"] = limit
	}
	fileFilter, _ := cmd.Flags().GetString("file")
	if fileFilter != "" {
		params["file"] = fileFilter
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "task.search", params)
	if err != nil {
		return fmt.Errorf("task.search: %w", err)
	}

	var result struct {
		Tasks []storage.ArchivedTask `json:"tasks"`
		Count int                    `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	return printArchivedTasks(result.Tasks, result.Count, outputJSON)
}

// printArchivedTasks displays archived tasks in table or JSON format.
func printArchivedTasks(tasks []storage.ArchivedTask, count int, outputJSON bool) error {
	if outputJSON {
		data, err := json.MarshalIndent(map[string]any{
			"tasks": tasks,
			"count": count,
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(data))

		return nil
	}

	if len(tasks) == 0 {
		fmt.Println("No matching archived tasks")

		return nil
	}

	fmt.Printf("%-30s  %-12s  %-20s  %s\n", "Title", "State", "Source", "Completed")
	fmt.Printf("%-30s  %-12s  %-20s  %s\n",
		strings.Repeat("-", 30), strings.Repeat("-", 12),
		strings.Repeat("-", 20), strings.Repeat("-", 19))
	for _, t := range tasks {
		title := t.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		source := t.Source
		if len(source) > 20 {
			source = source[:17] + "..."
		}

		completed := t.CompletedAt.Format("2006-01-02 15:04:05")

		fmt.Printf("%-30s  %-12s  %-20s  %s\n", title, t.FinalState, source, completed)
	}

	fmt.Printf("\n%d task(s) found\n", count)

	return nil
}

// runListHistoryLegacy handles the legacy "list --history" flag path.
func runListHistoryLegacy(cmd *cobra.Command) error {
	search, _ := cmd.Flags().GetString("search")
	if search != "" {
		return runTaskSearch(cmd, search, false)
	}

	// Check if any filter flags are set
	tag, _ := cmd.Flags().GetString("tag")
	sinceStr, _ := cmd.Flags().GetString("since")
	untilStr, _ := cmd.Flags().GetString("until")
	state, _ := cmd.Flags().GetString("state")
	limit, _ := cmd.Flags().GetInt("limit")

	fileFilter, _ := cmd.Flags().GetString("file")

	if tag != "" || sinceStr != "" || untilStr != "" || state != "" || limit > 0 || fileFilter != "" {
		return runTaskSearch(cmd, "", false)
	}

	return runTaskHistory(false)
}

// parseFlexibleTime parses a time string as either RFC3339 or YYYY-MM-DD.
func parseFlexibleTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD format, got %q", s)
}
