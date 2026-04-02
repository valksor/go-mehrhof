package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// expectedInProgressState returns the state the machine should be in
// while a phase is actively running. Used by skip policy to verify the
// machine hasn't been moved by a retry cycle before dispatching completion.
func expectedInProgressState(phase string) State {
	switch Event(phase) { //nolint:exhaustive // Only phase events have in-progress states
	case EventPlan:
		return StatePlanning
	case EventImplement:
		return StateImplementing
	case EventSimplify:
		return StateSimplifying
	case EventOptimize:
		return StateOptimizing
	default:
		return ""
	}
}

// phaseFromEvent maps a completion event to its phase name.
func phaseFromEvent(event Event) string {
	switch event { //nolint:exhaustive // Only completion events map to phases
	case EventPlanDone:
		return string(EventPlan)
	case EventImplementDone:
		return string(EventImplement)
	case EventSimplifyDone:
		return string(EventSimplify)
	case EventOptimizeDone:
		return string(EventOptimize)
	default:
		return ""
	}
}

// evaluateAndMaybeIterate checks the job output using the phase strategy's
// EvaluateOutput. If the strategy signals "needs_iteration" and we haven't
// exceeded maxIterations, it re-submits the phase and returns true (caller
// should skip the normal completion path). Returns false if the result is
// accepted (either complete or max iterations reached).
// Must NOT be called with c.mu held.
func (c *Conductor) evaluateAndMaybeIterate(_ context.Context, completionEvent Event, output string) bool {
	phase := phaseFromEvent(completionEvent)
	if phase == "" {
		return false
	}

	c.mu.RLock()
	start := c.resolveStrategy(phase)
	c.mu.RUnlock()

	result := start.EvaluateOutput(output)
	if result.Status != "needs_iteration" {
		// Reset iteration count on successful completion.
		c.mu.Lock()
		delete(c.iterationCount, phase)
		c.mu.Unlock()

		return false
	}

	c.mu.Lock()
	c.iterationCount[phase]++
	attempt := c.iterationCount[phase]
	maxIter := c.maxIterations
	c.mu.Unlock()

	if attempt >= maxIter {
		slog.Warn("max iterations reached, accepting result",
			"phase", phase, "attempts", attempt, "marker", result.Metadata["marker"])
		c.mu.Lock()
		delete(c.iterationCount, phase)
		c.mu.Unlock()

		return false
	}

	slog.Info("strategy requests iteration",
		"phase", phase, "attempt", attempt, "marker", result.Metadata["marker"])

	c.emit(ConductorEvent{
		Type:    "iteration_retry",
		Message: fmt.Sprintf("Retrying %s (attempt %d/%d): unresolved %s marker", phase, attempt, maxIter, result.Metadata["marker"]),
	})

	// Re-submit the phase in a goroutine to avoid blocking.
	// Use lifecycleCtx so re-submission survives request context cancellation.
	go func() {
		// Guard against re-submission on a closed conductor.
		if c.closed.Load() {
			return
		}
		select {
		case <-c.lifecycleCtx.Done():
			return
		default:
		}

		var err error
		switch Event(phase) { //nolint:exhaustive // Only iteratable phase events
		case EventImplement:
			_, err = c.Implement(c.lifecycleCtx)
		case EventSimplify:
			_, err = c.Simplify(c.lifecycleCtx)
		case EventOptimize:
			_, err = c.Optimize(c.lifecycleCtx)
		case EventPlan:
			_, err = c.Plan(c.lifecycleCtx)
		}
		if err != nil {
			slog.Warn("iteration re-submit failed", "phase", phase, "error", err)
		}
	}()

	return true
}

// applyFailurePolicy checks the per-phase failure policy and handles the error
// accordingly. Returns true if the error was handled (retry or skip), in which
// case the caller should skip the default error path.
// Must NOT be called with c.mu held.
func (c *Conductor) applyFailurePolicy(ctx context.Context, completionEvent Event, errMsg string) bool {
	phase := phaseFromEvent(completionEvent)
	if phase == "" {
		return false
	}

	// Classify the failure and store it for status reporting
	failErr := fmt.Errorf("%s", errMsg)
	failClass := ClassifyError(failErr, phase)
	c.mu.Lock()
	c.lastFailureClass = failClass
	c.mu.Unlock()

	c.emit(ConductorEvent{
		Type:           "phase_failure_classified",
		Phase:          phase,
		Message:        fmt.Sprintf("Phase %s failed (%s): %s", phase, failClass, errMsg),
		Error:          errMsg,
		FailureClass:   failClass,
		FailureMessage: errMsg,
	})

	c.mu.RLock()
	policy, ok := c.phasePolicies[phase]
	c.mu.RUnlock()

	if !ok {
		return false
	}

	switch policy.Policy {
	case FailurePolicyRetry:
		c.mu.Lock()
		c.retryCount[phase]++
		attempt := c.retryCount[phase]
		c.mu.Unlock()

		if attempt <= policy.MaxRetries {
			slog.Info("retrying failed phase", "phase", phase, "attempt", attempt, "max", policy.MaxRetries)
			c.emit(ConductorEvent{
				Type:    "phase_retry",
				Message: fmt.Sprintf("Retrying %s (attempt %d/%d)", phase, attempt, policy.MaxRetries),
			})
			time.AfterFunc(policy.RetryDelay, func() {
				c.dispatchAutoAdvance(c.lifecycleCtx, phase)
			})

			return true
		}
		// Exhausted retries — fall through to default error handling.
		slog.Warn("retry limit reached", "phase", phase, "attempts", attempt)
		c.mu.Lock()
		delete(c.retryCount, phase)
		c.mu.Unlock()

		return false

	case FailurePolicySkip:
		slog.Info("skipping failed phase", "phase", phase)
		c.emit(ConductorEvent{
			Type:    "phase_skipped",
			Message: fmt.Sprintf("Skipping failed %s phase", phase),
		})
		c.mu.Lock()
		currentState := c.machine.State()
		expectedState := expectedInProgressState(phase)
		if expectedState == "" || currentState != expectedState {
			slog.Warn("skip policy: unexpected state, falling through to fail",
				"phase", phase, "current", currentState, "expected", expectedState)
			c.mu.Unlock()

			return false
		}
		c.machine.ClearPriorStableState()
		_ = c.machine.Dispatch(ctx, completionEvent)
		c.persistState()
		c.mu.Unlock()
		c.maybeAutoAdvance(ctx, completionEvent)

		return true

	case FailurePolicyFail:
		return false // Default behavior: stop and wait for user
	}

	return false
}
