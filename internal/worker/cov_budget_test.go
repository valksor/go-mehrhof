package worker

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestTokenBudget_TryAdd(t *testing.T) {
	t.Run("unlimited always succeeds", func(t *testing.T) {
		b := NewTokenBudget(0)
		if !b.TryAdd(1_000_000) {
			t.Error("TryAdd on unlimited budget should always succeed")
		}
		if b.Consumed() != 1_000_000 {
			t.Errorf("Consumed = %d, want 1000000", b.Consumed())
		}
		if b.Remaining() != -1 {
			t.Errorf("Remaining = %d, want -1 (unlimited)", b.Remaining())
		}
	})

	t.Run("within limit succeeds and tracks", func(t *testing.T) {
		b := NewTokenBudget(100)
		if !b.TryAdd(40) {
			t.Error("TryAdd(40) within budget should succeed")
		}
		if !b.TryAdd(60) {
			t.Error("TryAdd(60) reaching exactly the limit should succeed")
		}
		if b.Remaining() != 0 {
			t.Errorf("Remaining = %d, want 0", b.Remaining())
		}
	})

	t.Run("over limit is rejected without consuming", func(t *testing.T) {
		b := NewTokenBudget(100)
		_ = b.TryAdd(90)
		if b.TryAdd(20) {
			t.Error("TryAdd(20) exceeding budget should be rejected")
		}
		if b.Consumed() != 90 {
			t.Errorf("Consumed = %d, want 90 (rejected add must not consume)", b.Consumed())
		}
		if b.Remaining() != 10 {
			t.Errorf("Remaining = %d, want 10", b.Remaining())
		}
	})

	t.Run("concurrent TryAdd never exceeds limit", func(t *testing.T) {
		b := NewTokenBudget(1000)
		var wg sync.WaitGroup
		for range 200 {
			wg.Go(func() {
				b.TryAdd(10)
			})
		}
		wg.Wait()
		if b.Consumed() > 1000 {
			t.Errorf("Consumed = %d, exceeded limit 1000", b.Consumed())
		}
	})
}

func TestBudgetMiddleware(t *testing.T) {
	t.Run("nil budget passes through", func(t *testing.T) {
		called := false
		mw := BudgetMiddleware(nil)
		exec := mw(func(_ context.Context, _ string) error {
			called = true

			return nil
		})
		if err := exec(context.Background(), "anything"); err != nil {
			t.Fatalf("exec error = %v", err)
		}
		if !called {
			t.Error("next func should be called with nil budget")
		}
	})

	t.Run("within budget invokes next", func(t *testing.T) {
		b := NewTokenBudget(1000)
		called := false
		mw := BudgetMiddleware(b)
		exec := mw(func(_ context.Context, _ string) error {
			called = true

			return nil
		})
		if err := exec(context.Background(), "short prompt"); err != nil {
			t.Fatalf("exec error = %v", err)
		}
		if !called {
			t.Error("next func should be invoked when within budget")
		}
	})

	t.Run("exhausted budget blocks next", func(t *testing.T) {
		b := NewTokenBudget(2) // tiny limit; a prompt of >8 chars exceeds it
		called := false
		mw := BudgetMiddleware(b)
		exec := mw(func(_ context.Context, _ string) error {
			called = true

			return nil
		})
		err := exec(context.Background(), strings.Repeat("x", 400))
		if err == nil {
			t.Fatal("expected budget_exhausted error")
		}
		if !strings.Contains(err.Error(), "budget_exhausted") {
			t.Errorf("error = %v, want budget_exhausted", err)
		}
		if called {
			t.Error("next func must NOT run when budget is exhausted")
		}
	})

	t.Run("warns at 80% but still proceeds", func(t *testing.T) {
		// Limit 100 tokens; a 320-char prompt = 80 tokens => crosses 80%.
		b := NewTokenBudget(100)
		mw := BudgetMiddleware(b)
		exec := mw(func(_ context.Context, _ string) error { return nil })
		if err := exec(context.Background(), strings.Repeat("y", 320)); err != nil {
			t.Fatalf("exec error = %v", err)
		}
		if b.Consumed() != 80 {
			t.Errorf("Consumed = %d, want 80", b.Consumed())
		}
	})
}
