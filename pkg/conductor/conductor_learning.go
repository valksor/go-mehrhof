package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/settings"
)

// extractLearnings generates a summary of learnings from the completed task
// and stores them in the memory store for future task context.
// Snapshots work unit data under the lock, then performs I/O without holding it.
// Must be called with c.mu held — the caller's lock is temporarily released.
func (c *Conductor) extractLearnings(ctx context.Context) {
	if c.workUnit == nil || c.memoryIndexer == nil {
		return
	}

	store := c.memoryIndexer.Store()
	if store == nil {
		return
	}

	// Snapshot data under the lock.
	taskID := c.workUnit.ID
	title := c.workUnit.Title
	var sourceRef, sourceProvider string
	if c.workUnit.Source != nil {
		sourceRef = c.workUnit.Source.Reference
		sourceProvider = c.workUnit.Source.Provider
	}

	// Copy phase metrics.
	phaseMetrics := make(map[string]*PhaseMetrics, len(c.workUnit.PhaseMetrics))
	for k, v := range c.workUnit.PhaseMetrics {
		pm := *v
		phaseMetrics[k] = &pm
	}

	qualityPassed := c.workUnit.QualityGatePassed
	qualityError := c.workUnit.QualityGateError
	tags := append([]string{"learning"}, c.workUnit.Tags...)
	if sourceProvider != "" {
		tags = append(tags, sourceProvider)
	}

	// Release lock before I/O.
	c.mu.Unlock()
	defer c.mu.Lock()

	// Build summary.
	var parts []string
	parts = append(parts, "Task: "+title)
	if sourceRef != "" {
		parts = append(parts, "Source: "+sourceRef)
	}
	for phase, pm := range phaseMetrics {
		detail := fmt.Sprintf("Phase %s: duration=%s", phase, pm.Duration)
		if pm.Agent != "" {
			detail += ", agent=" + pm.Agent
		}
		parts = append(parts, detail)
	}
	if qualityPassed != nil {
		if *qualityPassed {
			parts = append(parts, "Quality gate: passed")
		} else {
			parts = append(parts, fmt.Sprintf("Quality gate: failed (%s)", qualityError))
		}
	}

	doc := &memory.Document{
		ID:         fmt.Sprintf("learning-%s-%d", taskID, time.Now().UnixNano()),
		TaskID:     taskID,
		Type:       memory.TypeSolution,
		Content:    strings.Join(parts, "\n"),
		Category:   "learnings",
		Importance: 0.8,
		Tags:       tags,
		CreatedAt:  time.Now(),
	}

	if err := store.Store(ctx, doc); err != nil {
		slog.Warn("failed to store task learning", "task_id", taskID, "error", err)
	} else {
		slog.Info("task learning stored", "task_id", taskID, "doc_id", doc.ID)
	}
}

// linkTaskOutcome determines the outcome of the current task and links it to
// all memory documents for that task. Must be called with c.mu held — the
// caller's lock is temporarily released for I/O.
func (c *Conductor) linkTaskOutcome(ctx context.Context) {
	if c.workUnit == nil || c.memoryIndexer == nil {
		return
	}

	// Check if outcome linking is enabled in settings.
	if s := c.getEffectiveSettings(); s != nil {
		if !settings.BoolValue(s.Storage.OutcomeLinking, true) {
			return
		}
	}

	store := c.memoryIndexer.Store()
	if store == nil {
		return
	}

	// Snapshot data under the lock.
	taskID := c.workUnit.ID
	qualityPassed := c.workUnit.QualityGatePassed
	prID := c.workUnit.PRID
	cancelledBy := c.workUnit.CancelledBy

	// Determine outcome from task state.
	outcome := memory.DocumentOutcome{}

	// Success: quality gate passed and not cancelled.
	if qualityPassed != nil && *qualityPassed && cancelledBy == "" {
		outcome.Success = true
	}

	// CI passed first try: quality gate passed without prior failure.
	if qualityPassed != nil && *qualityPassed {
		outcome.CIPassedFirstTry = true
	}

	// PR was created (merged status determined later at finish).
	if prID != "" {
		// Check PR status if providers available.
		if c.providers != nil && c.workUnit.Source != nil {
			pr, err := c.providers.GetPRStatus(ctx, c.workUnit.Source.Reference)
			if err == nil && pr.Merged {
				outcome.PRMerged = true
			}
		}
	}

	// Human changes: review was done and checklist items were checked.
	if len(c.workUnit.ChecklistChecked) > 0 {
		outcome.HumanChangesNeeded = true
	}

	// Release lock before I/O.
	c.mu.Unlock()
	defer c.mu.Lock()

	if err := store.LinkOutcome(ctx, taskID, outcome); err != nil {
		slog.Warn("failed to link task outcome to memory", "task_id", taskID, "error", err)
	}
}
