package conductor

import (
	"context"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/events"
)

// createCheckpointIfNeeded creates a git checkpoint if there are changes.
func (c *Conductor) createCheckpointIfNeeded(ctx context.Context, taskID, message string) *events.Event {
	if c.git == nil || !c.activeTask.UseGit {
		return nil
	}

	hasChanges, err := c.git.HasChanges(ctx)
	if err != nil {
		// If we can't determine changes, log but continue (treat as no changes)
		// This allows checkpoint creation to fail gracefully
		c.publishProgress(fmt.Sprintf("Warning: could not check git changes: %v", err), 0)

		return nil
	}
	if !hasChanges {
		return nil
	}

	// Use stored commit prefix, fallback to default [taskID] format
	commitPrefix := ""
	if c.taskWork != nil {
		commitPrefix = c.taskWork.Git.CommitPrefix
	}
	if commitPrefix == "" {
		commitPrefix = fmt.Sprintf("[%s]", taskID)
	}

	checkpoint, err := c.git.CreateCheckpointWithPrefix(ctx, taskID, message, commitPrefix)
	if err != nil {
		c.logError(fmt.Errorf("create checkpoint: %w", err))

		return nil
	}

	return &events.Event{
		Type: events.TypeCheckpoint,
		Data: map[string]any{
			"action":     "create",
			"checkpoint": checkpoint.Number,
			"commit":     checkpoint.ID,
		},
	}
}

// saveCurrentSession saves the current session if one exists.
func (c *Conductor) saveCurrentSession(taskID string) {
	if c.currentSession == nil || c.currentSessionFile == "" {
		return
	}

	// Set end time
	c.currentSession.Metadata.EndedAt = time.Now()

	// Save session
	if err := c.workspace.SaveSession(taskID, c.currentSessionFile, c.currentSession); err != nil {
		c.logError(fmt.Errorf("save session: %w", err))
	}

	// Clear current session
	c.currentSession = nil
	c.currentSessionFile = ""
}
