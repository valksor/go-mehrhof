package worker

import (
	"context"
	"time"

	"github.com/valksor/kvelmo/metrics"
)

// MetricsMiddleware records agent execution duration.
func MetricsMiddleware(m *metrics.Metrics) AgentMiddleware {
	return func(next AgentExecFunc) AgentExecFunc {
		return func(ctx context.Context, prompt string) error {
			start := time.Now()
			err := next(ctx, prompt)
			duration := time.Since(start)

			if m != nil {
				m.RecordAgentLatency(duration)
			}

			return err
		}
	}
}
