package conductor

import (
	"context"
	"testing"
)

func TestStateDescription(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateNone, "not started"},
		{StateLoaded, "loaded but not planned"},
		{StatePlanning, "currently planning"},
		{StatePlanned, "planned but not implemented"},
		{StateImplementing, "currently implementing"},
		{StateImplemented, "implemented but not reviewed"},
		{StateSimplifying, "currently simplifying"},
		{StateOptimizing, "currently optimizing"},
		{StateReviewing, "under review"},
		{StateSubmitted, "submitted — can re-enter plan/implement/review or finish"},
		{StateFailed, "in failed state"},
		{StateWaiting, "waiting for your input"},
		{StatePaused, "paused"},
		{State("unknown"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := stateDescription(tt.state)
			if got != tt.want {
				t.Errorf("stateDescription(%s) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestSuggestNextAction(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateNone, "Run: kvelmo start --from <provider:reference>"},
		{StateLoaded, "Run: kvelmo plan"},
		{StatePlanning, "Wait for planning to complete"},
		{StatePlanned, "Run: kvelmo implement"},
		{StateImplementing, "Wait for implementation to complete"},
		{StateImplemented, "Run: kvelmo review"},
		{StateSimplifying, "Wait for simplification to complete"},
		{StateOptimizing, "Wait for optimization to complete"},
		{StateReviewing, "Run: kvelmo submit"},
		{StateSubmitted, "PR submitted. Run: kvelmo finish (cleanup), kvelmo plan (re-plan), or kvelmo implement (re-implement)"},
		{StateFailed, "Run: kvelmo reset to recover"},
		{StateWaiting, "Answer the pending question"},
		{StatePaused, "Run: kvelmo resume"},
		{State("unknown"), ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := suggestNextAction(tt.state, nil)
			if got != tt.want {
				t.Errorf("suggestNextAction(%s) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestEventDescription(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{EventStart, "start task"},
		{EventPlan, "start planning"},
		{EventPlanDone, "complete planning"},
		{EventImplement, "start implementation"},
		{EventImplementDone, "complete implementation"},
		{EventSimplify, "start simplification"},
		{EventSimplifyDone, "complete simplification"},
		{EventOptimize, "start optimization"},
		{EventOptimizeDone, "complete optimization"},
		{EventReview, "start review"},
		{EventReviewDone, "complete review"},
		{EventSubmit, "submit"},
		{EventFinish, "finish task"},
		{EventUndo, "undo"},
		{EventUndoDone, "complete undo"},
		{EventRedo, "redo"},
		{EventRedoDone, "complete redo"},
		{EventError, "handle error"},
		{EventAbort, "abort"},
		{EventReset, "reset"},
		{EventReject, "reject changes"},
		{EventWait, "wait for input"},
		{EventAnswer, "answer question"},
		{EventPause, "pause"},
		{EventResume, "resume"},
		{Event("custom"), "custom"},
	}
	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			got := eventDescription(tt.event)
			if got != tt.want {
				t.Errorf("eventDescription(%s) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}

func TestTruncateSHA(t *testing.T) {
	tests := []struct {
		sha  string
		n    int
		want string
	}{
		{"abc123def456", 8, "abc123de"},
		{"short", 8, "short"},
		{"", 8, ""},
		{"exactly8", 8, "exactly8"},
	}
	for _, tt := range tests {
		t.Run(tt.sha, func(t *testing.T) {
			got := truncateSHA(tt.sha, tt.n)
			if got != tt.want {
				t.Errorf("truncateSHA(%q, %d) = %q, want %q", tt.sha, tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatTransitionError(t *testing.T) {
	err := formatTransitionError(StateNone, EventImplement, nil)
	if err == nil {
		t.Fatal("formatTransitionError should return non-nil error")
	}
	// Should mention the action and state
	errStr := err.Error()
	if errStr == "" {
		t.Error("formatTransitionError returned empty error string")
	}
}

func TestFormatGuardError(t *testing.T) {
	wu := &WorkUnit{Source: &Source{Reference: ""}} // empty reference fails guardHasSource
	transitions := []Transition{
		{From: StateNone, Event: EventStart, To: StateLoaded, Guards: []Guard{
			{Check: guardHasSource, Message: "no task source specified"},
		}},
	}
	err := formatGuardError(StateNone, EventStart, wu, transitions)
	if err == nil {
		t.Fatal("formatGuardError should return non-nil error")
	}
	if got := err.Error(); got == "" {
		t.Error("formatGuardError returned empty error string")
	}
}

func TestFormatGuardError_AllPass(t *testing.T) {
	wu := &WorkUnit{Source: &Source{Reference: "ref", Provider: "github"}}
	transitions := []Transition{
		{From: StateNone, Event: EventStart, To: StateLoaded, Guards: []Guard{
			{Check: guardHasSource, Message: "no task source specified"},
		}},
	}
	// All guards pass, so formatGuardError falls through to the generic message
	err := formatGuardError(StateNone, EventStart, wu, transitions)
	if err == nil {
		t.Fatal("formatGuardError should return non-nil error even when guards pass")
	}
}

func TestConductorAbort_FromNone(t *testing.T) {
	c, _ := New()
	// StateNone doesn't support Abort
	err := c.Abort(context.Background())
	if err == nil {
		t.Error("Abort() from StateNone should return error")
	}
}

func TestConductorReset_FromFailed(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "r1", Title: "T"})
	c.machine.ForceState(StateFailed)

	err := c.Reset(context.Background())
	if err != nil {
		t.Errorf("Reset() from failed state error = %v", err)
	}
	if c.State() != StateLoaded {
		t.Errorf("after Reset: state = %s, want loaded", c.State())
	}
}

func TestConductorUpdateTask_NoSource(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "u1", Title: "T"})
	// Source is nil
	_, _, err := c.UpdateTask(context.Background())
	if err == nil {
		t.Error("UpdateTask() with nil source should return error")
	}
}

func TestConductorClose_Idempotent(t *testing.T) {
	c, _ := New()
	if err := c.Close(); err != nil {
		t.Errorf("Close() first call error = %v", err)
	}
	// Close again should not panic
	if err := c.Close(); err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestConductorClose_DrainEvents(t *testing.T) {
	c, _ := New()
	ch := c.Events()
	_ = c.Close()
	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("Events channel should be closed after Close()")
	}
}
