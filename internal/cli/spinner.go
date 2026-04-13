package cli

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// Braille dot spinner frames (smooth rotation effect).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 80 * time.Millisecond

// Spinner displays an animated progress indicator in the terminal.
// Safe to use from multiple goroutines.
type Spinner struct {
	message string
	writer  *os.File
	done    chan struct{}
	mu      sync.Mutex
	wg      sync.WaitGroup
	running bool
}

// NewSpinner creates a spinner with the given message.
// Output goes to stderr by default (stdout may be piped).
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		writer:  os.Stderr,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a background goroutine.
// Does nothing if not connected to a terminal (non-TTY) or if --quiet mode is active.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	// Suppress spinner entirely in quiet mode
	if Quiet {
		s.running = true
		s.done = make(chan struct{})

		return
	}

	// Skip animation if not a TTY (e.g., piped output)
	if !term.IsTerminal(int(s.writer.Fd())) {
		s.running = true // Prevent duplicate prints on repeated Start() calls
		_, _ = fmt.Fprintf(s.writer, "%s\n", s.message)

		return
	}

	s.running = true
	s.done = make(chan struct{})
	s.wg.Add(1)

	go s.animate()
}

// Stop halts the spinner animation and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()

		return
	}

	close(s.done)
	s.running = false
	s.mu.Unlock()

	// Wait for animate() to exit before clearing line
	s.wg.Wait()

	// Clear line only if TTY
	if term.IsTerminal(int(s.writer.Fd())) {
		_, _ = fmt.Fprintf(s.writer, "\r\033[K")
	}
}

// Success stops the spinner and prints a success message with checkmark.
func (s *Spinner) Success(msg string) {
	s.Stop()
	if term.IsTerminal(int(s.writer.Fd())) {
		_, _ = fmt.Fprintf(s.writer, "\033[32m✓\033[0m %s\n", msg)
	} else {
		_, _ = fmt.Fprintf(s.writer, "✓ %s\n", msg)
	}
}

// Fail stops the spinner and prints a failure message with X mark.
func (s *Spinner) Fail(msg string) {
	s.Stop()
	if term.IsTerminal(int(s.writer.Fd())) {
		_, _ = fmt.Fprintf(s.writer, "\033[31m✗\033[0m %s\n", msg)
	} else {
		_, _ = fmt.Fprintf(s.writer, "✗ %s\n", msg)
	}
}

func (s *Spinner) animate() {
	defer s.wg.Done()

	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(s.writer, "\r\033[K%s %s", spinnerFrames[frame], s.message)
			frame = (frame + 1) % len(spinnerFrames)
		}
	}
}
