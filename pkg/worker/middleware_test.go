package worker

import (
	"context"
	"errors"
	"testing"
)

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next AgentExecFunc) AgentExecFunc {
		return func(ctx context.Context, prompt string) error {
			order = append(order, "m1-before")
			err := next(ctx, prompt)
			order = append(order, "m1-after")

			return err
		}
	}

	m2 := func(next AgentExecFunc) AgentExecFunc {
		return func(ctx context.Context, prompt string) error {
			order = append(order, "m2-before")
			err := next(ctx, prompt)
			order = append(order, "m2-after")

			return err
		}
	}

	inner := func(_ context.Context, _ string) error {
		order = append(order, "inner")

		return nil
	}

	chained := Chain(m1, m2)(inner)
	_ = chained(context.Background(), "test")

	expected := []string{"m1-before", "m2-before", "inner", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("step %d: expected %q, got %q", i, want, order[i])
		}
	}
}

func TestChainPropagatesError(t *testing.T) {
	m := func(next AgentExecFunc) AgentExecFunc {
		return func(ctx context.Context, prompt string) error {
			return next(ctx, prompt)
		}
	}

	inner := func(_ context.Context, _ string) error {
		return errors.New("inner error")
	}

	chained := Chain(m)(inner)
	err := chained(context.Background(), "test")
	if err == nil || err.Error() != "inner error" {
		t.Errorf("expected inner error, got %v", err)
	}
}

func TestTokenBudget(t *testing.T) {
	b := NewTokenBudget(100)

	if !b.Add(50) {
		t.Error("should be within budget")
	}
	if b.Consumed() != 50 {
		t.Errorf("expected 50, got %d", b.Consumed())
	}
	if b.Remaining() != 50 {
		t.Errorf("expected 50 remaining, got %d", b.Remaining())
	}

	b.Add(60) // Over budget
	if b.Remaining() != 0 {
		t.Errorf("expected 0 remaining, got %d", b.Remaining())
	}
}

func TestTokenBudgetUnlimited(t *testing.T) {
	b := NewTokenBudget(0)
	b.Add(999999)
	if b.Remaining() != -1 {
		t.Errorf("expected -1 (unlimited), got %d", b.Remaining())
	}
}

func TestBudgetMiddleware_ExceedsBudget(t *testing.T) {
	budget := NewTokenBudget(10) // Very small budget

	inner := func(_ context.Context, _ string) error {
		return nil
	}

	wrapped := BudgetMiddleware(budget)(inner)
	// A long prompt should exceed the budget
	err := wrapped(context.Background(), "this is a very long prompt that will exceed the tiny budget limit of 10 tokens")
	if err == nil {
		t.Error("expected budget exceeded error")
	}
}
