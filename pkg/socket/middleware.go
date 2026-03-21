package socket

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/metrics"
	"github.com/valksor/kvelmo/pkg/ratelimit"
	"github.com/valksor/kvelmo/pkg/trace"
)

// HandlerFunc is the unified handler signature for middleware.
// It receives a context and request, returning a response. Connection
// information is carried in the context when needed.
type HandlerFunc func(ctx context.Context, req *Request) *Response

// Middleware wraps a HandlerFunc with additional behavior.
type Middleware func(HandlerFunc) HandlerFunc

// context key for connection metadata.
type connInfoKeyCtx struct{}

// ConnFromContext returns the net.Conn stored in ctx, or nil.
// This is an alias for ConnFromCtx for use by middleware and adapted handlers.
func ConnFromContext(ctx context.Context) net.Conn {
	return ConnFromCtx(ctx)
}

// ConnInfo holds connection metadata stored in the context.
type ConnInfo struct {
	Key string
}

// ConnInfoFromContext returns the ConnInfo stored in ctx, or a zero value.
func ConnInfoFromContext(ctx context.Context) ConnInfo {
	info, _ := ctx.Value(connInfoKeyCtx{}).(ConnInfo)

	return info
}

// withConnInfo stores ConnInfo in the context.
func withConnInfo(ctx context.Context, info ConnInfo) context.Context {
	return context.WithValue(ctx, connInfoKeyCtx{}, info)
}

// RecoveryMiddleware catches panics in handlers, logs the stack trace, and
// returns an internal error response.
func RecoveryMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		//nolint:nonamedreturns // Named return required for defer/recover to modify return value
		return func(ctx context.Context, req *Request) (resp *Response) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("rpc handler panic",
						"method", req.Method,
						"id", req.ID,
						"panic", r,
						"stack", string(debug.Stack()),
					)
					resp = NewErrorResponse(req.ID, ErrCodeInternal, "internal server error")
				}
			}()

			return next(ctx, req)
		}
	}
}

// TraceMiddleware generates a correlation ID via trace.NewID and adds it to
// the context. Downstream middleware and handlers can retrieve it with
// trace.ID(ctx).
func TraceMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			corrID := trace.NewID()
			ctx = trace.WithID(ctx, corrID)

			return next(ctx, req)
		}
	}
}

// MetricsMiddleware records request count, duration, and errors using the
// provided Metrics instance. If m is nil, metrics.Global() is used.
func MetricsMiddleware(m *metrics.Metrics) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			start := time.Now()

			resp := next(ctx, req)

			duration := time.Since(start)
			var err error
			if resp != nil && resp.Error != nil {
				err = resp.Error
			}

			recorder := m
			if recorder == nil {
				recorder = metrics.Global()
			}
			recorder.RecordRPCRequest(req.Method, duration, err)

			corrID := trace.ID(ctx)
			if err != nil {
				// RPC errors (returned to client) are expected; log at debug.
				// Only unexpected handler failures warrant error level.
				slog.Debug("rpc request failed",
					"method", req.Method,
					"id", req.ID,
					"correlation_id", corrID,
					"error", err,
					"duration_ms", duration.Milliseconds(),
				)
			} else {
				slog.Debug("rpc request",
					"method", req.Method,
					"id", req.ID,
					"correlation_id", corrID,
					"duration_ms", duration.Milliseconds(),
				)
			}

			return resp
		}
	}
}

// ActivityLogMiddleware records each RPC call to the ActivityLogger returned by
// loggerFn. The function is called on every request so that loggers set after
// server construction (e.g. via SetActivityLogger) are picked up.
// When writeOnly is true, only mutating methods are logged.
func ActivityLogMiddleware(loggerFn func() ActivityLogger, writeOnly bool) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			start := time.Now()

			resp := next(ctx, req)

			logger := loggerFn()
			if logger == nil {
				return resp
			}
			if writeOnly && !isWriteMethod(req.Method) {
				return resp
			}

			duration := time.Since(start)
			corrID := trace.ID(ctx)
			var errMsg string
			if resp != nil && resp.Error != nil {
				errMsg = resp.Error.Message
			}

			entry := ActivityEntry{
				Method:        req.Method,
				CorrelationID: corrID,
				DurationMs:    duration.Milliseconds(),
				Error:         errMsg,
				ParamsSize:    len(req.Params),
			}

			// Extract task trace ID from RPCContext if available.
			if rc := RPCContextFromCtx(ctx); rc != nil {
				entry.TaskTraceID = rc.TaskTraceID
			}

			// Fall back to trace.TaskTrace from context.
			if entry.TaskTraceID == "" {
				entry.TaskTraceID = trace.TaskTrace(ctx)
			}

			logger.Record(entry)

			return resp
		}
	}
}

// writeVerbs are the method prefixes considered mutating for activity logging.
var writeVerbs = []string{
	"set", "create", "delete", "remove", "update", "start", "stop",
	"abort", "reset", "submit", "approve", "reject", "finish",
	"plan", "implement", "simplify", "optimize", "review",
	"undo", "redo", "retry", "abandon", "shutdown",
}

// isWriteMethod reports whether the RPC method name starts with a known
// write verb (before the first dot separator, if any).
func isWriteMethod(method string) bool {
	// Extract the verb part: "projects.register" -> "register", "start" -> "start"
	verb := method
	if idx := strings.IndexByte(method, '.'); idx >= 0 {
		// For dotted methods like "task.start", use the action part after the dot
		verb = method[idx+1:]
	}
	verb = strings.ToLower(verb)

	for _, wv := range writeVerbs {
		if strings.HasPrefix(verb, wv) {
			return true
		}
	}

	return false
}

// RateLimitMiddleware checks the rate limiter before passing the request to
// the next handler. The key function extracts a rate-limit key from the
// context (typically the connection key stored by the server). If the limit
// is exceeded, an error response with ErrCodeRateLimited is returned.
func RateLimitMiddleware(limiter *ratelimit.Limiter, keyFn func(ctx context.Context) string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) *Response {
			if limiter == nil {
				return next(ctx, req)
			}

			key := "unknown"
			if keyFn != nil {
				key = keyFn(ctx)
			}

			if !limiter.Allow(key) {
				return NewErrorResponse(req.ID, ErrCodeRateLimited, "rate limit exceeded")
			}

			return next(ctx, req)
		}
	}
}

// buildChain applies middleware in reverse order so that the first middleware
// added is the outermost wrapper (executes first).
func buildChain(handler HandlerFunc, middleware []Middleware) HandlerFunc {
	wrapped := handler
	for i := len(middleware) - 1; i >= 0; i-- {
		wrapped = middleware[i](wrapped)
	}

	return wrapped
}

// adaptHandler converts a Handler (with net.Conn parameter) into a
// HandlerFunc by pulling the connection from the context.
func adaptHandler(h Handler) HandlerFunc {
	return func(ctx context.Context, req *Request) *Response {
		conn := ConnFromContext(ctx)
		resp, err := h(ctx, req, conn)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
		}

		return resp
	}
}

// adaptLegacyHandler converts a LegacyHandler into a HandlerFunc.
func adaptLegacyHandler(h LegacyHandler) HandlerFunc {
	return func(ctx context.Context, req *Request) *Response {
		resp, err := h(ctx, req)
		if err != nil {
			return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
		}

		return resp
	}
}

// defaultConnKeyFn extracts the connection key from context for rate limiting.
func defaultConnKeyFn(ctx context.Context) string {
	info := ConnInfoFromContext(ctx)
	if info.Key != "" {
		return info.Key
	}

	return fmt.Sprintf("ctx-%p", ctx)
}
