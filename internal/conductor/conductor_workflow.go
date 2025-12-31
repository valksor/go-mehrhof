package conductor

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// Plan enters the planning phase to create specifications
func (c *Conductor) Plan(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Update state
	c.activeTask.State = "planning"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	// Dispatch planning event
	if err := c.machine.Dispatch(ctx, workflow.EventPlan); err != nil {
		return fmt.Errorf("enter planning: %w", err)
	}

	return nil
}

// Implement enters the implementation phase
func (c *Conductor) Implement(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Check for specifications
	specifications, err := c.workspace.ListSpecifications(c.activeTask.ID)
	if err != nil {
		return fmt.Errorf("list specifications: %w", err)
	}
	if len(specifications) == 0 {
		return fmt.Errorf("no specifications found - run 'task plan' first")
	}

	// Update machine with specifications
	wu := c.machine.WorkUnit()
	if wu != nil {
		wu.Specifications = make([]string, len(specifications))
		for i, num := range specifications {
			wu.Specifications[i] = fmt.Sprintf("specification-%d.md", num)
		}
	}

	// Update state
	c.activeTask.State = "implementing"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	// Dispatch implement event
	if err := c.machine.Dispatch(ctx, workflow.EventImplement); err != nil {
		return fmt.Errorf("enter implementation: %w", err)
	}

	return nil
}

// Review enters the review phase
func (c *Conductor) Review(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Update state
	c.activeTask.State = "reviewing"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	// Dispatch review event
	if err := c.machine.Dispatch(ctx, workflow.EventReview); err != nil {
		return fmt.Errorf("enter review: %w", err)
	}

	return nil
}

// Undo reverts to the previous checkpoint
func (c *Conductor) Undo(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	if c.git == nil {
		return fmt.Errorf("git not available")
	}

	taskID := c.activeTask.ID

	// Check if undo is possible
	can, err := c.git.CanUndo(taskID)
	if err != nil {
		return err
	}
	if !can {
		return fmt.Errorf("nothing to undo")
	}

	// Dispatch undo event
	if err := c.machine.Dispatch(ctx, workflow.EventUndo); err != nil {
		return fmt.Errorf("undo workflow: %w", err)
	}

	// Perform git undo
	checkpoint, err := c.git.Undo(taskID)
	if err != nil {
		return fmt.Errorf("git undo: %w", err)
	}

	// Publish event
	c.eventBus.PublishRaw(events.Event{
		Type: events.TypeCheckpoint,
		Data: map[string]any{
			"action":     "undo",
			"checkpoint": checkpoint.Number,
			"commit":     checkpoint.ID,
		},
	})

	// Complete undo
	_ = c.machine.Dispatch(ctx, workflow.EventUndoDone)

	return nil
}

// Redo moves forward to the next checkpoint
func (c *Conductor) Redo(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	if c.git == nil {
		return fmt.Errorf("git not available")
	}

	taskID := c.activeTask.ID

	// Check if redo is possible
	can, err := c.git.CanRedo(taskID)
	if err != nil {
		return err
	}
	if !can {
		return fmt.Errorf("nothing to redo")
	}

	// Dispatch redo event
	if err := c.machine.Dispatch(ctx, workflow.EventRedo); err != nil {
		return fmt.Errorf("redo workflow: %w", err)
	}

	// Perform git redo
	checkpoint, err := c.git.Redo(taskID)
	if err != nil {
		return fmt.Errorf("git redo: %w", err)
	}

	// Publish event
	c.eventBus.PublishRaw(events.Event{
		Type: events.TypeCheckpoint,
		Data: map[string]any{
			"action":     "redo",
			"checkpoint": checkpoint.Number,
			"commit":     checkpoint.ID,
		},
	})

	// Complete redo
	_ = c.machine.Dispatch(ctx, workflow.EventRedoDone)

	return nil
}

// Finish completes the task
func (c *Conductor) Finish(ctx context.Context, opts FinishOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Determine action based on flags and provider support
	if opts.ForceMerge {
		// User explicitly requested local merge
		if err := c.finishWithMerge(ctx, opts); err != nil {
			return err
		}
	} else if c.providerSupportsPR(ctx) {
		// Provider supports PR, create one by default
		prResult, err := c.finishWithPR(ctx, opts)
		if err != nil {
			return err
		}
		// Store PR info for later reference
		if prResult != nil {
			c.logVerbose("Created PR #%d: %s", prResult.Number, prResult.URL)
		}
	} else if c.git != nil && c.activeTask.UseGit && c.activeTask.Branch != "" {
		// Provider doesn't support PR, ask user what to do
		action, err := c.askUserFinishAction()
		if err != nil {
			return err
		}
		switch action {
		case "merge":
			if err := c.finishWithMerge(ctx, opts); err != nil {
				return err
			}
		case "done":
			// Just mark as done, no merge
			c.logVerbose("Marking task as done without merging")
		case "cancel":
			return fmt.Errorf("cancelled by user")
		}
	} else {
		// No git, just mark as done
		c.logVerbose("No git branch associated, marking task as done")
	}

	// Update state
	c.activeTask.State = "done"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		c.logError(fmt.Errorf("save active task: %w", err))
	}

	// Dispatch finish event
	if err := c.machine.Dispatch(ctx, workflow.EventFinish); err != nil {
		return fmt.Errorf("finish workflow: %w", err)
	}

	// Clear active task
	if err := c.workspace.ClearActiveTask(); err != nil {
		c.logError(fmt.Errorf("clear active task: %w", err))
	}

	// Delete work directory based on: CLI flag > config > default (keep)
	shouldDelete := false
	if opts.DeleteWork != nil {
		shouldDelete = *opts.DeleteWork // CLI explicitly set
	} else {
		cfg, _ := c.workspace.LoadConfig() // ignore error, use defaults
		shouldDelete = cfg.Workflow.DeleteWorkOnFinish
	}
	if shouldDelete {
		taskID := c.activeTask.ID
		if err := c.workspace.DeleteWork(taskID); err != nil {
			c.logError(fmt.Errorf("delete work directory: %w", err))
		}
	}

	c.activeTask = nil
	c.taskWork = nil

	return nil
}

// buildWorkUnit creates a workflow.WorkUnit from current state
func (c *Conductor) buildWorkUnit() *workflow.WorkUnit {
	if c.taskWork == nil {
		return nil
	}

	wu := &workflow.WorkUnit{
		ID:         c.taskWork.Metadata.ID,
		ExternalID: c.taskWork.Source.Ref,
		Title:      c.taskWork.Metadata.Title,
		Source: &workflow.Source{
			Reference: c.taskWork.Source.Ref,
			Content:   c.taskWork.Source.Content,
		},
	}

	// Add specifications if any - errors ignored; empty list is acceptable for WorkUnit
	specifications, _ := c.workspace.ListSpecifications(c.taskWork.Metadata.ID)
	for _, num := range specifications {
		wu.Specifications = append(wu.Specifications, fmt.Sprintf("specification-%d.md", num))
	}

	return wu
}

// onStateChanged handles state change events
func (c *Conductor) onStateChanged(e events.Event) {
	if c.opts.OnStateChange == nil {
		return
	}

	from, ok := e.Data["from"].(string)
	if !ok {
		from = ""
	}
	to, ok := e.Data["to"].(string)
	if !ok {
		to = ""
	}
	c.opts.OnStateChange(from, to)
}

// countCheckpoints returns the number of checkpoints for current task
func (c *Conductor) countCheckpoints() int {
	if c.activeTask == nil || c.git == nil {
		return 0
	}
	checkpoints, err := c.git.ListCheckpoints(c.activeTask.ID)
	if err != nil {
		return 0
	}
	return len(checkpoints)
}

// publishProgress publishes a progress event
func (c *Conductor) publishProgress(message string, percent int) {
	c.eventBus.PublishRaw(events.Event{
		Type: events.TypeProgress,
		Data: map[string]any{
			"message": message,
			"percent": percent,
		},
	})

	if c.opts.OnProgress != nil {
		c.opts.OnProgress(message, percent)
	}
}

// finishWithMerge performs a local merge operation
func (c *Conductor) finishWithMerge(ctx context.Context, opts FinishOptions) error {
	// Handle git merge operations if applicable
	if c.git == nil || !c.activeTask.UseGit || c.activeTask.Branch == "" {
		return fmt.Errorf("git not available or no branch associated with task")
	}

	if err := c.performMerge(opts); err != nil {
		return err
	}

	// Push if requested
	if opts.PushAfter {
		targetBranch := c.resolveTargetBranch(opts.TargetBranch)
		if err := c.git.PushBranch(targetBranch, "origin", false); err != nil {
			return fmt.Errorf("push: %w", err)
		}
	}

	// Cleanup branch and worktree if requested
	c.cleanupAfterMerge(opts)
	return nil
}

// providerSupportsPR checks if the current task's provider supports PR creation
func (c *Conductor) providerSupportsPR(ctx context.Context) bool {
	if c.activeTask == nil || c.activeTask.Ref == "" {
		return false
	}

	// Resolve provider from the stored reference
	resolveOpts := provider.ResolveOptions{
		DefaultProvider: c.opts.DefaultProvider,
	}
	p, _, err := c.providers.Resolve(ctx, c.activeTask.Ref, provider.Config{}, resolveOpts)
	if err != nil {
		return false
	}

	// Check if provider implements PRCreator interface
	_, ok := p.(provider.PRCreator)
	return ok
}

// askUserFinishAction prompts the user to choose an action when PR is not supported
func (c *Conductor) askUserFinishAction() (string, error) {
	// For non-interactive use (auto mode), default to "done"
	if c.opts.AutoMode || c.opts.SkipAgentQuestions {
		return "done", nil
	}

	fmt.Println("\nThe provider for this task does not support pull requests.")
	fmt.Println("What would you like to do?")
	fmt.Println("  1. Merge changes to target branch locally")
	fmt.Println("  2. Mark task as done (no merge)")
	fmt.Println("  3. Cancel")

	for {
		var choice string
		fmt.Print("\nEnter choice (1-3): ")
		if _, err := fmt.Scanln(&choice); err != nil {
			// Handle EOF or empty input
			fmt.Println("\nCancelled")
			return "cancel", nil
		}

		switch choice {
		case "1", "merge":
			return "merge", nil
		case "2", "done":
			return "done", nil
		case "3", "cancel", "q":
			return "cancel", nil
		default:
			fmt.Println("Invalid choice. Please enter 1, 2, or 3.")
		}
	}
}
