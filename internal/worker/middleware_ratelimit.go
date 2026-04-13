package worker

import (
	"context"
	"errors"

	"github.com/valksor/kvelmo/internal/ratelimit"
)

// RateLimitMiddleware checks the rate limiter before execution.
func RateLimitMiddleware(limiter *ratelimit.Limiter) AgentMiddleware {
	return func(next AgentExecFunc) AgentExecFunc {
		return func(ctx context.Context, prompt string) error {
			if limiter != nil && !limiter.Allow("agent") {
				return errors.New("rate_limit: agent execution rate exceeded")
			}

			return next(ctx, prompt)
		}
	}
}
