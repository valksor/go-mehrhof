package worker

import (
	"context"
	"log/slog"

	"github.com/valksor/kvelmo/internal/trace"
)

// TracingMiddleware logs agent execution lifecycle with trace correlation.
func TracingMiddleware() AgentMiddleware {
	return func(next AgentExecFunc) AgentExecFunc {
		return func(ctx context.Context, prompt string) error {
			traceID := trace.ID(ctx)
			if traceID == "" {
				traceID = "unknown"
			}
			slog.Info("agent execution started", "trace_id", traceID, "prompt_len", len(prompt))
			err := next(ctx, prompt)
			if err != nil {
				slog.Warn("agent execution failed", "trace_id", traceID, "error", err)
			} else {
				slog.Info("agent execution completed", "trace_id", traceID)
			}

			return err
		}
	}
}
