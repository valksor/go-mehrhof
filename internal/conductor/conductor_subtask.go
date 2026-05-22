package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/worker"
)

// validSubTaskPhases are the lifecycle phases a sub-task is allowed to run.
var validSubTaskPhases = map[string]bool{
	PhasePlan:      true,
	PhaseImplement: true,
	PhaseSimplify:  true,
	PhaseOptimize:  true,
	PhaseReview:    true,
}

// RunSubTask creates an isolated worktree and runs the specified phases within
// it as a nested phase graph, returning the combined output of all phases. It
// is wired into the graph scheduler via graph.WithSubTaskExecutor, so a graph
// node carrying a SubTaskConfig spawns this isolated lifecycle. The sub-task's
// committed work is preserved on its branch; the worktree directory is removed
// when the sub-task finishes.
func (c *Conductor) RunSubTask(ctx context.Context, config graph.SubTaskConfig) (string, error) {
	if err := validateSubTaskConfig(config); err != nil {
		return "", err
	}

	c.mu.RLock()
	repo := c.git
	cfg := c.getEffectiveSettings()
	var wuID string
	if c.workUnit != nil {
		wuID = c.workUnit.ID
	}
	c.mu.RUnlock()

	if c.pool == nil {
		return "", errors.New("sub-task: worker pool not available")
	}
	if repo == nil {
		return "", errors.New("sub-task: git repository not available")
	}

	// Base the sub-task worktree on the current commit.
	baseSHA, err := repo.CurrentCommit(ctx)
	if err != nil {
		return "", fmt.Errorf("sub-task: resolve base commit: %w", err)
	}

	label := sanitizeBranchLabel(config.Title)
	if label == "" {
		label = "subtask"
	}
	// Cap the label so the derived branch and worktree-directory paths stay
	// within filesystem name limits even for a very long title.
	if len(label) > 40 {
		label = label[:40]
	}

	shortID := uuid.New().String()[:8]

	branchName := config.Branch
	if branchName == "" {
		// Include the short ID so re-running the same task does not collide
		// with a branch left behind by a previous run.
		branchName = fmt.Sprintf("kvelmo-subtask/%s/%s-%s", wuID, label, shortID)
	}
	// Reject a branch name git would parse as a flag (e.g. a leading "-" from a
	// crafted graph definition reaching the `git worktree add -b` argument).
	if strings.HasPrefix(branchName, "-") {
		return "", fmt.Errorf("sub-task: invalid branch name %q", branchName)
	}

	worktreeDir := filepath.Join(
		filepath.Dir(repo.Path()),
		".kvelmo-worktrees",
		fmt.Sprintf("subtask-%s-%s", label, shortID),
	)

	// Create the branch only when it does not already exist; otherwise check out
	// the existing branch into the new worktree (an explicit config.Branch may
	// be reused across runs).
	createBranch := !repo.LocalBranchExists(ctx, branchName)
	if err := repo.AddWorktree(ctx, worktreeDir, branchName, createBranch, baseSHA); err != nil {
		return "", fmt.Errorf("sub-task: create worktree: %w", err)
	}
	// Always tear down the worktree directory; the branch is retained so the
	// sub-task's committed work survives. Use WithoutCancel so cleanup runs
	// even when the parent context was canceled mid-execution.
	defer func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if rmErr := repo.RemoveWorktree(cleanupCtx, worktreeDir, true); rmErr != nil {
			slog.Warn("sub-task: failed to remove worktree", "dir", worktreeDir, "error", rmErr)
		}
		_ = repo.PruneWorktrees(cleanupCtx)
	}()

	// Provision the worktree (copy configs, symlink deps) like a normal task.
	if cfg != nil {
		c.provisionWorktree(ctx, cfg, repo.Path(), worktreeDir)
	}

	c.emit(ConductorEvent{
		Type:      "sub_task_started",
		Message:   "Sub-task started: " + config.Title,
		SubTaskID: branchName,
	})

	// Build a linear phase chain and run it in the isolated worktree. The
	// nested scheduler intentionally has no sub-task executor: a sub-task may
	// not itself spawn sub-tasks, which prevents unbounded recursion.
	g := buildSubTaskGraph(config)
	sched := graph.NewScheduler(g, c.pool)

	opts := graph.JobOpts{
		WorktreeID:  worktreeDir,
		WorkDir:     worktreeDir,
		Environment: map[string]string{},
		Metadata: map[string]any{
			"sub_task":        config.Title,
			"sub_task_branch": branchName,
		},
	}
	for k, v := range config.Metadata {
		opts.Metadata[k] = v
	}

	for evt := range sched.Run(ctx, opts) {
		if evt.Type == graph.EventNodeOutput {
			c.emit(ConductorEvent{
				Type:      "sub_task_output",
				NodeID:    string(evt.NodeID),
				Message:   evt.Content,
				SubTaskID: branchName,
			})
		}
	}

	// Persist the sub-task's file changes onto its branch so removing the
	// worktree does not discard the agent's work. WithoutCancel ensures a
	// canceled parent context does not abandon the commit, and the message
	// records whether the run failed so the retained branch is self-describing.
	failed := sched.State().HasFailures()
	commitMsg := "Sub-task: " + config.Title
	if failed {
		commitMsg = "Sub-task (failed): " + config.Title
	}
	if commitErr := commitSubTaskWork(context.WithoutCancel(ctx), worktreeDir, commitMsg); commitErr != nil {
		slog.Warn("sub-task: failed to commit work", "branch", branchName, "error", commitErr)
	}

	output := aggregateSubTaskOutput(config, sched.State())

	if failed {
		c.emit(ConductorEvent{
			Type:      "sub_task_failed",
			Message:   "Sub-task failed: " + config.Title,
			SubTaskID: branchName,
		})

		return output, fmt.Errorf("sub-task %q failed during execution", config.Title)
	}

	c.emit(ConductorEvent{
		Type:      "sub_task_completed",
		Message:   "Sub-task completed: " + config.Title,
		SubTaskID: branchName,
	})

	slog.Info("sub-task completed", "title", config.Title, "branch", branchName, "phases", config.Phases)

	return output, nil
}

// validateSubTaskConfig rejects empty or unknown-phase sub-tasks.
func validateSubTaskConfig(config graph.SubTaskConfig) error {
	if len(config.Phases) == 0 {
		return errors.New("sub-task: no phases specified")
	}
	for _, p := range config.Phases {
		if !validSubTaskPhases[p] {
			return fmt.Errorf("sub-task: unknown phase %q", p)
		}
	}

	return nil
}

// subTaskNodeID builds the deterministic node ID for a sub-task phase.
func subTaskNodeID(phase string, index int) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("%s-%d", phase, index))
}

// buildSubTaskGraph builds a linear chain of phase nodes from the config, each
// depending on the previous so the phases run in order.
func buildSubTaskGraph(config graph.SubTaskConfig) *graph.Graph {
	g := graph.New()

	var prev graph.NodeID
	for i, phase := range config.Phases {
		id := subTaskNodeID(phase, i)
		node := &graph.Node{
			ID:      id,
			Label:   phase,
			JobType: worker.JobType(phase),
			Prompt:  buildSubTaskPhasePrompt(config, phase),
		}
		if prev != "" {
			node.DependsOn = []graph.NodeID{prev}
		}
		_ = g.AddNode(node)
		prev = id
	}

	return g
}

// buildSubTaskPhasePrompt builds a self-contained prompt for a sub-task phase.
func buildSubTaskPhasePrompt(config graph.SubTaskConfig, phase string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Sub-task: %s\n", config.Title)
	if config.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", config.Description)
	}
	fmt.Fprintf(&b, "\nPhase: %s\n%s\n", phase, subTaskPhaseInstruction(phase))

	return b.String()
}

// subTaskPhaseInstruction returns the instruction line for a sub-task phase.
func subTaskPhaseInstruction(phase string) string {
	switch phase {
	case PhasePlan:
		return "Write a focused specification for this sub-task."
	case PhaseImplement:
		return "Implement the sub-task according to its specification."
	case PhaseSimplify:
		return "Simplify the implementation without changing its behavior."
	case PhaseOptimize:
		return "Improve the quality and performance of the implementation."
	case PhaseReview:
		return "Review the implementation and report any issues."
	default:
		return "Run the requested phase."
	}
}

// aggregateSubTaskOutput concatenates phase results in execution order.
func aggregateSubTaskOutput(config graph.SubTaskConfig, state *graph.StateManager) string {
	results := state.Results()

	var b strings.Builder
	for i, phase := range config.Phases {
		out := results[subTaskNodeID(phase, i)]
		if out == "" {
			continue
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", phase, out)
	}

	return strings.TrimSpace(b.String())
}

// commitSubTaskWork commits any uncommitted changes in the sub-task worktree to
// its branch so they are not lost when the worktree directory is removed.
func commitSubTaskWork(ctx context.Context, worktreeDir, message string) error {
	repo, err := git.Open(worktreeDir)
	if err != nil {
		return fmt.Errorf("open worktree repo: %w", err)
	}

	dirty, err := repo.HasUncommittedChanges(ctx)
	if err != nil {
		return fmt.Errorf("check changes: %w", err)
	}
	if !dirty {
		return nil
	}

	if err := repo.StageAll(ctx); err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	if _, err := repo.Commit(ctx, message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
