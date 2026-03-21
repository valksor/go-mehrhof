// Package conductor provides the task lifecycle state machine for kvelmo.
// Based on flow_v2.md design specification.
package conductor

import (
	"context"
	"fmt"
	"sync"
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
	Check   func(ctx context.Context, wu *WorkUnit) bool
	Message string // Shown when this guard fails
}

// GuardFunc is a predicate that must return true for a transition to occur.
//
// Deprecated: Use Guard struct instead for better error messages.
type GuardFunc func(ctx context.Context, wu *WorkUnit) bool

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
	CancelledAt      time.Time                 `json:"cancelled_at,omitempty"`
	Approvals        map[string]ApprovalRecord `json:"approvals,omitempty"`         // Event -> approval record
	ChecklistChecked []string                  `json:"checklist_checked,omitempty"` // Checked review items
	Tags             []string                  `json:"tags,omitempty"`
	Priority         int                       `json:"priority,omitempty"`
	DependsOn        []string                  `json:"depends_on,omitempty"`
	VarPoolPath      string                    `json:"var_pool_path,omitempty"` // Path to varpool.json
	// HasImplemented is set to true when EventImplementDone fires.
	// Used by guardHasImplementation to gate simplify/optimize/review transitions.
	HasImplemented bool      `json:"has_implemented,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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

// TransitionKey uniquely identifies a state+event pair.
type TransitionKey struct {
	From  State
	Event Event
}

// TransitionTable defines all valid transitions per the design doc state machine.
// flow_v2.md state diagram:
//
//	None -> Loaded (start)
//	Loaded -> Planning (plan)
//	Planning -> Planned (plan_done)
//	Planned -> Implementing (implement)
//	Implementing -> Implemented (implement_done)
//	Implemented -> Reviewing (review)
//	Reviewing -> Submitted (submit)
//	Reviewing -> Planning (reject/revise)
var TransitionTable = map[TransitionKey][]Transition{
	// === Start: Load task from provider ===
	{StateNone, EventStart}: {
		{From: StateNone, Event: EventStart, To: StateLoaded, Guards: []Guard{
			{Check: guardHasSource, Message: "no task source specified. Run: kvelmo start --from <provider:reference>"},
		}},
	},

	// === Planning Phase ===
	{StateLoaded, EventPlan}: {
		{From: StateLoaded, Event: EventPlan, To: StatePlanning, Guards: []Guard{
			{Check: guardHasDescription, Message: "task has no description. Check the task source content"},
		}},
	},
	{StatePlanning, EventPlanDone}: {
		{From: StatePlanning, Event: EventPlanDone, To: StatePlanned},
	},
	{StatePlanning, EventError}: {
		{From: StatePlanning, Event: EventError, To: StateLoaded},
	},
	{StatePlanning, EventWait}: {
		{From: StatePlanning, Event: EventWait, To: StateWaiting},
	},
	{StatePlanning, EventPause}: {
		{From: StatePlanning, Event: EventPause, To: StatePaused},
	},

	// === Implementation Phase ===
	{StatePlanned, EventImplement}: {
		{From: StatePlanned, Event: EventImplement, To: StateImplementing, Guards: []Guard{
			{Check: guardHasSpecifications, Message: "no specification found. Run: kvelmo plan first"},
		}},
	},
	// Skip planning: implement directly from loaded state using task description as spec.
	{StateLoaded, EventImplement}: {
		{From: StateLoaded, Event: EventImplement, To: StateImplementing, Guards: []Guard{
			{Check: guardHasDescription, Message: "task has no description. Check the task source content"},
		}},
	},
	{StateImplementing, EventImplementDone}: {
		{From: StateImplementing, Event: EventImplementDone, To: StateImplemented},
	},
	{StateImplementing, EventError}: {
		{From: StateImplementing, Event: EventError, To: StatePlanned},
	},
	{StateImplementing, EventWait}: {
		{From: StateImplementing, Event: EventWait, To: StateWaiting},
	},
	{StateImplementing, EventPause}: {
		{From: StateImplementing, Event: EventPause, To: StatePaused},
	},
	{StateImplementing, EventUndo}: {
		{From: StateImplementing, Event: EventUndo, To: StateImplementing, Guards: []Guard{
			{Check: guardCanUndo, Message: "no checkpoints to undo"},
		}},
	},

	// === Simplification Phase (optional) ===
	{StateImplemented, EventSimplify}: {
		{From: StateImplemented, Event: EventSimplify, To: StateSimplifying},
	},
	{StateSimplifying, EventSimplifyDone}: {
		{From: StateSimplifying, Event: EventSimplifyDone, To: StateImplemented},
	},
	{StateSimplifying, EventError}: {
		{From: StateSimplifying, Event: EventError, To: StateImplemented},
	},
	{StateSimplifying, EventWait}: {
		{From: StateSimplifying, Event: EventWait, To: StateWaiting},
	},
	{StateSimplifying, EventPause}: {
		{From: StateSimplifying, Event: EventPause, To: StatePaused},
	},
	{StateSimplifying, EventAbort}: {
		{From: StateSimplifying, Event: EventAbort, To: StateFailed},
	},

	// === Optimization Phase (optional) ===
	{StateImplemented, EventOptimize}: {
		{From: StateImplemented, Event: EventOptimize, To: StateOptimizing},
	},
	{StateOptimizing, EventOptimizeDone}: {
		{From: StateOptimizing, Event: EventOptimizeDone, To: StateImplemented},
	},
	{StateOptimizing, EventError}: {
		{From: StateOptimizing, Event: EventError, To: StateImplemented},
	},
	{StateOptimizing, EventWait}: {
		{From: StateOptimizing, Event: EventWait, To: StateWaiting},
	},
	{StateOptimizing, EventPause}: {
		{From: StateOptimizing, Event: EventPause, To: StatePaused},
	},
	{StateOptimizing, EventAbort}: {
		{From: StateOptimizing, Event: EventAbort, To: StateFailed},
	},

	// === Review Phase ===
	{StateImplemented, EventReview}: {
		{From: StateImplemented, Event: EventReview, To: StateReviewing},
	},
	{StateReviewing, EventSubmit}: {
		{From: StateReviewing, Event: EventSubmit, To: StateSubmitted, Guards: []Guard{
			{Check: guardCanSubmit, Message: "cannot submit: no provider configured"},
			{Check: guardQualityGatePassed, Message: "quality gate failed. Fix issues and re-run: kvelmo review"},
		}},
	},
	{StateReviewing, EventReject}: {
		{From: StateReviewing, Event: EventReject, To: StatePlanning},
	},
	{StateReviewing, EventError}: {
		{From: StateReviewing, Event: EventError, To: StateImplemented},
	},

	// === Waiting (user input needed) ===
	// Note: DispatchWithResume overrides the target state to the previous state.
	// This fallback to StateLoaded is a safety net if Dispatch is called directly.
	{StateWaiting, EventAnswer}: {
		{From: StateWaiting, Event: EventAnswer, To: StateLoaded},
	},
	{StateWaiting, EventAbort}: {
		{From: StateWaiting, Event: EventAbort, To: StateFailed},
	},

	// === Paused ===
	// Note: DispatchWithResume overrides the target state to the previous state.
	// This fallback to StateLoaded is a safety net if Dispatch is called directly.
	{StatePaused, EventResume}: {
		{From: StatePaused, Event: EventResume, To: StateLoaded},
	},
	{StatePaused, EventAbort}: {
		{From: StatePaused, Event: EventAbort, To: StateFailed},
	},

	// === Failed State Recovery ===
	{StateFailed, EventReset}: {
		{From: StateFailed, Event: EventReset, To: StateLoaded},
	},

	// === Undo/Redo from stable states ===
	{StateLoaded, EventUndo}: {
		{From: StateLoaded, Event: EventUndo, To: StateLoaded, Guards: []Guard{
			{Check: guardCanUndo, Message: "no checkpoints to undo"},
		}},
	},
	{StatePlanned, EventUndo}: {
		{From: StatePlanned, Event: EventUndo, To: StatePlanned, Guards: []Guard{
			{Check: guardCanUndo, Message: "no checkpoints to undo"},
		}},
	},
	{StateImplemented, EventUndo}: {
		{From: StateImplemented, Event: EventUndo, To: StateImplemented, Guards: []Guard{
			{Check: guardCanUndo, Message: "no checkpoints to undo"},
		}},
	},
	{StateLoaded, EventRedo}: {
		{From: StateLoaded, Event: EventRedo, To: StateLoaded, Guards: []Guard{
			{Check: guardCanRedo, Message: "no checkpoints to redo"},
		}},
	},
	{StatePlanned, EventRedo}: {
		{From: StatePlanned, Event: EventRedo, To: StatePlanned, Guards: []Guard{
			{Check: guardCanRedo, Message: "no checkpoints to redo"},
		}},
	},
	{StateImplemented, EventRedo}: {
		{From: StateImplemented, Event: EventRedo, To: StateImplemented, Guards: []Guard{
			{Check: guardCanRedo, Message: "no checkpoints to redo"},
		}},
	},

	// === Finish: Clean up after PR merge ===
	{StateSubmitted, EventFinish}: {
		{From: StateSubmitted, Event: EventFinish, To: StateNone},
	},

	// === Abort from any active phase ===
	{StateLoaded, EventAbort}: {
		{From: StateLoaded, Event: EventAbort, To: StateFailed},
	},
	{StatePlanning, EventAbort}: {
		{From: StatePlanning, Event: EventAbort, To: StateFailed},
	},
	{StatePlanned, EventAbort}: {
		{From: StatePlanned, Event: EventAbort, To: StateFailed},
	},
	{StateImplementing, EventAbort}: {
		{From: StateImplementing, Event: EventAbort, To: StateFailed},
	},
	{StateImplemented, EventAbort}: {
		{From: StateImplemented, Event: EventAbort, To: StateFailed},
	},
	{StateReviewing, EventAbort}: {
		{From: StateReviewing, Event: EventAbort, To: StateFailed},
	},

	// === Stop (graceful interrupt, returns to previous stable state) ===
	{StatePlanning, EventStop}: {
		{From: StatePlanning, Event: EventStop, To: StateLoaded},
	},
	{StateImplementing, EventStop}: {
		{From: StateImplementing, Event: EventStop, To: StatePlanned},
	},
	{StateSimplifying, EventStop}: {
		{From: StateSimplifying, Event: EventStop, To: StateImplemented},
	},
	{StateOptimizing, EventStop}: {
		{From: StateOptimizing, Event: EventStop, To: StateImplemented},
	},
	{StateReviewing, EventStop}: {
		{From: StateReviewing, Event: EventStop, To: StateImplemented},
	},

	// === Free phase movement from stable states ===

	// Re-plan from any stable state
	{StatePlanned, EventPlan}: {
		{From: StatePlanned, Event: EventPlan, To: StatePlanning, Guards: []Guard{
			{Check: guardHasDescription, Message: "task has no description"},
		}},
	},
	{StateImplemented, EventPlan}: {
		{From: StateImplemented, Event: EventPlan, To: StatePlanning, Guards: []Guard{
			{Check: guardHasDescription, Message: "task has no description"},
		}},
	},
	{StateSubmitted, EventPlan}: {
		{From: StateSubmitted, Event: EventPlan, To: StatePlanning, Guards: []Guard{
			{Check: guardHasDescription, Message: "task has no description"},
		}},
	},

	// Re-implement from any stable state
	{StateImplemented, EventImplement}: {
		{From: StateImplemented, Event: EventImplement, To: StateImplementing, Guards: []Guard{
			{Check: guardHasSpecifications, Message: "no specifications found. Run plan first"},
		}},
	},
	{StateSubmitted, EventImplement}: {
		{From: StateSubmitted, Event: EventImplement, To: StateImplementing, Guards: []Guard{
			{Check: guardHasSpecifications, Message: "no specifications found. Run plan first"},
		}},
	},

	// Simplify/optimize from any stable state (where code exists)
	{StatePlanned, EventSimplify}: {
		{From: StatePlanned, Event: EventSimplify, To: StateSimplifying, Guards: []Guard{
			{Check: guardHasImplementation, Message: "no implementation yet. Run implement first"},
		}},
	},
	{StatePlanned, EventOptimize}: {
		{From: StatePlanned, Event: EventOptimize, To: StateOptimizing, Guards: []Guard{
			{Check: guardHasImplementation, Message: "no implementation yet. Run implement first"},
		}},
	},
	{StateSubmitted, EventSimplify}: {
		{From: StateSubmitted, Event: EventSimplify, To: StateSimplifying, Guards: []Guard{
			{Check: guardHasImplementation, Message: "no implementation yet. Run implement first"},
		}},
	},
	{StateSubmitted, EventOptimize}: {
		{From: StateSubmitted, Event: EventOptimize, To: StateOptimizing, Guards: []Guard{
			{Check: guardHasImplementation, Message: "no implementation yet. Run implement first"},
		}},
	},

	// Review from any stable state (where code exists)
	{StatePlanned, EventReview}: {
		{From: StatePlanned, Event: EventReview, To: StateReviewing, Guards: []Guard{
			{Check: guardHasImplementation, Message: "no implementation yet. Run implement first"},
		}},
	},
	{StateSubmitted, EventReview}: {
		{From: StateSubmitted, Event: EventReview, To: StateReviewing},
	},

	// Undo/Redo from Submitted
	{StateSubmitted, EventUndo}: {
		{From: StateSubmitted, Event: EventUndo, To: StateSubmitted, Guards: []Guard{
			{Check: guardCanUndo, Message: "no checkpoints to undo"},
		}},
	},
	{StateSubmitted, EventRedo}: {
		{From: StateSubmitted, Event: EventRedo, To: StateSubmitted, Guards: []Guard{
			{Check: guardCanRedo, Message: "nothing to redo"},
		}},
	},

	// Abort from Submitted
	{StateSubmitted, EventAbort}: {
		{From: StateSubmitted, Event: EventAbort, To: StateFailed},
	},
}

// Guard functions

func guardHasSource(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && wu.Source != nil && wu.Source.Reference != ""
}

func guardHasDescription(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && wu.Description != ""
}

func guardHasSpecifications(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Specifications) > 0
}

func guardCanUndo(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Checkpoints) > 0
}

func guardCanRedo(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.RedoStack) > 0
}

// guardHasImplementation checks that code has been implemented at least once.
// Uses explicit HasImplemented flag — NOT checkpoint count, which is unreliable
// (safety checkpoints from Plan() inflate the count before any code is written).
func guardHasImplementation(_ context.Context, wu *WorkUnit) bool {
	return wu != nil && wu.HasImplemented
}

func guardCanSubmit(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && wu.Source != nil && wu.Source.Provider != ""
}

// guardQualityGatePassed checks that quality gates passed (or haven't been run yet).
// When QualityGatePassed is nil (not yet run), we allow submission to avoid blocking
// workflows that don't use quality gates. When explicitly false, we block.
func guardQualityGatePassed(_ context.Context, wu *WorkUnit) bool {
	if wu == nil {
		return false
	}
	// nil = not yet run → allow (don't block non-quality-gate workflows)
	// true = passed → allow
	// false = failed → block
	return wu.QualityGatePassed == nil || *wu.QualityGatePassed
}

// EvaluateGuards checks if all guards pass for a transition.
func EvaluateGuards(ctx context.Context, wu *WorkUnit, guards []Guard) bool {
	for _, guard := range guards {
		if !guard.Check(ctx, wu) {
			return false
		}
	}

	return true
}

// formatTransitionError creates a user-friendly error when no transition exists.
func formatTransitionError(from State, event Event, wu *WorkUnit) error {
	stateDesc := stateDescription(from)
	actionDesc := eventDescription(event)
	suggestion := suggestNextAction(from, wu)

	return fmt.Errorf("cannot %s: task is %s. %s", actionDesc, stateDesc, suggestion)
}

// formatGuardError creates a user-friendly error when guards fail.
// Each Guard carries its own failure message, so the error is always precise.
func formatGuardError(_ State, event Event, wu *WorkUnit, transitions []Transition) error {
	actionDesc := eventDescription(event)

	for _, t := range transitions {
		for _, guard := range t.Guards {
			if !guard.Check(context.Background(), wu) {
				return fmt.Errorf("cannot %s: %s", actionDesc, guard.Message)
			}
		}
	}

	return fmt.Errorf("cannot %s: prerequisites not met", actionDesc)
}

// stateDescription returns a human-readable description of a state.
func stateDescription(s State) string {
	switch s {
	case StateNone:
		return "not started"
	case StateLoaded:
		return "loaded but not planned"
	case StatePlanning:
		return "currently planning"
	case StatePlanned:
		return "planned but not implemented"
	case StateImplementing:
		return "currently implementing"
	case StateImplemented:
		return "implemented but not reviewed"
	case StateSimplifying:
		return "currently simplifying"
	case StateOptimizing:
		return "currently optimizing"
	case StateReviewing:
		return "under review"
	case StateSubmitted:
		return "submitted — can re-enter plan/implement/review or finish"
	case StateFailed:
		return "in failed state"
	case StateWaiting:
		return "waiting for your input"
	case StatePaused:
		return "paused"
	default:
		return string(s)
	}
}

// eventDescription returns a human-readable description of an action.
func eventDescription(e Event) string {
	switch e {
	case EventStart:
		return "start task"
	case EventPlan:
		return "start planning"
	case EventPlanDone:
		return "complete planning"
	case EventImplement:
		return "start implementation"
	case EventImplementDone:
		return "complete implementation"
	case EventSimplify:
		return "start simplification"
	case EventSimplifyDone:
		return "complete simplification"
	case EventOptimize:
		return "start optimization"
	case EventOptimizeDone:
		return "complete optimization"
	case EventReview:
		return "start review"
	case EventReviewDone:
		return "complete review"
	case EventSubmit:
		return "submit"
	case EventFinish:
		return "finish task"
	case EventUndo:
		return "undo"
	case EventUndoDone:
		return "complete undo"
	case EventRedo:
		return "redo"
	case EventRedoDone:
		return "complete redo"
	case EventError:
		return "handle error"
	case EventAbort:
		return "abort"
	case EventReset:
		return "reset"
	case EventReject:
		return "reject changes"
	case EventWait:
		return "wait for input"
	case EventAnswer:
		return "answer question"
	case EventPause:
		return "pause"
	case EventResume:
		return "resume"
	case EventStop:
		return "stop"
	}

	return string(e)
}

// suggestNextAction provides guidance on what the user should do.
func suggestNextAction(from State, _ *WorkUnit) string {
	switch from {
	case StateNone:
		return "Run: kvelmo start --from <provider:reference>"
	case StateLoaded:
		return "Run: kvelmo plan"
	case StatePlanning:
		return "Wait for planning to complete"
	case StatePlanned:
		return "Run: kvelmo implement"
	case StateImplementing:
		return "Wait for implementation to complete"
	case StateImplemented:
		return "Run: kvelmo review"
	case StateSimplifying:
		return "Wait for simplification to complete"
	case StateOptimizing:
		return "Wait for optimization to complete"
	case StateReviewing:
		return "Run: kvelmo submit"
	case StateSubmitted:
		return "PR submitted. Run: kvelmo finish (cleanup), kvelmo plan (re-plan), or kvelmo implement (re-implement)"
	case StateFailed:
		return "Run: kvelmo reset to recover"
	case StateWaiting:
		return "Answer the pending question"
	case StatePaused:
		return "Run: kvelmo resume"
	default:
		return ""
	}
}

// Machine manages workflow state transitions.
type Machine struct {
	mu sync.RWMutex

	state         State
	workUnit      *WorkUnit
	history       []HistoryEntry
	listeners     []StateListener
	previousState State // For resuming after wait/pause
	// priorStableState records the stable state before a re-entry transition.
	// Used by EventError/EventStop to roll back to the correct state instead of
	// the hardcoded default. Set by conductor before re-entry dispatches,
	// cleared after error/stop uses it.
	// Intentionally NOT persisted: only relevant while a job is running, and
	// running jobs don't survive server restarts. On restart, LoadState restores
	// the last persisted state (the in-progress state), which is acceptable.
	// Access via SetPriorStableState/ClearPriorStableState (holds m.mu).
	priorStableState State
}

// HistoryEntry records a state transition.
type HistoryEntry struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Event     Event     `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// StateListener is called when state changes.
type StateListener func(from, to State, event Event, wu *WorkUnit)

// NewMachine creates a new state machine.
func NewMachine() *Machine {
	return &Machine{
		state:   StateNone,
		history: make([]HistoryEntry, 0),
	}
}

// State returns the current state.
func (m *Machine) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.state
}

// WorkUnit returns the current work unit.
func (m *Machine) WorkUnit() *WorkUnit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.workUnit
}

// SetWorkUnit sets the work unit.
func (m *Machine) SetWorkUnit(wu *WorkUnit) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workUnit = wu
	if wu != nil {
		wu.UpdatedAt = time.Now()
	}
}

// SetPriorStableState records the stable state before a re-entry transition.
// Thread-safe: holds m.mu so callers do not need to coordinate with Dispatch.
func (m *Machine) SetPriorStableState(s State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priorStableState = s
}

// ClearPriorStableState resets the prior stable state (e.g. on dispatch failure).
func (m *Machine) ClearPriorStableState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priorStableState = ""
}

// PriorStableState returns the current prior stable state (for testing).
func (m *Machine) PriorStableState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.priorStableState
}

// AddListener registers a state change listener.
func (m *Machine) AddListener(listener StateListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// Dispatch attempts to transition based on an event.
func (m *Machine) Dispatch(ctx context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	from := m.state

	// Get possible transitions
	key := TransitionKey{From: from, Event: event}
	transitions, ok := TransitionTable[key]
	if !ok || len(transitions) == 0 {
		return formatTransitionError(from, event, m.workUnit)
	}

	// Find first transition whose guards pass
	var validTransition *Transition
	for i := range transitions {
		if EvaluateGuards(ctx, m.workUnit, transitions[i].Guards) {
			validTransition = &transitions[i]

			break
		}
	}

	if validTransition == nil {
		return formatGuardError(from, event, m.workUnit, transitions) //nolint:contextcheck // Guard check only
	}

	// Track previous state for wait/pause resume
	if event == EventWait || event == EventPause {
		m.previousState = from
	}

	// For error/stop during re-entry, override the hardcoded rollback target
	// with the actual prior stable state.
	targetState := validTransition.To
	if (event == EventError || event == EventStop) && m.priorStableState != "" {
		targetState = m.priorStableState
		m.priorStableState = "" // Consumed
	}

	// Execute transition
	m.state = targetState
	m.history = append(m.history, HistoryEntry{
		From:      from,
		To:        targetState,
		Event:     event,
		Timestamp: time.Now(),
	})

	// Update work unit timestamp
	if m.workUnit != nil {
		m.workUnit.UpdatedAt = time.Now()
	}

	// Notify listeners (copy to avoid holding lock during callbacks)
	listeners := make([]StateListener, len(m.listeners))
	copy(listeners, m.listeners)
	wu := m.workUnit

	// Call listeners outside lock
	go func() {
		for _, listener := range listeners {
			listener(from, targetState, event, wu)
		}
	}()

	return nil
}

// DispatchWithResume handles Answer/Resume events by returning to previous state.
func (m *Machine) DispatchWithResume(ctx context.Context, event Event) error {
	m.mu.Lock()

	if event == EventAnswer || event == EventResume {
		if m.previousState != "" {
			// Modify transition table temporarily to go back to previous state
			from := m.state
			to := m.previousState
			m.state = to
			m.previousState = ""
			m.history = append(m.history, HistoryEntry{
				From:      from,
				To:        to,
				Event:     event,
				Timestamp: time.Now(),
			})
			m.mu.Unlock()

			return nil
		}
	}

	m.mu.Unlock()

	return m.Dispatch(ctx, event)
}

// CanDispatch checks if a transition is possible.
func (m *Machine) CanDispatch(ctx context.Context, event Event) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := TransitionKey{From: m.state, Event: event}
	transitions, ok := TransitionTable[key]
	if !ok || len(transitions) == 0 {
		return false, formatTransitionError(m.state, event, m.workUnit).Error()
	}

	for _, t := range transitions {
		if EvaluateGuards(ctx, m.workUnit, t.Guards) {
			return true, ""
		}
	}

	return false, formatGuardError(m.state, event, m.workUnit, transitions).Error() //nolint:contextcheck // Guard check only
}

// History returns the transition history.
func (m *Machine) History() []HistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := make([]HistoryEntry, len(m.history))
	copy(history, m.history)

	return history
}

// RestoreHistory replaces the machine's transition history with the provided entries.
// Used when restoring persisted state from disk.
func (m *Machine) RestoreHistory(entries []HistoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = entries
}

// Reset resets the machine to None state.
func (m *Machine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateNone
	m.workUnit = nil
	m.history = nil
	m.previousState = ""
}

// ForceState forcefully sets the state without checking transitions.
// Used for re-running phases with --force flag.
func (m *Machine) ForceState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

// IsTerminal returns true if current state is terminal.
func (m *Machine) IsTerminal() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := StateRegistry[m.state]

	return ok && info.Terminal
}

// IsPhase returns true if current state is a main phase.
func (m *Machine) IsPhase() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := StateRegistry[m.state]

	return ok && info.Phase
}

// AvailableEvents returns events that can be dispatched from current state.
func (m *Machine) AvailableEvents(ctx context.Context) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []Event
	for key, transitions := range TransitionTable {
		if key.From != m.state {
			continue
		}
		for _, t := range transitions {
			if EvaluateGuards(ctx, m.workUnit, t.Guards) {
				events = append(events, key.Event)

				break
			}
		}
	}

	return events
}

// CanTransition checks if a direct state transition is valid.
func CanTransition(from, to State) bool {
	for key, transitions := range TransitionTable {
		if key.From != from {
			continue
		}
		for _, t := range transitions {
			if t.To == to {
				return true
			}
		}
	}

	return false
}

// NextStates returns possible next states from a given state.
func NextStates(from State) []State {
	seen := make(map[State]bool)
	var next []State
	for key, transitions := range TransitionTable {
		if key.From != from {
			continue
		}
		for _, t := range transitions {
			if !seen[t.To] {
				seen[t.To] = true
				next = append(next, t.To)
			}
		}
	}

	return next
}
