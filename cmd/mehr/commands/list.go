package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks in workspace",
	Long: `List all tasks in the workspace with their worktree paths and states.

This is useful for seeing all parallel tasks across multiple terminals.
Tasks with worktrees can be worked on independently in separate terminals.

Examples:
  mehr list              # List all tasks
  mehr list --worktrees  # Show only tasks with worktrees`,
	RunE: runList,
}

var listWorktreesOnly bool

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVarP(&listWorktreesOnly, "worktrees", "w", false, "Show only tasks with worktrees")
}

func runList(cmd *cobra.Command, args []string) error {
	// Find workspace
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Try to get git root but allow non-git directories
	var root string
	git, err := vcs.New(cwd)
	if err == nil {
		// If in a worktree, use main repo for storage
		if git.IsWorktree() {
			mainRepo, err := git.GetMainWorktreePath()
			if err != nil {
				return fmt.Errorf("get main repo from worktree: %w", err)
			}
			root = mainRepo
		} else {
			root = git.Root()
		}
	} else {
		root = cwd
	}

	ws, err := storage.OpenWorkspace(root)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	// Get all tasks
	taskIDs, err := ws.ListWorks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if len(taskIDs) == 0 {
		fmt.Println("No tasks found in workspace.")
		fmt.Println("\nUse 'mehr start <reference>' to create a new task.")
		return nil
	}

	// Check which task is active (in main repo)
	var activeID string
	if ws.HasActiveTask() {
		active, _ := ws.LoadActiveTask()
		if active != nil {
			activeID = active.ID
		}
	}

	// Get current worktree path if we're in one
	var currentWorktreePath string
	if git != nil && git.IsWorktree() {
		currentWorktreePath = git.Root()
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "TASK ID\tSTATE\tTITLE\tWORKTREE\tACTIVE"); err != nil {
		return fmt.Errorf("print header: %w", err)
	}

	var shownCount int
	for _, taskID := range taskIDs {
		work, err := ws.LoadWork(taskID)
		if err != nil {
			continue
		}

		// Filter by worktrees if requested
		if listWorktreesOnly && work.Git.WorktreePath == "" {
			continue
		}

		shownCount++

		// Get state
		state := "idle"
		isActive := taskID == activeID
		if isActive {
			active, _ := ws.LoadActiveTask()
			if active != nil {
				state = display.FormatStateString(active.State)
			}
		}

		// Format title
		title := work.Metadata.Title
		if len(title) > 35 {
			title = title[:32] + "..."
		}

		// Format worktree path (relative if possible)
		worktreePath := "-"
		if work.Git.WorktreePath != "" {
			worktreePath = work.Git.WorktreePath
			// Try to make it relative
			if rel, err := filepath.Rel(root, worktreePath); err == nil && len(rel) < len(worktreePath) {
				worktreePath = rel
			}
		}

		// Active marker
		activeMarker := ""
		if isActive {
			activeMarker = "*"
		}
		// Mark if we're currently in this worktree
		if currentWorktreePath != "" && work.Git.WorktreePath == currentWorktreePath {
			activeMarker = "→" // Arrow indicates current worktree
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			taskID,
			state,
			title,
			worktreePath,
			activeMarker); err != nil {
			return fmt.Errorf("print row: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush list table: %w", err)
	}

	if shownCount == 0 && listWorktreesOnly {
		fmt.Println("\nNo tasks with worktrees found.")
		fmt.Println("Use 'mehr start --worktree <reference>' to create a task with a worktree.")
	} else {
		fmt.Println()
		fmt.Println("Legend: * = active task in main repo, → = current worktree")
	}

	return nil
}
