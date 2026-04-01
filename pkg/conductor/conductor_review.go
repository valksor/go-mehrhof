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
//
//nolint:contextcheck // goroutines intentionally use lifecycleCtx to outlive the RPC request context
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

		var job *worker.Job
		var err error
		if cached, ok := c.lookupResponseCache(fixPrompt); ok {
			job, err = pool.SubmitCached(worker.JobTypeImplement, workDir, fixPrompt, cached, nil)
		} else {
			job, err = pool.Submit(worker.JobTypeImplement, workDir, fixPrompt)
		}
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

	// Run consensus review if configured (non-blocking, results logged).
	if cfg := c.getConsensusConfig(); cfg != nil && cfg.Enabled && len(cfg.Agents) > 0 {
		go func() { //nolint:contextcheck // intentionally uses lifecycle context that outlives the request
			reviewPrompt := "Review the code changes for correctness, error handling, and code quality. Report each finding with file path, line number, and description."

			consensusFindings, err := c.runConsensusReview(lifecycleCtx, reviewPrompt)
			if err != nil {
				slog.Warn("consensus review failed", "error", err)

				return
			}

			c.emit(ConductorEvent{
				Type:    "consensus_review_complete",
				State:   c.machine.State(),
				Message: fmt.Sprintf("Consensus review found %d findings", len(consensusFindings)),
			})
		}()
	}

	// Run adversarial review if configured (non-blocking, results stored).
	if cfg := c.getAdversarialConfig(); cfg != nil && cfg.Enabled && len(cfg.Personas) > 0 {
		go func() { //nolint:contextcheck // intentionally uses lifecycle context that outlives the request
			adversarialFindings, err := c.runAdversarialReview(lifecycleCtx)
			if err != nil {
				slog.Warn("adversarial review failed", "error", err)

				return
			}

			c.mu.Lock()
			c.adversarialFindings = adversarialFindings
			c.mu.Unlock()

			if cfg.BlockOnFindings && len(adversarialFindings) > 0 {
				c.emit(ConductorEvent{
					Type:    "adversarial_review_blocked",
					State:   c.machine.State(),
					Message: fmt.Sprintf("Adversarial review found %d findings that require attention", len(adversarialFindings)),
				})
			}
		}()
	}

	// Run spec alignment check in background if specifications exist.
	// Compares the diff against the spec to detect drift or missing items.
	go c.runSpecAlignmentCheckAsync(lifecycleCtx, workDir, pool)

	// Start quality gate in background so result is ready for Submit()
	// This runs the lint/vet/typecheck checks asynchronously, avoiding
	// the 60-second blocking wait in Submit().
	c.runQualityGateAsync()

	return nil
}

// runSpecAlignmentCheckAsync submits a background review job that compares
// the implementation diff against the specification to detect drift.
func (c *Conductor) runSpecAlignmentCheckAsync(lifecycleCtx context.Context, workDir string, pool *worker.Pool) {
	c.mu.RLock()
	if c.workUnit == nil || c.store == nil || pool == nil {
		c.mu.RUnlock()

		return
	}
	taskID := c.workUnit.ID
	store := c.store
	c.mu.RUnlock()

	specStore := storage.NewSpecStore(store)
	specContent, _, err := specStore.GetLatestSpecificationContent(taskID)
	if err != nil || specContent == "" {
		return // No specs to compare against
	}

	// Truncate spec to avoid overwhelming the agent prompt
	const maxSpecLen = 20000
	specRunes := []rune(specContent)
	if len(specRunes) > maxSpecLen {
		specContent = string(specRunes[:maxSpecLen]) + "\n\n…(specification truncated)"
	}

	// Get the diff against base branch
	var diff string
	if c.git != nil {
		if base, bErr := c.getBaseBranch(lifecycleCtx); bErr == nil && base != "" {
			if d, dErr := c.git.DiffAgainst(lifecycleCtx, "origin/"+base, false); dErr == nil {
				diff = d
			}
		}
	}
	if diff == "" {
		return // No diff available to compare
	}

	// Truncate diff to avoid overwhelming the agent (rune-safe)
	const maxDiffLen = 50000
	diffRunes := []rune(diff)
	if len(diffRunes) > maxDiffLen {
		diff = string(diffRunes[:maxDiffLen]) + "\n\n…(diff truncated)"
	}

	prompt := fmt.Sprintf(`Compare the implementation changes against the specification below and verify alignment.

## Specification

%s

## Changes (diff)

%s

## Instructions

1. Check each requirement in the specification against the diff
2. Report any specification items that were NOT implemented
3. Report any changes that were NOT in the specification (scope creep)
4. Report any deviations from the specified approach
5. Write a structured review with pass/fail per spec item

Output your review as markdown. Start with a summary line: "Spec alignment: X of Y items implemented" followed by details.`, specContent, diff)

	var job *worker.Job
	if cached, ok := c.lookupResponseCache(prompt); ok {
		j, submitErr := pool.SubmitCached(worker.JobTypeReview, workDir, prompt, cached, nil)
		if submitErr != nil {
			slog.Warn("spec alignment check failed to submit (cached)", "error", submitErr)

			return
		}
		job = j
	} else {
		j, submitErr := pool.Submit(worker.JobTypeReview, workDir, prompt)
		if submitErr != nil {
			slog.Warn("spec alignment check failed to submit", "error", submitErr)

			return
		}
		job = j
	}

	c.mu.Lock()
	if c.workUnit != nil {
		c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
		c.workUnit.UpdatedAt = time.Now()
		c.persistState()
	}
	c.mu.Unlock()

	c.emit(ConductorEvent{
		Type:    "spec_alignment_started",
		State:   c.machine.State(),
		JobID:   job.ID,
		Message: "Spec alignment check started",
	})

	go c.watchSpecAlignmentJob(c.lifecycleCtx, pool, job.ID) //nolint:contextcheck // intentionally uses lifecycle context
}

// watchSpecAlignmentJob monitors the spec alignment job and saves the result as a review.
func (c *Conductor) watchSpecAlignmentJob(ctx context.Context, pool *worker.Pool, jobID string) {
	stream := pool.Stream(jobID)
	if stream == nil {
		return
	}

	var result string
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-stream:
			if !ok {
				goto done
			}
			if event.Type == "job_completed" {
				result = event.Content
			}
		}
	}
done:

	if result == "" {
		return
	}

	// Save as a review document. Do not auto-approve — the alignment result
	// may contain findings. Let the user assess the review content.
	c.AddReview(false, "## Spec Alignment Check\n\n"+result)

	c.emit(ConductorEvent{
		Type:    "spec_alignment_complete",
		State:   c.machine.State(),
		JobID:   jobID,
		Message: "Spec alignment check complete",
	})
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
