// Package conductor provides the task lifecycle state machine for kvelmo.
// Based on flow_v2.md design specification.
package conductor

import (
	"context"
	"time"
)

// State represents a task workflow state.
// Named descriptively per design doc: "Task: Planned" not "Idle".
type State string

const (
	// Core task states from flow_v2.md.
	StateNone         State = "none"         // No active task
	StateLoaded       State = "loaded"       // Task fetched from provider, branch created
	StatePlanning     State = "planning"     // Agent generating specification (in progress)
	StatePlanned      State = "planned"      // Specification complete, ready for implementation
	StateImplementing State = "implementing" // Agent executing specification (in progress)
	StateImplemented  State = "implemented"  // Implementation complete, ready for review
	StateSimplifying  State = "simplifying"  // Agent simplifying code for clarity (optional)
	StateOptimizing   State = "optimizing"   // Agent improving code quality (optional)
	StateReviewing    State = "reviewing"    // Human review + security scan (in progress)
	StateSubmitted    State = "submitted"    // Task submitted to provider (PR created)

	// Auxiliary states.
	StateFailed  State = "failed"  // Error state (recoverable)
	StateWaiting State = "waiting" // Waiting for user input (agent question)
	StatePaused  State = "paused"  // Paused (budget limits, manual pause)
)

// Event represents a workflow event that triggers transitions.
type Event string

const (
	// Phase transitions.
	EventStart     Event = "start"     // Begin working on task (load from provider)
	EventPlan      Event = "plan"      // Enter planning phase
	EventImplement Event = "implement" // Enter implementation phase
	EventSimplify  Event = "simplify"  // Optional simplification pass
	EventOptimize  Event = "optimize"  // Optional optimization pass
	EventReview    Event = "review"    // Enter review state
	EventSubmit    Event = "submit"    // Submit to provider (PR, issue update)
	EventFinish    Event = "finish"    // Complete task

	// Phase completion.
	EventPlanDone      Event = "plan_done"      // Planning completed
	EventImplementDone Event = "implement_done" // Implementation completed
	EventSimplifyDone  Event = "simplify_done"  // Simplification completed
	EventOptimizeDone  Event = "optimize_done"  // Optimization completed
	EventReviewDone    Event = "review_done"    // Review completed

	// Navigation.
	EventUndo     Event = "undo"      // Revert to previous checkpoint
	EventUndoDone Event = "undo_done" // Undo complete
	EventRedo     Event = "redo"      // Restore next checkpoint
	EventRedoDone Event = "redo_done" // Redo complete

	// Error handling.
	EventError  Event = "error"  // Error occurred
	EventAbort  Event = "abort"  // Abort task
	EventReset  Event = "reset"  // Recover from failed state
	EventReject Event = "reject" // Review rejected, back to planning

	// Control.
	EventWait   Event = "wait"   // Agent asked a question
	EventAnswer Event = "answer" // User answered question
	EventPause  Event = "pause"  // Pause execution
	EventResume Event = "resume" // Resume after pause
	EventStop   Event = "stop"   // Stop current operation, go back to previous stable state
)

// Transition defines a valid state transition.
type Transition struct {
	From   State
	Event  Event
	To     State
	Guards []Guard
}

// Guard pairs a predicate with a human-readable failure message.
type Guard struct {
	Check   func(ctx context.Context, wu *WorkUnit) bool `tstype:"-"`
	Message string                                       // Shown when this guard fails
}

// TaskSummary is a compact representation of a task used for hierarchy context.
// It contains only the fields needed to build meaningful AI prompt sections.
type TaskSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Status      string `json:"status"`
}

// HierarchyContext holds parent and sibling task summaries for a WorkUnit.
// It is populated during task loading when the provider supports hierarchy
// (currently Wrike) and hierarchy fetching is enabled in settings.
type HierarchyContext struct {
	// Parent is the direct parent task of the current task, or nil.
	Parent *TaskSummary `json:"parent,omitempty"`
	// Siblings are other tasks sharing the same parent, capped to ~5 entries.
	Siblings []TaskSummary `json:"siblings,omitempty"`
}

// ApprovalRecord tracks who approved a transition and when.
type ApprovalRecord struct {
	ApprovedBy string    `json:"approved_by,omitempty"` // User identity (from UserID or hostname)
	ApprovedAt time.Time `json:"approved_at"`
}

// WorkUnit represents the current task being worked on.
type WorkUnit struct {
	ID             string            `json:"id"`
	ExternalID     string            `json:"external_id"` // Provider-specific ID
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Source         *Source           `json:"source"`
	Branch         string            `json:"branch"`         // Git branch name
	WorktreePath   string            `json:"worktree_path"`  // Isolated git worktree path (if used)
	Specifications []string          `json:"specifications"` // Spec file paths
	Checkpoints    []string          `json:"checkpoints"`    // Git checkpoint SHAs
	RedoStack      []string          `json:"redo_stack"`     // For redo after undo
	Jobs           []string          `json:"jobs"`           // Job IDs submitted
	Metadata       map[string]string `json:"metadata"`
	// TaskTraceID links all activity log entries across the entire task lifecycle.
	// Generated when a task is loaded and propagated through context to all phases.
	TaskTraceID string `json:"task_trace_id,omitempty"`
	// PRID stores the PR/MR ID after submission (e.g., "owner/repo#123").
	// Used by ApprovePR and MergePR conductor methods.
	PRID string `json:"pr_id,omitempty"`
	// Hierarchy holds parent and sibling context fetched from the provider.
	// Nil when hierarchy fetching is disabled or the provider does not support it.
	Hierarchy *HierarchyContext `json:"hierarchy,omitempty"`
	// QualityGate caches the result of async quality gate (run during Review).
	// nil = not yet run, true = passed, false = failed
	QualityGatePassed *bool  `json:"quality_gate_passed,omitempty"`
	QualityGateError  string `json:"quality_gate_error,omitempty"`
	// CancelledBy records who/what initiated cancellation: "user", "timeout", "policy", or "" (not cancelled).
	CancelledBy string `json:"cancelled_by,omitempty"`
	// CancelledAt records when cancellation occurred.
	CancelledAt      time.Time                 `json:"cancelled_at,omitzero"`
	Approvals        map[string]ApprovalRecord `json:"approvals,omitempty"`         // Event -> approval record
	ChecklistChecked []string                  `json:"checklist_checked,omitempty"` // Checked review items
	ContextItems     []ContextItem             `json:"context_items,omitempty"`     // Attached context references (resolved at dispatch)
	Tags             []string                  `json:"tags,omitempty"`
	Priority         int                       `json:"priority,omitempty"`
	DependsOn        []string                  `json:"depends_on,omitempty"`
	VarPoolPath      string                    `json:"var_pool_path,omitempty"` // Path to varpool.json
	// HasImplemented is set to true when EventImplementDone fires.
	// Used by guardHasImplementation to gate simplify/optimize/review transitions.
	HasImplemented bool      `json:"has_implemented,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// CheckpointMeta stores rich metadata for each checkpoint SHA.
	// Keyed by commit SHA. Persisted to task.yaml so metadata survives restarts.
	CheckpointMeta map[string]CheckpointMeta `json:"checkpoint_meta,omitempty"`
	// PhaseMetrics tracks per-phase execution metrics (duration, agent used).
	// Populated on phase completion.
	PhaseMetrics map[string]*PhaseMetrics `json:"phase_metrics,omitempty"`
	// RouteHistory tracks routing decisions made after each phase completion.
	// Useful for debugging adaptive phase progression.
	RouteHistory []RouteDecision `json:"route_history,omitempty"`
	// Forks holds active conversation forks for parallel approaches.
	Forks []ForkInfo `json:"forks,omitempty"`
}

// CheckpointMeta stores rich metadata for a single checkpoint.
// Persisted alongside bare SHA lists so checkpoint context survives socket restarts.
type CheckpointMeta struct {
	Message   string    `json:"message" yaml:"message"`
	State     string    `json:"state" yaml:"state"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// PhaseMetrics captures execution metrics for a single phase.
type PhaseMetrics struct {
	Agent    string        `json:"agent,omitempty"`
	Duration time.Duration `json:"duration"`
	// Token fields are populated when agents report usage data.
	InputTokens   int64   `json:"input_tokens,omitempty"`
	OutputTokens  int64   `json:"output_tokens,omitempty"`
	TotalTokens   int64   `json:"total_tokens,omitempty"`
	EstCostUSD    float64 `json:"est_cost_usd,omitempty"`
	RecordingPath string  `json:"recording_path,omitempty"` // Path to agent recording file for this phase
	CheckpointSHA string  `json:"checkpoint_sha,omitempty"` // Git checkpoint SHA after this phase
}

// Source represents where the task came from.
type Source struct {
	Provider  string `json:"provider"`  // "file", "github", "gitlab", "wrike"
	Reference string `json:"reference"` // "file:task.md", "github:owner/repo#123"
	URL       string `json:"url"`       // Original URL if applicable
	Content   string `json:"content"`   // Snapshot of task content
}

// StateInfo holds metadata about a state.
type StateInfo struct {
	Name        State  `json:"name"`
	Description string `json:"description"`
	Terminal    bool   `json:"terminal"` // No more transitions possible
	Phase       bool   `json:"phase"`    // Is this a main phase state
}

// StateRegistry maps states to their metadata.
var StateRegistry = map[State]StateInfo{
	StateNone: {
		Name:        StateNone,
		Description: "No active task",
		Terminal:    false,
		Phase:       true,
	},
	StateLoaded: {
		Name:        StateLoaded,
		Description: "Task fetched from provider, branch created",
		Terminal:    false,
		Phase:       true,
	},
	StatePlanning: {
		Name:        StatePlanning,
		Description: "Agent generating specification",
		Terminal:    false,
		Phase:       true,
	},
	StatePlanned: {
		Name:        StatePlanned,
		Description: "Specification complete, ready for implementation",
		Terminal:    false,
		Phase:       true,
	},
	StateImplementing: {
		Name:        StateImplementing,
		Description: "Agent executing specification",
		Terminal:    false,
		Phase:       true,
	},
	StateImplemented: {
		Name:        StateImplemented,
		Description: "Implementation complete, ready for review",
		Terminal:    false,
		Phase:       true,
	},
	StateSimplifying: {
		Name:        StateSimplifying,
		Description: "Agent simplifying code for clarity",
		Terminal:    false,
		Phase:       true,
	},
	StateOptimizing: {
		Name:        StateOptimizing,
		Description: "Agent improving code quality",
		Terminal:    false,
		Phase:       true,
	},
	StateReviewing: {
		Name:        StateReviewing,
		Description: "Human review + security scan in progress",
		Terminal:    false,
		Phase:       true,
	},
	StateSubmitted: {
		Name:        StateSubmitted,
		Description: "Task submitted to provider (PR created)",
		Terminal:    false, // Can transition to StateNone via EventFinish
		Phase:       true,
	},
	StateFailed: {
		Name:        StateFailed,
		Description: "Task failed with error",
		Terminal:    false, // Recoverable via reset
		Phase:       false,
	},
	StateWaiting: {
		Name:        StateWaiting,
		Description: "Waiting for user input",
		Terminal:    false,
		Phase:       false,
	},
	StatePaused: {
		Name:        StatePaused,
		Description: "Execution paused",
		Terminal:    false,
		Phase:       false,
	},
}
