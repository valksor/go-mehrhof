package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/worker"
)

// Implement begins the implementation phase.
// When called from the planned state, requires specifications to exist.
// When called from the loaded state (skip-plan), uses the task description as spec.
// Accepts force parameter to allow re-running from already-implemented state.
func (c *Conductor) Implement(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.emitEnrichedError(err, "implement")

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, "implement")

		return "", err
	}

	// Check approval requirement
	if err := c.checkApproval(EventImplement); err != nil {
		return "", err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventImplement); err != nil {
		c.emitEnrichedError(err, "implement")

		return "", err
	}
	c.mu.Lock()

	// Handle force: allow re-running from implemented state
	if force && c.machine.State() == StateImplemented {
		c.machine.ForceState(StatePlanned)
	}

	// Skip-plan: when implementing from loaded state, use description as implicit spec
	skippingPlan := c.machine.State() == StateLoaded

	// Dispatch implement event to transition state
	if err := c.machine.Dispatch(ctx, EventImplement); err != nil {
		wrapped := fmt.Errorf("cannot implement: %w", err)
		c.emitEnrichedError(wrapped, "implement")

		return "", wrapped
	}

	if skippingPlan {
		c.logVerbosef("Skipping planning phase — using task description as specification")
	}

	prompt := c.buildImplementPrompt()
	opts := c.buildJobOptions()
	job, err := c.pool.SubmitWithOptions(worker.JobTypeImplement, c.getWorkDir(), prompt, opts)
	if err != nil {
		// Rollback state
		_ = c.machine.Dispatch(ctx, EventError)

		wrapped := fmt.Errorf("submit implement job: %w", err)
		c.emitEnrichedError(wrapped, "implement")

		return "", wrapped
	}

	c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
	c.workUnit.UpdatedAt = time.Now()
	c.activeJobID = job.ID
	c.saveJobSession(job.ID, "implementing", "")
	c.persistState()

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
		c.emitEnrichedError(err, "optimize")

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, "optimize")

		return "", err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventOptimize); err != nil {
		c.emitEnrichedError(err, "optimize")

		return "", err
	}
	c.mu.Lock()

	// Dispatch optimize event to transition state
	if err := c.machine.Dispatch(ctx, EventOptimize); err != nil {
		wrapped := fmt.Errorf("cannot optimize: %w", err)
		c.emitEnrichedError(wrapped, "optimize")

		return "", wrapped
	}

	prompt := c.buildOptimizePrompt()
	opts := c.buildJobOptions()
	job, err := c.pool.SubmitWithOptions(worker.JobTypeOptimize, c.getWorkDir(), prompt, opts)
	if err != nil {
		// Rollback state
		_ = c.machine.Dispatch(ctx, EventError)

		wrapped := fmt.Errorf("submit optimize job: %w", err)
		c.emitEnrichedError(wrapped, "optimize")

		return "", wrapped
	}

	c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
	c.workUnit.UpdatedAt = time.Now()
	c.activeJobID = job.ID
	c.saveJobSession(job.ID, "optimizing", "")
	c.persistState()

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
		c.emitEnrichedError(err, "simplify")

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, "simplify")

		return "", err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventSimplify); err != nil {
		c.emitEnrichedError(err, "simplify")

		return "", err
	}
	c.mu.Lock()

	// Dispatch simplify event to transition state
	if err := c.machine.Dispatch(ctx, EventSimplify); err != nil {
		wrapped := fmt.Errorf("cannot simplify: %w", err)
		c.emitEnrichedError(wrapped, "simplify")

		return "", wrapped
	}

	prompt := c.buildSimplifyPrompt()
	opts := c.buildJobOptions()
	job, err := c.pool.SubmitWithOptions(worker.JobTypeSimplify, c.getWorkDir(), prompt, opts)
	if err != nil {
		// Rollback state
		_ = c.machine.Dispatch(ctx, EventError)

		wrapped := fmt.Errorf("submit simplify job: %w", err)
		c.emitEnrichedError(wrapped, "simplify")

		return "", wrapped
	}

	c.workUnit.Jobs = append(c.workUnit.Jobs, job.ID)
	c.workUnit.UpdatedAt = time.Now()
	c.activeJobID = job.ID
	c.saveJobSession(job.ID, "simplifying", "")
	c.persistState()

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
// Called asynchronously after a job completes.
func (c *Conductor) maybeAutoAdvance(ctx context.Context, completedEvent Event) {
	c.mu.RLock()
	enabled := c.autoAdvance
	c.mu.RUnlock()

	if !enabled {
		return
	}

	var nextPhase string
	switch completedEvent { //nolint:exhaustive // only PlanDone and ImplementDone trigger auto-advance
	case EventPlanDone:
		nextPhase = "implement"
	case EventImplementDone:
		nextPhase = "review"
	default:
		return
	}

	slog.Info("auto-advance: triggering next phase", "completed", completedEvent, "next", nextPhase)
	c.emit(ConductorEvent{
		Type:    "auto_advance",
		State:   c.machine.State(),
		Message: "Auto-advancing to " + nextPhase,
	})

	switch completedEvent { //nolint:exhaustive // mirrors the switch above
	case EventPlanDone:
		if _, err := c.Implement(ctx, false); err != nil {
			slog.Warn("auto-advance: implement failed", "error", err)
			c.emit(ConductorEvent{
				Type:    "auto_advance_failed",
				State:   c.machine.State(),
				Message: "Auto-advance to implement failed: " + err.Error(),
			})
		}
	case EventImplementDone:
		if err := c.Review(ctx, false); err != nil {
			slog.Warn("auto-advance: review failed", "error", err)
			c.emit(ConductorEvent{
				Type:    "auto_advance_failed",
				State:   c.machine.State(),
				Message: "Auto-advance to review failed: " + err.Error(),
			})
		}
	}
}

func (c *Conductor) buildImplementPrompt() string {
	wu := c.workUnit

	// Format specifications as readable list instead of Go slice notation
	specs := ""
	if len(wu.Specifications) > 0 {
		specStr := strings.Join(wu.Specifications, "\n- ")
		specs = "\n\nSpecifications:\n- " + specStr
	}

	hierarchySection := buildHierarchySection(wu.Hierarchy)

	// When implementing without specs (skip-plan), emphasize the description as the sole guide
	header := "Implement the following task based on the specification:"
	if len(wu.Specifications) == 0 {
		header = "Implement the following task directly from the description (planning was skipped):"
	}

	prompt := fmt.Sprintf(`%s

Title: %s
Description: %s
%s%s
%s
Please implement the code following the plan. Create all necessary files and make required modifications.
Commit your changes with meaningful commit messages.
`, header, wu.Title, wu.Description, hierarchySection, specs, browserToolsSection())

	prompt += c.buildGitConventionInstructions()

	return prompt
}

// browserToolsSection returns guidance for using browser automation tools.
func browserToolsSection() string {
	return `## Browser Automation

If you need to interact with a browser (navigate, click, screenshot, etc.), use these CLI commands instead of Playwright MCP tools:

| Command | Description |
|---------|-------------|
| kvelmo browser navigate <url> | Navigate to a URL |
| kvelmo browser snapshot | Capture accessibility tree (for understanding page structure) |
| kvelmo browser screenshot | Take a screenshot (auto-saved to Screenshots panel) |
| kvelmo browser click <selector> | Click an element |
| kvelmo browser type <selector> <text> | Type text into an element |
| kvelmo browser wait <selector> | Wait for an element to appear |
| kvelmo browser eval <js> | Evaluate JavaScript |
| kvelmo browser console | Show console messages |
| kvelmo browser network | Show network requests |

These commands integrate with kvelmo's screenshot store - screenshots appear in the web UI's Screenshots panel for user visibility.
`
}

func (c *Conductor) buildSimplifyPrompt() string {
	wu := c.workUnit

	return fmt.Sprintf(`Simplify the implementation for the following task:

Title: %s
Description: %s

Please review the code that was just implemented and simplify it for clarity:
1. Remove unnecessary complexity and abstractions
2. Simplify control flow where possible
3. Remove dead code and unused variables
4. Consolidate duplicate logic
5. Use clearer, more descriptive names
6. Break down overly long functions
7. Prefer standard library solutions over custom implementations

Focus on making the code easier to understand and maintain.
Do NOT add new features or change functionality - only simplify.
Commit your changes with meaningful commit messages.
`, wu.Title, wu.Description)
}

func (c *Conductor) buildOptimizePrompt() string {
	wu := c.workUnit

	return fmt.Sprintf(`Review and optimize the implementation for the following task:

Title: %s
Description: %s

Please review the code that was just implemented and optimize it:
1. Improve code quality and readability
2. Add missing error handling
3. Optimize performance where applicable
4. Ensure proper documentation/comments
5. Check for edge cases and add handling
6. Ensure tests are comprehensive

Make any improvements while maintaining the existing functionality.
Commit your changes with meaningful commit messages.
`, wu.Title, wu.Description)
}
