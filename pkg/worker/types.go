package worker

import (
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
)

type WorkerStatus string

const (
	StatusAvailable    WorkerStatus = "available"
	StatusWorking      WorkerStatus = "working"
	StatusDisconnected WorkerStatus = "disconnected"
)

type JobType string

const (
	JobTypePlan      JobType = "plan"
	JobTypeImplement JobType = "implement"
	JobTypeReview    JobType = "review"
	JobTypeSimplify  JobType = "simplify"
	JobTypeOptimize  JobType = "optimize"
	JobTypeChat      JobType = "chat"
	JobTypeDryRun    JobType = "dry_run"
)

type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusInProgress JobStatus = "in_progress"
	JobStatusDone       JobStatus = "done"
	JobStatusFailed     JobStatus = "failed"
)

type Worker struct {
	ID         string       `json:"id"`
	Status     WorkerStatus `json:"status"`
	CurrentJob string       `json:"current_job,omitempty"`
	StartedAt  time.Time    `json:"started_at"`
	AgentName  string       `json:"agent_name,omitempty"`
	IsDefault  bool         `json:"is_default,omitempty"`

	// Agent is the underlying AI agent (not serialized)
	Agent agent.Agent `json:"-"`
}

type Job struct {
	ID          string     `json:"id"`
	Type        JobType    `json:"type"`
	WorktreeID  string     `json:"worktree_id"`
	Prompt      string     `json:"prompt"`
	Status      JobStatus  `json:"status"`
	WorkerID    string     `json:"worker_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`

	// Execution context for multi-project support
	WorkDir     string            `json:"work_dir,omitempty"`    // Directory where agent executes
	Environment map[string]string `json:"environment,omitempty"` // Environment variables for agent
	Metadata    map[string]any    `json:"metadata,omitempty"`    // Provider info, task refs, etc.
}

// JobOptions provides optional configuration when submitting a job.
type JobOptions struct {
	WorkDir     string
	Environment map[string]string
	Metadata    map[string]any
}

// Event represents a streaming event from a job.
type Event struct {
	Type      string    `json:"type"`
	JobID     string    `json:"job_id,omitempty"`
	NodeID    string    `json:"node_id,omitempty"` // Graph node that produced this event
	Content   string    `json:"content,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}
