// Package progress provides status line tracking for long-running operations.
package progress

import (
	"fmt"
	"math"
	"strings"
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

// ProgressBar tracks progress with percentage completion and elapsed time.
type ProgressBar struct {
	phase      string
	agent      string
	current    int
	total      int
	startTime  time.Time
	lastUpdate time.Time
	mu         sync.Mutex
	width      int // Width of progress bar in characters
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(phase string, total int) *ProgressBar {
	return &ProgressBar{
		phase:      phase,
		total:      total,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
		width:      30, // Default width
	}
}

// SetAgent sets the agent/model name for display.
func (p *ProgressBar) SetAgent(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.agent = name
}

// SetWidth sets the width of the progress bar.
func (p *ProgressBar) SetWidth(w int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.width = w
}

// Increment advances progress by one.
func (p *ProgressBar) Increment() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current++
	p.lastUpdate = time.Now()
	p.render()
}

// Update sets the current progress value.
func (p *ProgressBar) Update(current int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = current
	p.lastUpdate = time.Now()
	p.render()
}

// render displays the current progress state.
func (p *ProgressBar) render() {
	if p.total <= 0 {
		return
	}

	percent := float64(p.current) / float64(p.total)
	elapsed := time.Since(p.startTime)

	// Build progress bar
	filled := int(math.Round(float64(p.width) * percent))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	// Format elapsed time
	elapsedStr := formatDuration(elapsed)

	// Build output
	output := fmt.Sprintf("\r  [%s] %d%% %s", bar, int(percent*100), elapsedStr)

	// Add phase if set
	if p.phase != "" {
		output += fmt.Sprintf(" - %s", p.phase)
	}

	// Add agent if set
	if p.agent != "" {
		output += fmt.Sprintf(" (%s)", p.agent)
	}

	// Clear rest of line using ANSI escape sequence
	output += "\x1b[K"

	fmt.Print(output)
}

// Done marks the progress as complete.
func (p *ProgressBar) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = p.total
	elapsed := time.Since(p.startTime)

	bar := strings.Repeat("█", p.width)
	elapsedStr := formatDuration(elapsed)

	output := fmt.Sprintf("\r  [%s] 100%% %s", bar, elapsedStr)

	if p.phase != "" {
		output += fmt.Sprintf(" - %s", p.phase)
	}
	if p.agent != "" {
		output += fmt.Sprintf(" (%s)", p.agent)
	}

	fmt.Println(output + " ✓")
}

// formatDuration formats a duration as MM:SS.
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// StatusWithProgress combines status updates with progress tracking.
type StatusWithProgress struct {
	status    *StatusLine
	progress  *ProgressBar
	showAgent bool
	agentName string
	mu        sync.Mutex
}

// NewStatusWithProgress creates a combined status and progress tracker.
func NewStatusWithProgress(phase string, total int) *StatusWithProgress {
	return &StatusWithProgress{
		status:   NewStatusLine(phase),
		progress: NewProgressBar(phase, total),
	}
}

// SetAgent sets the agent name for display.
func (s *StatusWithProgress) SetAgent(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentName = name
	s.showAgent = true
	s.progress.SetAgent(name)
}

// OnEvent implements the agent event handler.
func (s *StatusWithProgress) OnEvent(event agent.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Forward to status line for activity tracking
	if err := s.status.OnEvent(event); err != nil {
		return err
	}

	// Update progress bar on tool use completion
	if event.Type == agent.EventToolResult && event.ToolCall != nil {
		s.progress.Increment()
	}

	return nil
}

// Increment advances the progress by one.
func (s *StatusWithProgress) Increment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.Increment()
}

// Done marks both status and progress as complete.
func (s *StatusWithProgress) Done() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Show final status before clearing
	s.progress.Done()
	s.status.Done()
}

// GetStatusLine returns the underlying status line.
func (s *StatusWithProgress) GetStatusLine() *StatusLine {
	return s.status
}

// GetProgressBar returns the underlying progress bar.
func (s *StatusWithProgress) GetProgressBar() *ProgressBar {
	return s.progress
}
