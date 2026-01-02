// Package httpclient tests
package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Benchmark_NewHTTPClient_Default benchmarks the default client creation.
// After optimization, this should return a shared singleton instance.
func Benchmark_NewHTTPClient_Default(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewHTTPClient()
	}
}

// Benchmark_HTTPClient_SequentialRequests benchmarks sequential requests
// using the pooled client. Connection pooling should keep this fast.
func Benchmark_HTTPClient_SequentialRequests(b *testing.B) {
	// Create test server that responds quickly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewHTTPClient()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		//nolint:noctx // Benchmark: no context cancellation needed
		resp, err := client.Get(server.URL)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}

// Benchmark_HTTPClient_ConcurrentRequests benchmarks concurrent requests
// to demonstrate connection pooling benefits.
func Benchmark_HTTPClient_ConcurrentRequests(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := NewHTTPClient()
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//nolint:noctx // Benchmark: no context cancellation needed
			resp, err := client.Get(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			_ = resp.Body.Close()
		}
	})
}

// Benchmark_HTTPClient_NoPooling benchmarks a client without connection pooling
// for comparison. This simulates the old behavior.
func Benchmark_HTTPClient_NoPooling(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create a new client each time - no pooling (old behavior)
		client := &http.Client{Timeout: 30 * time.Second}
		//nolint:noctx // Benchmark: no context cancellation needed
		resp, err := client.Get(server.URL)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close()
	}
}

// Benchmark_DefaultTransport_Creation benchmarks the transport creation.
// This should be fast as it's created once and reused.
func Benchmark_DefaultTransport_Creation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = defaultTransport()
	}
}

// Benchmark_NewHTTPClientWithTimeout benchmarks custom timeout client creation.
func Benchmark_NewHTTPClientWithTimeout(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewHTTPClientWithTimeout(60 * time.Second)
	}
}

// Benchmark_HTTPClient_ParallelClients benchmarks creating multiple clients
// in parallel, testing the sync.Once correctness.
func Benchmark_HTTPClient_ParallelClients(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = NewHTTPClient()
		}
	})
}

// Benchmark_WithRetry_NoFailure benchmarks retry logic overhead when no failures occur.
func Benchmark_WithRetry_NoFailure(b *testing.B) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := WithRetry(ctx, config, func() error {
			return nil // No error
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_SharedClient_Singleton verifies that repeated calls return
// the same instance (zero allocations after first call).
func Benchmark_SharedClient_Singleton(b *testing.B) {
	// Reset to ensure clean state
	sharedClient = nil
	sharedClientOnce = sync.Once{}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewHTTPClient()
	}
}
