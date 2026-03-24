package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

// TokenBudget tracks cumulative token usage with configurable limits.
type TokenBudget struct {
	consumed atomic.Int64
	limit    int64
}

// NewTokenBudget creates a budget with the given token limit. 0 = unlimited.
func NewTokenBudget(limit int64) *TokenBudget {
	return &TokenBudget{limit: limit}
}

// Add records token consumption. Returns true if within budget.
func (b *TokenBudget) Add(tokens int64) bool {
	newTotal := b.consumed.Add(tokens)

	return b.limit == 0 || newTotal <= b.limit
}

// Consumed returns total tokens consumed.
func (b *TokenBudget) Consumed() int64 {
	return b.consumed.Load()
}

// Remaining returns tokens remaining. Returns -1 if unlimited.
func (b *TokenBudget) Remaining() int64 {
	if b.limit == 0 {
		return -1
	}
	remaining := b.limit - b.consumed.Load()
	if remaining < 0 {
		return 0
	}

	return remaining
}

// TryAdd atomically reserves tokens only if within budget. Returns true if added.
func (b *TokenBudget) TryAdd(tokens int64) bool {
	if b.limit == 0 {
		b.consumed.Add(tokens)

		return true
	}
	for {
		current := b.consumed.Load()
		if current+tokens > b.limit {
			return false
		}
		if b.consumed.CompareAndSwap(current, current+tokens) {
			return true
		}
	}
}

// BudgetMiddleware enforces a token budget across agent calls.
func BudgetMiddleware(budget *TokenBudget) AgentMiddleware {
	return func(next AgentExecFunc) AgentExecFunc {
		var warned atomic.Bool

		return func(ctx context.Context, prompt string) error {
			if budget == nil {
				return next(ctx, prompt)
			}

			// Estimate prompt tokens (rough: 1 token ~ 4 chars)
			estimatedTokens := int64(len(prompt) / 4)

			// Atomically check and record prompt tokens
			if !budget.TryAdd(estimatedTokens) {
				return fmt.Errorf("budget_exhausted: token budget exceeded (consumed: %d, limit: %d)", budget.Consumed(), budget.limit)
			}

			// Warn at 80%
			if budget.limit > 0 {
				used := float64(budget.Consumed()) / float64(budget.limit)
				if used >= 0.8 && warned.CompareAndSwap(false, true) {
					slog.Warn("token budget 80% consumed", "consumed", budget.Consumed(), "limit", budget.limit)
				}
			}

			return next(ctx, prompt)
		}
	}
}
