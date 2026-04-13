package graph

import (
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/worker"
)

func TestValidation_IterationRequiresCheck(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:            "a",
		JobType:       worker.JobTypeImplement,
		Prompt:        "do it",
		MaxIterations: 3,
		// Missing IterationCheck
	})

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for MaxIterations without IterationCheck")
	}
	if !strings.Contains(err.Error(), "IterationCheck") {
		t.Fatalf("error should mention IterationCheck: %v", err)
	}
}

func TestValidation_NegativeMaxIterations(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:            "a",
		JobType:       worker.JobTypeImplement,
		Prompt:        "do it",
		MaxIterations: -1,
	})

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for negative MaxIterations")
	}
}

func TestValidation_IterationCheckWithoutMax(t *testing.T) {
	// IterationCheck without MaxIterations=0 is fine — check is simply unused
	g := New()
	_ = g.AddNode(&Node{
		ID:             "a",
		JobType:        worker.JobTypeImplement,
		Prompt:         "do it",
		MaxIterations:  0,
		IterationCheck: func(string) bool { return true },
	})

	if err := g.Validate(); err != nil {
		t.Fatalf("should be valid: %v", err)
	}
}

func TestValidation_ValidIteration(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:             "a",
		JobType:        worker.JobTypeImplement,
		Prompt:         "do it",
		MaxIterations:  3,
		IterationCheck: func(string) bool { return true },
	})

	if err := g.Validate(); err != nil {
		t.Fatalf("should be valid: %v", err)
	}
}

func TestValidation_ErrorRetryThenFailRequiresMaxRetries(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:            "a",
		JobType:       worker.JobTypeImplement,
		Prompt:        "do it",
		ErrorStrategy: ErrorRetryThenFail,
		MaxRetries:    0, // Invalid
	})

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for ErrorRetryThenFail with MaxRetries=0")
	}
	if !strings.Contains(err.Error(), "ErrorRetryThenFail") {
		t.Fatalf("error should mention ErrorRetryThenFail: %v", err)
	}
}

func TestValidation_ErrorDefaultValueValid(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:            "a",
		JobType:       worker.JobTypeImplement,
		Prompt:        "do it",
		ErrorStrategy: ErrorDefaultValue,
		DefaultOutput: "fallback",
	})

	if err := g.Validate(); err != nil {
		t.Fatalf("should be valid: %v", err)
	}
}

func TestStateManager_IterationCount(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	_ = g.Validate()

	sm := NewStateManager(g)

	if sm.IterationCount("a") != 0 {
		t.Fatal("expected 0 initial iteration count")
	}

	n := sm.IncrementIteration("a")
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}

	n = sm.IncrementIteration("a")
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

func TestStateManager_RetryCount(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	_ = g.Validate()

	sm := NewStateManager(g)

	if sm.RetryCount("a") != 0 {
		t.Fatal("expected 0 initial retry count")
	}

	n := sm.IncrementRetry("a")
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestStateManager_ResetForIteration(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	_ = g.Validate()

	sm := NewStateManager(g)

	// Cannot reset from Pending
	if err := sm.ResetForIteration("a"); err == nil {
		t.Fatal("should not reset from pending")
	}

	// Can reset from Done
	_ = sm.Transition("a", StateQueued)
	_ = sm.Transition("a", StateRunning)
	_ = sm.Transition("a", StateDone)

	if err := sm.ResetForIteration("a"); err != nil {
		t.Fatalf("should reset from done: %v", err)
	}
	if sm.Get("a") != StatePending {
		t.Fatal("should be pending after reset")
	}

	// Can reset from Failed
	_ = sm.Transition("a", StateQueued)
	_ = sm.Transition("a", StateRunning)
	_ = sm.Transition("a", StateFailed)
	sm.SetError("a", "some error")

	if err := sm.ResetForIteration("a"); err != nil {
		t.Fatalf("should reset from failed: %v", err)
	}
	if sm.Get("a") != StatePending {
		t.Fatal("should be pending after reset from failed")
	}
	if sm.Error("a") != "" {
		t.Fatal("error should be cleared after reset")
	}
}

func TestStateManager_ResetForIteration_UnknownNode(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{ID: "a"})
	_ = g.Validate()

	sm := NewStateManager(g)

	err := sm.ResetForIteration("unknown")
	if err == nil {
		t.Fatal("should error for unknown node")
	}
	if !strings.Contains(err.Error(), "unknown node") {
		t.Fatal("error should mention unknown node")
	}
}
