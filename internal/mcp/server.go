package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/time/rate"
)

const (
	maxRequestSize = 10 * 1024 * 1024
	// maxConcurrentReqs caps in-flight request handling. The current
	// ServeReadWriter loop reads and dispatches synchronously in a single
	// goroutine, so the cap is effectively 1 in practice and the semaphore
	// is a forward-looking guard for a future concurrent dispatch design.
	// Keep at 10 to match the prototype's setting.
	maxConcurrentReqs = 10
	defaultRateLimit  = 10
	defaultRateBurst  = 20
	ioBufferSize      = 32 * 1024
)

// ServerOption configures the MCP server.
type ServerOption func(*Server)

// WithRateLimit sets a custom rate limit (requests per second) and burst size.
func WithRateLimit(ratePerSec float64, burst int) ServerOption {
	return func(s *Server) {
		if ratePerSec > 0 && burst > 0 {
			s.rateLimiter = rate.NewLimiter(rate.Limit(ratePerSec), burst)
		}
	}
}

// WithServerInfo overrides the advertised server name/version.
func WithServerInfo(info ServerInfo) ServerOption {
	return func(s *Server) {
		s.serverInfo = info
	}
}

// Server implements an MCP server over stdio.
type Server struct {
	toolRegistry *ToolRegistry
	initialized  atomic.Bool
	serverInfo   ServerInfo
	semaphore    chan struct{}
	shutdownChan chan struct{}
	shutdownOnce sync.Once
	rateLimiter  *rate.Limiter
}

// NewServer creates a new MCP server.
func NewServer(toolRegistry *ToolRegistry, opts ...ServerOption) *Server {
	s := &Server{
		toolRegistry: toolRegistry,
		serverInfo: ServerInfo{
			Name:    "kvelmo-mcp",
			Version: "1.0.0",
		},
		semaphore:    make(chan struct{}, maxConcurrentReqs),
		shutdownChan: make(chan struct{}),
		rateLimiter:  rate.NewLimiter(rate.Limit(defaultRateLimit), defaultRateBurst),
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Serve starts the MCP server, reading from stdin and writing to stdout.
func (s *Server) Serve(ctx context.Context) error {
	return s.ServeReadWriter(ctx, os.Stdin, os.Stdout)
}

// ServeReadWriter starts the MCP server using the provided reader/writer
// (useful for tests with io.Pipe).
func (s *Server) ServeReadWriter(ctx context.Context, r io.Reader, w io.Writer) error {
	reader := bufio.NewReaderSize(r, ioBufferSize)
	writer := bufio.NewWriter(w)
	defer func() { _ = writer.Flush() }()

	slog.Info("mcp server started", "max_request_size", maxRequestSize, "max_concurrent", maxConcurrentReqs)

	for {
		select {
		case <-s.shutdownChan:
			slog.Info("mcp server: shutdown signalled")

			return nil
		case <-ctx.Done():
			slog.Info("mcp server: context cancelled")

			return ctx.Err()
		default:
		}

		select {
		case s.semaphore <- struct{}{}:
		case <-s.shutdownChan:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}

		select {
		case <-s.shutdownChan:
			<-s.semaphore

			return nil
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			<-s.semaphore
			if errors.Is(err, io.EOF) {
				slog.Info("mcp server: stdin closed")

				return nil
			}

			return fmt.Errorf("read stdin: %w", err)
		}

		if len(line) == 0 || line == "\n" {
			<-s.semaphore

			continue
		}

		if len(line) > maxRequestSize {
			<-s.semaphore
			slog.Warn("mcp request too large", "size", len(line), "max", maxRequestSize)
			_ = s.writeResponse(writer, &Response{
				JSONRPC: "2.0",
				Error: &Error{
					Code:    InvalidRequest,
					Message: fmt.Sprintf("request too large: %d bytes (max %d)", len(line), maxRequestSize),
				},
			})

			continue
		}

		response := s.handleRequest(ctx, line)
		if response != nil {
			if err := s.writeResponse(writer, response); err != nil {
				<-s.semaphore

				return err
			}
		}
		<-s.semaphore
	}
}

// handleRequest processes a single JSON-RPC request.
func (s *Server) handleRequest(ctx context.Context, line string) *Response {
	var req Request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return errorResponse(nil, ParseError, fmt.Sprintf("parse error: %v", err))
	}
	if req.JSONRPC != "2.0" {
		return errorResponse(req.ID, InvalidRequest, "invalid jsonrpc version (expected 2.0)")
	}
	switch req.Method {
	case MethodInitialize:
		return s.handleInitialize(&req)
	case MethodToolsList:
		return s.handleToolsList(&req)
	case MethodToolsCall:
		return s.handleToolsCall(ctx, &req)
	case MethodShutdown:
		return s.handleShutdown(&req)
	}
	if strings.HasPrefix(req.Method, MethodNotifications) {
		// Notifications have no response.
		return nil
	}

	return errorResponse(req.ID, MethodNotFound, "method not found: "+req.Method)
}

func (s *Server) handleInitialize(req *Request) *Response {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, InvalidParams, fmt.Sprintf("invalid params: %v", err))
	}
	if params.ProtocolVersion != ProtocolVersion {
		// Don't reject — Anthropic ships new MCP versions regularly, and a
		// hard reject here would brick the adapter on every claude upgrade.
		// Log and proceed; tools/list and tools/call use the same wire format
		// across recent versions.
		slog.Warn("mcp protocol version mismatch (continuing anyway)",
			"client", params.ProtocolVersion, "server", ProtocolVersion)
	}
	if params.ClientInfo.Name == "" {
		return errorResponse(req.ID, InvalidParams, "missing required field: clientInfo.name")
	}
	s.initialized.Store(true)
	slog.Info("mcp client initialized", "client", params.ClientInfo.Name, "version", params.ClientInfo.Version)

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapabilities{ListChanged: false},
		},
		ServerInfo: s.serverInfo,
	}

	return marshalResponse(req.ID, result)
}

func (s *Server) handleToolsList(req *Request) *Response {
	if !s.initialized.Load() {
		return errorResponse(req.ID, InvalidRequest, "server not initialized")
	}

	return marshalResponse(req.ID, ToolsListResult{Tools: s.toolRegistry.ListTools()})
}

func (s *Server) handleToolsCall(ctx context.Context, req *Request) *Response {
	if !s.initialized.Load() {
		return errorResponse(req.ID, InvalidRequest, "server not initialized")
	}
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return errorResponse(req.ID, InternalError, "rate limit exceeded")
	}
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, InvalidParams, fmt.Sprintf("invalid params: %v", err))
	}
	result, err := s.toolRegistry.CallTool(ctx, params.Name, params.Arguments)
	if err != nil {
		slog.Error("mcp tool execution failed", "tool", params.Name, "error", err)

		return errorResponse(req.ID, InternalError, fmt.Sprintf("tool execution failed: %v", err))
	}

	return marshalResponse(req.ID, result)
}

func (s *Server) handleShutdown(req *Request) *Response {
	s.initialized.Store(false)
	s.shutdownOnce.Do(func() { close(s.shutdownChan) })
	slog.Info("mcp server: shutdown request received")

	return marshalResponse(req.ID, EmptyResult{})
}

func (s *Server) writeResponse(writer *bufio.Writer, response *Response) error {
	data, err := json.Marshal(response)
	if err != nil {
		slog.Error("mcp: marshal response failed", "error", err)

		return err
	}
	if _, err := writer.Write(data); err != nil {
		return checkPipe(err)
	}
	if _, err := writer.WriteString("\n"); err != nil {
		return checkPipe(err)
	}
	if err := writer.Flush(); err != nil {
		return checkPipe(err)
	}

	return nil
}

func errorResponse(id json.RawMessage, code int, msg string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: msg}}
}

func marshalResponse(id json.RawMessage, result any) *Response {
	data, err := json.Marshal(result)
	if err != nil {
		slog.Error("mcp: marshal result failed", "error", err)

		return errorResponse(id, InternalError, fmt.Sprintf("marshal result: %v", err))
	}

	return &Response{JSONRPC: "2.0", ID: id, Result: data}
}

func checkPipe(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, io.ErrClosedPipe) ||
		strings.Contains(strings.ToLower(err.Error()), "broken pipe") {
		slog.Info("mcp client disconnected (broken pipe)")

		return fmt.Errorf("client disconnected: %w", err)
	}

	return err
}
