package conductor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Exported phase name constants used as the canonical short string for each
// workflow phase across the conductor, MCP server, and socket layers. These
// are stable wire-format values (used in MCP tool arguments, CLI flags, and
// log messages) and are intentionally kept separate from the Event* values
// (which are the state-machine transition vocabulary) — they happen to
// match today but the conceptual coupling is one-way.
const (
	PhasePlan      = "plan"
	PhaseImplement = "implement"
	PhaseSimplify  = "simplify"
	PhaseOptimize  = "optimize"
	PhaseReview    = "review"
	PhaseSubmit    = "submit"
)

// Guard failure messages reused across state_table transitions.
const (
	msgNoCheckpointsToUndo = "no checkpoints to undo"
	msgNoImplementationYet = "no implementation yet. Run implement first"
)

// Artifact kinds accepted by SaveArtifact when called via the MCP server's
// kvelmo_save_artifact tool.
const (
	artifactKindSpec           = "spec"
	artifactKindSpecification  = "specification"
	artifactKindPlan           = "plan"
	artifactKindImplementation = "implementation"
	artifactKindImplSummary    = "implementation_summary"
)

// SaveArtifact persists a free-form artifact produced by an MCP-driven agent
// session. It is a thin dispatcher over the existing per-kind storage paths so
// that the kvelmo MCP server (see internal/mcp) can write content on the
// agent's behalf without taking on the storage details.
//
// Supported kinds:
//   - "spec" / "specification": delegates to SaveSpecification (numbered
//     specification file under .valksor/work/<task-id>/specifications/).
//   - "plan": writes content as plan-<n>.md under .valksor/work/<task-id>/plans/.
//   - "implementation_summary": writes content as implementation-<n>.md under
//     .valksor/work/<task-id>/implementations/.
//
// Other kinds return an error so we fail loudly rather than silently dropping
// artifacts on the floor.
func (c *Conductor) SaveArtifact(_ context.Context, kind, content string) (string, error) {
	switch kind {
	case artifactKindSpec, artifactKindSpecification:
		return c.SaveSpecification(content)
	case artifactKindPlan:
		return c.saveSimpleArtifact("plans", artifactKindPlan, content)
	case artifactKindImplSummary, artifactKindImplementation:
		return c.saveSimpleArtifact("implementations", artifactKindImplementation, content)
	default:
		return "", fmt.Errorf("unsupported artifact kind: %s", kind)
	}
}

// saveSimpleArtifact writes content as <basename>-<n>.md under
// .valksor/work/<task-id>/<subdir>/, choosing the next sequence number.
func (c *Conductor) saveSimpleArtifact(subdir, basename, content string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workUnit == nil {
		return "", errors.New("no task loaded")
	}
	if c.store == nil {
		return "", errors.New("no storage configured")
	}

	dir := filepath.Join(c.store.WorkDir(c.workUnit.ID), subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s dir: %w", subdir, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %s dir: %w", subdir, err)
	}
	next := 1
	prefix := basename + "-"
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			next++
		}
	}

	path := filepath.Join(dir, fmt.Sprintf("%s-%d.md", basename, next))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	c.workUnit.UpdatedAt = time.Now()
	c.persistState()

	return path, nil
}

// phaseLabelFromEvent returns the short phase label used in error messages
// (e.g. "plan" for EventPlanDone). Falls back to the raw event name for
// unknown events. Only handles *Done events — all other events fall through
// to the default branch.
//
//nolint:exhaustive // intentionally only covers *Done events; other events fall through to string(e)
func phaseLabelFromEvent(e Event) string {
	switch e {
	case EventPlanDone:
		return PhasePlan
	case EventImplementDone:
		return PhaseImplement
	case EventSimplifyDone:
		return PhaseSimplify
	case EventOptimizeDone:
		return PhaseOptimize
	case EventReviewDone:
		return PhaseReview
	}

	return string(e)
}

// FinalizePhase performs the post-phase side effects that handleGraphCompletion
// runs on the success path: detecting any specification files the agent wrote,
// copying spec/plan files into the repo (if configured), committing them, and
// creating the completion checkpoint.
//
// MCP-driven agent sessions call this after signal_complete so the kvelmo state
// machine sees the same artifacts and checkpoint as the existing graph path.
//
// The corresponding state machine event (EventPlanDone, EventImplementDone, ...)
// is the caller's responsibility — FinalizePhase only owns the post-phase side
// effects, not the state transition itself.
func (c *Conductor) FinalizePhase(ctx context.Context, completionEvent Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.finalizePhaseLocked(ctx, completionEvent)
}

// finalizePhaseLocked is the shared implementation used both by FinalizePhase
// (acquires lock) and the graph success branch (already holds lock).
//
// Must be called with c.mu held.
func (c *Conductor) finalizePhaseLocked(ctx context.Context, completionEvent Event) error {
	if c.workUnit == nil {
		return errors.New("no task loaded")
	}

	if completionEvent == EventPlanDone {
		c.detectSpecificationFiles()
		if len(c.workUnit.Specifications) == 0 {
			return errors.New("plan phase produced no specification file")
		}
		c.copySpecsToRepo()
		c.copyPlanToRepo()
		c.commitRepoSpecs(ctx)
	}

	c.createCompletionCheckpoint(ctx, completionEvent)

	return nil
}
