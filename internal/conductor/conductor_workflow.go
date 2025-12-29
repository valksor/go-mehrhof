package conductor

import (
	"context"
	"fmt"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
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

// Chat enters dialogue mode to add notes
func (c *Conductor) Chat(ctx context.Context, message string, opts ChatOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	taskID := c.activeTask.ID

	// Dispatch dialogue start
	if err := c.machine.Dispatch(ctx, workflow.EventDialogueStart); err != nil {
		return fmt.Errorf("enter chat mode: %w", err)
	}

	// Get agent for dialogue step
	dialogueAgent, err := c.GetAgentForStep(workflow.StepDialogue)
	if err != nil {
		_ = c.machine.Dispatch(ctx, workflow.EventDialogueEnd)
		return fmt.Errorf("get dialogue agent: %w", err)
	}

	// Build context-aware prompt for chat mode
	// This ensures Claude has full awareness of the task when answering
	sourceContent, notes, specs, pendingQ := c.readOptionalWorkspaceData(taskID)

	prompt := buildChatPrompt(c.taskWork.Metadata.Title, sourceContent, notes, specs, pendingQ, message)

	// Run agent with context-aware prompt
	response, err := dialogueAgent.Run(ctx, prompt)
	if err != nil {
		// End dialogue even on error
		_ = c.machine.Dispatch(ctx, workflow.EventDialogueEnd)
		return fmt.Errorf("agent run: %w", err)
	}

	// Save response as note
	noteContent := response.Summary
	if noteContent == "" && len(response.Messages) > 0 {
		noteContent = response.Messages[0]
	}
	if noteContent != "" {
		if err := c.workspace.AppendNote(taskID, noteContent, c.activeTask.State); err != nil {
			c.logError(fmt.Errorf("append note: %w", err))
		}
	}

	// Apply file changes if not dry-run
	if !c.opts.DryRun && len(response.Files) > 0 {
		if err := c.applyFileChanges(ctx, response.Files); err != nil {
			c.logError(fmt.Errorf("apply changes: %w", err))
		}
	}

	// Clear pending question if it existed (user has answered via chat)
	if c.workspace.HasPendingQuestion(taskID) {
		_ = c.workspace.ClearPendingQuestion(taskID)
	}

	// Return to previous state
	if err := c.machine.Dispatch(ctx, workflow.EventDialogueEnd); err != nil {
		return fmt.Errorf("exit chat mode: %w", err)
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

	// Handle PR creation if requested
	if opts.CreatePR {
		prResult, err := c.finishWithPR(ctx, opts)
		if err != nil {
			return err
		}
		// Store PR info for later reference
		if prResult != nil {
			c.logVerbose("Created PR #%d: %s", prResult.Number, prResult.URL)
		}
	} else if c.git != nil && c.activeTask.UseGit && c.activeTask.Branch != "" {
		// Handle git merge operations if applicable
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

		// Cleanup branch and worktree
		c.cleanupAfterMerge(opts)
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

// applyFileChanges applies agent file changes to disk
func (c *Conductor) applyFileChanges(ctx context.Context, files []agent.FileChange) error {
	return applyFiles(ctx, c, files)
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
