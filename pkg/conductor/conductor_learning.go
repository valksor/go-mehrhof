package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/memory"
)

// extractLearnings generates a summary of learnings from the completed task
// and stores them in the memory store for future task context.
// Must be called with c.mu held.
func (c *Conductor) extractLearnings(ctx context.Context) {
	if c.workUnit == nil || c.memoryIndexer == nil {
		return
	}

	store := c.memoryIndexer.Store()
	if store == nil {
		return
	}

	// Build a summary of what happened during this task.
	var parts []string
	parts = append(parts, "Task: "+c.workUnit.Title)

	if c.workUnit.Source != nil {
		parts = append(parts, "Source: "+c.workUnit.Source.Reference)
	}

	// Summarize phase metrics.
	for phase, pm := range c.workUnit.PhaseMetrics {
		detail := fmt.Sprintf("Phase %s: duration=%s", phase, pm.Duration)
		if pm.Agent != "" {
			detail += ", agent=" + pm.Agent
		}
		parts = append(parts, detail)
	}

	// Note any quality gate results.
	if c.workUnit.QualityGatePassed != nil {
		if *c.workUnit.QualityGatePassed {
			parts = append(parts, "Quality gate: passed")
		} else {
			parts = append(parts, fmt.Sprintf("Quality gate: failed (%s)", c.workUnit.QualityGateError))
		}
	}

	// Include tags for future retrieval.
	tags := append([]string{"learning"}, c.workUnit.Tags...)
	if c.workUnit.Source != nil {
		tags = append(tags, c.workUnit.Source.Provider)
	}

	content := strings.Join(parts, "\n")

	doc := &memory.Document{
		ID:         fmt.Sprintf("learning-%s-%d", c.workUnit.ID, time.Now().Unix()),
		TaskID:     c.workUnit.ID,
		Type:       memory.TypeSolution,
		Content:    content,
		Category:   "learnings",
		Importance: 0.8, // Learnings are high importance
		Tags:       tags,
		CreatedAt:  time.Now(),
	}

	if err := store.Store(ctx, doc); err != nil {
		slog.Warn("failed to store task learning", "task_id", c.workUnit.ID, "error", err)
	} else {
		slog.Info("task learning stored", "task_id", c.workUnit.ID, "doc_id", doc.ID)
	}
}
