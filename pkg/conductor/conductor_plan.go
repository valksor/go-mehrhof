package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/eventlog"
	"github.com/valksor/kvelmo/pkg/graph"
	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/worker"
)

// Plan begins the planning phase.
// Submits a planning job to the worker pool.
func (c *Conductor) Plan(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		err := errors.New("no task loaded")
		c.emitEnrichedError(err, "plan")

		return "", err
	}

	// Check pool BEFORE transitioning state to avoid leaving machine in bad state
	if c.pool == nil {
		err := errors.New("no worker pool available")
		c.emitEnrichedError(err, "plan")

		return "", err
	}

	// Check approval requirement
	if err := c.checkApproval(EventPlan); err != nil {
		return "", err
	}

	// Run pre-transition hooks (release lock during shell execution)
	c.mu.Unlock()
	if err := c.RunTransitionHooks(ctx, EventPlan); err != nil {
		c.mu.Lock() // Re-lock so deferred Unlock is balanced
		c.emitEnrichedError(err, "plan")

		return "", err
	}
	c.mu.Lock()

	// Record prior stable state for error/stop rollback during re-entry.
	// Set AFTER lock re-acquisition to avoid race with concurrent calls.
	currentState := c.machine.State()
	if currentState != StateLoaded {
		c.machine.SetPriorStableState(currentState)
	}

	// Clear per-phase transient state to prevent leakage across re-entries.
	c.resetPhaseState("plan")

	// Dispatch plan event to transition state
	if err := c.machine.Dispatch(ctx, EventPlan); err != nil {
		c.machine.ClearPriorStableState()
		wrapped := fmt.Errorf("cannot plan: %w", err)
		c.emitEnrichedError(wrapped, "plan")

		return "", wrapped
	}

	// Run pre-phase guardrails (release lock during execution).
	c.mu.Unlock()
	if err := c.runPreGuardrails(ctx, "plan"); err != nil {
		c.mu.Lock()
		// Rollback state transition.
		_ = c.machine.Dispatch(ctx, EventError)
		c.emitEnrichedError(err, "plan")

		return "", err
	}
	c.mu.Lock()

	// Load existing specs so re-planning iterates rather than restarts from scratch
	var existingSpecs string
	if c.store != nil {
		specStore := storage.NewSpecStore(c.store)
		existingSpecs, _ = specStore.GatherSpecificationsContent(c.workUnit.ID)
	}

	// Auto-detect complexity and choose appropriate prompt
	complexity := DetectTaskComplexity(
		c.workUnit.Title,
		c.workUnit.Description,
		0, // file count hint not available here
		"",
		nil,
		false,
	)
	prompt := c.buildPlanPromptForComplexity(complexity, existingSpecs)
	prompt = c.applyStrategy(ctx, "plan", prompt)

	c.setupCanaryHarness()

	// Build a phase graph (single node for planning, future: multiple sub-tasks).
	jobType := worker.JobTypePlan
	if c.dryRun {
		jobType = worker.JobTypeDryRun
	}
	g := buildPhaseGraph(jobType, "planning", prompt, c.getWorkDir())
	sched := graph.NewScheduler(g, c.pool)

	// Create pre-job safety checkpoint before the scheduler starts.
	c.createSafetyCheckpoint(ctx, "pre-plan_done checkpoint")

	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	c.phaseStartedAt = time.Now()
	c.emitEventLog(eventlog.Entry{Type: eventlog.EventPhaseStarted, Phase: "plan"})
	c.emit(ConductorEvent{
		Type:    "planning_started",
		State:   c.machine.State(),
		Message: fmt.Sprintf("Planning started (complexity: %s)", complexity),
	})

	// Watch graph completion in background using lifecycle context
	// (not request ctx which may be cancelled when handler returns)
	go c.watchGraph(c.lifecycleCtx, sched, EventPlanDone) //nolint:contextcheck // intentionally uses lifecycle context

	return sched.ID(), nil
}

// GenerateDeltaSpecification creates a specification file describing what changed between old and new content.
// Caller must hold c.mu (either Lock or RLock) before calling this method.
func (c *Conductor) GenerateDeltaSpecification(ctx context.Context, oldContent, newContent string) (string, error) {
	// Access shared state directly - caller holds the lock.
	wu := c.workUnit
	store := c.store

	if wu == nil {
		return "", errors.New("no task loaded")
	}
	if store == nil {
		return "", errors.New("no storage configured")
	}

	// Build the delta specification content
	deltaContent := buildDeltaSpecificationContent(oldContent, newContent)

	specStore := storage.NewSpecStore(store)
	num, err := specStore.NextSpecificationNumber(wu.ID)
	if err != nil {
		return "", fmt.Errorf("find next specification number: %w", err)
	}
	if err := specStore.SaveSpecification(wu.ID, num, deltaContent); err != nil {
		return "", fmt.Errorf("save delta specification: %w", err)
	}

	return specStore.SpecificationPath(wu.ID, num), nil
}

// buildDeltaSpecificationContent builds the markdown content for a delta specification.
func buildDeltaSpecificationContent(oldContent, newContent string) string {
	var sb strings.Builder

	sb.WriteString("# Delta Specification\n\n")
	sb.WriteString("This specification describes changes detected in the task since the last planning cycle.\n\n")
	sb.WriteString("## Summary of Changes\n\n")

	// Simple line-based diff summary
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	added := 0
	removed := 0

	oldSet := make(map[string]bool)
	for _, l := range oldLines {
		oldSet[l] = true
	}
	newSet := make(map[string]bool)
	for _, l := range newLines {
		newSet[l] = true
	}

	for _, l := range newLines {
		if !oldSet[l] {
			added++
		}
	}
	for _, l := range oldLines {
		if !newSet[l] {
			removed++
		}
	}

	fmt.Fprintf(&sb, "- Lines added: %d\n", added)
	fmt.Fprintf(&sb, "- Lines removed: %d\n\n", removed)

	sb.WriteString("## Previous Content\n\n")
	sb.WriteString("```\n")
	sb.WriteString(oldContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## New Content\n\n")
	sb.WriteString("```\n")
	sb.WriteString(newContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Implementation Notes\n\n")
	sb.WriteString("Review the changes above and update the implementation accordingly.\n")
	sb.WriteString("Focus only on what has changed — do not re-implement existing functionality.\n")

	return sb.String()
}

// SaveSpecification saves planning output as the next specification file.
// Specification files are named specification-1.md, specification-2.md, etc.
func (c *Conductor) SaveSpecification(content string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return "", errors.New("no task loaded")
	}
	if c.store == nil {
		return "", errors.New("no storage configured")
	}

	specStore := storage.NewSpecStore(c.store)
	num, err := specStore.NextSpecificationNumber(c.workUnit.ID)
	if err != nil {
		return "", fmt.Errorf("find next specification number: %w", err)
	}
	if err := specStore.SaveSpecification(c.workUnit.ID, num, content); err != nil {
		return "", fmt.Errorf("save specification: %w", err)
	}
	specPath := specStore.SpecificationPath(c.workUnit.ID, num)
	c.workUnit.Specifications = append(c.workUnit.Specifications, specPath)
	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	return specPath, nil
}

// buildPlanPromptForComplexity selects the appropriate planning prompt based on detected complexity.
// If existingSpecs is non-empty, the prompt instructs the agent to iterate rather than start from scratch.
func (c *Conductor) buildPlanPromptForComplexity(complexity TaskComplexity, existingSpecs string) string {
	wu := c.workUnit
	hierarchySection := buildHierarchySection(wu.Hierarchy)

	// Get specification path based on storage config
	specPath := c.getSpecificationPath()
	if specPath == "" {
		// Fallback to a sensible default if storage isn't configured
		specPath = ".kvelmo/specifications/specification-1.md"
	}

	// Critical instruction to write spec file - this is the PRIMARY deliverable
	fileWriteInstruction := fmt.Sprintf(`
## CRITICAL: Your Primary Deliverable

YOUR ONLY JOB IS TO WRITE A SPECIFICATION FILE. Do NOT just output text.

You MUST use the Write tool to create this file:
- Path: %s
- Create the parent directory if it doesn't exist

The implementation phase CANNOT proceed without this file. This is not optional.

Do this FIRST before any other output.

`, specPath)

	specReminder := fmt.Sprintf("\nRemember: Use the Write tool to save the spec to `%s`\n", specPath)

	existingSpecsSection := ""
	if existingSpecs != "" {
		existingSpecsSection = fmt.Sprintf(`
## Previous Specifications

IMPORTANT: The following specifications already exist from previous planning iterations.
DO NOT start from scratch. Build upon these, refine them, or address any gaps:

%s

Your new specification should either refine the existing one or address gaps/feedback.

`, existingSpecs)
	}

	projectCmds := c.buildProjectCommandsSection()

	switch complexity { //nolint:exhaustive // ComplexitySimple is the only special case
	case ComplexitySimple:
		return fmt.Sprintf(
			"%s"+
				"Create a concise implementation plan for this straightforward task.\n\n"+
				"Title: %s\nDescription: %s\n%s\n"+
				"%s"+
				"Provide:\n"+
				"1. Brief overview of the approach\n"+
				"2. Files to create/modify\n"+
				"3. Key changes needed\n\n"+
				"%s"+
				"%s"+
				"%s",
			fileWriteInstruction, wu.Title, wu.Description, hierarchySection, existingSpecsSection, browserToolsSection(), projectCmds, specReminder)

	default: // ComplexityMedium, ComplexityComplex
		return fmt.Sprintf(
			"%s"+
				"You are an expert software engineer. Create a detailed implementation specification for the following task.\n\n"+
				"Title: %s\nDescription: %s\n%s\n"+
				"%s"+
				"Think step by step through the problem before writing the specification.\n\n"+
				"## Constraints\n"+
				"- Follow existing code patterns and conventions\n"+
				"- Ensure backward compatibility unless explicitly breaking\n"+
				"- Consider edge cases and error handling\n"+
				"- Include testing strategy\n\n"+
				"## Required Sections\n"+
				"1. **Overview**: High-level approach and rationale\n"+
				"2. **Implementation Plan**: Step-by-step with specific files to create/modify\n"+
				"3. **Data Structures / Interfaces**: Any new types or interface changes\n"+
				"4. **Testing Strategy**: What to test and how\n"+
				"5. **Risks & Mitigations**: Potential issues and how to address them\n\n"+
				"%s"+
				"%s"+
				"%s",
			fileWriteInstruction, wu.Title, wu.Description, hierarchySection, existingSpecsSection, browserToolsSection(), projectCmds, specReminder)
	}
}

// getSpecificationPath returns the path where specifications should be written.
// Respects the saveInProject setting from config.
func (c *Conductor) getSpecificationPath() string {
	if c.store == nil || c.workUnit == nil {
		slog.Warn("getSpecificationPath called without store or workUnit")

		return ""
	}

	specStore := storage.NewSpecStore(c.store)
	num, err := specStore.NextSpecificationNumber(c.workUnit.ID)
	if err != nil {
		slog.Warn("failed to determine next specification number, defaulting to 1",
			"error", err, "task_id", c.workUnit.ID)
		num = 1
	}

	return specStore.SpecificationPath(c.workUnit.ID, num)
}

// buildHierarchySection formats parent and sibling task context as a markdown
// section suitable for inclusion in AI planning and implementation prompts.
// Returns an empty string when hierarchy is nil or contains no data.
func buildHierarchySection(hierarchy *HierarchyContext) string {
	if hierarchy == nil {
		return ""
	}

	var sections []string

	if hierarchy.Parent != nil {
		parentDesc := hierarchy.Parent.Description
		const maxParentDesc = 500
		if len(parentDesc) > maxParentDesc {
			parentDesc = parentDesc[:maxParentDesc] + "..."
		}
		descPart := ""
		if parentDesc != "" {
			descPart = fmt.Sprintf("\n**Description:**\n%s\n", parentDesc)
		}
		sections = append(sections, fmt.Sprintf(
			"### Parent Task Context\n**Title:** %s\n**Status:** %s%s\nThis is a subtask of the parent task above. Consider how your work fits into the broader context.\n",
			hierarchy.Parent.Title, hierarchy.Parent.Status, descPart,
		))
	}

	if len(hierarchy.Siblings) > 0 {
		var lines []string
		for _, s := range hierarchy.Siblings {
			line := fmt.Sprintf("- **%s**", s.Title)
			if s.Status != "" {
				line += fmt.Sprintf(" (Status: %s)", s.Status)
			}
			lines = append(lines, line)
		}
		sections = append(sections, fmt.Sprintf(
			"### Related Subtasks\n%s\n\nConsider how your implementation relates to these sibling tasks. Avoid duplicating work and ensure consistency.\n",
			strings.Join(lines, "\n"),
		))
	}

	if len(sections) == 0 {
		return ""
	}

	return fmt.Sprintf("\n## Hierarchical Context\n%s\n", strings.Join(sections, "\n"))
}
