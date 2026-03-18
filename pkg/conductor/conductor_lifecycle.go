package conductor

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// UpdateTask re-fetches the task from its provider and generates a delta specification if changed.
// Returns whether the task changed, and the path to the new specification file (if any).
func (c *Conductor) UpdateTask(ctx context.Context) (bool, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return false, "", errors.New("no task loaded")
	}

	if c.workUnit.Source == nil {
		return false, "", errors.New("work unit has no source")
	}

	// Re-fetch from provider
	task, fetchErr := c.providers.Fetch(ctx, c.workUnit.Source.Provider, c.workUnit.Source.Reference)
	if fetchErr != nil {
		return false, "", fmt.Errorf("fetch task: %w", fetchErr)
	}

	oldContent := c.workUnit.Description
	newContent := task.Description

	// Check if content changed
	if oldContent == newContent {
		return false, "", nil
	}

	// Generate delta specification
	deltaPath, deltaErr := c.GenerateDeltaSpecification(ctx, oldContent, newContent)
	if deltaErr != nil {
		return true, "", fmt.Errorf("generate delta specification: %w", deltaErr)
	}

	// Update work unit with new content
	c.workUnit.Description = newContent
	c.workUnit.Title = task.Title
	c.workUnit.Source.Content = newContent
	c.workUnit.Specifications = append(c.workUnit.Specifications, deltaPath)
	c.workUnit.UpdatedAt = time.Now()
	c.machine.SetWorkUnit(c.workUnit)
	c.persistState()

	c.emit(ConductorEvent{
		Type:    "task_updated",
		State:   c.machine.State(),
		Message: "Task updated, delta specification: " + deltaPath,
	})

	return true, deltaPath, nil
}

// Undo reverts to the previous checkpoint.
func (c *Conductor) Undo(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	// Need at least 2 checkpoints: one to save for redo, one to reset to
	if len(c.workUnit.Checkpoints) < 2 {
		return errors.New("no checkpoints to undo (need at least 2)")
	}

	// Determine checkpoints before any mutations
	currentCheckpoint := c.workUnit.Checkpoints[len(c.workUnit.Checkpoints)-1]
	targetCheckpoint := c.workUnit.Checkpoints[len(c.workUnit.Checkpoints)-2]

	// Perform git reset FIRST to avoid state/git divergence
	if c.git != nil {
		if err := c.git.Reset(ctx, targetCheckpoint, true); err != nil {
			return fmt.Errorf("git reset: %w", err)
		}
	}

	// Transition state machine AFTER git succeeds to keep them in sync
	if err := c.machine.Dispatch(ctx, EventUndo); err != nil {
		return fmt.Errorf("cannot undo: %w", err)
	}

	// Only mutate in-memory state after git and state machine succeed
	c.workUnit.Checkpoints = c.workUnit.Checkpoints[:len(c.workUnit.Checkpoints)-1]
	c.workUnit.RedoStack = append(c.workUnit.RedoStack, currentCheckpoint)

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "undo_completed",
		State:   c.machine.State(),
		Message: "Reverted to checkpoint " + truncateSHA(targetCheckpoint, 8),
	})

	return nil
}

// Redo restores to the next checkpoint.
func (c *Conductor) Redo(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	if len(c.workUnit.RedoStack) == 0 {
		return errors.New("no checkpoints to redo")
	}

	// Get checkpoint to restore before any mutations
	checkpoint := c.workUnit.RedoStack[len(c.workUnit.RedoStack)-1]

	// Perform git reset FIRST to avoid state/git divergence
	if c.git != nil {
		if err := c.git.Reset(ctx, checkpoint, true); err != nil {
			return fmt.Errorf("git reset: %w", err)
		}
	}

	// Transition state machine AFTER git succeeds to keep them in sync
	if err := c.machine.Dispatch(ctx, EventRedo); err != nil {
		return fmt.Errorf("cannot redo: %w", err)
	}

	// Only mutate in-memory state after git and state machine succeed
	c.workUnit.RedoStack = c.workUnit.RedoStack[:len(c.workUnit.RedoStack)-1]
	c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, checkpoint)

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "redo_completed",
		State:   c.machine.State(),
		Message: "Restored to checkpoint " + truncateSHA(checkpoint, 8),
	})

	return nil
}

// GotoCheckpoint resets the repo to a specific checkpoint SHA and restructures
// the checkpoint/redo stacks so undo/redo remains consistent.
// Newer checkpoints (those after the target) are moved to the redo stack.
func (c *Conductor) GotoCheckpoint(ctx context.Context, sha string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	idx := -1
	for i, s := range c.workUnit.Checkpoints {
		if s == sha {
			idx = i

			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("checkpoint %s not found", truncateSHA(sha, 8))
	}

	// Identify newer checkpoints to move to redo stack
	newer := c.workUnit.Checkpoints[idx+1:]

	// Perform git reset FIRST to avoid state/git divergence
	if c.git != nil {
		if err := c.git.Reset(ctx, sha, true); err != nil {
			return fmt.Errorf("git reset: %w", err)
		}
	}

	// Dispatch state machine event AFTER git succeeds to keep them in sync
	// (consistent with Undo/Redo behavior). Ignore errors if undo isn't
	// allowed from the current state - checkpoint manipulation still proceeds.
	if canUndo, _ := c.machine.CanDispatch(ctx, EventUndo); canUndo {
		_ = c.machine.Dispatch(ctx, EventUndo)
	}

	// Only mutate in-memory state after git succeeds
	// Newer checkpoints move to redo stack in reverse order so redo restores them in sequence
	for i := len(newer) - 1; i >= 0; i-- {
		c.workUnit.RedoStack = append(c.workUnit.RedoStack, newer[i])
	}
	c.workUnit.Checkpoints = c.workUnit.Checkpoints[:idx+1]

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "checkpoint_goto",
		State:   c.machine.State(),
		Message: "Restored to checkpoint " + truncateSHA(sha, 8),
	})

	return nil
}

// Stop gracefully stops the current operation and returns to the previous stable state.
// Unlike Abort (which transitions to Failed), Stop allows the user to continue
// from a recoverable state:
//   - Planning → Loaded (can re-plan)
//   - Implementing → Planned (can re-implement)
//   - Simplifying → Implemented (can re-simplify)
//   - Optimizing → Implemented (can re-optimize)
func (c *Conductor) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we can dispatch stop from current state
	canStop, _ := c.machine.CanDispatch(ctx, EventStop)
	if !canStop {
		return errors.New("cannot stop: not in a stoppable state")
	}

	// Interrupt the agent first (sends protocol-level interrupt)
	if c.pool != nil && c.activeJobID != "" {
		_ = c.pool.InterruptJob(c.activeJobID)
	}

	// Cancel the job (stops worker, marks job as failed)
	if c.pool != nil && c.activeJobID != "" {
		_ = c.pool.CancelJob(c.activeJobID)
	}

	// Clear active job
	jobID := c.activeJobID
	c.activeJobID = ""

	// Record cancellation reason
	if c.workUnit != nil {
		c.workUnit.CancelledBy = "user"
		c.workUnit.CancelledAt = time.Now()
	}

	// Transition to previous stable state
	if err := c.machine.Dispatch(ctx, EventStop); err != nil {
		return fmt.Errorf("cannot stop: %w", err)
	}

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "task_stopped",
		State:   c.machine.State(),
		JobID:   jobID,
		Message: "Operation stopped, returned to " + string(c.machine.State()),
	})

	return nil
}

// Abort aborts the current task.
func (c *Conductor) Abort(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.machine.Dispatch(ctx, EventAbort); err != nil {
		return fmt.Errorf("cannot abort: %w", err)
	}

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "task_aborted",
		State:   c.machine.State(),
		Message: "Task aborted",
	})

	return nil
}

// Reset resets from failed state.
func (c *Conductor) Reset(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.machine.State() != StateFailed {
		return errors.New("can only reset from failed state")
	}

	if err := c.machine.Dispatch(ctx, EventReset); err != nil {
		return fmt.Errorf("cannot reset: %w", err)
	}

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "task_reset",
		State:   c.machine.State(),
		Message: "Task reset to loaded state",
	})

	return nil
}

// CreateCheckpoint creates a git checkpoint.
func (c *Conductor) CreateCheckpoint(ctx context.Context, message string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.git == nil {
		return "", errors.New("git not available")
	}

	if c.workUnit == nil {
		return "", errors.New("no task loaded")
	}

	sha, err := c.git.Commit(ctx, message)
	if err != nil {
		return "", fmt.Errorf("create checkpoint: %w", err)
	}

	c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
	c.workUnit.RedoStack = nil // Clear redo stack on new checkpoint
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	c.emit(ConductorEvent{
		Type:    "checkpoint_created",
		State:   c.machine.State(),
		Message: "Checkpoint created: " + truncateSHA(sha, 8),
	})

	return sha, nil
}

// truncateSHA safely truncates a SHA hash to the specified length.
// Returns the full string if it's shorter than n characters.
//

func truncateSHA(sha string, n int) string {
	if len(sha) < n {
		return sha
	}

	return sha[:n]
}
