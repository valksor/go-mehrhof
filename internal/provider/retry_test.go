package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithRetry_Success(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, DefaultRetryConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", calls.Load())
	}
}

func TestDoWithRetry_5xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls (1 fail + 1 success), got %d", calls.Load())
	}
}

func TestDoWithRetry_429RateLimit(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

func TestDoWithRetry_4xxNoRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call (no retry for 4xx), got %d", calls.Load())
	}
}

func TestDoWithRetry_MaxRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	// 1 initial + 2 retries = 3 calls
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", calls.Load())
	}
}

func TestDoWithRetry_NetworkError(t *testing.T) {
	// Server that immediately closes connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	server.Close() // Close immediately to cause network error

	cfg := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(http.DefaultClient, req, cfg)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if err == nil {
		t.Error("expected error for closed server")
	}
}

func TestDoWithRetry_ContextCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after first request
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	cfg := RetryConfig{MaxRetries: 10, BaseDelay: 50 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, cfg)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if err == nil {
		t.Error("expected context cancellation error")
	}
	// Should have stopped early due to context cancellation
	if calls.Load() > 3 {
		t.Errorf("expected early stop due to cancellation, got %d calls", calls.Load())
	}
}

func TestDoWithRetry_NoRetryConfig(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := RetryConfig{MaxRetries: 0} // No retries
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := DoWithRetry(server.Client(), req, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if calls.Load() != 1 {
		t.Errorf("expected 1 call with MaxRetries=0, got %d", calls.Load())
	}
}

func TestDoWithRetry_ExponentialBackoff(t *testing.T) {
	var mu sync.Mutex
	var timestamps []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 20 * time.Millisecond, MaxDelay: 200 * time.Millisecond}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, _ := DoWithRetry(server.Client(), req, cfg)
	if resp != nil {
		_ = resp.Body.Close()
	}

	mu.Lock()
	ts := make([]time.Time, len(timestamps))
	copy(ts, timestamps)
	mu.Unlock()

	if len(ts) < 3 {
		t.Fatalf("expected at least 3 timestamps, got %d", len(ts))
	}

	// Verify delays increase (accounting for jitter)
	delay1 := ts[1].Sub(ts[0])
	delay2 := ts[2].Sub(ts[1])

	// With jitter ±25% and 2x multiplier, worst-case ratio is 2*0.75/1.25 = 1.2x
	// Use 1.1x threshold to account for timing measurement overhead
	if delay1 < 10*time.Millisecond {
		t.Errorf("first delay too short: %v", delay1)
	}
	if delay2 < time.Duration(float64(delay1)*1.1) {
		t.Errorf("expected exponential increase (>=1.1x), delay1=%v delay2=%v", delay1, delay2)
	}
}
