package socket

import (
	"context"
	"log/slog"
	"net"
)

// contextKey is an unexported type for context keys to prevent collisions.
type contextKey int

const (
	rpcContextKey contextKey = iota
	connKey
)

// RPCContext carries metadata about the current RPC request.
type RPCContext struct {
	TraceID     string // Correlation ID for this request
	TaskID      string // Active task ID (if any)
	TaskTraceID string // Task lifecycle trace ID (spans all RPC calls for one task)
	WorktreeID  string // Worktree identifier
	Phase       string // Current conductor phase
	AgentID     string // Connected agent ID
	ConnID      string // Connection identifier
	Method      string // RPC method being called
	RemoteAddr  string // Client address
}

// WithRPCContext stores an RPCContext in the context.
func WithRPCContext(ctx context.Context, rc *RPCContext) context.Context {
	return context.WithValue(ctx, rpcContextKey, rc)
}

// RPCContextFromCtx retrieves the RPCContext from context.
// Returns nil if not present.
func RPCContextFromCtx(ctx context.Context) *RPCContext {
	rc, _ := ctx.Value(rpcContextKey).(*RPCContext)

	return rc
}

// WithConn stores the net.Conn in the context.
func WithConn(ctx context.Context, conn net.Conn) context.Context {
	return context.WithValue(ctx, connKey, conn)
}

// ConnFromCtx retrieves the net.Conn from context.
// Returns nil if not present.
func ConnFromCtx(ctx context.Context) net.Conn {
	conn, _ := ctx.Value(connKey).(net.Conn)

	return conn
}

// SlogAttrs returns slog attributes for structured logging.
// Empty fields are omitted from the output.
func (rc *RPCContext) SlogAttrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 8)

	if rc.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", rc.TraceID))
	}
	if rc.Method != "" {
		attrs = append(attrs, slog.String("method", rc.Method))
	}
	if rc.TaskID != "" {
		attrs = append(attrs, slog.String("task_id", rc.TaskID))
	}
	if rc.TaskTraceID != "" {
		attrs = append(attrs, slog.String("task_trace_id", rc.TaskTraceID))
	}
	if rc.WorktreeID != "" {
		attrs = append(attrs, slog.String("worktree_id", rc.WorktreeID))
	}
	if rc.Phase != "" {
		attrs = append(attrs, slog.String("phase", rc.Phase))
	}
	if rc.AgentID != "" {
		attrs = append(attrs, slog.String("agent_id", rc.AgentID))
	}
	if rc.ConnID != "" {
		attrs = append(attrs, slog.String("conn_id", rc.ConnID))
	}
	if rc.RemoteAddr != "" {
		attrs = append(attrs, slog.String("remote_addr", rc.RemoteAddr))
	}

	return attrs
}
