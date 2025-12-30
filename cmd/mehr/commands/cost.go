package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/storage"
)

var (
	costByStep   bool
	costAllTasks bool
	costSummary  bool
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Show token usage and costs",
	Long: `Show token usage and API costs for tasks.

Displays input/output tokens, cached tokens, and estimated costs.
Costs are tracked per workflow step (planning, implementing, etc.).

Examples:
  mehr cost               # Show costs for active task
  mehr cost --by-step     # Break down by workflow step
  mehr cost --all         # Show costs for all tasks
  mehr cost --summary     # Summary of all tasks`,
	RunE: runCost,
}

func init() {
	rootCmd.AddCommand(costCmd)

	costCmd.Flags().BoolVar(&costByStep, "by-step", false, "Show breakdown by workflow step")
	costCmd.Flags().BoolVarP(&costAllTasks, "all", "a", false, "Show costs for all tasks")
	costCmd.Flags().BoolVarP(&costSummary, "summary", "s", false, "Show summary of all tasks")
}

func runCost(cmd *cobra.Command, args []string) error {
	// Resolve workspace root and git context
	res, err := ResolveWorkspaceRoot()
	if err != nil {
		return err
	}

	ws, err := storage.OpenWorkspace(res.Root)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	if costAllTasks || costSummary {
		return showAllCosts(ws, costSummary)
	}

	// If in a worktree, auto-detect task from worktree path
	if res.IsWorktree {
		return showWorktreeCost(ws, res.Git)
	}

	return showActiveCost(ws)
}

func showWorktreeCost(ws *storage.Workspace, git interface{}) error {
	if git == nil {
		return fmt.Errorf("not in a worktree")
	}
	active, err := ws.FindTaskByWorktreePath(ws.Root())
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

	return showTaskCost(ws, active.ID, active.ID)
}

func showActiveCost(ws *storage.Workspace) error {
	if !ws.HasActiveTask() {
		fmt.Print(display.NoActiveTaskError())
		return nil
	}

	active, err := ws.LoadActiveTask()
	if err != nil {
		return fmt.Errorf("load active task: %w", err)
	}

	return showTaskCost(ws, active.ID, active.ID)
}

func showTaskCost(ws *storage.Workspace, taskID, label string) error {
	work, err := ws.LoadWork(taskID)
	if err != nil {
		return fmt.Errorf("load work: %w", err)
	}

	costs := work.Costs

	// Check if any costs have been recorded
	if costs.TotalInputTokens == 0 && costs.TotalOutputTokens == 0 {
		fmt.Printf("No cost data available for task: %s\n", display.Bold(label))
		fmt.Printf("\nRun 'mehr plan' or 'mehr implement' to generate costs.\n")
		return nil
	}

	fmt.Printf("Costs for task: %s\n", display.Bold(label))
	if work.Metadata.Title != "" {
		fmt.Printf("  Title: %s\n", work.Metadata.Title)
	}

	// Calculate totals
	totalTokens := costs.TotalInputTokens + costs.TotalOutputTokens
	cachedPercent := 0.0
	if totalTokens > 0 && costs.TotalCachedTokens > 0 {
		cachedPercent = float64(costs.TotalCachedTokens) / float64(totalTokens) * 100
	}

	fmt.Printf("\n%s\n", display.Bold("Total Usage:"))
	fmt.Printf("  Input tokens:  %s\n", formatNumber(costs.TotalInputTokens))
	fmt.Printf("  Output tokens: %s\n", formatNumber(costs.TotalOutputTokens))
	fmt.Printf("  Cached tokens: %s", formatNumber(costs.TotalCachedTokens))
	if costs.TotalCachedTokens > 0 {
		fmt.Printf(" (%.1f%% of total)", cachedPercent)
	}
	fmt.Println()
	fmt.Printf("  Total tokens:  %s\n", formatNumber(totalTokens))
	fmt.Printf("  Total cost:    %s\n", display.Bold(formatCost(costs.TotalCostUSD)))

	// Show by-step breakdown if requested or if there are multiple steps
	if costByStep || len(costs.ByStep) > 1 {
		if len(costs.ByStep) > 0 {
			fmt.Printf("\n%s\n", display.Bold("By Step:"))

			// Sort steps by name for consistent output
			steps := make([]string, 0, len(costs.ByStep))
			for step := range costs.ByStep {
				steps = append(steps, step)
			}
			sort.Strings(steps)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "  STEP\t\tINPUT\tOUTPUT\tCACHED\tCOST\tCALLS")
			for _, step := range steps {
				stats := costs.ByStep[step]
				_, _ = fmt.Fprintf(w, "  %s\t\t%s\t%s\t%s\t%s\t%d\n",
					formatStepName(step),
					formatNumber(stats.InputTokens),
					formatNumber(stats.OutputTokens),
					formatNumber(stats.CachedTokens),
					formatCost(stats.CostUSD),
					stats.Calls,
				)
			}
			_ = w.Flush()
		}
	}

	return nil
}

func showAllCosts(ws *storage.Workspace, summaryMode bool) error {
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

	if summaryMode {
		return showCostSummary(ws, taskIDs, activeID)
	}

	// Show all tasks with costs
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TASK ID\tTITLE\tINPUT\tOUTPUT\tTOTAL\tCOST")

	for _, taskID := range taskIDs {
		work, err := ws.LoadWork(taskID)
		if err != nil {
			continue
		}

		costs := work.Costs
		totalTokens := costs.TotalInputTokens + costs.TotalOutputTokens

		title := work.Metadata.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		if title == "" {
			title = "(untitled)"
		}

		// Mark active task
		if taskID == activeID {
			title = "* " + title
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			taskID,
			title,
			formatNumber(costs.TotalInputTokens),
			formatNumber(costs.TotalOutputTokens),
			formatNumber(totalTokens),
			formatCost(costs.TotalCostUSD),
		)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush table: %w", err)
	}

	// Calculate grand total
	var grandTotalInput, grandTotalOutput, grandTotalCost int64
	for _, taskID := range taskIDs {
		work, err := ws.LoadWork(taskID)
		if err != nil {
			continue
		}
		grandTotalInput += int64(work.Costs.TotalInputTokens)
		grandTotalOutput += int64(work.Costs.TotalOutputTokens)
		grandTotalCost += int64(work.Costs.TotalCostUSD * 10000) // Convert to fixed-point
	}

	fmt.Println()
	fmt.Printf("Total: %s input, %s output, %s\n",
		formatNumber(int(grandTotalInput)),
		formatNumber(int(grandTotalOutput)),
		formatCost(float64(grandTotalCost)/10000),
	)

	return nil
}

func showCostSummary(ws *storage.Workspace, taskIDs []string, activeID string) error {
	var grandTotalInput, grandTotalOutput, grandTotalCached int
	var grandTotalCost float64
	var taskCount int

	// Per-step totals
	stepTotals := make(map[string]*storage.StepCostStats)

	for _, taskID := range taskIDs {
		work, err := ws.LoadWork(taskID)
		if err != nil {
			continue
		}

		taskCount++
		costs := work.Costs

		grandTotalInput += costs.TotalInputTokens
		grandTotalOutput += costs.TotalOutputTokens
		grandTotalCached += costs.TotalCachedTokens
		grandTotalCost += costs.TotalCostUSD

		// Aggregate step totals
		for step, stats := range costs.ByStep {
			if stepTotals[step] == nil {
				stepTotals[step] = &storage.StepCostStats{}
			}
			s := stepTotals[step]
			s.InputTokens += stats.InputTokens
			s.OutputTokens += stats.OutputTokens
			s.CachedTokens += stats.CachedTokens
			s.CostUSD += stats.CostUSD
			s.Calls += stats.Calls
		}
	}

	if taskCount == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	fmt.Printf("Cost Summary for %d task(s)\n", taskCount)
	fmt.Printf("%s\n", strings.Repeat("─", 40))

	totalTokens := grandTotalInput + grandTotalOutput
	cachedPercent := 0.0
	if totalTokens > 0 && grandTotalCached > 0 {
		cachedPercent = float64(grandTotalCached) / float64(totalTokens) * 100
	}

	fmt.Printf("\n%s\n", display.Bold("Grand Totals:"))
	fmt.Printf("  Input tokens:  %s\n", formatNumber(grandTotalInput))
	fmt.Printf("  Output tokens: %s\n", formatNumber(grandTotalOutput))
	fmt.Printf("  Cached tokens: %s", formatNumber(grandTotalCached))
	if grandTotalCached > 0 {
		fmt.Printf(" (%.1f%% of total)", cachedPercent)
	}
	fmt.Println()
	fmt.Printf("  Total tokens:  %s\n", formatNumber(totalTokens))
	fmt.Printf("  Total cost:    %s\n", display.Bold(formatCost(grandTotalCost)))

	if len(stepTotals) > 0 {
		fmt.Printf("\n%s\n", display.Bold("By Step:"))

		// Sort steps by name
		steps := make([]string, 0, len(stepTotals))
		for step := range stepTotals {
			steps = append(steps, step)
		}
		sort.Strings(steps)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  STEP\t\tINPUT\tOUTPUT\tCACHED\tCOST\tCALLS")
		for _, step := range steps {
			stats := stepTotals[step]
			_, _ = fmt.Fprintf(w, "  %s\t\t%s\t%s\t%s\t%s\t%d\n",
				formatStepName(step),
				formatNumber(stats.InputTokens),
				formatNumber(stats.OutputTokens),
				formatNumber(stats.CachedTokens),
				formatCost(stats.CostUSD),
				stats.Calls,
			)
		}
		_ = w.Flush()
	}

	return nil
}

// formatNumber formats a number with thousands separator
func formatNumber(n int) string {
	if n == 0 {
		return "0"
	}

	s := fmt.Sprintf("%d", n)
	var result []byte

	// Add commas from right to left
	for i := len(s) - 1; i >= 0; i-- {
		pos := len(s) - i - 1
		result = append([]byte{s[i]}, result...)
		if pos%3 == 0 && pos != 0 && i != 0 {
			result = append([]byte{','}, result...)
		}
	}

	return string(result)
}

// formatCost formats a cost in USD with 4 decimal places
func formatCost(cost float64) string {
	if cost == 0 {
		return "$0.0000"
	}
	return fmt.Sprintf("$%.4f", cost)
}

// formatStepName formats a step name for display
func formatStepName(step string) string {
	// Capitalize first letter
	if len(step) == 0 {
		return step
	}
	return strings.ToUpper(step[:1]) + step[1:]
}
