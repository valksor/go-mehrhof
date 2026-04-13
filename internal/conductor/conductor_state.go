package conductor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/valksor/kvelmo/agent/strategy"
	"github.com/valksor/kvelmo/internal/eventlog"
	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/storage"
)

// Notifier delivers notifications about conductor events to external systems
// (e.g. Slack webhooks, email). Implementations should be safe for concurrent use.
type Notifier interface {
	Notify(ctx context.Context, eventType string, data map[string]any) error
}

// QualityRunner executes quality gates against project code and reports
// whether the gates passed along with any findings.
type QualityRunner interface {
	RunGates(ctx context.Context, workDir string) (passed bool, findings []string, err error)
}

// MetricsRecorder records phase-level metrics for observability dashboards.
type MetricsRecorder interface {
	RecordPhaseTransition(phase string, duration time.Duration, err error)
}

// TaskGroupChecker verifies whether a task is allowed to submit based on
// cross-repo task group readiness. When syncSubmit is true and the task
// belongs to a group, all group members must be ready before any can submit.
type TaskGroupChecker interface {
	CanSubmit(taskID string, syncSubmit bool) (bool, error)
}

// SetTaskGroupChecker configures the task group checker used during submit.
// Safe to call with nil (disables task group checking).
func (c *Conductor) SetTaskGroupChecker(checker TaskGroupChecker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskGroupChecker = checker
}

// SetNotifier configures an optional notification service.
// Safe to call with nil (clears the notifier).
func (c *Conductor) SetNotifier(n Notifier) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifier = n
}

// SetQualityRunner configures an optional quality gate runner.
// Safe to call with nil (clears the runner).
func (c *Conductor) SetQualityRunner(q QualityRunner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.qualityRunner = q
}

// SetMetricsRecorder configures an optional phase-level metrics recorder.
// Safe to call with nil (clears the recorder).
func (c *Conductor) SetMetricsRecorder(m MetricsRecorder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricsRecorder = m
}

// SetEventLog configures the event log for orchestration state auditing.
func (c *Conductor) SetEventLog(log *eventlog.Log) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventLog = log
}

// EventLog returns the configured event log, or nil if not set.
func (c *Conductor) EventLog() *eventlog.Log {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.eventLog
}

// emitEventLog appends an entry to the event log if configured.
// Non-blocking: logs on error but never blocks the caller.
func (c *Conductor) emitEventLog(entry eventlog.Entry) {
	if c.eventLog == nil {
		return
	}
	if err := c.eventLog.Append(entry); err != nil {
		slog.Warn("event log append failed", "type", entry.Type, "error", err)
	}
}

// SetMemoryIndexer configures the memory indexer used to index task artefacts
// after major phase completions (plan, implement, submit).
// Calling this is optional; if not set, memory indexing is skipped.
func (c *Conductor) SetMemoryIndexer(indexer *memory.Indexer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memoryIndexer = indexer
}

// SetStore configures the storage.Store used for persisting specifications,
// reviews, and session metadata. Must be called before using storage-dependent
// operations like saving specifications.
func (c *Conductor) SetStore(store *storage.Store) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = store
}

// SetStrategy sets the default agent strategy. Per-phase overrides
// (set via SetPhaseStrategy) take precedence over this default.
func (c *Conductor) SetStrategy(s strategy.Strategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.strategy = s
}

// SetPhaseStrategy sets the agent strategy for a specific phase.
// Pass nil to clear the override and fall back to the default strategy.
func (c *Conductor) SetPhaseStrategy(phase string, s strategy.Strategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phaseStrategies == nil {
		c.phaseStrategies = make(map[string]strategy.Strategy)
	}
	if s == nil {
		delete(c.phaseStrategies, phase)
	} else {
		c.phaseStrategies[phase] = s
	}
}

// archiveTask records the current work unit in the archive before clearing.
// Non-fatal: logs on error. Caller must hold c.mu.
func (c *Conductor) archiveTask(finalState string) {
	if c.store == nil || c.workUnit == nil {
		return
	}

	source := ""
	if c.workUnit.Source != nil {
		source = c.workUnit.Source.Reference
	}

	archived := storage.ArchivedTask{
		ID:          c.workUnit.ID,
		Title:       c.workUnit.Title,
		Description: c.workUnit.Description,
		Branch:      c.workUnit.Branch,
		Source:      source,
		FinalState:  finalState,
		StartedAt:   c.workUnit.CreatedAt,
		CompletedAt: time.Now(),
		Tags:        c.workUnit.Tags,
	}

	// Record which phases ran and which agent was used (for pattern detection).
	if c.workUnit.PhaseMetrics != nil {
		phases := make([]string, 0, len(c.workUnit.PhaseMetrics))
		for phase := range c.workUnit.PhaseMetrics {
			phases = append(phases, phase)
		}
		slices.Sort(phases)
		for _, phase := range phases {
			pm := c.workUnit.PhaseMetrics[phase]
			archived.PhasesRun = append(archived.PhasesRun, phase)
			if archived.AgentUsed == "" && pm.Agent != "" {
				archived.AgentUsed = pm.Agent
			}
		}
	}

	if err := c.store.ArchiveTask(archived); err != nil {
		slog.Warn("archive task failed", "task_id", c.workUnit.ID, "error", err)
	}
}

// TaskHistory returns archived tasks for this project.
func (c *Conductor) TaskHistory() ([]storage.ArchivedTask, error) {
	if c.store == nil {
		return nil, nil
	}

	return c.store.ListArchivedTasks()
}

// SearchTaskHistory returns archived tasks matching the given filters.
func (c *Conductor) SearchTaskHistory(opts storage.SearchOptions) ([]storage.ArchivedTask, error) {
	if c.store == nil {
		return nil, nil
	}

	return c.store.SearchArchivedTasks(opts)
}

// persistState writes the current WorkUnit and state to task.yaml.
// Non-fatal: logs on error and never blocks the caller.
// Safe to call without c.mu held - reads only stable/atomic fields.
func (c *Conductor) persistState() {
	if c.store == nil || c.workUnit == nil {
		return
	}

	// Persist variable pool first so VarPoolPath is set before snapshot.
	c.persistVarPool()

	history := c.machine.History()
	ts := workUnitToTaskState(c.machine.State(), c.workUnit, history)
	ts.ProjectRoot = c.worktree // Tag task with owning project for cross-project filtering
	if err := c.store.SaveTaskState(ts); err != nil {
		slog.Warn("persist task state failed", "task_id", c.workUnit.ID, "error", err)
	}
}

// LoadState restores WorkUnit and state machine from task.yaml written by a prior session.
// No-op if task.yaml does not exist or no store is configured.
func (c *Conductor) LoadState(ctx context.Context) error {
	// Read store under lock to avoid race with SetStore
	c.mu.RLock()
	store := c.store
	c.mu.RUnlock()

	if store == nil {
		return nil
	}

	taskID, err := store.FindActiveTask()
	if err != nil {
		return fmt.Errorf("find active task: %w", err)
	}
	if taskID == "" {
		return nil
	}

	ts, err := store.LoadTaskState(taskID)
	if err != nil {
		// Use errors.Is for proper handling of wrapped errors
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("load task state: %w", err)
	}

	state, wu, history := taskStateToWorkUnit(ts)
	c.mu.Lock()
	c.workUnit = wu
	c.machine.ForceState(state)
	c.machine.SetWorkUnit(wu)
	c.machine.RestoreHistory(history)
	c.loadVarPool()
	c.mu.Unlock()

	slog.Info("task state restored", "task_id", taskID, "state", state)

	// Check for interrupted phase and log a warning.
	if isActivePhaseState(state) {
		slog.Warn("task was interrupted mid-phase", "task_id", taskID, "state", state,
			"hint", "run 'kvelmo retry' to resume or 'kvelmo reset' to start over")
	}

	return nil
}

// NeedsRecovery returns true if the current state indicates the task was
// interrupted during an active phase (planning, implementing, etc.).
func (c *Conductor) NeedsRecovery() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.workUnit == nil {
		return false
	}

	return isActivePhaseState(c.machine.State())
}

// RecoveryState returns the interrupted phase name, or empty string if no recovery needed.
func (c *Conductor) RecoveryState() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.workUnit == nil {
		return ""
	}
	state := c.machine.State()
	if isActivePhaseState(state) {
		return string(state)
	}

	return ""
}

// isActivePhaseState returns true for states that represent mid-execution phases.
func isActivePhaseState(s State) bool {
	switch s {
	case StatePlanning, StateImplementing, StateSimplifying, StateOptimizing, StateReviewing:
		return true
	case StateNone, StateLoaded, StatePlanned, StateImplemented, StateSubmitted,
		StateFailed, StateWaiting, StatePaused:
		return false
	}

	return false
}
