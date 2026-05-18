package claudemcp

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/mcp"
)

// TestRendezvousShutdownNoPanic exercises the panic fix from Pass 1: a
// connection goroutine that is mid-scan when the listener Close() fires must
// not panic by sending to a closed channel.
func TestRendezvousShutdownNoPanic(t *testing.T) {
	// Use /tmp explicitly to avoid macOS's 104-char Unix socket path limit;
	// t.TempDir() returns very long /var/folders/... paths.
	dir, err := os.MkdirTemp("/tmp", "kvelmo-rdz-") //nolint:usetesting // t.TempDir paths exceed Unix socket length limit on macOS
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "s")

	ctx := t.Context()

	rdz, events, err := startRendezvous(ctx, sockPath)
	if err != nil {
		t.Fatalf("startRendezvous: %v", err)
	}

	// Spawn N writers that each open a connection and send an event slowly.
	// The Close() call should cleanly tear everything down without panic.
	const writers = 8
	var wg sync.WaitGroup
	wg.Add(writers)
	dialer := net.Dialer{Timeout: time.Second}
	for range writers {
		go func() {
			defer wg.Done()
			conn, err := dialer.DialContext(ctx, "unix", sockPath)
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			data, err := json.Marshal(mcp.RendezvousEvent{Type: "complete", Phase: "plan"})
			if err != nil {
				return
			}
			// Sleep BEFORE writing so Close() can fire mid-handler in some
			// goroutines.
			time.Sleep(50 * time.Millisecond)
			_, _ = conn.Write(append(data, '\n'))
		}()
	}

	// Give writers a moment to connect.
	time.Sleep(20 * time.Millisecond)

	// Close while writers are still in flight — must not panic.
	if err := rdz.Close(); err != nil {
		// Removing an already-removed socket file returns ENOENT, that's fine.
		_ = err
	}

	// Drain any events that did arrive before close.
	drainDeadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				wg.Wait()

				return
			}
		case <-drainDeadline:
			wg.Wait()

			return
		}
	}
}

// TestRendezvousCloseIdempotent verifies that calling Close multiple times
// does not panic on the closeOnce.
func TestRendezvousCloseIdempotent(t *testing.T) {
	// Use /tmp explicitly to avoid macOS's 104-char Unix socket path limit;
	// t.TempDir() returns very long /var/folders/... paths.
	dir, err := os.MkdirTemp("/tmp", "kvelmo-rdz-") //nolint:usetesting // t.TempDir paths exceed Unix socket length limit on macOS
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "s")

	ctx := t.Context()

	rdz, _, err := startRendezvous(ctx, sockPath)
	if err != nil {
		t.Fatalf("startRendezvous: %v", err)
	}

	for range 3 {
		_ = rdz.Close()
	}
}
