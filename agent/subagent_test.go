package agent

import (
	"testing"
	"time"
)

func TestSubagentTracker_OnToolUse_Task(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	input := map[string]any{
		"subagent_type": "Explore",
		"description":   "Find auth patterns",
	}

	isSubagent := tracker.OnToolUse("call-123", "Task", input)
	if !isSubagent {
		t.Error("OnToolUse(Task) should return true")
	}

	if tracker.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", tracker.ActiveCount())
	}

	// Should have emitted a started event
	select {
	case event := <-events:
		if event.Type != EventSubagent {
			t.Errorf("event.Type = %q, want %q", event.Type, EventSubagent)
		}
		if event.Subagent == nil {
			t.Fatal("event.Subagent is nil")
		}
		if event.Subagent.Status != SubagentStarted {
			t.Errorf("Subagent.Status = %q, want %q", event.Subagent.Status, SubagentStarted)
		}
		if event.Subagent.Type != "Explore" {
			t.Errorf("Subagent.Type = %q, want %q", event.Subagent.Type, "Explore")
		}
		if event.Subagent.Description != "Find auth patterns" {
			t.Errorf("Subagent.Description = %q, want %q", event.Subagent.Description, "Find auth patterns")
		}
	default:
		t.Error("No event emitted")
	}
}

func TestSubagentTracker_OnToolUse_NonTask(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	input := map[string]any{"command": "ls"}
	isSubagent := tracker.OnToolUse("call-123", "Bash", input)
	if isSubagent {
		t.Error("OnToolUse(Bash) should return false")
	}

	if tracker.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d, want 0", tracker.ActiveCount())
	}

	// Should not have emitted an event
	select {
	case <-events:
		t.Error("Should not emit event for non-Task tool")
	default:
		// Good, no event
	}
}

func TestSubagentTracker_OnToolResult_Completion(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	// Start a subagent
	input := map[string]any{
		"subagent_type": "Plan",
		"description":   "Design login flow",
	}
	tracker.OnToolUse("call-456", "Task", input)
	<-events // Drain started event

	time.Sleep(10 * time.Millisecond) // Ensure some duration

	// Complete it
	isSubagent := tracker.OnToolResult("call-456", true, "")
	if !isSubagent {
		t.Error("OnToolResult should return true for tracked subagent")
	}

	if tracker.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d after completion, want 0", tracker.ActiveCount())
	}

	// Should have emitted a completed event
	select {
	case event := <-events:
		if event.Subagent == nil {
			t.Fatal("event.Subagent is nil")
		}
		if event.Subagent.Status != SubagentCompleted {
			t.Errorf("Subagent.Status = %q, want %q", event.Subagent.Status, SubagentCompleted)
		}
		if event.Subagent.Duration == 0 {
			t.Error("Subagent.Duration should be non-zero")
		}
	default:
		t.Error("No completion event emitted")
	}
}

func TestSubagentTracker_OnToolResult_Failure(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	// Start a subagent
	input := map[string]any{
		"subagent_type": "Explore",
		"description":   "Search files",
	}
	tracker.OnToolUse("call-789", "Task", input)
	<-events // Drain started event

	// Fail it
	tracker.OnToolResult("call-789", false, "context exceeded")

	// Should have emitted a failed event
	select {
	case event := <-events:
		if event.Subagent.Status != SubagentFailed {
			t.Errorf("Subagent.Status = %q, want %q", event.Subagent.Status, SubagentFailed)
		}
		if event.Subagent.ExitReason != "context exceeded" {
			t.Errorf("Subagent.ExitReason = %q, want %q", event.Subagent.ExitReason, "context exceeded")
		}
	default:
		t.Error("No failure event emitted")
	}
}

func TestSubagentTracker_OnToolResult_Unknown(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	// Try to complete a subagent that was never started
	isSubagent := tracker.OnToolResult("unknown-id", true, "")
	if isSubagent {
		t.Error("OnToolResult for unknown ID should return false")
	}

	// Should not have emitted an event
	select {
	case <-events:
		t.Error("Should not emit event for unknown subagent")
	default:
		// Good, no event
	}
}

func TestSubagentTracker_Clear(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	// Start multiple subagents
	tracker.OnToolUse("call-1", "Task", map[string]any{"subagent_type": "A"})
	tracker.OnToolUse("call-2", "Task", map[string]any{"subagent_type": "B"})
	<-events
	<-events

	if tracker.ActiveCount() != 2 {
		t.Errorf("ActiveCount() = %d, want 2", tracker.ActiveCount())
	}

	tracker.Clear()

	if tracker.ActiveCount() != 0 {
		t.Errorf("ActiveCount() after Clear = %d, want 0", tracker.ActiveCount())
	}
}

func TestSubagentTracker_DescriptionFromPrompt(t *testing.T) {
	events := make(chan Event, 10)
	tracker := NewSubagentTracker(events)

	// Input without description but with prompt
	input := map[string]any{
		"subagent_type": "Explore",
		"prompt":        "This is a very long prompt that should be truncated to 50 characters for the description field",
	}
	tracker.OnToolUse("call-123", "Task", input)

	select {
	case event := <-events:
		if len(event.Subagent.Description) > 50 {
			t.Errorf("Description should be truncated: %q", event.Subagent.Description)
		}
	default:
		t.Error("No event emitted")
	}
}
