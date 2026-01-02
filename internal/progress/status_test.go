package progress

import (
	"sync"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

func TestNewStatusLine(t *testing.T) {
	phase := "Testing"
	sl := NewStatusLine(phase)

	if sl.phase != phase {
		t.Errorf("phase = %q, want %q", sl.phase, phase)
	}

	if sl.updateCount != 0 {
		t.Errorf("updateCount = %d, want 0", sl.updateCount)
	}
}

func TestStatusLineOnEvent(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Create a tool use event
	event := agent.Event{
		Type: agent.EventToolUse,
		ToolCall: &agent.ToolCall{
			Name:        "Read",
			Description: "Reading test file",
		},
	}

	_ = sl.OnEvent(event)

	if sl.updateCount != 1 {
		t.Errorf("updateCount = %d, want 1", sl.updateCount)
	}
}

func TestStatusLineOnEventText(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Create a text event
	event := agent.Event{
		Type: agent.EventText,
	}

	// First text event should not update (too soon)
	err := sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() error = %v", err)
	}

	if sl.updateCount != 0 {
		t.Errorf("updateCount = %d, want 0 (text event should not update immediately)", sl.updateCount)
	}

	// Wait for threshold and try again
	time.Sleep(3 * time.Second)
	err = sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() error = %v", err)
	}

	if sl.updateCount != 1 {
		t.Errorf("updateCount = %d, want 1 (after threshold)", sl.updateCount)
	}
}

func TestStatusLineDone(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Should not panic
	sl.Done()

	if sl.updateCount != 0 {
		t.Errorf("updateCount = %d, want 0", sl.updateCount)
	}
}

func TestStatusLineConcurrent(t *testing.T) {
	sl := NewStatusLine("Concurrent")

	// Test concurrent access
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = sl.OnEvent(agent.Event{
				Type: agent.EventToolUse,
				ToolCall: &agent.ToolCall{
					Name:        "Bash",
					Description: "Running command",
				},
			})
		})
	}
	wg.Wait()

	// Should not panic and should have recorded some updates
	if sl.updateCount != 10 {
		t.Errorf("updateCount = %d, want 10", sl.updateCount)
	}
}

func TestStatusLineToolDescriptionTruncation(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Create an event with a very long description
	longDesc := "This is a very long tool description that should be truncated because it exceeds the maximum length allowed for status updates in the progress tracker."
	if len(longDesc) < 50 {
		longDesc = longDesc + "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	}

	event := agent.Event{
		Type: agent.EventToolUse,
		ToolCall: &agent.ToolCall{
			Name:        "Read",
			Description: longDesc,
		},
	}

	err := sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() error = %v", err)
	}

	// Should have updated without error
	if sl.updateCount != 1 {
		t.Errorf("updateCount = %d, want 1", sl.updateCount)
	}
}

func TestStatusLineUnknownEventType(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Create an unknown event type (using a string that won't match known types)
	event := agent.Event{
		Type: agent.EventType("unknown-event-type"),
	}

	err := sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() with unknown event type should not error, got %v", err)
	}

	// Unknown event types should be ignored
	if sl.updateCount != 0 {
		t.Errorf("updateCount = %d, want 0 (unknown event should be ignored)", sl.updateCount)
	}
}

func TestStatusLineNilToolCall(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Create an event with nil ToolCall
	event := agent.Event{
		Type:     agent.EventToolUse,
		ToolCall: nil,
	}

	// Should not panic
	err := sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() with nil ToolCall should not error, got %v", err)
	}

	// Should not have updated
	if sl.updateCount != 0 {
		t.Errorf("updateCount = %d, want 0 (nil ToolCall should be ignored)", sl.updateCount)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0:00"},
		{"30 seconds", 30 * time.Second, "0:30"},
		{"1 minute", 1 * time.Minute, "1:00"},
		{"1:30", 90 * time.Second, "1:30"},
		{"10 minutes", 10 * time.Minute, "10:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDuration(tt.duration); got != tt.want {
				t.Errorf("formatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusLineStartTime(t *testing.T) {
	before := time.Now()
	sl := NewStatusLine("Testing")
	after := time.Now()

	// startTime should be set between before and after
	if sl.startTime.Before(before) || sl.startTime.After(after) {
		t.Errorf("startTime not set correctly, got %v, want between %v and %v", sl.startTime, before, after)
	}
}

func TestStatusLineElapsedTimeThreshold(t *testing.T) {
	// This test verifies that the elapsed time logic doesn't cause errors
	// when under or over the 5-second threshold
	sl := NewStatusLine("Testing")

	// Immediately call update (under 5 seconds) - should not show time
	event := agent.Event{
		Type: agent.EventToolUse,
		ToolCall: &agent.ToolCall{
			Name:        "Read",
			Description: "test",
		},
	}
	err := sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() error = %v", err)
	}

	// Call Done immediately - should show elapsed time in format 0:00
	sl.Done()

	// Verify no panic and update was recorded
	if sl.updateCount != 1 {
		t.Errorf("updateCount = %d, want 1", sl.updateCount)
	}
}
