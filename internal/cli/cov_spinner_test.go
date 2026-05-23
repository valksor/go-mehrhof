package cli

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// drainPTY reads from the pty master in the background so the slave's writes
// never block, returning the accumulated output once stop() is called.
func drainPTY(t *testing.T, master io.Reader) func() string {
	t.Helper()
	var (
		buf strings.Builder
		m   sync.Mutex
	)
	go func() {
		b := make([]byte, 256)
		for {
			n, err := master.Read(b)
			if n > 0 {
				m.Lock()
				buf.Write(b[:n])
				m.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	return func() string {
		m.Lock()
		defer m.Unlock()

		return buf.String()
	}
}

func TestSpinner_Start_QuietModeSuppresses(t *testing.T) {
	prev := Quiet
	Quiet = true
	t.Cleanup(func() { Quiet = prev })

	s := NewSpinner("working")
	s.Start()

	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		t.Error("quiet-mode Start should mark the spinner running")
	}

	// Stop must be safe and reset state even though no goroutine was spawned.
	s.Stop()

	s.mu.Lock()
	running = s.running
	s.mu.Unlock()
	if running {
		t.Error("spinner still running after Stop in quiet mode")
	}
}

func TestSpinner_Start_AnimatesOnTTY(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	t.Cleanup(func() { _ = master.Close(); _ = slave.Close() })

	read := drainPTY(t, master)

	s := NewSpinner("loading")
	s.writer = slave
	s.Start()

	// Let the animation tick a few times (interval is 80ms).
	deadline := time.After(2 * time.Second)
	for !strings.Contains(read(), "loading") {
		select {
		case <-deadline:
			s.Stop()
			t.Fatal("animation never wrote the spinner frame to the TTY")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	s.Stop()

	out := read()
	// At least one braille frame should have been emitted.
	hasFrame := false
	for _, f := range spinnerFrames {
		if strings.Contains(out, f) {
			hasFrame = true

			break
		}
	}
	if !hasFrame {
		t.Errorf("expected a spinner frame in TTY output, got %q", out)
	}
}

func TestSpinner_Success_TTYUsesColor(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	t.Cleanup(func() { _ = master.Close(); _ = slave.Close() })

	read := drainPTY(t, master)

	s := NewSpinner("work")
	s.writer = slave
	s.Success("done")

	// Allow the pty drain goroutine to observe the write.
	waitFor(t, read, "done")
	out := read()
	if !strings.Contains(out, "\033[32m") {
		t.Errorf("expected green ANSI color in TTY success output, got %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected checkmark, got %q", out)
	}
}

func TestSpinner_Fail_TTYUsesColor(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	t.Cleanup(func() { _ = master.Close(); _ = slave.Close() })

	read := drainPTY(t, master)

	s := NewSpinner("work")
	s.writer = slave
	s.Fail("boom")

	waitFor(t, read, "boom")
	out := read()
	if !strings.Contains(out, "\033[31m") {
		t.Errorf("expected red ANSI color in TTY fail output, got %q", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected X mark, got %q", out)
	}
}

// waitFor polls read() until substr appears or the deadline elapses.
func waitFor(t *testing.T, read func() string, substr string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if strings.Contains(read(), substr) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("never saw %q in output, got %q", substr, read())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestIsInteractive_NonTTYStdin(t *testing.T) {
	// Under `go test` stdin is normally not a TTY, so this exercises the call
	// and asserts it returns a bool without panicking. The exact value depends
	// on the environment, so we only require it to be callable.
	_ = IsInteractive()
}
