package commands

import (
	"context"
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

var (
	statusAll  bool
	statusJSON bool
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Full task inspection: specs, checkpoints, sessions",
	Long: `Comprehensive view of your task - everything in one place.

Use this when you need the complete picture:
  • Task metadata (ID, title, source, branch, started time)
  • Specifications with status icons and progress summary
  • Git checkpoints available for undo/redo
  • Session history and token usage
  • Agent configuration

CHOOSING THE RIGHT COMMAND:
  guide     - "What's my next command?" (fastest, minimal output)
  status    - "Show me everything" (full inspection, all details)  <-- you are here
  continue  - "Resume and optionally auto-execute" (--auto runs next step)

OUTPUT FORMATS:
  Default: Human-readable text with colors
  --json:  Machine-parseable JSON (for scripts and integrations)

Examples:
  mehr status        # Full details for active task
  mehr st            # Same (shorthand alias)
  mehr status --all  # List all tasks in workspace
  mehr status --json # JSON output for scripting`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVarP(&statusAll, "all", "a", false, "Show all tasks in workspace")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Resolve workspace root and git context
	res, err := ResolveWorkspaceRoot(ctx)
	if err != nil {
		return err
	}

	ws, err := storage.OpenWorkspace(res.Root, nil)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	if statusAll {
		return showAllTasks(ws)
	}

	// If in a worktree, auto-detect task from worktree path
	if res.IsWorktree {
		return showWorktreeTask(ctx, ws, res.Git)
	}

	return showActiveTask(ctx, ws, res.Git)
}

func showWorktreeTask(ctx context.Context, ws *storage.Workspace, git *vcs.Git) error {
	// Auto-detect task from current worktree
	if git == nil {
		return fmt.Errorf("not in a worktree")
	}
	active, err := ws.FindTaskByWorktreePath(git.Root())
	if err != nil {
		return fmt.Errorf("find task by worktree: %w", err)
	}

	if active == nil {
		if statusJSON {
			return outputJSON(jsonStatusTask{})
		}
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

	// JSON output path
	if statusJSON {
		return outputJSON(buildJSONStatusTask(ctx, ws, git, active, work, git.Root()))
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
	checkpoints, _ := git.ListCheckpoints(ctx, active.ID)
	if len(checkpoints) > 0 {
		fmt.Printf("\nCheckpoints: %d\n", len(checkpoints))
		for _, cp := range checkpoints {
			fmt.Printf("  - #%d: %s (%s)\n", cp.Number, cp.Message, cp.ID[:8])
		}
	}

	// Show spec icon legend if there are specifications
	if len(specifications) > 0 {
		printSpecLegend()
	}

	// Show next actions
	fmt.Printf("\nAvailable commands:\n")
	if len(specifications) == 0 {
		fmt.Printf("  mehr plan      - Create implementation specifications\n")
	} else {
		fmt.Printf("  mehr implement - Implement the specifications\n")
		fmt.Printf("  mehr plan      - Create additional specifications\n")
	}
	fmt.Printf("  mehr note      - Add notes to the task\n")
	fmt.Printf("  mehr finish    - Complete and optionally merge\n")

	return nil
}

func showActiveTask(ctx context.Context, ws *storage.Workspace, git *vcs.Git) error {
	if !ws.HasActiveTask() {
		if statusJSON {
			return outputJSON(jsonStatusTask{})
		}
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

	// JSON output path
	if statusJSON {
		return outputJSON(buildJSONStatusTask(ctx, ws, git, active, work, ""))
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
			summaryParts = append(summaryParts, fmt.Sprintf("%d ready", summary[storage.SpecificationStatusReady]))
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
		checkpoints, err := git.ListCheckpoints(ctx, active.ID)
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

	// Show spec icon legend if there are specifications
	if len(specifications) > 0 {
		printSpecLegend()
	}

	// Show next actions based on state
	fmt.Printf("\nAvailable commands:\n")
	if len(specifications) == 0 {
		fmt.Printf("  mehr plan      - Create implementation specifications\n")
	} else {
		fmt.Printf("  mehr implement - Implement the specifications\n")
		fmt.Printf("  mehr plan      - Create additional specifications\n")
	}
	fmt.Printf("  mehr note      - Add notes to the task\n")
	fmt.Printf("  mehr finish    - Complete and optionally merge\n")

	return nil
}

func showAllTasks(ws *storage.Workspace) error {
	taskIDs, err := ws.ListWorks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if len(taskIDs) == 0 {
		if statusJSON {
			return outputJSON(jsonStatusAllOutput{Tasks: []jsonStatusTask{}})
		}
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

	// JSON output path
	if statusJSON {
		var tasks []jsonStatusTask
		for _, taskID := range taskIDs {
			work, err := ws.LoadWork(taskID)
			if err != nil {
				continue
			}
			isActive := taskID == activeID
			state := "unknown"
			if isActive {
				active, _ := ws.LoadActiveTask()
				if active != nil {
					state = active.State
				}
			}

			title := work.Metadata.Title
			if title == "" {
				title = "(untitled)"
			}

			tasks = append(tasks, jsonStatusTask{
				TaskID:   taskID,
				Title:    title,
				State:    state,
				IsActive: isActive,
			})
		}
		return outputJSON(jsonStatusAllOutput{Tasks: tasks})
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
	printSpecLegend()
	return nil
}

// printSpecLegend prints the specification status icon legend.
func printSpecLegend() {
	fmt.Println()
	fmt.Println(display.Muted("Specification icons:"))
	fmt.Println(display.Muted("  ○ = Draft (not yet finalized)"))
	fmt.Println(display.Muted("  ◐ = Ready (queued for implementation)"))
	fmt.Println(display.Muted("  ◑ = Implementing (in progress)"))
	fmt.Println(display.Muted("  ● = Completed"))
}

// JSON output structures for status command.
type jsonStatusTask struct {
	TaskID         string              `json:"task_id"`
	Title          string              `json:"title,omitempty"`
	State          string              `json:"state"`
	StateDesc      string              `json:"state_description"`
	Source         string              `json:"source"`
	ExternalKey    string              `json:"external_key,omitempty"`
	WorkDir        string              `json:"work_dir,omitempty"`
	WorktreePath   string              `json:"worktree_path,omitempty"`
	Branch         string              `json:"branch,omitempty"`
	Started        string              `json:"started_at"`
	AgentName      string              `json:"agent_name,omitempty"`
	AgentSource    string              `json:"agent_source,omitempty"`
	IsActive       bool                `json:"is_active"`
	Specifications []jsonSpecification `json:"specifications,omitempty"`
	SpecSummary    *jsonSpecSummary    `json:"specifications_summary,omitempty"`
	Checkpoints    []jsonCheckpoint    `json:"checkpoints,omitempty"`
	Sessions       []jsonSession       `json:"sessions,omitempty"`
	TotalTokens    int                 `json:"total_tokens,omitempty"`
}

type jsonSpecification struct {
	Number      int    `json:"number"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type jsonSpecSummary struct {
	Draft        int `json:"draft"`
	Ready        int `json:"ready"`
	Implementing int `json:"implementing"`
	Done         int `json:"done"`
}

type jsonCheckpoint struct {
	Number    int    `json:"number"`
	Message   string `json:"message"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp,omitempty"`
}

type jsonSession struct {
	Kind         string `json:"kind"`
	StartTime    string `json:"start_time,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

type jsonStatusAllOutput struct {
	Tasks []jsonStatusTask `json:"tasks"`
}

// buildJSONStatusTask constructs a jsonStatusTask from workspace data.
func buildJSONStatusTask(ctx context.Context, ws *storage.Workspace, git *vcs.Git, active *storage.ActiveTask, work *storage.TaskWork, worktreePath string) jsonStatusTask {
	task := jsonStatusTask{
		TaskID:       active.ID,
		Title:        work.Metadata.Title,
		State:        active.State,
		StateDesc:    display.GetStateDescription(workflow.State(active.State)),
		Source:       active.Ref,
		ExternalKey:  work.Metadata.ExternalKey,
		WorkDir:      active.WorkDir,
		WorktreePath: worktreePath,
		Branch:       active.Branch,
		Started:      active.Started.Format("2006-01-02T15:04:05Z"),
		AgentName:    work.Agent.Name,
		AgentSource:  work.Agent.Source,
		IsActive:     true,
	}

	// Get specifications with status
	specifications, _ := ws.ListSpecificationsWithStatus(active.ID)
	for _, spec := range specifications {
		task.Specifications = append(task.Specifications, jsonSpecification{
			Number:      spec.Number,
			Title:       spec.Title,
			Status:      spec.Status,
			CreatedAt:   spec.CreatedAt.Format("2006-01-02T15:04:05Z"),
			CompletedAt: spec.CompletedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	// Get specification summary
	summary, _ := ws.GetSpecificationsSummary(active.ID)
	task.SpecSummary = &jsonSpecSummary{
		Draft:        summary[storage.SpecificationStatusDraft],
		Ready:        summary[storage.SpecificationStatusReady],
		Implementing: summary[storage.SpecificationStatusImplementing],
		Done:         summary[storage.SpecificationStatusDone],
	}

	// Get checkpoints if git is available
	if git != nil {
		checkpoints, _ := git.ListCheckpoints(ctx, active.ID)
		for _, cp := range checkpoints {
			task.Checkpoints = append(task.Checkpoints, jsonCheckpoint{
				Number:    cp.Number,
				Message:   cp.Message,
				ID:        cp.ID,
				Timestamp: cp.Timestamp.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	// Get sessions and token usage
	sessions, _ := ws.ListSessions(active.ID)
	for _, s := range sessions {
		inputTokens := 0
		outputTokens := 0
		if s.Usage != nil {
			inputTokens = s.Usage.InputTokens
			outputTokens = s.Usage.OutputTokens
			task.TotalTokens += inputTokens + outputTokens
		}
		task.Sessions = append(task.Sessions, jsonSession{
			Kind:         s.Kind,
			StartTime:    s.Metadata.StartedAt.Format("2006-01-02T15:04:05Z"),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		})
	}

	return task
}
