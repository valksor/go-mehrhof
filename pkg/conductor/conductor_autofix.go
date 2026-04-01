// conductor_autofix.go — bounded retry loop for quality gate failures.
// When quality gates fail, structured error output is fed back to the agent
// for correction. Bounded by AutoFixSettings.MaxAttempts.
package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/valksor/kvelmo/pkg/worker"
)

const defaultMaxAutoFixAttempts = 3

// runQualityAutoFix attempts to fix quality gate failures by feeding structured
// error output back to the agent. Bounded by MaxAttempts.
// Must be called without c.mu held.
func (c *Conductor) runQualityAutoFix(ctx context.Context, qualityErr error) error {
	c.mu.RLock()
	settings := c.getEffectiveSettings()
	autoFixCfg := settings.Workflow.AutoFix
	pool := c.pool
	workUnit := c.workUnit
	c.mu.RUnlock()

	if pool == nil {
		return fmt.Errorf("auto-fix: %w", errors.New("no worker pool available"))
	}

	if workUnit == nil {
		return fmt.Errorf("auto-fix: %w", errors.New("no task loaded"))
	}

	maxAttempts := autoFixCfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAutoFixAttempts
	}

	lastErr := qualityErr

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		slog.Info("autofix: attempting quality gate fix",
			"attempt", attempt, "max", maxAttempts,
			"error", lastErr.Error())

		// Update tracking state
		c.mu.Lock()
		c.autoFixAttempt = attempt
		c.autoFixLastErr = lastErr.Error()
		c.mu.Unlock()

		if data, err := json.Marshal(map[string]int{"attempt": attempt, "max_attempts": maxAttempts}); err == nil {
			c.emit(ConductorEvent{
				Type:    "autofix_attempt",
				State:   c.machine.State(),
				Message: fmt.Sprintf("Auto-fix attempt %d/%d: %s", attempt, maxAttempts, lastErr.Error()),
				Data:    data,
			})
		}

		// Build fix prompt from quality gate error
		prompt := buildQualityFixPrompt(workUnit.Title, workUnit.Description, lastErr.Error(), attempt)

		// Dispatch fix job
		c.mu.Lock()
		opts := c.buildJobOptionsForPhase("implement")
		workDir := c.getWorkDir()
		c.mu.Unlock()

		var job *worker.Job
		var err error
		if cached, ok := c.lookupResponseCache(prompt); ok {
			job, err = pool.SubmitCached(worker.JobTypeImplement, workDir, prompt, cached, opts)
		} else {
			job, err = pool.SubmitWithOptions(worker.JobTypeImplement, workDir, prompt, opts)
		}
		if err != nil {
			c.clearAutoFixState()

			return fmt.Errorf("auto-fix: submit fix job: %w", err)
		}

		c.emit(ConductorEvent{
			Type:    "autofix_started",
			JobID:   job.ID,
			Message: fmt.Sprintf("Auto-fix agent started (attempt %d/%d)", attempt, maxAttempts),
		})

		// Wait for job completion by streaming events
		if err := c.waitForAutoFixJob(ctx, pool, job.ID); err != nil {
			slog.Warn("autofix: fix job failed", "attempt", attempt, "error", err)

			c.emit(ConductorEvent{
				Type:    "autofix_job_failed",
				Error:   err.Error(),
				Message: fmt.Sprintf("Auto-fix attempt %d/%d job failed: %s", attempt, maxAttempts, err.Error()),
			})

			if attempt >= maxAttempts {
				break
			}

			lastErr = err

			continue
		}

		// Re-run quality gate to check if the fix worked
		slog.Info("autofix: re-running quality gate", "attempt", attempt)

		gateErr := c.runQualityGateChecks(ctx)
		if gateErr == nil {
			slog.Info("autofix: quality gate passed", "attempt", attempt)

			c.emit(ConductorEvent{
				Type:    "autofix_success",
				State:   c.machine.State(),
				Message: fmt.Sprintf("Auto-fix succeeded on attempt %d/%d", attempt, maxAttempts),
			})

			c.clearAutoFixState()

			return nil
		}

		slog.Warn("autofix: quality gate still failing", "attempt", attempt, "error", gateErr)
		lastErr = gateErr

		if attempt >= maxAttempts {
			break
		}
	}

	// Exhausted all attempts
	slog.Warn("autofix: exhausted all fix attempts", "max", maxAttempts)

	c.emit(ConductorEvent{
		Type:    "autofix_exhausted",
		State:   c.machine.State(),
		Message: fmt.Sprintf("Auto-fix exhausted after %d attempts", maxAttempts),
	})

	c.clearAutoFixState()

	return fmt.Errorf("auto-fix exhausted after %d attempts: %w", maxAttempts, lastErr)
}

// clearAutoFixState resets the auto-fix tracking fields.
func (c *Conductor) clearAutoFixState() {
	c.mu.Lock()
	c.autoFixAttempt = 0
	c.autoFixLastErr = ""
	c.mu.Unlock()
}

// waitForAutoFixJob streams job events and blocks until the job completes.
func (c *Conductor) waitForAutoFixJob(ctx context.Context, pool *worker.Pool, jobID string) error {
	stream := pool.Stream(jobID)
	if stream == nil {
		return fmt.Errorf("could not stream auto-fix job %s", jobID)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("auto-fix job timed out: %w", ctx.Err())
		case event, ok := <-stream:
			if !ok {
				return nil
			}

			// Forward streaming output
			c.emit(ConductorEvent{
				Type:    "job_output",
				JobID:   jobID,
				Message: event.Content,
			})

			if event.Type == "job_completed" {
				return nil
			}

			if event.Type == "job_failed" {
				return fmt.Errorf("auto-fix job failed: %s", event.Content)
			}
		}
	}
}

// stateToPhase maps an in-progress state to the phase name used in configuration.
func stateToPhase(s State) string {
	switch s {
	case StatePlanning:
		return "plan"
	case StateImplementing:
		return "implement"
	case StateSimplifying:
		return "simplify"
	case StateOptimizing:
		return "optimize"
	case StateNone, StateLoaded, StatePlanned, StateImplemented,
		StateReviewing, StateSubmitted, StateFailed, StateWaiting, StatePaused:
		return ""
	}

	return ""
}

// shouldAutoFix checks whether the auto-fix loop should run for the current
// phase and settings configuration. Must be called without c.mu held.
func (c *Conductor) shouldAutoFix() bool {
	c.mu.RLock()
	settings := c.getEffectiveSettings()
	c.mu.RUnlock()

	if settings == nil {
		return false
	}

	cfg := settings.Workflow.AutoFix
	if !cfg.Enabled {
		return false
	}

	// Determine current phase from state
	phase := stateToPhase(c.machine.State())
	if phase == "" {
		return false
	}

	// Check if the phase is in the configured phases list
	phases := cfg.Phases
	if len(phases) == 0 {
		// Default phases when none configured
		phases = []string{"implement", "simplify", "optimize"}
	}

	return slices.Contains(phases, phase)
}

// buildQualityFixPrompt constructs the prompt for the quality gate fix agent.
func buildQualityFixPrompt(title, description, qualityError string, attempt int) string {
	return fmt.Sprintf(`Fix quality gate failures for the following task:

Title: %s
Description: %s

## Quality Gate Failure (attempt %d)

The quality gate failed after the phase completed. Below are the quality gate errors.
Analyze the failures and fix the code to make the quality gate pass.

### Quality Gate Errors

%s

## Instructions

1. Read the quality gate errors carefully
2. Identify the root cause of each failure
3. Fix the code to resolve the failures
4. Do NOT introduce new features or change functionality beyond what is needed to fix quality
5. Focus on: compilation errors, lint failures, type errors, slop detection findings
6. If a finding references a file and line, fix that specific location
`, title, description, attempt, qualityError)
}
