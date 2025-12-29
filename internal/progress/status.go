// Package progress provides status line tracking for long-running operations.
package progress

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// StatusLine tracks and displays status updates for a single phase.
type StatusLine struct {
	phase       string
	lastTool    string
	lastUpdate  time.Time
	updateCount int
	mu          sync.Mutex
}

// NewStatusLine creates a new status line for the given phase.
func NewStatusLine(phase string) *StatusLine {
	return &StatusLine{
		phase:      phase,
		lastUpdate: time.Now(),
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

	// Clear previous line and print new status
	fmt.Printf("\r→ %s... (%s)", s.phase, activity+strings.Repeat(" ", 50))
	s.lastTool = activity
	s.lastUpdate = time.Now()
	s.updateCount++
}

// Done marks the phase as complete with a checkmark.
func (s *StatusLine) Done() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear the in-progress line and show done
	fmt.Printf("\r→ %s ✓%s\n", s.phase, strings.Repeat(" ", 60))
}

// GetPhase returns the phase name.
func (s *StatusLine) GetPhase() string {
	return s.phase
}

// GetUpdateCount returns the number of status updates.
func (s *StatusLine) GetUpdateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateCount
}

// MultiProgress tracks multiple concurrent status lines.
type MultiProgress struct {
	lines map[string]*StatusLine
	mu    sync.Mutex
}

// NewMultiProgress creates a new multi-progress tracker.
func NewMultiProgress() *MultiProgress {
	return &MultiProgress{
		lines: make(map[string]*StatusLine),
	}
}

// Add adds a new status line for a phase.
func (m *MultiProgress) Add(phase string) *StatusLine {
	m.mu.Lock()
	defer m.mu.Unlock()

	line := NewStatusLine(phase)
	m.lines[phase] = line
	return line
}

// Remove removes and completes a status line.
func (m *MultiProgress) Remove(phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if line, ok := m.lines[phase]; ok {
		line.Done()
		delete(m.lines, phase)
	}
}

// DoneAll completes all remaining status lines.
func (m *MultiProgress) DoneAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for phase := range m.lines {
		m.lines[phase].Done()
		delete(m.lines, phase)
	}
}

// SimpleProgress provides simple status updates without live tracking.
// Use this when you don't have access to streaming events.
type SimpleProgress struct {
	phase string
	mu    sync.Mutex
}

// NewSimpleProgress creates a new simple progress tracker.
func NewSimpleProgress(phase string) *SimpleProgress {
	return &SimpleProgress{phase: phase}
}

// Update shows a status update.
func (p *SimpleProgress) Update(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Printf("→ %s... %s\n", p.phase, message)
}

// Done marks the phase as complete.
func (p *SimpleProgress) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Printf("→ %s ✓\n", p.phase)
}

// Message prints a message without status formatting.
func (p *SimpleProgress) Message(message string) {
	fmt.Println(message)
}
