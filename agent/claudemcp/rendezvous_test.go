package claudemcp

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/mcp"
)

func dialUnix(t *testing.T, path string) net.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	return conn
}

func TestRendezvousReceivesEvent(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "adapter.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rdz, events, err := startRendezvous(ctx, sockPath)
	if err != nil {
		t.Fatalf("startRendezvous: %v", err)
	}
	defer func() { _ = rdz.Close() }()

	// Connect as the MCP server would and send a complete event.
	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	evt := mcp.RendezvousEvent{Type: "complete", Phase: "plan", Summary: "ok"}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case got := <-events:
		if got.Type != "complete" || got.Phase != "plan" || got.Summary != "ok" {
			t.Errorf("unexpected event: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}
