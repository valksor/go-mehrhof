// Internal test package to access unexported helpers.
package watchdog

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestStart_WindowTrimsWithoutTrigger drives the sampling loop long enough to
// overflow the window (exercising the samples[1:] trim and the in-loop
// isMonotonicallyGrowing call) while keeping ThresholdMB high enough that the
// leak branch never fires, so the process is not killed.
func TestStart_WindowTrimsWithoutTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		Interval:    2 * time.Millisecond,
		WindowSize:  3,
		ThresholdMB: 1_000_000, // unreachable: real heap growth stays far below this
		NoiseMB:     5,
	}

	done := Start(ctx, cfg)

	// Let many ticks fire so the window fills and is trimmed repeatedly.
	deadline := time.After(2 * time.Second)
	select {
	case <-done:
		t.Fatal("watchdog goroutine exited before cancel")
	case <-deadline:
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog goroutine did not exit after cancel")
	}
}

// TestStart_LeakDetectionExits runs the watchdog in a subprocess with a
// configuration guaranteed to trip the leak detector (threshold 0, every
// non-negative consecutive delta is within noise), verifying it writes a heap
// profile and exits non-zero. Running it in-process would kill the test binary,
// so the standard re-exec pattern is used.
func TestStart_LeakDetectionExits(t *testing.T) {
	if os.Getenv("WATCHDOG_LEAK_SUBPROCESS") == "1" {
		// Child process: a window of size 1 with threshold 0 trips on the first
		// full sample (last-first == 0 >= 0, and no consecutive pair to dip).
		ctx := context.Background()
		cfg := Config{
			Interval:    1 * time.Millisecond,
			WindowSize:  1,
			ThresholdMB: 0,
			NoiseMB:     5,
		}
		Start(ctx, cfg)
		// Block; the watchdog goroutine will call os.Exit(1).
		time.Sleep(10 * time.Second)

		return
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run", "TestStart_LeakDetectionExits")
	cmd.Env = append(os.Environ(), "WATCHDOG_LEAK_SUBPROCESS=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	if err == nil {
		t.Fatal("subprocess exited 0, want non-zero from leak detection")
	}
	if !asExitError(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("subprocess error = %v, want exit code 1", err)
	}

	// The leak path writes a heap profile to /tmp/kvelmo-leak-*.pprof.
	matches, _ := filepath.Glob("/tmp/kvelmo-leak-*.pprof")
	if len(matches) == 0 {
		t.Error("expected a heap profile to be written under /tmp")
	}
	t.Cleanup(func() {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	})
}

func asExitError(err error, target **exec.ExitError) bool {
	ee := &exec.ExitError{}
	ok := errors.As(err, &ee)
	if ok {
		*target = ee
	}

	return ok
}
