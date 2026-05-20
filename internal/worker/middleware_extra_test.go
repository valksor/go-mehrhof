package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/ratelimit"
	"github.com/valksor/kvelmo/metrics"
)

func TestMetricsMiddleware_RecordsLatency(t *testing.T) {
	m := metrics.New()
	mw := MetricsMiddleware(m)

	called := false
	chain := mw(func(_ context.Context, _ string) error {
		called = true
		time.Sleep(5 * time.Millisecond)

		return nil
	})

	if err := chain(context.Background(), "hi"); err != nil {
		t.Errorf("call: %v", err)
	}
	if !called {
		t.Error("inner func not called")
	}
	if got := m.Snapshot().AgentAvgLatencyMs; got <= 0 {
		t.Errorf("AgentAvgLatencyMs = %f, want > 0", got)
	}
}

func TestMetricsMiddleware_NilMetrics(t *testing.T) {
	mw := MetricsMiddleware(nil)
	chain := mw(func(_ context.Context, _ string) error { return nil })

	if err := chain(context.Background(), ""); err != nil {
		t.Errorf("nil-metrics middleware: %v", err)
	}
}

func TestMetricsMiddleware_PreservesError(t *testing.T) {
	mw := MetricsMiddleware(metrics.New())
	sentinel := errors.New("boom")
	chain := mw(func(_ context.Context, _ string) error { return sentinel })

	if err := chain(context.Background(), ""); !errors.Is(err, sentinel) {
		t.Errorf("error not propagated, got %v", err)
	}
}

func TestRateLimitMiddleware_Allows(t *testing.T) {
	limiter := ratelimit.NewLimiter(100, 10)
	defer limiter.Stop()
	mw := RateLimitMiddleware(limiter)

	calls := 0
	chain := mw(func(_ context.Context, _ string) error {
		calls++

		return nil
	})

	if err := chain(context.Background(), ""); err != nil {
		t.Errorf("first call: %v", err)
	}
	if calls != 1 {
		t.Errorf("inner called %d times", calls)
	}
}

func TestRateLimitMiddleware_NilLimiter(t *testing.T) {
	mw := RateLimitMiddleware(nil)
	chain := mw(func(_ context.Context, _ string) error { return nil })
	if err := chain(context.Background(), ""); err != nil {
		t.Errorf("nil limiter should allow: %v", err)
	}
}

func TestRateLimitMiddleware_Blocks(t *testing.T) {
	limiter := ratelimit.NewLimiter(0.001, 1)
	defer limiter.Stop()
	mw := RateLimitMiddleware(limiter)

	chain := mw(func(_ context.Context, _ string) error { return nil })

	_ = chain(context.Background(), "")
	if err := chain(context.Background(), ""); err == nil {
		t.Error("expected rate limit error")
	}
}

func TestTracingMiddleware_LogsAndPropagates(t *testing.T) {
	mw := TracingMiddleware()

	chain := mw(func(_ context.Context, _ string) error { return nil })
	if err := chain(context.Background(), "p"); err != nil {
		t.Errorf("success: %v", err)
	}

	sentinel := errors.New("trace fail")
	chain = mw(func(_ context.Context, _ string) error { return sentinel })
	if err := chain(context.Background(), "p"); !errors.Is(err, sentinel) {
		t.Errorf("error not propagated, got %v", err)
	}
}
