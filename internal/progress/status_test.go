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

	if sl.GetPhase() != phase {
		t.Errorf("GetPhase() = %q, want %q", sl.GetPhase(), phase)
	}

	if sl.GetUpdateCount() != 0 {
		t.Errorf("GetUpdateCount() = %d, want 0", sl.GetUpdateCount())
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

	if sl.GetUpdateCount() != 1 {
		t.Errorf("GetUpdateCount() = %d, want 1", sl.GetUpdateCount())
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

	if sl.GetUpdateCount() != 0 {
		t.Errorf("GetUpdateCount() = %d, want 0 (text event should not update immediately)", sl.GetUpdateCount())
	}

	// Wait for threshold and try again
	time.Sleep(3 * time.Second)
	err = sl.OnEvent(event)
	if err != nil {
		t.Errorf("OnEvent() error = %v", err)
	}

	if sl.GetUpdateCount() != 1 {
		t.Errorf("GetUpdateCount() = %d, want 1 (after threshold)", sl.GetUpdateCount())
	}
}

func TestStatusLineDone(t *testing.T) {
	sl := NewStatusLine("Testing")

	// Should not panic
	sl.Done()

	if sl.GetUpdateCount() != 0 {
		t.Errorf("GetUpdateCount() = %d, want 0", sl.GetUpdateCount())
	}
}

func TestStatusLineConcurrent(t *testing.T) {
	sl := NewStatusLine("Concurrent")

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = sl.OnEvent(agent.Event{
				Type: agent.EventToolUse,
				ToolCall: &agent.ToolCall{
					Name:        "Bash",
					Description: "Running command",
				},
			})
		}()
		go func() {
			defer wg.Done()
			sl.GetUpdateCount()
		}()
	}
	wg.Wait()

	// Should not panic and should have recorded some updates
	if sl.GetUpdateCount() != 10 {
		t.Errorf("GetUpdateCount() = %d, want 10", sl.GetUpdateCount())
	}
}

func TestMultiProgress(t *testing.T) {
	mp := NewMultiProgress()

	// Add some status lines
	sl1 := mp.Add("Phase1")
	sl2 := mp.Add("Phase2")

	if sl1.GetPhase() != "Phase1" {
		t.Errorf("first status line phase = %q, want Phase1", sl1.GetPhase())
	}

	if sl2.GetPhase() != "Phase2" {
		t.Errorf("second status line phase = %q, want Phase2", sl2.GetPhase())
	}

	// Remove one
	mp.Remove("Phase1")

	// Remove all
	mp.DoneAll()

	// Should not panic
	mp.DoneAll() // Double call should be safe
}

func TestMultiProgressConcurrent(t *testing.T) {
	mp := NewMultiProgress()

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			phase := string(rune('A' + i))
			mp.Add(phase)
			mp.Remove(phase)
		}(i)
	}
	wg.Wait()

	// Should not panic
	mp.DoneAll()
}

func TestSimpleProgress(t *testing.T) {
	sp := NewSimpleProgress("Simple")

	// Should not panic
	sp.Update("test message")
	sp.Done()
	sp.Message("just a message")
}

func TestSimpleProgressConcurrent(t *testing.T) {
	sp := NewSimpleProgress("Concurrent")

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sp.Update("message")
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sp.Done()
		}(i)
	}
	wg.Wait()

	// Should not panic
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
	if sl.GetUpdateCount() != 1 {
		t.Errorf("GetUpdateCount() = %d, want 1", sl.GetUpdateCount())
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
	if sl.GetUpdateCount() != 0 {
		t.Errorf("GetUpdateCount() = %d, want 0 (unknown event should be ignored)", sl.GetUpdateCount())
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
	if sl.GetUpdateCount() != 0 {
		t.Errorf("GetUpdateCount() = %d, want 0 (nil ToolCall should be ignored)", sl.GetUpdateCount())
	}
}
