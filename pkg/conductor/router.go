package conductor

import (
	"context"
	"fmt"
	"log/slog"
)

// RouteAction represents what should happen after a phase completes.
type RouteAction string

const (
	RouteAdvance  RouteAction = "advance"  // Proceed to next phase
	RouteRetry    RouteAction = "retry"    // Retry current phase
	RouteSkip     RouteAction = "skip"     // Skip next phase
	RouteRollback RouteAction = "rollback" // Roll back to an earlier phase
)

// RouteDecision is the result of evaluating a phase's output.
type RouteDecision struct {
	Action      RouteAction `json:"action"`
	Reason      string      `json:"reason"`
	TargetPhase string      `json:"target_phase,omitempty"` // For rollback: which phase to return to
	Attempt     int         `json:"attempt"`                // Current retry attempt number
	MaxRetries  int         `json:"max_retries"`
}

// PhaseRouter evaluates phase output and decides the next action.
type PhaseRouter interface {
	Route(ctx context.Context, phase string, output string, attempt int) RouteDecision
}

// DefaultRouter implements basic routing logic that always advances.
// It provides the framework for configurable routing strategies.
type DefaultRouter struct {
	MaxRetries int // Default: 2
}

// NewDefaultRouter creates a DefaultRouter with sensible defaults.
func NewDefaultRouter() *DefaultRouter {
	return &DefaultRouter{MaxRetries: 2}
}

// Route evaluates phase output and returns a routing decision.
// The default implementation always advances to the next phase.
func (r *DefaultRouter) Route(_ context.Context, _ string, _ string, attempt int) RouteDecision {
	return RouteDecision{
		Action:     RouteAdvance,
		Reason:     "phase completed successfully",
		Attempt:    attempt,
		MaxRetries: r.MaxRetries,
	}
}

// applyRouteDecision acts on a routing decision after a phase completes.
// Returns true if the decision was handled (retry/skip/rollback), meaning the
// caller should NOT proceed with normal advance logic. Returns false if the
// normal advance path should continue.
//
// Must NOT be called with c.mu held.
func (c *Conductor) applyRouteDecision(ctx context.Context, decision RouteDecision, completionEvent Event) bool {
	phase := phaseFromEvent(completionEvent)

	// Track routing decision in work unit history.
	c.mu.Lock()
	if c.workUnit != nil {
		c.workUnit.RouteHistory = append(c.workUnit.RouteHistory, decision)
		c.persistState()
	}
	c.mu.Unlock()

	switch decision.Action {
	case RouteAdvance:
		// Normal advance — caller handles this.
		return false

	case RouteRetry:
		if decision.Attempt >= decision.MaxRetries {
			slog.Warn("router: retry limit reached, advancing instead",
				"phase", phase, "attempts", decision.Attempt, "max", decision.MaxRetries)

			return false
		}

		slog.Info("router: retrying phase",
			"phase", phase, "attempt", decision.Attempt, "reason", decision.Reason)

		c.emit(ConductorEvent{
			Type:    "route_retry",
			Phase:   phase,
			Message: fmt.Sprintf("Router retrying %s (attempt %d/%d): %s", phase, decision.Attempt+1, decision.MaxRetries, decision.Reason),
		})

		go func() {
			if c.closed.Load() {
				return
			}
			select {
			case <-c.lifecycleCtx.Done():
				return
			default:
			}

			c.dispatchAutoAdvance(c.lifecycleCtx, phase)
		}()

		return true

	case RouteSkip:
		slog.Info("router: skipping next phase",
			"phase", phase, "target", decision.TargetPhase, "reason", decision.Reason)

		c.emit(ConductorEvent{
			Type:    "route_skip",
			Phase:   phase,
			Message: fmt.Sprintf("Router skipping %s after %s: %s", decision.TargetPhase, phase, decision.Reason),
		})

		// Dispatch completion event to advance past this phase.
		c.mu.Lock()
		c.machine.ClearPriorStableState()
		if err := c.machine.Dispatch(ctx, completionEvent); err != nil {
			slog.Warn("router: skip dispatch failed", "event", completionEvent, "error", err)
		}
		c.persistState()
		c.mu.Unlock()

		c.maybeAutoAdvance(ctx, completionEvent)

		return true

	case RouteRollback:
		targetPhase := decision.TargetPhase
		if targetPhase == "" {
			slog.Warn("router: rollback requested but no target phase specified, advancing instead",
				"phase", phase)

			return false
		}

		slog.Info("router: rolling back to earlier phase",
			"phase", phase, "target", targetPhase, "reason", decision.Reason)

		c.emit(ConductorEvent{
			Type:    "route_rollback",
			Phase:   phase,
			Message: fmt.Sprintf("Router rolling back from %s to %s: %s", phase, targetPhase, decision.Reason),
		})

		// Dispatch error to leave the current in-progress state, then
		// re-submit the target phase.
		c.mu.Lock()
		if err := c.machine.Dispatch(ctx, EventError); err != nil {
			slog.Warn("router: rollback dispatch failed", "error", err)
		}
		c.persistState()
		c.mu.Unlock()

		go func() {
			if c.closed.Load() {
				return
			}
			select {
			case <-c.lifecycleCtx.Done():
				return
			default:
			}

			c.dispatchAutoAdvance(c.lifecycleCtx, targetPhase)
		}()

		return true
	}

	return false
}
