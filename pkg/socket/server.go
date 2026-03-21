package socket

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/pkg/metrics"
	"github.com/valksor/kvelmo/pkg/ratelimit"
)

const (
	ShutdownTimeout   = 5 * time.Second
	StaleCheckTimeout = 100 * time.Millisecond
)

// Handler handles a JSON-RPC request and returns a response.
// The conn parameter allows handlers to send streaming events outside the request/response cycle.
type Handler func(ctx context.Context, req *Request, conn net.Conn) (*Response, error)

// LegacyHandler is the old handler signature without connection access.
//
// Deprecated: Use Handler instead for new handlers.
type LegacyHandler func(ctx context.Context, req *Request) (*Response, error)

// ActivityLogger is called after each RPC dispatch to record activity.
type ActivityLogger interface {
	Record(entry ActivityEntry)
}

// ActivityEntry holds the data for a single activity log entry.
type ActivityEntry struct {
	Method        string
	CorrelationID string
	DurationMs    int64
	Error         string
	ParamsSize    int
	UserID        string
	TaskID        string
	AgentModel    string
	TaskTraceID   string
}

// ServerOption configures a Server during construction.
type ServerOption func(*Server)

// WithActivityLogger sets the activity logger for RPC call recording.
func WithActivityLogger(logger ActivityLogger) ServerOption {
	return func(s *Server) {
		if logger != nil {
			s.activityLogger = logger
		}
	}
}

// WithMetrics sets a custom metrics recorder. When nil, the global
// metrics.Global() instance is used (default behaviour in dispatch).
func WithMetrics(m *metrics.Metrics) ServerOption {
	return func(s *Server) {
		s.metrics = m
	}
}

// WithDrainTimeout overrides the default shutdown drain timeout.
func WithDrainTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		if d > 0 {
			s.drainTimeout = d
		}
	}
}

// WithRateLimiter replaces the default rate limiter.
func WithRateLimiter(rl *ratelimit.Limiter) ServerOption {
	return func(s *Server) {
		if rl != nil {
			s.rateLimiter = rl
		}
	}
}

type Server struct {
	path           string
	listener       net.Listener
	handlers       map[string]Handler
	legacyHandlers map[string]LegacyHandler
	mu             sync.RWMutex
	conns          map[net.Conn]struct{}
	connsMu        sync.Mutex
	activeConns    atomic.Int32
	shutdownCh     chan struct{}
	isShutdown     atomic.Bool
	shutdownOnce   sync.Once
	rateLimiter    *ratelimit.Limiter
	activityLogger ActivityLogger
	metrics        *metrics.Metrics // nil means use metrics.Global()
	drainTimeout   time.Duration    // 0 means use ShutdownTimeout
	username       string           // Cached OS username for audit logging
	middleware     []Middleware     // Middleware chain applied to every handler
}

// NewServer creates a new Unix domain socket server.
// Options are applied after default initialization, so callers that pass
// no options get the same behaviour as before.
func NewServer(path string, opts ...ServerOption) *Server {
	s := &Server{
		path:           path,
		handlers:       make(map[string]Handler),
		legacyHandlers: make(map[string]LegacyHandler),
		conns:          make(map[net.Conn]struct{}),
		shutdownCh:     make(chan struct{}),
		rateLimiter:    ratelimit.NewLimiter(16.67, 100), // ~1000 requests/min with burst of 100
	}
	if u, err := user.Current(); err == nil {
		s.username = u.Username
	}
	for _, opt := range opts {
		opt(s)
	}

	// Register default middleware in order: Recovery (outermost), Trace,
	// RateLimit, Metrics, ActivityLog (innermost).
	// ActivityLogMiddleware takes a function so that loggers set after
	// construction (via SetActivityLogger) are picked up.
	s.middleware = []Middleware{
		RecoveryMiddleware(),
		TraceMiddleware(),
		RateLimitMiddleware(s.rateLimiter, defaultConnKeyFn),
		MetricsMiddleware(s.metrics),
		ActivityLogMiddleware(func() ActivityLogger { return s.activityLogger }, false),
	}

	return s
}

// SetActivityLogger sets the activity logger for RPC call recording.
//
// Deprecated: Use WithActivityLogger option in NewServer instead.
func (s *Server) SetActivityLogger(l ActivityLogger) {
	s.activityLogger = l
}

// Use adds middleware that wraps every handler in the dispatch chain.
// Middleware is applied in the order added (first added = outermost wrapper).
func (s *Server) Use(mw ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.middleware = append(s.middleware, mw...)
}

// getDrainTimeout returns the configured drain timeout, falling back to
// ShutdownTimeout when none was provided via WithDrainTimeout.
func (s *Server) getDrainTimeout() time.Duration {
	if s.drainTimeout > 0 {
		return s.drainTimeout
	}

	return ShutdownTimeout
}

// Handle registers a legacy handler (without connection access).
// For handlers that need to send streaming events, use HandleWithConn.
func (s *Server) Handle(method string, h LegacyHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.legacyHandlers[method] = h
}

// HandleWithConn registers a handler that receives the connection for streaming events.
func (s *Server) HandleWithConn(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

func (s *Server) ActiveConnections() int {
	return int(s.activeConns.Load())
}

func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	if _, err := CleanupStaleSocket(s.path); err != nil {
		return fmt.Errorf("cleanup stale socket: %w", err)
	}

	var err error
	s.listener, err = net.Listen("unix", s.path) //nolint:noctx // Context cancellation handled via server shutdown
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	if err := os.Chmod(s.path, 0o600); err != nil {
		_ = s.listener.Close()

		return fmt.Errorf("chmod socket: %w", err)
	}

	s.rateLimiter.Start(ctx)

	go func() {
		<-ctx.Done()
		s.initiateShutdown()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				s.waitForDrain()

				return ctx.Err()
			default:
				if s.isShutdown.Load() {
					s.waitForDrain()

					return nil
				}

				return fmt.Errorf("accept: %w", err)
			}
		}

		s.trackConn(conn, true)
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) initiateShutdown() {
	s.shutdownOnce.Do(func() {
		s.isShutdown.Store(true)
		if s.listener != nil {
			_ = s.listener.Close()
		}
		close(s.shutdownCh)

		s.connsMu.Lock()
		for conn := range s.conns {
			resp := NewErrorResponse("", ErrCodeShuttingDown, "server shutting down")
			data, _ := EncodeResponse(resp)
			if _, err := conn.Write(data); err != nil {
				slog.Debug("failed to send shutdown notice", "error", err)
			}
		}
		s.connsMu.Unlock()
	})
}

func (s *Server) waitForDrain() {
	deadline := time.After(s.getDrainTimeout())
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			s.forceCloseAll()

			return
		case <-ticker.C:
			if s.activeConns.Load() == 0 {
				return
			}
		}
	}
}

func (s *Server) forceCloseAll() {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

func (s *Server) trackConn(conn net.Conn, add bool) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if add {
		s.conns[conn] = struct{}{}
		s.activeConns.Add(1)
	} else {
		delete(s.conns, conn)
		s.activeConns.Add(-1)
	}
}

func (s *Server) connKey(conn net.Conn) string {
	if addr := conn.RemoteAddr(); addr != nil {
		if key := addr.String(); key != "" {
			return key
		}
	}

	return fmt.Sprintf("conn-%p", conn)
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	key := s.connKey(conn)

	// Enrich context with connection info so middleware can access it.
	ctx = WithConn(ctx, conn)
	ctx = withConnInfo(ctx, ConnInfo{Key: key})

	defer func() {
		if r := recover(); r != nil {
			slog.Error("socket handler panic recovered", "panic", r, "stack", string(debug.Stack()))
		}
		_ = conn.Close()
		s.trackConn(conn, false)
		s.rateLimiter.Remove(key)
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // allow messages up to 4MB
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		req, err := DecodeRequest(line)
		if err != nil {
			resp := NewErrorResponse("", ErrCodeParse, err.Error())
			if err := s.writeResponse(conn, resp); err != nil {
				slog.Debug("failed to write parse error response", "error", err)

				return
			}

			continue
		}

		resp := s.dispatch(ctx, req, conn)
		if err := s.writeResponse(conn, resp); err != nil {
			slog.Debug("failed to write response", "method", req.Method, "id", req.ID, "error", err)

			return
		}
	}
}

func (s *Server) dispatch(ctx context.Context, req *Request, _ net.Conn) *Response {
	// The shutdown method is handled directly, outside the middleware chain,
	// because it triggers server teardown.
	if req.Method == "shutdown" {
		go s.initiateShutdown()
		resp, _ := NewResultResponse(req.ID, map[string]bool{"ok": true})

		return resp
	}

	// Look up the registered handler.
	s.mu.RLock()
	handler, hasHandler := s.handlers[req.Method]
	legacyHandler, hasLegacy := s.legacyHandlers[req.Method]
	middleware := make([]Middleware, len(s.middleware))
	copy(middleware, s.middleware)
	s.mu.RUnlock()

	if !hasHandler && !hasLegacy {
		slog.Warn("rpc method not found", "method", req.Method, "id", req.ID)

		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found: "+req.Method)
	}

	// Convert the handler to a HandlerFunc and wrap with the middleware chain.
	var hf HandlerFunc
	if hasHandler {
		hf = adaptHandler(handler)
	} else {
		hf = adaptLegacyHandler(legacyHandler)
	}

	wrapped := buildChain(hf, middleware)

	return wrapped(ctx, req)
}

func (s *Server) writeResponse(conn net.Conn, resp *Response) error {
	data, err := EncodeResponse(resp)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)

	return err
}

// WriteEvent writes a streaming event to a connection.
// Events are JSON objects without an id field (to distinguish from RPC responses).
func WriteEvent(conn net.Conn, event any) error {
	data, err := encodeEvent(event)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)

	return err
}

// Broadcast sends data to all connected clients.
// Errors on individual connections are silently ignored.
func (s *Server) Broadcast(data []byte) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for conn := range s.conns {
		if _, err := conn.Write(data); err != nil {
			slog.Debug("broadcast write failed", "error", err)
		}
	}
}

func (s *Server) Path() string {
	return s.path
}

// Stop gracefully stops the server.
func (s *Server) Stop() error {
	s.rateLimiter.Stop()
	s.initiateShutdown()
	s.waitForDrain()
	s.forceCloseAll()

	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Remove socket file
	_ = os.Remove(s.path)

	return nil
}

func CleanupStaleSocket(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if info.Mode()&os.ModeSocket == 0 {
		if err := os.Remove(path); err != nil {
			return false, err
		}

		return true, nil
	}

	conn, err := net.DialTimeout("unix", path, StaleCheckTimeout) //nolint:noctx // Timeout provides cancellation
	if err != nil {
		if err := os.Remove(path); err != nil {
			return false, err
		}

		return true, nil
	}
	_ = conn.Close()

	return false, nil
}
