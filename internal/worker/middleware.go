package worker

import (
	"context"
	"slices"
)

// AgentExecFunc represents the core agent execution call.
type AgentExecFunc func(ctx context.Context, prompt string) error

// AgentMiddleware wraps an AgentExecFunc with additional behavior.
type AgentMiddleware func(next AgentExecFunc) AgentExecFunc

// Chain composes multiple middlewares into a single middleware.
// Middlewares are applied in order: first middleware is the outermost wrapper.
func Chain(middlewares ...AgentMiddleware) AgentMiddleware {
	return func(next AgentExecFunc) AgentExecFunc {
		for _, v := range slices.Backward(middlewares) {
			next = v(next)
		}

		return next
	}
}
