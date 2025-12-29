package display

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Spinner provides animated progress indication for long-running operations.
type Spinner struct {
	writer   io.Writer
	stopCh   chan struct{}
	doneCh   chan struct{}
	message  string
	frames   []string
	delay    time.Duration
	frameIdx int
	mu       sync.Mutex
	running  bool
}

// Default spinner frames (braille pattern for smooth animation)
var defaultFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		writer:  os.Stdout,
		frames:  defaultFrames,
		delay:   80 * time.Millisecond,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// WithWriter sets the output writer (useful for testing).
func (s *Spinner) WithWriter(w io.Writer) *Spinner {
	s.writer = w
	return s
}

// WithFrames sets custom animation frames.
func (s *Spinner) WithFrames(frames []string) *Spinner {
	s.frames = frames
	return s
}

// WithDelay sets the animation delay between frames.
func (s *Spinner) WithDelay(d time.Duration) *Spinner {
	s.delay = d
	return s
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	go s.run()
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	// Wait for the goroutine to finish
	<-s.doneCh
}

// StopWithSuccess stops the spinner and shows a success message.
func (s *Spinner) StopWithSuccess(message string) {
	s.Stop()
	s.clearLine()
	_, _ = fmt.Fprintln(s.writer, SuccessMsg("%s", message))
}

// StopWithError stops the spinner and shows an error message.
func (s *Spinner) StopWithError(message string) {
	s.Stop()
	s.clearLine()
	_, _ = fmt.Fprintln(s.writer, ErrorMsg("%s", message))
}

// StopWithWarning stops the spinner and shows a warning message.
func (s *Spinner) StopWithWarning(message string) {
	s.Stop()
	s.clearLine()
	_, _ = fmt.Fprintln(s.writer, WarningMsg("%s", message))
}

// UpdateMessage changes the spinner message while running.
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

// run is the main animation loop.
func (s *Spinner) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.delay)
	defer ticker.Stop()

	// Initial render
	s.render()

	for {
		select {
		case <-s.stopCh:
			s.clearLine()
			return
		case <-ticker.C:
			s.mu.Lock()
			s.frameIdx = (s.frameIdx + 1) % len(s.frames)
			s.mu.Unlock()
			s.render()
		}
	}
}

// render draws the current spinner state.
func (s *Spinner) render() {
	s.mu.Lock()
	frame := s.frames[s.frameIdx]
	message := s.message
	s.mu.Unlock()

	if ColorsEnabled() {
		// Animated spinner with colors
		s.clearLine()
		_, _ = fmt.Fprintf(s.writer, "%s %s", Info(frame), message)
	} else {
		// Static fallback without animation
		// Only render once at the start
		if s.frameIdx == 0 {
			_, _ = fmt.Fprintf(s.writer, "... %s\n", message)
		}
	}
}

// clearLine clears the current line (for re-rendering).
func (s *Spinner) clearLine() {
	if ColorsEnabled() {
		// ANSI escape: move to column 0, clear to end of line
		_, _ = fmt.Fprint(s.writer, "\r\033[K")
	}
}
