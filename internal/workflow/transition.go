package workflow

// Transition defines a state transition
type Transition struct {
	From    State
	Event   Event
	To      State
	Guards  []GuardFunc
	Effects []EffectFunc
}

// TransitionKey uniquely identifies a transition
type TransitionKey struct {
	From  State
	Event Event
}

// TransitionTable maps (state, event) pairs to transitions
// The new workflow is phase-based:
//
//	idle -> planning -> idle (specs created)
//	idle -> implementing -> idle (code changed)
//	idle -> reviewing -> idle (review done)
//	idle -> done (finish)
//
// User can enter dialogue (talk) from idle to add notes
var TransitionTable = map[TransitionKey][]Transition{
	// === Start: Register task ===
	{StateIdle, EventStart}: {
		{From: StateIdle, Event: EventStart, To: StateIdle, Guards: []GuardFunc{GuardHasSource}},
	},

	// === Planning Phase ===
	{StateIdle, EventPlan}: {
		{From: StateIdle, Event: EventPlan, To: StatePlanning},
	},
	{StatePlanning, EventPlanDone}: {
		{From: StatePlanning, Event: EventPlanDone, To: StateIdle},
	},
	{StatePlanning, EventError}: {
		{From: StatePlanning, Event: EventError, To: StateIdle}, // Return to idle on error
	},
	{StatePlanning, EventCheckpoint}: {
		{From: StatePlanning, Event: EventCheckpoint, To: StateCheckpointing},
	},
	{StatePlanning, EventWait}: {
		{From: StatePlanning, Event: EventWait, To: StateWaiting},
	},

	// === Waiting (agent asked question) ===
	{StateWaiting, EventAnswer}: {
		{From: StateWaiting, Event: EventAnswer, To: StateIdle}, // Ready to re-plan
	},
	{StateWaiting, EventDialogueStart}: {
		{From: StateWaiting, Event: EventDialogueStart, To: StateDialogue},
	},
	{StateWaiting, EventPlan}: {
		{From: StateWaiting, Event: EventPlan, To: StatePlanning}, // Re-enter planning after answer
	},

	// === Implementation Phase ===
	{StateIdle, EventImplement}: {
		{From: StateIdle, Event: EventImplement, To: StateImplementing, Guards: []GuardFunc{GuardHasSpecifications}},
	},
	{StateImplementing, EventImplementDone}: {
		{From: StateImplementing, Event: EventImplementDone, To: StateIdle},
	},
	{StateImplementing, EventError}: {
		{From: StateImplementing, Event: EventError, To: StateIdle},
	},
	{StateImplementing, EventCheckpoint}: {
		{From: StateImplementing, Event: EventCheckpoint, To: StateCheckpointing},
	},
	{StateImplementing, EventUndo}: {
		{From: StateImplementing, Event: EventUndo, To: StateReverting, Guards: []GuardFunc{GuardCanUndo}},
	},

	// === Review Phase ===
	{StateIdle, EventReview}: {
		{From: StateIdle, Event: EventReview, To: StateReviewing, Guards: []GuardFunc{GuardCanReview}},
	},
	{StateReviewing, EventReviewDone}: {
		{From: StateReviewing, Event: EventReviewDone, To: StateIdle},
	},
	{StateReviewing, EventError}: {
		{From: StateReviewing, Event: EventError, To: StateIdle},
	},

	// === Finish ===
	{StateIdle, EventFinish}: {
		{From: StateIdle, Event: EventFinish, To: StateDone, Guards: []GuardFunc{GuardCanFinish}},
	},

	// === Failed State Recovery ===
	{StateFailed, EventReset}: {
		{From: StateFailed, Event: EventReset, To: StateIdle},
	},

	// === Dialogue (Talk) - from idle only ===
	{StateIdle, EventDialogueStart}: {
		{From: StateIdle, Event: EventDialogueStart, To: StateDialogue},
	},
	{StateDialogue, EventDialogueEnd}: {
		{From: StateDialogue, Event: EventDialogueEnd, To: StateIdle},
	},
	{StateDialogue, EventError}: {
		{From: StateDialogue, Event: EventError, To: StateIdle},
	},

	// === Checkpointing (return to previous phase) ===
	{StateCheckpointing, EventCheckpointDone}: {
		// After checkpoint, return to idle for user to decide next step
		{From: StateCheckpointing, Event: EventCheckpointDone, To: StateIdle},
	},

	// === Undo/Redo ===
	{StateIdle, EventUndo}: {
		{From: StateIdle, Event: EventUndo, To: StateReverting, Guards: []GuardFunc{GuardCanUndo}},
	},
	{StateIdle, EventRedo}: {
		{From: StateIdle, Event: EventRedo, To: StateRestoring, Guards: []GuardFunc{GuardCanRedo}},
	},
	{StateReverting, EventUndoDone}: {
		{From: StateReverting, Event: EventUndoDone, To: StateIdle},
	},
	{StateRestoring, EventRedoDone}: {
		{From: StateRestoring, Event: EventRedoDone, To: StateIdle},
	},
}

// GlobalTransitions apply from any state
var GlobalTransitions = map[Event]State{
	EventAbort: StateFailed,
}

// GetTransitions returns possible transitions for a state/event pair
func GetTransitions(from State, event Event) []Transition {
	key := TransitionKey{From: from, Event: event}
	return TransitionTable[key]
}

// GetGlobalTransition returns the target state for global events
func GetGlobalTransition(event Event) (State, bool) {
	to, ok := GlobalTransitions[event]
	return to, ok
}

// CanTransition checks if a transition is possible without guards
func CanTransition(from State, event Event) bool {
	_, ok := GlobalTransitions[event]
	if ok {
		return true
	}
	key := TransitionKey{From: from, Event: event}
	transitions, ok := TransitionTable[key]
	return ok && len(transitions) > 0
}
