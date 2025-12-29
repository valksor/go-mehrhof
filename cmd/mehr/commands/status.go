package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

var statusAll bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detailed task state (specs, checkpoints, sessions)",
	Long: `Show a detailed read-only view of the active task(s).

Displays comprehensive information about:
- Task ID, title, state, and source reference
- Specifications and their completion status
- Git checkpoints for undo/redo
- Session history and token usage

DIFFERENCES FROM OTHER COMMANDS:
- 'mehr guide' - Quick, lightweight next-action suggestions
- 'mehr continue' - Resume workflow with optional auto-execution
- 'mehr status' - Detailed state inspection (this command)

Examples:
  mehr status              # Show active task state
  mehr status --all        # Show all tasks in workspace`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVarP(&statusAll, "all", "a", false, "Show all tasks in workspace")
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Resolve workspace root and git context
	res, err := ResolveWorkspaceRoot()
	if err != nil {
		return err
	}

	ws, err := storage.OpenWorkspace(res.Root)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	if statusAll {
		return showAllTasks(ws, res.Git)
	}

	// If in a worktree, auto-detect task from worktree path
	if res.IsWorktree {
		return showWorktreeTask(ws, res.Git)
	}

	return showActiveTask(ws, res.Git)
}

func showWorktreeTask(ws *storage.Workspace, git *vcs.Git) error {
	// Auto-detect task from current worktree
	if git == nil {
		return fmt.Errorf("not in a worktree")
	}
	active, err := ws.FindTaskByWorktreePath(git.Root())
	if err != nil {
		return fmt.Errorf("find task by worktree: %w", err)
	}

	if active == nil {
		fmt.Print(display.ErrorWithSuggestions(
			"No task associated with this worktree",
			[]display.Suggestion{
				{Command: "mehr start <reference>", Description: "Start a new task in this worktree"},
				{Command: "mehr list --all", Description: "View all tasks in workspace"},
			},
		))
		return nil
	}

	work, err := ws.LoadWork(active.ID)
	if err != nil {
		return fmt.Errorf("load work: %w", err)
	}

	fmt.Printf("Worktree Task: %s\n", display.Bold(active.ID))
	fmt.Printf("  Title:    %s\n", work.Metadata.Title)
	if work.Metadata.ExternalKey != "" {
		fmt.Printf("  Key:      %s\n", work.Metadata.ExternalKey)
	}
	fmt.Printf("  State:    %s - %s\n", display.FormatStateStringColored(active.State), display.Muted(display.GetStateDescription(workflow.State(active.State))))
	fmt.Printf("  Source:   %s\n", active.Ref)
	fmt.Printf("  Worktree: %s\n", git.Root())
	fmt.Printf("  Started:  %s\n", active.Started.Format("2006-01-02 15:04:05"))
	if work.Agent.Name != "" {
		agentInfo := work.Agent.Name
		if work.Agent.Source != "" && work.Agent.Source != "auto" {
			agentInfo += fmt.Sprintf(" (from %s)", work.Agent.Source)
		}
		fmt.Printf("  Agent:    %s\n", agentInfo)
	}

	if active.Branch != "" {
		fmt.Printf("  Branch:   %s\n", active.Branch)
	}

	// Show specifications with status
	specifications, _ := ws.ListSpecificationsWithStatus(active.ID)
	if len(specifications) > 0 {
		fmt.Printf("\nSpecifications: %d\n", len(specifications))
		for _, specification := range specifications {
			statusIcon := display.GetSpecificationStatusIcon(specification.Status)
			title := specification.Title
			if title == "" {
				title = "(untitled)"
			}
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Printf("  %s specification-%d: %s [%s]\n", statusIcon, specification.Number, title, display.FormatSpecificationStatus(specification.Status))
		}
	} else {
		fmt.Printf("\nNo specifications yet. Run 'mehr plan' to create them.\n")
	}

	// Show checkpoints
	checkpoints, _ := git.ListCheckpoints(active.ID)
	if len(checkpoints) > 0 {
		fmt.Printf("\nCheckpoints: %d\n", len(checkpoints))
		for _, cp := range checkpoints {
			fmt.Printf("  - #%d: %s (%s)\n", cp.Number, cp.Message, cp.ID[:8])
		}
	}

	// Show next actions
	fmt.Printf("\nAvailable commands:\n")
	if len(specifications) == 0 {
		fmt.Printf("  mehr plan      - Create implementation specifications\n")
	} else {
		fmt.Printf("  mehr implement - Implement the specifications\n")
		fmt.Printf("  mehr plan      - Create additional specifications\n")
	}
	fmt.Printf("  mehr chat      - Add notes or discuss the task\n")
	fmt.Printf("  mehr finish    - Complete and optionally merge\n")

	return nil
}

func showActiveTask(ws *storage.Workspace, git *vcs.Git) error {
	if !ws.HasActiveTask() {
		fmt.Print(display.NoActiveTaskError())
		return nil
	}

	active, err := ws.LoadActiveTask()
	if err != nil {
		return fmt.Errorf("load active task: %w", err)
	}

	work, err := ws.LoadWork(active.ID)
	if err != nil {
		return fmt.Errorf("load work: %w", err)
	}

	fmt.Printf("Active Task: %s\n", display.Bold(active.ID))
	fmt.Printf("  Title:   %s\n", work.Metadata.Title)
	if work.Metadata.ExternalKey != "" {
		fmt.Printf("  Key:     %s\n", work.Metadata.ExternalKey)
	}
	fmt.Printf("  State:   %s - %s\n", display.FormatStateStringColored(active.State), display.Muted(display.GetStateDescription(workflow.State(active.State))))
	fmt.Printf("  Source:  %s\n", active.Ref)
	fmt.Printf("  WorkDir: %s\n", active.WorkDir)
	fmt.Printf("  Started: %s\n", active.Started.Format("2006-01-02 15:04:05"))
	if work.Agent.Name != "" {
		agentInfo := work.Agent.Name
		if work.Agent.Source != "" && work.Agent.Source != "auto" {
			agentInfo += fmt.Sprintf(" (from %s)", work.Agent.Source)
		}
		fmt.Printf("  Agent:   %s\n", agentInfo)
	}

	if active.Branch != "" {
		fmt.Printf("  Branch:  %s\n", active.Branch)
	}

	// Show specifications with status
	specifications, _ := ws.ListSpecificationsWithStatus(active.ID)
	if len(specifications) > 0 {
		fmt.Printf("\nSpecifications: %d\n", len(specifications))
		for _, specification := range specifications {
			statusIcon := display.GetSpecificationStatusIcon(specification.Status)
			title := specification.Title
			if title == "" {
				title = "(untitled)"
			}
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Printf("  %s specification-%d: %s [%s]\n", statusIcon, specification.Number, title, display.FormatSpecificationStatus(specification.Status))
		}

		// Show a summary with user-friendly status names
		summary, _ := ws.GetSpecificationsSummary(active.ID)
		var summaryParts []string
		if summary[storage.SpecificationStatusDone] > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d completed", summary[storage.SpecificationStatusDone]))
		}
		if summary[storage.SpecificationStatusImplementing] > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d implementing", summary[storage.SpecificationStatusImplementing]))
		}
		if summary[storage.SpecificationStatusReady] > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d pending", summary[storage.SpecificationStatusReady]))
		}
		if summary[storage.SpecificationStatusDraft] > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d draft", summary[storage.SpecificationStatusDraft]))
		}
		if len(summaryParts) > 0 {
			fmt.Printf("  Summary: %s\n", strings.Join(summaryParts, ", "))
		}
	} else {
		fmt.Printf("\nNo specifications yet. Run 'mehr plan' to create them.\n")
	}

	// Show checkpoints
	if git != nil {
		checkpoints, err := git.ListCheckpoints(active.ID)
		if err == nil && len(checkpoints) > 0 {
			fmt.Printf("\nCheckpoints: %d\n", len(checkpoints))
			for _, cp := range checkpoints {
				fmt.Printf("  - #%d: %s (%s)\n", cp.Number, cp.Message, cp.ID[:8])
			}
		}
	}

	// Show sessions and token usage
	sessions, _ := ws.ListSessions(active.ID)
	if len(sessions) > 0 {
		fmt.Printf("\nSessions: %d\n", len(sessions))
		var totalTokens int
		for _, s := range sessions {
			if s.Usage != nil {
				totalTokens += s.Usage.InputTokens + s.Usage.OutputTokens
			}
		}
		if totalTokens > 0 {
			fmt.Printf("  Total tokens: %d\n", totalTokens)
		}
	}

	// Show next actions based on state
	fmt.Printf("\nAvailable commands:\n")
	if len(specifications) == 0 {
		fmt.Printf("  mehr plan      - Create implementation specifications\n")
	} else {
		fmt.Printf("  mehr implement - Implement the specifications\n")
		fmt.Printf("  mehr plan      - Create additional specifications\n")
	}
	fmt.Printf("  mehr chat      - Add notes or discuss the task\n")
	fmt.Printf("  mehr finish    - Complete and optionally merge\n")

	return nil
}

func showAllTasks(ws *storage.Workspace, git *vcs.Git) error {
	taskIDs, err := ws.ListWorks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if len(taskIDs) == 0 {
		fmt.Println("No tasks found in workspace.")
		return nil
	}

	// Check which task is active
	var activeID string
	if ws.HasActiveTask() {
		active, _ := ws.LoadActiveTask()
		if active != nil {
			activeID = active.ID
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "TASK ID\tSTATE\tTITLE\tSPECS\tACTIVE"); err != nil {
		return fmt.Errorf("print header: %w", err)
	}

	for _, taskID := range taskIDs {
		work, err := ws.LoadWork(taskID)
		if err != nil {
			continue
		}

		specifications, _ := ws.ListSpecifications(taskID)
		state := "unknown"

		// Check if this is the active task
		isActive := taskID == activeID
		activeMarker := ""
		if isActive {
			active, _ := ws.LoadActiveTask()
			if active != nil {
				state = display.FormatStateStringColored(active.State)
			}
			activeMarker = "*"
		}

		title := work.Metadata.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			taskID,
			state,
			title,
			len(specifications),
			activeMarker); err != nil {
			return fmt.Errorf("print row: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush status table: %w", err)
	}

	// Add legend for symbols
	fmt.Println()
	fmt.Println(display.Muted("Legend:"))
	fmt.Println(display.Muted("  * = active task"))
	fmt.Println(display.Muted("  ○ = draft spec"))
	fmt.Println(display.Muted("  ◐ = ready to implement"))
	fmt.Println(display.Muted("  ◑ = implementing"))
	fmt.Println(display.Muted("  ● = completed"))
	return nil
}
