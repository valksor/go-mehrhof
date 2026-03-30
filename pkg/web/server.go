// Package web provides the HTTP server for the kvelmo web UI.
package web

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/valksor/kvelmo/pkg/socket"
	"github.com/valksor/kvelmo/pkg/web/static"
)

// WorktreeCreator creates worktree sockets on-demand.
// This allows the web server to ensure sockets exist before proxying.
type WorktreeCreator interface {
	// GetOrCreateWorktreeSocket returns an existing or new worktree socket.
	// The socket is started automatically if created.
	GetOrCreateWorktreeSocket(projectPath string) (any, error)
}

// Server serves the web UI and proxies WebSocket connections to Unix sockets.
type Server struct {
	httpServer       *http.Server
	listener         net.Listener
	staticDir        string
	embeddedFS       fs.FS // Fallback when staticDir is empty
	port             int
	allowedOrigins   []string // If empty, only localhost is allowed
	worktreeCreator  WorktreeCreator
	globalSocketPath string
	tlsCertFile      string
	tlsKeyFile       string
	apiOnly          bool
}

// ServerOption configures the web server.
type ServerOption func(*Server)

// WithAllowedOrigins sets allowed CORS origins.
// If origins is empty or nil, only localhost is allowed (secure default).
// Use []string{"*"} to allow all origins (development only).
func WithAllowedOrigins(origins []string) ServerOption {
	return func(s *Server) {
		s.allowedOrigins = origins
	}
}

// WithWorktreeCreator sets the worktree creator for on-demand socket creation.
// When set, the server will ensure worktree sockets exist before proxying.
func WithWorktreeCreator(creator WorktreeCreator) ServerOption {
	return func(s *Server) {
		s.worktreeCreator = creator
	}
}

// WithTLS configures TLS for the web server.
func WithTLS(certFile, keyFile string) ServerOption {
	return func(s *Server) {
		s.tlsCertFile = certFile
		s.tlsKeyFile = keyFile
	}
}

// WithAPIOnly enables API-only mode, disabling web UI static file serving.
func WithAPIOnly() ServerOption {
	return func(s *Server) {
		s.apiOnly = true
	}
}

// WithGlobalSocketPath sets the global socket path for readiness checks.
func WithGlobalSocketPath(path string) ServerOption {
	return func(s *Server) {
		s.globalSocketPath = path
	}
}

// NewServer creates a new web server with the given port.
// Use port 0 for a random available port.
func NewServer(staticDir string, port int, opts ...ServerOption) (*Server, error) {
	addr := fmt.Sprintf("localhost:%d", port)
	ln, err := net.Listen("tcp", addr) //nolint:noctx // Context cancellation handled via server shutdown
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	// Get actual port (important when port=0)
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()

		return nil, errors.New("unexpected listener address type")
	}
	actualPort := tcpAddr.Port

	s := &Server{
		staticDir:  staticDir,
		embeddedFS: static.Dist(),
		listener:   ln,
		port:       actualPort,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()

	// WebSocket proxy endpoints
	mux.HandleFunc("/ws/global", s.handleGlobalWS)
	mux.HandleFunc("/ws/worktree/", s.handleWorktreeWS)

	// Health and metrics endpoints
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/metrics", s.handleMetrics)

	// JSON API endpoints for external aggregation (G1: reporting)
	mux.HandleFunc("/api/state", s.handleAPIState)
	mux.HandleFunc("/api/tasks", s.handleAPITasks)

	// Static file serving (SPA)
	// Serve from disk (staticDir) or embedded assets (fallback for production)
	if !s.apiOnly && (staticDir != "" || s.embeddedFS != nil) {
		mux.HandleFunc("/", s.handleStatic)
	}

	s.httpServer = &http.Server{
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second, // generous for WebSocket upgrades
		IdleTimeout:       120 * time.Second,
	}

	return s, nil
}

// acceptOptions returns WebSocket accept options with origin validation.
// By default, only localhost is allowed (secure default).
func (s *Server) acceptOptions() *websocket.AcceptOptions {
	patterns := s.originPatterns()

	return &websocket.AcceptOptions{
		OriginPatterns: patterns,
	}
}

// originPatterns converts allowedOrigins to coder/websocket OriginPatterns.
func (s *Server) originPatterns() []string {
	// Check for wildcard
	if slices.Contains(s.allowedOrigins, "*") {
		return []string{"*"}
	}

	// Start with localhost defaults
	patterns := []string{"localhost:*", "127.0.0.1:*", "[::1]:*"}

	// Add configured origins as patterns (preserve port if present)
	for _, allowed := range s.allowedOrigins {
		u, err := url.Parse(allowed)
		if err != nil {
			slog.Warn("skipping unparseable allowed origin", "origin", allowed, "error", err)

			continue
		}
		host := u.Host // Includes port if present (e.g. "app.example.com:8443")
		if host == "" {
			host = u.Hostname()
		}
		if host == "" {
			slog.Warn("skipping allowed origin with empty host", "origin", allowed)

			continue
		}
		patterns = append(patterns, host)
	}

	return patterns
}

// securityHeaders wraps a handler with security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server using the pre-bound listener.
func (s *Server) Start() error {
	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		return s.httpServer.ServeTLS(s.listener, s.tlsCertFile, s.tlsKeyFile)
	}

	return s.httpServer.Serve(s.listener)
}

// Port returns the actual port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// URL returns the full URL to access the server.
func (s *Server) URL() string {
	scheme := "http"
	if s.tlsCertFile != "" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://localhost:%d", scheme, s.port)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Try disk first (development mode)
	if s.staticDir != "" {
		fullPath := filepath.Join(s.staticDir, path)
		if _, err := os.Stat(fullPath); err == nil {
			http.ServeFile(w, r, fullPath)

			return
		}
		// SPA fallback: serve index.html for routes that don't match files
		indexPath := filepath.Join(s.staticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)

			return
		}
	}

	// Fall back to embedded assets (production mode)
	if s.embeddedFS != nil {
		// Try to open the requested file
		f, err := s.embeddedFS.Open(path)
		if err == nil {
			defer func() { _ = f.Close() }()
			stat, _ := f.Stat()
			if !stat.IsDir() {
				http.ServeFileFS(w, r, s.embeddedFS, path)

				return
			}
		}
		// SPA fallback: serve index.html for routes that don't match files
		http.ServeFileFS(w, r, s.embeddedFS, "index.html")

		return
	}

	http.NotFound(w, r)
}

// handleGlobalWS proxies WebSocket connections to the global Unix socket.
func (s *Server) handleGlobalWS(w http.ResponseWriter, r *http.Request) {
	wsConn, err := websocket.Accept(w, r, s.acceptOptions())
	if err != nil {
		return
	}
	defer func() { _ = wsConn.CloseNow() }()

	// Connect to global Unix socket
	sockPath := socket.GlobalSocketPath()
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	unixConn, err := dialer.DialContext(r.Context(), "unix", sockPath)
	if err != nil {
		_ = wsConn.Write(r.Context(), websocket.MessageText, fmt.Appendf(nil, `{"error":"failed to connect to global socket: %v"}`, err))

		return
	}
	defer func() { _ = unixConn.Close() }()

	// Proxy bidirectionally
	s.proxyConnections(r.Context(), wsConn, unixConn)
}

// handleWorktreeWS proxies WebSocket connections to a worktree Unix socket.
func (s *Server) handleWorktreeWS(w http.ResponseWriter, r *http.Request) {
	// Extract worktree ID from path: /ws/worktree/{id}
	// The ID is a URL-encoded filesystem path (e.g., %2Fprivate%2Fvar%2F...).
	// We use RawPath to get the encoded version, then decode it ourselves,
	// because URL.Path auto-decodes %2F to / which breaks our parsing.
	const prefix = "/ws/worktree/"
	rawPath := r.URL.RawPath
	if rawPath == "" {
		rawPath = r.URL.Path // Fallback if RawPath not set
	}
	if !strings.HasPrefix(rawPath, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)

		return
	}
	encodedID := strings.TrimPrefix(rawPath, prefix)
	if encodedID == "" {
		http.Error(w, "missing worktree id", http.StatusBadRequest)

		return
	}
	worktreeID, err := url.PathUnescape(encodedID)
	if err != nil {
		http.Error(w, "invalid worktree id encoding", http.StatusBadRequest)

		return
	}

	// Ensure worktree socket exists (create on-demand if creator is configured)
	sockPath := socket.WorktreeSocketPath(worktreeID)
	if s.worktreeCreator != nil {
		if _, err := s.worktreeCreator.GetOrCreateWorktreeSocket(worktreeID); err != nil {
			slog.Error("failed to create worktree socket", "worktree_id", worktreeID, "error", err)
			http.Error(w, "failed to create worktree socket", http.StatusInternalServerError)

			return
		}
	}

	wsConn, err := websocket.Accept(w, r, s.acceptOptions())
	if err != nil {
		return
	}
	defer func() { _ = wsConn.CloseNow() }()

	// Connect to worktree Unix socket with retry (socket may still be starting)
	var unixConn net.Conn
	dialer := &net.Dialer{Timeout: 2 * time.Second}
connectLoop:
	for attempt := range 5 {
		// Check context before attempting
		if r.Context().Err() != nil {
			err = r.Context().Err()

			break
		}
		unixConn, err = dialer.DialContext(r.Context(), "unix", sockPath)
		if err == nil {
			break
		}
		// Context-aware backoff (50ms, 100ms, 200ms, 400ms)
		backoff := time.Duration(50<<attempt) * time.Millisecond
		select {
		case <-r.Context().Done():
			err = r.Context().Err()

			break connectLoop
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}
	if err != nil {
		_ = wsConn.Write(r.Context(), websocket.MessageText, fmt.Appendf(nil, `{"error":"failed to connect to worktree socket: %v"}`, err))

		return
	}
	defer func() { _ = unixConn.Close() }()

	// Proxy bidirectionally
	s.proxyConnections(r.Context(), wsConn, unixConn)
}

// proxyConnections handles bidirectional proxying between WebSocket and Unix socket.
// When either connection fails, both are closed to prevent goroutine leaks.
// coder/websocket handles ping/pong automatically and is concurrent-write safe.
func (s *Server) proxyConnections(ctx context.Context, wsConn *websocket.Conn, unixConn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Close unixConn when context is cancelled to unblock scanner.Scan().
	// Without this, the scanner goroutine would deadlock on Read() when
	// the WebSocket side disconnects (browser tab close, network drop).
	go func() {
		<-ctx.Done()
		_ = unixConn.Close()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	// WebSocket -> Unix socket
	go func() {
		defer func() { cancel(); wg.Done() }()
		for {
			_, msg, err := wsConn.Read(ctx)
			if err != nil {
				return
			}
			_, err = unixConn.Write(msg)
			if err != nil {
				return
			}
		}
	}()

	// Unix socket -> WebSocket
	// Use a scanner to read complete NDJSON lines from the Unix socket,
	// ensuring each WebSocket message contains a complete JSON-RPC frame.
	// Append \n to preserve NDJSON framing — clients split on newlines and
	// buffer the last fragment, so without \n the message never gets processed.
	go func() {
		defer func() { cancel(); wg.Done() }()
		scanner := bufio.NewScanner(unixConn)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 64KB initial, 1MB max
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			msg := append(line, '\n')
			err := wsConn.Write(ctx, websocket.MessageText, msg)
			if err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			slog.Debug("unix->ws scanner error", "error", err)
		}
	}()

	wg.Wait()
}
