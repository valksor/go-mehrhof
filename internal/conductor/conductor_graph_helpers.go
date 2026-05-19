package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/varpool"
	"github.com/valksor/kvelmo/internal/worker"
)

// buildGraphJobOptsForPhase creates graph.JobOpts with per-phase agent override.
func (c *Conductor) buildGraphJobOptsForPhase(phase string) graph.JobOpts {
	opts := c.buildGraphJobOpts()
	if agentName := c.resolveAgent(phase); agentName != "" {
		opts.Metadata["agent_override"] = agentName
	}

	return opts
}

// buildGraphJobOpts creates graph.JobOpts from conductor state.
// Must be called with c.mu held or from a safe context.
func (c *Conductor) buildGraphJobOpts() graph.JobOpts {
	opts := graph.JobOpts{
		WorktreeID: c.getWorkDir(),
		WorkDir:    c.getWorkDir(),
		Metadata:   make(map[string]any),
	}

	if c.workUnit != nil {
		opts.Metadata["task_id"] = c.workUnit.ID
		opts.Metadata["task_title"] = c.workUnit.Title
		if c.workUnit.ExternalID != "" {
			opts.Metadata["external_id"] = c.workUnit.ExternalID
		}
		if c.workUnit.Source != nil {
			opts.Metadata["provider"] = c.workUnit.Source.Provider
			opts.Metadata["reference"] = c.workUnit.Source.Reference
		}
	}

	return opts
}

// partialResultsKey returns the varpool key for caching partial graph results.
func partialResultsKey(phase string) string {
	return "_graph_partial_results_" + phase
}

// savePartialResults saves completed node results to the varpool so they can
// be restored on retry, enabling partial re-execution.
// Must be called with c.mu held.
func (c *Conductor) savePartialResults(sched *graph.Scheduler, completionEvent Event) {
	phase := phaseFromEvent(completionEvent)
	if phase == "" || c.varPool == nil || sched == nil {
		return
	}

	results := sched.State().Results()

	// Filter to only nodes that completed successfully (exclude internal keys).
	completed := make(map[graph.NodeID]string, len(results))
	for id, val := range results {
		if strings.HasPrefix(string(id), "__") {
			continue
		}
		if sched.State().Get(id) == graph.StateDone {
			completed[id] = val
		}
	}

	if len(completed) == 0 {
		return
	}

	data, err := json.Marshal(completed)
	if err != nil {
		slog.Warn("failed to marshal partial results", "phase", phase, "error", err)

		return
	}

	c.varPool.SetScoped(varpool.ScopeSystem, partialResultsKey(phase), string(data), "graph-scheduler")
	c.persistVarPool()

	slog.Info("saved partial graph results for retry",
		"phase", phase, "completed_nodes", len(completed))
}

// loadPartialResults loads cached partial results from the varpool.
// Returns nil if no cached results exist for the phase.
// Must be called with c.mu held.
func (c *Conductor) loadPartialResults(completionEvent Event) map[graph.NodeID]string {
	phase := phaseFromEvent(completionEvent)
	if phase == "" || c.varPool == nil {
		return nil
	}

	raw := c.varPool.GetScopedString(varpool.ScopeSystem, partialResultsKey(phase))
	if raw == "" {
		return nil
	}

	var results map[graph.NodeID]string
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		slog.Warn("failed to unmarshal partial results", "phase", phase, "error", err)

		return nil
	}

	slog.Info("loaded cached partial results for resume",
		"phase", phase, "cached_nodes", len(results))

	return results
}

// clearPartialResults removes cached partial results after successful completion.
// Must be called with c.mu held.
func (c *Conductor) clearPartialResults(completionEvent Event) {
	phase := phaseFromEvent(completionEvent)
	if phase == "" || c.varPool == nil {
		return
	}

	// Match the key produced by SetScoped in savePartialResults: "sys." + name.
	c.varPool.Delete(varpool.ScopeSystem + "." + partialResultsKey(phase))
}

// createSafetyCheckpoint stages and commits all changes as a safety checkpoint.
// Must be called with c.mu held.
func (c *Conductor) createSafetyCheckpoint(ctx context.Context, message string) {
	workDir := c.getWorkDir()
	repo, err := git.Open(workDir)
	if err != nil {
		return
	}

	if err := repo.StageAll(ctx); err != nil {
		return
	}

	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if !hasChanges {
		return
	}

	sha, err := repo.Commit(ctx, c.formatCheckpointMessage(message))
	if err != nil {
		return
	}

	c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
	slog.Info("checkpoint created", "sha", sha, "message", message)
}

// createCompletionCheckpoint creates the post-job checkpoint.
// Must be called with c.mu held.
func (c *Conductor) createCompletionCheckpoint(ctx context.Context, completionEvent Event) {
	workDir := c.getWorkDir()
	repo, err := git.Open(workDir)
	if err != nil {
		slog.Debug("checkpoint: git open failed", "error", err, "workDir", workDir)

		return
	}

	if stageErr := repo.StageAll(ctx); stageErr != nil {
		slog.Warn("checkpoint: stage failed", "error", stageErr, "workDir", workDir)

		return
	}

	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if hasChanges {
		sha, commitErr := repo.Commit(ctx, c.formatCheckpointMessage(fmt.Sprintf("%s complete", completionEvent)))
		if commitErr == nil {
			c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
			slog.Info("checkpoint created", "sha", sha, "event", completionEvent)
		} else {
			slog.Warn("checkpoint: commit failed", "error", commitErr, "workDir", workDir)
		}
	} else {
		// Capture agent commits if any.
		if headSHA, headErr := repo.CurrentCommit(ctx); headErr == nil && headSHA != "" {
			if !slices.Contains(c.workUnit.Checkpoints, headSHA) {
				c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, headSHA)
				slog.Info("checkpoint captured (agent commit)", "sha", headSHA, "event", completionEvent)
			}
		}
	}
}

// buildPhaseGraph creates a graph for a phase.
// Checks for a YAML graph definition at <workDir>/.kvelmo/graphs/<phase>.yaml.
// Falls back to a single-node graph when no definition exists.
func buildPhaseGraph(jobType worker.JobType, label, prompt, workDir string) *graph.Graph {
	if workDir != "" {
		phase := string(jobType)
		defPath := filepath.Join(workDir, ".kvelmo", "graphs", phase+".yaml")

		g, err := graph.ParseGraphDefFile(defPath)
		if err == nil {
			slog.Info(
				"graph: loaded phase graph definition",
				"phase", phase,
				"path", defPath,
				"nodes", g.NodeCount(),
			)

			return g
		} else if !os.IsNotExist(err) {
			slog.Warn(
				"graph: failed to parse phase graph definition, using default",
				"phase", phase,
				"path", defPath,
				"error", err,
			)
		}
	}

	return graph.SingleNode(graph.NodeID(string(jobType)), label, jobType, prompt)
}
