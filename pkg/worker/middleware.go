package worker

import "context"

// AgentExecFunc represents the core agent execution call.
type AgentExecFunc func(ctx context.Context, prompt string) error

// AgentMiddleware wraps an AgentExecFunc with additional behavior.
type AgentMiddleware func(next AgentExecFunc) AgentExecFunc

// Chain composes multiple middlewares into a single middleware.
// Middlewares are applied in order: first middleware is the outermost wrapper.
func Chain(middlewares ...AgentMiddleware) AgentMiddleware {
	return func(next AgentExecFunc) AgentExecFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}

		return next
	}
}
