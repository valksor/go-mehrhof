package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/worker"
)

// Review begins the review phase.
// When fix=true, immediately submits a new implement job to auto-fix issues.
func (c *Conductor) Review(ctx context.Context, fix bool) error {
	c.mu.Lock()

	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.mu.Unlock()
		c.emitEnrichedError(err, "review")

		return err
	}

	// Check pool BEFORE transitioning state when fix mode is requested
	if fix && c.pool == nil {
		err := errors.New("no worker pool available")
		c.mu.Unlock()
		c.emitEnrichedError(err, "review")

		return err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventReview); err != nil {
		// Review() does NOT use defer c.mu.Unlock() — explicit lock management.
		// No re-lock needed here.
		c.emitEnrichedError(err, "review")

		return err
	}
	c.mu.Lock()

	// Record prior stable state for error/stop rollback during re-entry
	// (e.g., review from Submitted should stop back to Submitted, not Implemented).
	currentState := c.machine.State()
	if currentState != StateImplemented {
		c.machine.SetPriorStableState(currentState)
	}

	if err := c.machine.Dispatch(ctx, EventReview); err != nil {
		c.machine.ClearPriorStableState()
		wrapped := fmt.Errorf("cannot review: %w", err)
		c.mu.Unlock()
		c.emitEnrichedError(wrapped, "review")

		return wrapped
	}

	c.persistState()

	c.emit(ConductorEvent{
		Type:    "review_started",
		State:   c.machine.State(),
		Message: "Review started",
	})

	// Capture values we need for the potentially blocking Submit call
	workDir := c.getWorkDir()
	lifecycleCtx := c.lifecycleCtx
	pool := c.pool

	c.mu.Unlock() // Release lock before potentially blocking call

	// If fix mode: immediately submit an implement job to fix issues
	// Pool nil check already done above before state transition
	if fix {
		fixPrompt := `Review the changes and fix any issues you find. Focus on correctness, error handling, and code quality.
Go through each file that was modified and ensure:
1. All error cases are handled properly
2. The logic is correct and complete
3. Code follows project conventions
4. No obvious bugs or edge cases are missed
Commit your fixes with meaningful commit messages.`

		job, err := pool.Submit(worker.JobTypeImplement, workDir, fixPrompt)
		if err != nil {
			c.mu.Lock()
			if dispatchErr := c.machine.Dispatch(ctx, EventError); dispatchErr != nil {
				slog.Warn("failed to dispatch error event during rollback", "err", dispatchErr)
			}
			c.mu.Unlock()

			return fmt.Errorf("submit review-fix job: %w", err)
		}

		c.mu.Lock()
		c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
		c.workUnit.UpdatedAt = time.Now()
		c.persistState()

		c.emit(ConductorEvent{
			Type:    "review_fix_started",
			State:   c.machine.State(),
			JobID:   job.ID,
			Message: "Review fix job started",
		})
		c.mu.Unlock()

		// Watch job completion using lifecycle context
		// (not request ctx which may be cancelled when handler returns)
		go c.watchJob(lifecycleCtx, job.ID, EventImplementDone) //nolint:contextcheck // intentionally uses lifecycle context
	}

	// Start quality gate in background so result is ready for Submit()
	// This runs the lint/vet/typecheck checks asynchronously, avoiding
	// the 60-second blocking wait in Submit().
	c.runQualityGateAsync()

	return nil
}

// AddReview records a review result and persists it to disk.
// The review number is derived from disk (ReviewStore) so it remains correct across restarts.
func (c *Conductor) AddReview(approved bool, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return
	}
	if c.store == nil {
		return
	}

	c.workUnit.UpdatedAt = time.Now()

	reviStore := storage.NewReviewStore(c.store)

	number, err := reviStore.NextReviewNumber(c.workUnit.ID)
	if err != nil {
		slog.Warn("next review number failed", "task_id", c.workUnit.ID, "error", err)

		return
	}

	status := "rejected"
	if approved {
		status = "approved"
	}
	reviewContent := fmt.Sprintf("---\nstatus: %s\ncreated_at: %s\n---\n\n# Review %d\n\n**Status**: %s\n\n%s\n",
		status, time.Now().Format(time.RFC3339), number, status, message)
	if err := reviStore.SaveReview(c.workUnit.ID, number, reviewContent); err != nil {
		slog.Warn("persist review failed", "task_id", c.workUnit.ID, "number", number, "error", err)
	}

	c.persistState()
}

// ListReviews returns all persisted reviews for the current task, read from disk.
func (c *Conductor) ListReviews() ([]storage.Review, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.workUnit == nil {
		return nil, errors.New("no task loaded")
	}
	if c.store == nil {
		return nil, errors.New("no store configured")
	}

	reviStore := storage.NewReviewStore(c.store)
	numbers, err := reviStore.ListReviews(c.workUnit.ID)
	if err != nil {
		return nil, err
	}

	reviews := make([]storage.Review, 0, len(numbers))
	for _, num := range numbers {
		r, err := reviStore.ParseReview(c.workUnit.ID, num)
		if err != nil {
			slog.Warn("failed to parse review", "workUnitID", c.workUnit.ID, "reviewNum", num, "error", err)

			continue
		}
		reviews = append(reviews, *r)
	}

	return reviews, nil
}

// GetReview returns a single review by number, read from disk.
func (c *Conductor) GetReview(number int) (*storage.Review, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.workUnit == nil {
		return nil, errors.New("no task loaded")
	}
	if c.store == nil {
		return nil, errors.New("no store configured")
	}

	reviStore := storage.NewReviewStore(c.store)

	return reviStore.ParseReview(c.workUnit.ID, number)
}
