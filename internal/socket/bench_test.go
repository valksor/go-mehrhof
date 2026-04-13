package socket

import (
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkRequestMarshal(b *testing.B) {
	req := &Request{
		JSONRPC: "2.0",
		ID:      "bench-1",
		Method:  "status",
	}

	b.ResetTimer()
	for range b.N {
		if _, err := json.Marshal(req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRequestUnmarshal(b *testing.B) {
	data := []byte(`{"jsonrpc":"2.0","id":"bench-1","method":"status"}`)

	b.ResetTimer()
	for range b.N {
		var req Request
		_ = json.Unmarshal(data, &req)
	}
}

func BenchmarkResponseMarshal(b *testing.B) {
	result := map[string]any{"status": "ok", "state": "none"}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		b.Fatal(err)
	}
	resp := &Response{
		JSONRPC: "2.0",
		ID:      "bench-1",
		Result:  resultJSON,
	}

	b.ResetTimer()
	for range b.N {
		if _, err := json.Marshal(resp); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewResultResponse(b *testing.B) {
	result := map[string]string{"status": "ok"}

	b.ResetTimer()
	for range b.N {
		_, _ = NewResultResponse("bench-1", result)
	}
}

func BenchmarkWorktreetSocketPathHash(b *testing.B) {
	path := "/Users/test/workspace/myproject"

	b.ResetTimer()
	for range b.N {
		WorktreeSocketPath(path)
	}
}

func BenchmarkClientCallRoundtrip(b *testing.B) {
	tmpDir := b.TempDir()
	sockPath := filepath.Join(tmpDir, "bench.sock")

	gs := NewGlobalSocket(sockPath)
	ctx := b.Context()

	go func() { _ = gs.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath, WithTimeout(5*time.Second))
	if err != nil {
		b.Fatalf("NewClient() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	b.ResetTimer()
	for range b.N {
		_, _ = client.Call(ctx, "ping", nil)
	}
}

func BenchmarkConcurrentClientCalls(b *testing.B) {
	tmpDir := b.TempDir()
	sockPath := filepath.Join(tmpDir, "b.sock")

	gs := NewGlobalSocket(sockPath)
	ctx := b.Context()

	go func() { _ = gs.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// Pre-create clients before benchmark
	const numClients = 4
	clients := make([]*Client, numClients)
	for i := range numClients {
		client, err := NewClient(sockPath, WithTimeout(5*time.Second))
		if err != nil {
			b.Fatalf("NewClient() error = %v", err)
		}
		clients[i] = client
	}
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()

	var clientIdx atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine uses its own client
		idx := clientIdx.Add(1) % numClients
		client := clients[idx]

		for pb.Next() {
			_, _ = client.Call(ctx, "ping", nil)
		}
	})
}
