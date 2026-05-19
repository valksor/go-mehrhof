package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/valksor/kvelmo/internal/eventlog"
	"github.com/valksor/kvelmo/internal/worker"
)

// Implement begins the implementation phase.
// When called from the planned state, requires specifications to exist.
// When called from the loaded state (skip-plan), uses the task description as spec.
func (c *Conductor) Implement(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.emitEnrichedError(err, PhaseImplement)

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, PhaseImplement)

		return "", err
	}

	// Check approval requirement
	if err := c.checkApproval(ctx, EventImplement); err != nil {
		return "", err
	}

	// Check minimum specification files policy.
	// MinSpecSections counts spec files (not sections within a spec).
	if s := c.getEffectiveSettings(); s != nil {
		if minSpecs := s.Workflow.Policy.MinSpecSections; minSpecs > 0 {
			specCount := len(c.workUnit.Specifications)
			if specCount < minSpecs {
				return "", fmt.Errorf("minimum %d specification file(s) required, found %d", minSpecs, specCount)
			}
		}
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventImplement); err != nil {
		c.mu.Lock() // Re-lock so deferred Unlock is balanced
		c.emitEnrichedError(err, PhaseImplement)

		return "", err
	}
	c.mu.Lock()

	// Record prior stable state for error/stop rollback during re-entry.
	// Set AFTER lock re-acquisition to avoid race with concurrent calls.
	currentState := c.machine.State()
	if currentState != StateLoaded && currentState != StatePlanned {
		c.machine.SetPriorStableState(currentState)
	}

	// Clear per-phase transient state to prevent leakage across re-entries.
	c.resetPhaseState(PhaseImplement)

	// Skip-plan: when implementing from loaded state, use description as implicit spec
	skippingPlan := currentState == StateLoaded

	// Dispatch implement event to transition state
	if err := c.machine.Dispatch(ctx, EventImplement); err != nil {
		c.machine.ClearPriorStableState()
		wrapped := fmt.Errorf("cannot implement: %w", err)
		c.emitEnrichedError(wrapped, PhaseImplement)

		return "", wrapped
	}

	if skippingPlan {
		c.logVerbosef("Skipping planning phase — using task description as specification")
	}

	// Run pre-phase guardrails (release lock during execution).
	c.mu.Unlock()
	if err := c.runPreGuardrails(ctx, PhaseImplement); err != nil {
		c.mu.Lock()
		// Rollback state transition.
		_ = c.machine.Dispatch(ctx, EventError)
		c.emitEnrichedError(err, PhaseImplement)

		return "", err
	}
	c.mu.Lock()

	c.setupCanaryHarness()
	// Start watching spec files for mid-execution edits.
	c.specWatcher = newSpecWatcher(c.workUnit.Specifications)

	prompt := c.applyStrategy(ctx, PhaseImplement, c.buildImplementPrompt())
	implJobType := worker.JobTypeImplement
	if c.dryRun {
		implJobType = worker.JobTypeDryRun
	}
	opts := c.buildJobOptionsForPhase(PhaseImplement)

	var job *worker.Job
	var err error
	if cached, ok := c.lookupResponseCache(prompt); ok {
		job, err = c.pool.SubmitCached(implJobType, c.getWorkDir(), prompt, cached, opts)
	} else {
		job, err = c.pool.SubmitWithOptions(implJobType, c.getWorkDir(), prompt, opts)
	}
	if err != nil {
		// Rollback state
		_ = c.machine.Dispatch(ctx, EventError)

		wrapped := fmt.Errorf("submit implement job: %w", err)
		c.emitEnrichedError(wrapped, PhaseImplement)

		return "", wrapped
	}

	c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
	c.workUnit.UpdatedAt = time.Now()
	c.activeJobID = job.ID
	c.saveJobSession(job.ID, "implementing", "")
	c.persistState()

	c.phaseStartedAt = time.Now()
	c.initProgressEstimator(PhaseImplement)
	c.emitEventLog(eventlog.Entry{Type: eventlog.EventPhaseStarted, Phase: PhaseImplement})
	c.emit(ConductorEvent{
		Type:    "implementing_started",
		State:   c.machine.State(),
		JobID:   job.ID,
		Message: "Implementation started",
	})

	// Watch job completion using lifecycle context
	// (not request ctx which may be cancelled when handler returns)
	go c.watchJob(c.lifecycleCtx, job.ID, EventImplementDone) //nolint:contextcheck // intentionally uses lifecycle context

	return job.ID, nil
}

// Optimize begins the optional optimization phase.
// This runs an optimization pass on the implemented code.
func (c *Conductor) Optimize(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.emitEnrichedError(err, PhaseOptimize)

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, PhaseOptimize)

		return "", err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventOptimize); err != nil {
		c.mu.Lock() // Re-lock so deferred Unlock is balanced
		c.emitEnrichedError(err, PhaseOptimize)

		return "", err
	}
	c.mu.Lock()

	// Record prior stable state for error/stop rollback during re-entry
	currentState := c.machine.State()
	if currentState != StateImplemented {
		c.machine.SetPriorStableState(currentState)
	}

	// Clear per-phase transient state to prevent leakage across re-entries.
	c.resetPhaseState(PhaseOptimize)

	// Dispatch optimize event to transition state
	if err := c.machine.Dispatch(ctx, EventOptimize); err != nil {
		c.machine.ClearPriorStableState()
		wrapped := fmt.Errorf("cannot optimize: %w", err)
		c.emitEnrichedError(wrapped, PhaseOptimize)

		return "", wrapped
	}

	c.setupCanaryHarness()
	optJobType := worker.JobTypeOptimize
	if c.dryRun {
		optJobType = worker.JobTypeDryRun
	}
	prompt := c.applyStrategy(ctx, PhaseOptimize, c.buildOptimizePrompt())
	opts := c.buildJobOptionsForPhase(PhaseOptimize)

	var job *worker.Job
	var err error
	if cached, ok := c.lookupResponseCache(prompt); ok {
		job, err = c.pool.SubmitCached(optJobType, c.getWorkDir(), prompt, cached, opts)
	} else {
		job, err = c.pool.SubmitWithOptions(optJobType, c.getWorkDir(), prompt, opts)
	}
	if err != nil {
		// Rollback state
		_ = c.machine.Dispatch(ctx, EventError)

		wrapped := fmt.Errorf("submit optimize job: %w", err)
		c.emitEnrichedError(wrapped, PhaseOptimize)

		return "", wrapped
	}

	c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
	c.workUnit.UpdatedAt = time.Now()
	c.activeJobID = job.ID
	c.saveJobSession(job.ID, "optimizing", "")
	c.persistState()

	c.phaseStartedAt = time.Now()
	c.initProgressEstimator(PhaseOptimize)
	c.emitEventLog(eventlog.Entry{Type: eventlog.EventPhaseStarted, Phase: PhaseOptimize})
	c.emit(ConductorEvent{
		Type:    "optimizing_started",
		State:   c.machine.State(),
		JobID:   job.ID,
		Message: "Optimization started",
	})

	// Watch job completion using lifecycle context
	// (not request ctx which may be cancelled when handler returns)
	go c.watchJob(c.lifecycleCtx, job.ID, EventOptimizeDone) //nolint:contextcheck // intentionally uses lifecycle context

	return job.ID, nil
}

// Simplify begins the optional simplification phase.
// This runs a simplification pass on the implemented code for clarity.
func (c *Conductor) Simplify(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.emitEnrichedError(err, PhaseSimplify)

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, PhaseSimplify)

		return "", err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventSimplify); err != nil {
		c.mu.Lock() // Re-lock so deferred Unlock is balanced
		c.emitEnrichedError(err, PhaseSimplify)

		return "", err
	}
	c.mu.Lock()

	// Record prior stable state for error/stop rollback during re-entry
	currentState := c.machine.State()
	if currentState != StateImplemented {
		c.machine.SetPriorStableState(currentState)
	}

	// Clear per-phase transient state to prevent leakage across re-entries.
	c.resetPhaseState(PhaseSimplify)

	// Dispatch simplify event to transition state
	if err := c.machine.Dispatch(ctx, EventSimplify); err != nil {
		c.machine.ClearPriorStableState()
		wrapped := fmt.Errorf("cannot simplify: %w", err)
		c.emitEnrichedError(wrapped, PhaseSimplify)

		return "", wrapped
	}

	c.setupCanaryHarness()
	simJobType := worker.JobTypeSimplify
	if c.dryRun {
		simJobType = worker.JobTypeDryRun
	}
	prompt := c.applyStrategy(ctx, PhaseSimplify, c.buildSimplifyPrompt())
	opts := c.buildJobOptionsForPhase(PhaseSimplify)

	var job *worker.Job
	var err error
	if cached, ok := c.lookupResponseCache(prompt); ok {
		job, err = c.pool.SubmitCached(simJobType, c.getWorkDir(), prompt, cached, opts)
	} else {
		job, err = c.pool.SubmitWithOptions(simJobType, c.getWorkDir(), prompt, opts)
	}
	if err != nil {
		// Rollback state
		_ = c.machine.Dispatch(ctx, EventError)

		wrapped := fmt.Errorf("submit simplify job: %w", err)
		c.emitEnrichedError(wrapped, PhaseSimplify)

		return "", wrapped
	}

	c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
	c.workUnit.UpdatedAt = time.Now()
	c.activeJobID = job.ID
	c.saveJobSession(job.ID, "simplifying", "")
	c.persistState()

	c.phaseStartedAt = time.Now()
	c.initProgressEstimator(PhaseSimplify)
	c.emitEventLog(eventlog.Entry{Type: eventlog.EventPhaseStarted, Phase: PhaseSimplify})
	c.emit(ConductorEvent{
		Type:    "simplifying_started",
		State:   c.machine.State(),
		JobID:   job.ID,
		Message: "Simplification started",
	})

	// Watch job completion using lifecycle context
	// (not request ctx which may be cancelled when handler returns)
	go c.watchJob(c.lifecycleCtx, job.ID, EventSimplifyDone) //nolint:contextcheck // intentionally uses lifecycle context

	return job.ID, nil
}

// maybeAutoAdvance triggers the next phase automatically if autoAdvance is enabled.
// Called asynchronously after a job completes. When SkipPhases is configured in
// workflow settings, phases in the skip list are bypassed during auto-advance.
func (c *Conductor) maybeAutoAdvance(ctx context.Context, completedEvent Event) {
	c.mu.RLock()
	enabled := c.autoAdvance
	c.mu.RUnlock()

	if !enabled {
		return
	}

	// Load skip phases from settings + runtime overrides.
	skipPhases := c.SkipPhases()

	// Determine the next phase based on completed event and skip list.
	// The full auto-advance chain is: plan → implement → simplify → optimize → review.
	nextPhase := c.resolveNextPhase(completedEvent, skipPhases)
	if nextPhase == "" {
		return
	}

	slog.Info("auto-advance: triggering next phase", "completed", completedEvent, "next", nextPhase)
	c.emit(ConductorEvent{
		Type:    "auto_advance",
		State:   c.machine.State(),
		Message: "Auto-advancing to " + nextPhase,
	})

	c.dispatchAutoAdvance(ctx, nextPhase)
}

// resolveNextPhase determines which phase to advance to after completedEvent,
// skipping any phases in skipPhases.
func (c *Conductor) resolveNextPhase(completedEvent Event, skipPhases []string) string {
	// Map completed events to the ordered list of candidate next phases.
	var candidates []string
	switch completedEvent { //nolint:exhaustive // only specific done events trigger auto-advance
	case EventPlanDone:
		candidates = []string{PhaseImplement}
	case EventImplementDone:
		candidates = []string{PhaseSimplify, PhaseOptimize, PhaseReview}
	case EventSimplifyDone:
		candidates = []string{PhaseOptimize, PhaseReview}
	case EventOptimizeDone:
		candidates = []string{PhaseReview}
	default:
		return ""
	}

	for _, phase := range candidates {
		if !slices.Contains(skipPhases, phase) {
			return phase
		}
	}

	return ""
}

// dispatchAutoAdvance calls the appropriate conductor method for the given phase.
func (c *Conductor) dispatchAutoAdvance(ctx context.Context, phase string) {
	var err error
	switch phase {
	case PhaseImplement:
		_, err = c.Implement(ctx)
	case PhaseSimplify:
		_, err = c.Simplify(ctx)
	case PhaseOptimize:
		_, err = c.Optimize(ctx)
	case PhaseReview:
		err = c.Review(ctx, false)
	default:
		return
	}

	if err != nil {
		slog.Warn("auto-advance: "+phase+" failed", "error", err)
		c.emit(ConductorEvent{
			Type:    "auto_advance_failed",
			State:   c.machine.State(),
			Message: "Auto-advance to " + phase + " failed: " + err.Error(),
		})
	}
}
