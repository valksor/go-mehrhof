package conductor

import "context"

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
