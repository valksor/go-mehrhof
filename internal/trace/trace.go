package trace

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

type ctxKey struct{}

// taskTraceKey is used to store task-level trace IDs (spanning entire task lifecycle).
type taskTraceKey struct{}

// NewID generates a new correlation ID.
func NewID() string {
	return uuid.NewString()
}

// WithID returns a new context with the given correlation ID.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// ID returns the correlation ID from the context, or empty string if not set.
func ID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)

	return id
}

// SlogAttr returns a slog attribute with the correlation ID from the context.
// Returns an empty attribute if no correlation ID is set.
func SlogAttr(ctx context.Context) slog.Attr {
	id := ID(ctx)
	if id == "" {
		return slog.Attr{}
	}

	return slog.String("correlation_id", id)
}

// WithTaskTrace stores a task trace ID in the context. The task trace ID spans
// the entire lifecycle of a task, linking all RPC calls and activity log entries
// from load through finish.
func WithTaskTrace(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, taskTraceKey{}, traceID)
}

// TaskTrace retrieves the task trace ID from context, or empty string if not set.
func TaskTrace(ctx context.Context) string {
	id, _ := ctx.Value(taskTraceKey{}).(string)

	return id
}

// TaskTraceSlogAttr returns a slog attribute for the task trace ID from context.
// Returns an empty attribute if no task trace is set.
func TaskTraceSlogAttr(ctx context.Context) slog.Attr {
	id := TaskTrace(ctx)
	if id == "" {
		return slog.Attr{}
	}

	return slog.String("task_trace_id", id)
}
