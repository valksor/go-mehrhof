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
	"github.com/valksor/kvelmo/pkg/trace"
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

// RateLimiter tracks request counts per connection.
type RateLimiter struct {
	mu        sync.Mutex
	counts    map[net.Conn]int
	limit     int           // Max requests per window
	window    time.Duration // Time window
	lastReset time.Time     // Last reset time
}

func newRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		counts:    make(map[net.Conn]int),
		limit:     limit,
		window:    window,
		lastReset: time.Now(),
	}
}

func (r *RateLimiter) allow(conn net.Conn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reset counts if window has passed
	if time.Since(r.lastReset) > r.window {
		r.counts = make(map[net.Conn]int)
		r.lastReset = time.Now()
	}

	if r.counts[conn] >= r.limit {
		return false
	}
	r.counts[conn]++

	return true
}

func (r *RateLimiter) remove(conn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.counts, conn)
}

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
	rateLimiter    *RateLimiter
	activityLogger ActivityLogger
	username       string // Cached OS username for audit logging
}

func NewServer(path string) *Server {
	s := &Server{
		path:           path,
		handlers:       make(map[string]Handler),
		legacyHandlers: make(map[string]LegacyHandler),
		conns:          make(map[net.Conn]struct{}),
		shutdownCh:     make(chan struct{}),
		rateLimiter:    newRateLimiter(1000, time.Minute), // 1000 requests/min per connection
	}
	if u, err := user.Current(); err == nil {
		s.username = u.Username
	}

	return s
}

// SetActivityLogger sets the activity logger for RPC call recording.
func (s *Server) SetActivityLogger(l ActivityLogger) {
	s.activityLogger = l
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
	deadline := time.After(ShutdownTimeout)
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

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("socket handler panic recovered", "panic", r, "stack", string(debug.Stack()))
		}
		_ = conn.Close()
		s.trackConn(conn, false)
		s.rateLimiter.remove(conn)
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

		// Check rate limit
		if !s.rateLimiter.allow(conn) {
			resp := NewErrorResponse("", ErrCodeRateLimited, "rate limit exceeded")
			if err := s.writeResponse(conn, resp); err != nil {
				slog.Debug("failed to write rate limit response", "error", err)

				return
			}

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

func (s *Server) dispatch(ctx context.Context, req *Request, conn net.Conn) *Response {
	start := time.Now()
	corrID := trace.NewID()
	ctx = trace.WithID(ctx, corrID)

	if req.Method == "shutdown" {
		go s.initiateShutdown()
		resp, _ := NewResultResponse(req.ID, map[string]bool{"ok": true})
		slog.Debug("rpc request", "method", req.Method, "id", req.ID, "correlation_id", corrID, "duration_ms", time.Since(start).Milliseconds())

		return resp
	}

	s.mu.RLock()
	handler, hasHandler := s.handlers[req.Method]
	legacyHandler, hasLegacy := s.legacyHandlers[req.Method]
	s.mu.RUnlock()

	if !hasHandler && !hasLegacy {
		slog.Warn("rpc method not found", "method", req.Method, "id", req.ID, "correlation_id", corrID)
		// Record as failed RPC request for observability
		metrics.Global().RecordRPCRequest(req.Method, 0, fmt.Errorf("method not found: %s", req.Method))

		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, "method not found: "+req.Method)
	}

	var resp *Response
	var err error

	if hasHandler {
		resp, err = handler(ctx, req, conn)
	} else {
		resp, err = legacyHandler(ctx, req)
	}

	duration := time.Since(start)
	if err != nil {
		slog.Error("rpc request failed", "method", req.Method, "id", req.ID, "correlation_id", corrID, "error", err, "duration_ms", duration.Milliseconds())
		metrics.Global().RecordRPCRequest(req.Method, duration, err)
		s.recordActivity(req, corrID, duration, err.Error())

		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	slog.Debug("rpc request", "method", req.Method, "id", req.ID, "correlation_id", corrID, "duration_ms", duration.Milliseconds())
	metrics.Global().RecordRPCRequest(req.Method, duration, nil)
	s.recordActivity(req, corrID, duration, "")

	return resp
}

func (s *Server) recordActivity(req *Request, corrID string, duration time.Duration, errMsg string) {
	if s.activityLogger == nil {
		return
	}
	paramsSize := len(req.Params)
	s.activityLogger.Record(ActivityEntry{
		Method:        req.Method,
		CorrelationID: corrID,
		DurationMs:    duration.Milliseconds(),
		Error:         errMsg,
		ParamsSize:    paramsSize,
		UserID:        s.username,
	})
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
