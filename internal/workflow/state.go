package workflow

// State represents a workflow state.
type State string

const (
	// Core phases.
	StateIdle         State = "idle"         // Task registered but not started
	StatePlanning     State = "planning"     // Agent creating specs
	StateImplementing State = "implementing" // Agent implementing specs
	StateReviewing    State = "reviewing"    // Code review phase
	StateDone         State = "done"         // Task completed
	StateFailed       State = "failed"       // Error state
	StateWaiting      State = "waiting"      // Waiting for user answer to agent question

	// Auxiliary states (entered during phases).
	StateCheckpointing State = "checkpointing" // Creating git checkpoint
	StateReverting     State = "reverting"     // Undo operation
	StateRestoring     State = "restoring"     // Redo operation
)

// Event represents a workflow event that triggers transitions.
type Event string

const (
	// Phase transitions.
	EventStart     Event = "start"     // Begin working on task
	EventPlan      Event = "plan"      // Enter planning phase
	EventImplement Event = "implement" // Enter implementation phase
	EventReview    Event = "review"    // Enter review phase
	EventFinish    Event = "finish"    // Complete task

	// Phase completion.
	EventPlanDone      Event = "plan_done"      // Planning completed
	EventImplementDone Event = "implement_done" // Implementation completed
	EventReviewDone    Event = "review_done"    // Review completed

	// Checkpoint operations.
	EventCheckpoint     Event = "checkpoint"
	EventCheckpointDone Event = "checkpoint_done"

	// Undo/Redo.
	EventUndo     Event = "undo"
	EventUndoDone Event = "undo_done"
	EventRedo     Event = "redo"
	EventRedoDone Event = "redo_done"

	// Error handling.
	EventError Event = "error"
	EventAbort Event = "abort"

	// Waiting for user input (agent asked a question).
	EventWait   Event = "wait"   // Agent asked a question
	EventAnswer Event = "answer" // User answered the question
	EventReset  Event = "reset"  // Recover from failed state
)

// PhaseStates are the main workflow phases.
var PhaseStates = []State{
	StateIdle,
	StatePlanning,
	StateImplementing,
	StateReviewing,
	StateDone,
}

// StateInfo holds metadata about a state.
type StateInfo struct {
	Name        State
	Description string
	Terminal    bool // No more transitions possible
	Phase       bool // Is this a main phase state
}

// StateRegistry maps states to their metadata.
var StateRegistry = map[State]StateInfo{
	StateIdle: {
		Name:        StateIdle,
		Description: "Task registered, awaiting action",
		Terminal:    false,
		Phase:       true,
	},
	StatePlanning: {
		Name:        StatePlanning,
		Description: "Agent creating specifications",
		Terminal:    false,
		Phase:       true,
	},
	StateImplementing: {
		Name:        StateImplementing,
		Description: "Agent implementing specifications",
		Terminal:    false,
		Phase:       true,
	},
	StateReviewing: {
		Name:        StateReviewing,
		Description: "Code review in progress",
		Terminal:    false,
		Phase:       true,
	},
	StateDone: {
		Name:        StateDone,
		Description: "Task completed",
		Terminal:    true,
		Phase:       true,
	},
	StateFailed: {
		Name:        StateFailed,
		Description: "Task failed with error",
		Terminal:    false, // Changed to allow recovery via EventReset
		Phase:       false,
	},
	StateWaiting: {
		Name:        StateWaiting,
		Description: "Waiting for user answer to agent question",
		Terminal:    false,
		Phase:       false,
	},
	StateCheckpointing: {
		Name:        StateCheckpointing,
		Description: "Creating git checkpoint",
		Terminal:    false,
		Phase:       false,
	},
	StateReverting: {
		Name:        StateReverting,
		Description: "Undoing to previous checkpoint",
		Terminal:    false,
		Phase:       false,
	},
	StateRestoring: {
		Name:        StateRestoring,
		Description: "Redoing to next checkpoint",
		Terminal:    false,
		Phase:       false,
	},
}

// IsPhaseState returns true if the state is a main phase.
func IsPhaseState(s State) bool {
	info, ok := StateRegistry[s]

	return ok && info.Phase
}

// IsTerminal returns true if the state is terminal.
func IsTerminal(s State) bool {
	info, ok := StateRegistry[s]

	return ok && info.Terminal
}

// WorkUnit represents the current task being worked on by the state machine.
type WorkUnit struct {
	ID             string
	ExternalID     string // Provider-specific ID (original reference)
	Title          string
	Description    string
	Source         *Source
	Specifications []string // Specification file paths (specification-1.md, specification-2.md, etc.)
	Checkpoints    []string // Git checkpoint IDs
}

// Source represents where the task came from (read-only).
type Source struct {
	Reference string
	Provider  any    // Provider instance
	Content   string // Snapshot content
}
