package conductor

import (
	"context"
	"fmt"
)

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
