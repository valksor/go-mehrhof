package socket

import "github.com/valksor/kvelmo/pkg/conductor"

// --- Types ---

type TaskState string

const (
	StateNone         TaskState = "none"
	StateLoaded       TaskState = "loaded"
	StatePlanning     TaskState = "planning"
	StatePlanned      TaskState = "planned"
	StateImplementing TaskState = "implementing"
	StateImplemented  TaskState = "implemented"
	StateOptimizing   TaskState = "optimizing"
	StateReviewing    TaskState = "reviewing"
	StateSubmitted    TaskState = "submitted"
	StateFailed       TaskState = "failed"
	StateWaiting      TaskState = "waiting"
	StatePaused       TaskState = "paused"
)

type StatusResult struct {
	State            TaskState                          `json:"state"`
	Path             string                             `json:"path"`
	Task             *TaskInfo                          `json:"task,omitempty"`
	PendingPromptID  string                             `json:"pending_prompt_id,omitempty"`
	ActiveJobID      string                             `json:"active_job_id,omitempty"`
	QueueDepth       int                                `json:"queue_depth,omitempty"`
	LastError        string                             `json:"last_error,omitempty"`
	LastFailureClass string                             `json:"last_failure_class,omitempty"`
	PhaseMetrics     map[string]*conductor.PhaseMetrics `json:"phase_metrics,omitempty"`
	NeedsRecovery    string                             `json:"needs_recovery,omitempty"` // Interrupted phase name if recovery needed
	SkipPhases       []string                           `json:"skip_phases,omitempty"`    // Phases that will be skipped
}

type TaskInfo struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Source       string                  `json:"source"`
	Branch       string                  `json:"branch,omitempty"`
	WorktreePath string                  `json:"worktree_path,omitempty"`
	ContextItems []conductor.ContextItem `json:"context_items,omitempty"`
}
