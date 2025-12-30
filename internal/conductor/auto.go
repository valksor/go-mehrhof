package conductor

import (
	"cmp"
	"context"
	"fmt"
)

// AutoOptions configures the full automation run
type AutoOptions struct {
	// Quality settings
	QualityTarget string // Make target (default: "quality")
	MaxRetries    int    // Max quality retry attempts (0 = skip quality)

	// Finish settings
	SquashMerge  bool   // Use squash merge (default: true)
	DeleteBranch bool   // Delete branch after merge (default: true)
	TargetBranch string // Branch to merge into (default: auto-detect)
	Push         bool   // Push after merge
}

// DefaultAutoOptions returns sensible defaults for auto mode
func DefaultAutoOptions() AutoOptions {
	return AutoOptions{
		QualityTarget: "quality",
		MaxRetries:    3,
		SquashMerge:   true,
		DeleteBranch:  true,
		TargetBranch:  "", // Auto-detect base branch
		Push:          false,
	}
}

// AutoResult holds the result of a full auto run
type AutoResult struct {
	PlanningDone    bool   // Planning phase completed
	ImplementDone   bool   // Implementation phase completed
	QualityAttempts int    // Number of quality check attempts
	QualityPassed   bool   // Quality checks passed
	FinishDone      bool   // Task finished and merged
	Error           error  // First error encountered (if any)
	FailedAt        string // Phase where failure occurred
}

// RunAuto executes the full automation cycle: start -> plan -> implement -> quality -> finish
func (c *Conductor) RunAuto(ctx context.Context, reference string, opts AutoOptions) (*AutoResult, error) {
	result := &AutoResult{}

	// Step 1: Start task (register it)
	c.publishProgress("Starting task...", 5)
	if err := c.Start(ctx, reference); err != nil {
		result.Error = err
		result.FailedAt = "start"
		return result, fmt.Errorf("start: %w", err)
	}
	c.publishProgress("Task registered", 10)

	// Step 2: Planning phase
	c.publishProgress("Entering planning phase...", 15)
	if err := c.Plan(ctx); err != nil {
		result.Error = err
		result.FailedAt = "plan"
		return result, fmt.Errorf("enter planning: %w", err)
	}

	if err := c.RunPlanning(ctx); err != nil {
		// In auto mode, ErrPendingQuestion should not occur (skipped in handlers.go)
		result.Error = err
		result.FailedAt = "planning"
		return result, fmt.Errorf("planning: %w", err)
	}
	result.PlanningDone = true
	c.publishProgress("Planning complete", 30)

	// Step 3: Implementation phase
	c.publishProgress("Entering implementation phase...", 35)
	if err := c.Implement(ctx); err != nil {
		result.Error = err
		result.FailedAt = "implement"
		return result, fmt.Errorf("enter implementation: %w", err)
	}

	if err := c.RunImplementation(ctx); err != nil {
		result.Error = err
		result.FailedAt = "implementation"
		return result, fmt.Errorf("implementation: %w", err)
	}
	result.ImplementDone = true
	c.publishProgress("Implementation complete", 50)

	// Step 4: Quality retry loop (skip if MaxRetries is 0)
	if opts.MaxRetries > 0 {
		// Use opts.MaxRetries if positive, otherwise fall back to conductor default
		maxRetries := cmp.Or(opts.MaxRetries, c.opts.MaxQualityRetries)

		qualityOpts := QualityOptions{
			Target:       opts.QualityTarget,
			SkipPrompt:   true, // Always skip prompt in auto mode
			AllowFailure: true, // We handle failures in the retry loop
		}

		for attempt := 1; attempt <= maxRetries; attempt++ {
			result.QualityAttempts = attempt
			c.publishProgress(fmt.Sprintf("Quality check attempt %d/%d...", attempt, maxRetries), 50+(attempt*10))

			qualityResult, err := c.RunQuality(ctx, qualityOpts)
			if err != nil {
				// Quality command itself failed (not just checks)
				result.Error = err
				result.FailedAt = "quality"
				return result, fmt.Errorf("quality check attempt %d: %w", attempt, err)
			}

			// Quality passed
			if qualityResult.Passed {
				result.QualityPassed = true
				c.publishProgress("Quality checks passed", 80)
				break
			}

			// Quality failed, but we have retries left
			if attempt < maxRetries {
				c.publishProgress(fmt.Sprintf("Quality failed, re-implementing (attempt %d)...", attempt+1), 55)

				// Re-run implementation with quality feedback
				if err := c.reImplementWithFeedback(ctx, qualityResult.Output); err != nil {
					result.Error = err
					result.FailedAt = "re-implementation"
					return result, fmt.Errorf("re-implementation attempt %d: %w", attempt, err)
				}
				continue
			}

			// Max retries exceeded
			result.Error = fmt.Errorf("quality check failed after %d attempts", maxRetries)
			result.FailedAt = "quality"
			return result, result.Error
		}
	} else {
		// Quality skipped
		result.QualityPassed = true
		c.publishProgress("Quality checks skipped", 80)
	}

	// Step 5: Finish (merge)
	c.publishProgress("Finishing task...", 85)
	finishOpts := FinishOptions{
		SquashMerge:  opts.SquashMerge,
		DeleteBranch: opts.DeleteBranch,
		TargetBranch: opts.TargetBranch,
		PushAfter:    opts.Push,
	}

	if err := c.Finish(ctx, finishOpts); err != nil {
		result.Error = err
		result.FailedAt = "finish"
		return result, fmt.Errorf("finish: %w", err)
	}
	result.FinishDone = true
	c.publishProgress("Task completed", 100)

	return result, nil
}

// reImplementWithFeedback runs implementation phase with quality failure context
func (c *Conductor) reImplementWithFeedback(ctx context.Context, qualityOutput string) error {
	// Append quality feedback to notes so agent sees what failed
	if qualityOutput != "" {
		feedbackNote := fmt.Sprintf("## Quality Check Failed\n\nThe following issues need to be fixed:\n\n```\n%s\n```\n\nPlease address these issues in the next implementation.", qualityOutput)
		if err := c.workspace.AppendNote(c.activeTask.ID, feedbackNote, "implementing"); err != nil {
			c.logError(fmt.Errorf("append quality feedback: %w", err))
		}
	}

	// Re-enter implementation phase
	if err := c.Implement(ctx); err != nil {
		return fmt.Errorf("enter re-implementation: %w", err)
	}

	// Run implementation with feedback context
	if err := c.RunImplementation(ctx); err != nil {
		return fmt.Errorf("run re-implementation: %w", err)
	}

	return nil
}
