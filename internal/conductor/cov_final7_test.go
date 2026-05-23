package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubTaskPhaseInstruction(t *testing.T) {
	tests := []struct {
		phase string
		want  string
	}{
		{PhasePlan, "Write a focused specification for this sub-task."},
		{PhaseImplement, "Implement the sub-task according to its specification."},
		{PhaseSimplify, "Simplify the implementation without changing its behavior."},
		{PhaseOptimize, "Improve the quality and performance of the implementation."},
		{PhaseReview, "Review the implementation and report any issues."},
		{"unknown", "Run the requested phase."},
	}
	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			if got := subTaskPhaseInstruction(tt.phase); got != tt.want {
				t.Errorf("subTaskPhaseInstruction(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestPromptUser_RoundTrip(t *testing.T) {
	c := newTestConductor(t)
	c.ForceWorkUnit(&WorkUnit{ID: "prompt-1"})

	// Drain events so the prompt emit doesn't block.
	go func() {
		for range c.Events() {
		}
	}()

	answered := make(chan struct {
		answer bool
		err    error
	}, 1)

	go func() {
		ans, err := c.promptUser(context.Background(), "Proceed?")
		answered <- struct {
			answer bool
			err    error
		}{ans, err}
	}()

	// Wait for the prompt to register, then answer it.
	deadline := time.Now().Add(5 * time.Second)
	var promptID string
	for time.Now().Before(deadline) {
		ids := c.PendingPromptIDs()
		if len(ids) > 0 {
			promptID = ids[0]

			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if promptID == "" {
		t.Fatal("prompt was never registered")
	}

	if err := c.RespondToPrompt(promptID, true); err != nil {
		t.Fatalf("RespondToPrompt error = %v", err)
	}

	select {
	case res := <-answered:
		if res.err != nil {
			t.Errorf("promptUser error = %v", res.err)
		}
		if !res.answer {
			t.Error("promptUser should return the delivered answer (true)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("promptUser did not return after answer")
	}
}

func TestRespondToPrompt_Unknown(t *testing.T) {
	c := newTestConductor(t)
	if err := c.RespondToPrompt("does-not-exist", true); err == nil {
		t.Error("RespondToPrompt for an unknown prompt should error")
	}
}

func TestResumeFromCheckpoint_HappyPath(t *testing.T) {
	c, dir := setupExecConductor(t)
	ctx := context.Background()

	// Create a real checkpoint commit.
	if err := os.WriteFile(filepath.Join(dir, "cp.go"), []byte("package main\n\nvar CP = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := c.CreateCheckpoint(ctx, "phase checkpoint")
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	// Reference the checkpoint from a phase metric so ResumeFromCheckpoint can
	// find the target phase. Use implement so dispatchAutoAdvance re-runs it.
	c.workUnit.HasImplemented = true
	c.workUnit.Specifications = []string{"spec.md"}
	c.workUnit.PhaseMetrics = map[string]*PhaseMetrics{
		PhaseImplement: {CheckpointSHA: sha},
	}
	c.machine.ForceState(StateImplemented)

	if err := c.ResumeFromCheckpoint(ctx, sha); err != nil {
		t.Fatalf("ResumeFromCheckpoint error = %v", err)
	}
	// Resume dispatches the phase asynchronously; give it a moment to start.
	time.Sleep(200 * time.Millisecond)
	// Wait for the re-dispatched implement phase to complete so the async job
	// doesn't race the TempDir cleanup.
	waitForStateExec(t, c, StateImplemented, 15*time.Second)
}

func TestApplyFailurePolicy_RetrySchedules(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateImplementing)
	c.workUnit.Specifications = []string{"spec.md"}
	c.phasePolicies[PhaseImplement] = PhasePolicy{
		Policy: FailurePolicyRetry, MaxRetries: 2, RetryDelay: 10 * time.Millisecond,
	}

	// First failure with attempts under the limit schedules a retry (returns true).
	handled := c.applyFailurePolicy(context.Background(), EventImplementDone, "transient failure")
	if !handled {
		t.Error("retry policy under the limit should schedule a retry (true)")
	}
	// Let the scheduled retry fire (it re-dispatches implement via AfterFunc).
	time.Sleep(300 * time.Millisecond)
}
