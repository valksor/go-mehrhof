package socket

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProtocol(t *testing.T) {
	req := &Request{ID: "1", Method: "test"}
	data, err := EncodeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"jsonrpc":"2.0"`)) {
		t.Error("missing jsonrpc")
	}
	if !bytes.Contains(data, []byte(`"protocol_version":"1"`)) {
		t.Error("missing protocol_version")
	}
}

func TestServerClientRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	srv := NewServer(sockPath)
	srv.Handle("echo", func(ctx context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, req.Params)
	})

	ctx := t.Context()

	go func() { _ = srv.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	params := json.RawMessage(`{"hello":"world"}`)
	resp, err := client.Call(ctx, "echo", params)
	if err != nil {
		t.Fatal(err)
	}

	if string(resp.Result) != `{"hello":"world"}` {
		t.Errorf("got %s", resp.Result)
	}
}

func TestPaths(t *testing.T) {
	path := GlobalSocketPath()
	if !strings.Contains(path, ".valksor/kvelmo") {
		t.Errorf("expected .valksor/kvelmo in path: %s", path)
	}
}

func TestCleanupStaleSocket(t *testing.T) {
	tmpDir := t.TempDir()
	stalePath := filepath.Join(tmpDir, "stale.sock")
	_ = os.WriteFile(stalePath, []byte{}, 0o644)

	removed, err := CleanupStaleSocket(stalePath)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected stale file removed")
	}
}

func TestGlobalSocket(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "global.sock")

	global := NewGlobalSocket(sockPath)
	ctx := t.Context()

	go func() { _ = global.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	// Register
	resp, err := client.Call(ctx, "projects.register", RegisterParams{Path: "/test", SocketPath: "/test.sock"})
	if err != nil {
		t.Fatal(err)
	}

	var reg map[string]string
	_ = json.Unmarshal(resp.Result, &reg)
	if reg["id"] == "" {
		t.Error("expected id")
	}

	// List
	resp, err = client.Call(ctx, "projects.list", nil)
	if err != nil {
		t.Fatal(err)
	}

	var list ProjectListResult
	_ = json.Unmarshal(resp.Result, &list)
	if len(list.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(list.Projects))
	}
}

func TestWorktreeSocket(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "wt.sock")

	wt := NewWorktreeSocketSimple(sockPath, "/test/project")
	ctx := t.Context()

	go func() { _ = wt.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	resp, err := client.Call(ctx, "status", nil)
	if err != nil {
		t.Fatal(err)
	}

	var status StatusResult
	_ = json.Unmarshal(resp.Result, &status)
	if status.State != StateNone {
		t.Errorf("expected none, got %s", status.State)
	}
	if status.Path != "/test/project" {
		t.Errorf("expected /test/project, got %s", status.Path)
	}
}

func TestClientRetry(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "retry.sock")

	// Server starts after a delay to test retry
	srv := NewServer(sockPath)
	srv.Handle("ping", func(ctx context.Context, req *Request) (*Response, error) {
		return NewResultResponse(req.ID, map[string]string{"pong": "ok"})
	})

	ctx := t.Context()

	// Start server after 150ms to simulate delayed startup
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = srv.Start(ctx)
	}()

	// Client with retry should succeed
	start := time.Now()
	client, err := NewClient(sockPath, WithRetry(5, 50*time.Millisecond, 1*time.Second))
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	defer func() { _ = client.Close() }()

	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected retry delays, but connected in %v", elapsed)
	}

	// Verify connection works
	resp, err := client.Call(ctx, "ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	_ = json.Unmarshal(resp.Result, &result)
	if result["pong"] != "ok" {
		t.Errorf("expected pong=ok, got %v", result)
	}
}

func TestClientNoRetry(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "noretry.sock")

	// Without retry, connection to non-existent socket should fail immediately
	_, err := NewClient(sockPath)
	if err == nil {
		t.Error("expected error for non-existent socket")
	}
}
