// Package progress provides status line tracking for long-running operations.
package progress

import (
	"fmt"
	"sync"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// StatusLine tracks and displays status updates for a single phase.
type StatusLine struct {
	phase       string
	lastTool    string
	startTime   time.Time
	lastUpdate  time.Time
	updateCount int
	mu          sync.Mutex
}

// NewStatusLine creates a new status line for the given phase.
func NewStatusLine(phase string) *StatusLine {
	now := time.Now()
	return &StatusLine{
		phase:      phase,
		startTime:  now,
		lastUpdate: now,
	}
}

// OnEvent is called by RunWithCallback for each streaming event.
// It returns an error to stop processing, nil to continue.
func (s *StatusLine) OnEvent(event agent.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch event.Type {
	case agent.EventToolUse:
		// Agent is calling a tool - show what it's doing
		if event.ToolCall != nil {
			description := event.ToolCall.Description
			if description == "" {
				description = event.ToolCall.Name
			}
			s.update(description)
		}
	case agent.EventText:
		// Agent is generating text - show this periodically
		if time.Since(s.lastUpdate) > 3*time.Second {
			s.update("generating response...")
		}
	}

	return nil
}

// update updates the status line with a new activity message.
// It uses \r to overwrite the current line, showing "live" status.
func (s *StatusLine) update(activity string) {
	// Truncate very long activity strings
	if len(activity) > 50 {
		activity = activity[:47] + "..."
	}

	// Build elapsed time string (only show after 5 seconds)
	elapsed := time.Since(s.startTime)
	elapsedStr := ""
	if elapsed >= 5*time.Second {
		elapsedStr = fmt.Sprintf(" (%s)", formatDuration(elapsed))
	}

	// Clear previous line and print new status
	fmt.Printf("\r→ %s...%s (%s\x1b[K)", s.phase, elapsedStr, activity)
	s.lastTool = activity
	s.lastUpdate = time.Now()
	s.updateCount++
}

// Done marks the phase as complete with a checkmark.
func (s *StatusLine) Done() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build elapsed time string (always show on completion)
	elapsed := time.Since(s.startTime)
	elapsedStr := formatDuration(elapsed)

	// Clear the in-progress line and show done with elapsed time
	fmt.Printf("\r→ %s ✓ (%s)\x1b[K\n", s.phase, elapsedStr)
}

// formatDuration formats a duration as MM:SS.
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
