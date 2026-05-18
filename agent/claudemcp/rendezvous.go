package claudemcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/valksor/kvelmo/internal/mcp"
)

// rendezvousListener owns the per-task Unix socket the MCP subprocess writes
// to when claude calls kvelmo_signal_complete or kvelmo_signal_failure.
type rendezvousListener struct {
	path      string
	l         net.Listener
	done      chan struct{}
	closeOnce sync.Once
}

// startRendezvous binds a Unix domain socket at path (0600) and returns a
// listener plus a channel that emits each parsed RendezvousEvent until the
// context is cancelled or the listener is closed.
func startRendezvous(ctx context.Context, path string) (*rendezvousListener, <-chan mcp.RendezvousEvent, error) {
	_ = os.Remove(path) // ignore "not exist" errors
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "unix", path)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		_ = os.Remove(path)

		return nil, nil, fmt.Errorf("chmod %s: %w", path, err)
	}

	events := make(chan mcp.RendezvousEvent, 8)
	done := make(chan struct{})
	rl := &rendezvousListener{path: path, l: l, done: done}

	go func() {
		defer close(events)
		var connWG sync.WaitGroup
		go func() {
			<-ctx.Done()
			_ = l.Close()
		}()
		for {
			conn, err := l.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
					connWG.Wait()

					return
				}

				continue
			}
			connWG.Go(func() {
				handleRendezvousConn(conn, events, done)
			})
		}
	}()

	return rl, events, nil
}

// handleRendezvousConn reads newline-delimited JSON RendezvousEvent frames
// from conn and forwards them to out. If done closes before a send
// completes, the event is dropped instead of panicking on a closed channel.
func handleRendezvousConn(conn net.Conn, out chan<- mcp.RendezvousEvent, done <-chan struct{}) {
	defer func() { _ = conn.Close() }()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var evt mcp.RendezvousEvent
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}
		select {
		case out <- evt:
		case <-done:
			return
		}
	}
}

// Close stops the listener, signals in-flight connection goroutines to abort
// pending sends, and removes the socket file. Safe to call repeatedly.
func (r *rendezvousListener) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		close(r.done)
		_ = r.l.Close()
	})

	return os.Remove(r.path)
}
