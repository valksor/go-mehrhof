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

func TestProgressBar(t *testing.T) {
	t.Run("new progress bar", func(t *testing.T) {
		pb := NewProgressBar("Testing", 100)
		if pb == nil {
			t.Fatal("NewProgressBar returned nil")
		}
	})

	t.Run("increment updates progress", func(t *testing.T) {
		pb := NewProgressBar("Testing", 10)
		pb.Increment()
		pb.Increment()
		// Just ensure it doesn't panic
	})

	t.Run("update sets current value", func(t *testing.T) {
		pb := NewProgressBar("Testing", 100)
		pb.Update(50)
		// Just ensure it doesn't panic
	})

	t.Run("set agent name", func(t *testing.T) {
		pb := NewProgressBar("Testing", 100)
		pb.SetAgent("claude-opus-4")
		// Just ensure it doesn't panic
	})

	t.Run("set width", func(t *testing.T) {
		pb := NewProgressBar("Testing", 100)
		pb.SetWidth(40)
		// Just ensure it doesn't panic
	})

	t.Run("done marks complete", func(t *testing.T) {
		pb := NewProgressBar("Testing", 10)
		for i := 0; i < 10; i++ {
			pb.Increment()
		}
		pb.Done()
		// Ensure no panic
	})
}

func TestStatusWithProgress(t *testing.T) {
	t.Run("new combined tracker", func(t *testing.T) {
		swp := NewStatusWithProgress("Planning", 5)
		if swp == nil {
			t.Fatal("NewStatusWithProgress returned nil")
		}
		if swp.GetStatusLine() == nil {
			t.Error("GetStatusLine() returned nil")
		}
		if swp.GetProgressBar() == nil {
			t.Error("GetProgressBar() returned nil")
		}
	})

	t.Run("set agent propagates to progress bar", func(t *testing.T) {
		swp := NewStatusWithProgress("Planning", 5)
		swp.SetAgent("claude-opus-4")
		// Just ensure it doesn't panic
	})

	t.Run("on event updates progress on tool result", func(t *testing.T) {
		swp := NewStatusWithProgress("Planning", 5)
		event := agent.Event{
			Type: agent.EventToolResult,
			ToolCall: &agent.ToolCall{
				Name: "read_file",
			},
		}
		_ = swp.OnEvent(event)
		// Just ensure it doesn't panic
	})

	t.Run("increment advances progress", func(t *testing.T) {
		swp := NewStatusWithProgress("Planning", 5)
		swp.Increment()
		swp.Increment()
		swp.Done()
		// Ensure no panic
	})
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

func TestProgressBarConcurrent(t *testing.T) {
	pb := NewProgressBar("Concurrent", 100)

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pb.Increment()
		}()
	}
	wg.Wait()

	// Should not panic
	pb.Done()
}
