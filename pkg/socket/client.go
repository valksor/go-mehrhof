package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/valksor/kvelmo/pkg/cli"
)

const DefaultTimeout = 5 * time.Second

type ClientOption func(*clientConfig)

// clientConfig holds configuration applied before connection.
type clientConfig struct {
	timeout    time.Duration
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

func WithTimeout(d time.Duration) ClientOption {
	return func(cfg *clientConfig) {
		cfg.timeout = d
	}
}

// WithRetry enables connection retry with exponential backoff.
// maxRetries: maximum number of retry attempts (0 = no retries, default)
// baseDelay: initial delay between retries (doubled each attempt)
// maxDelay: maximum delay between retries.
func WithRetry(maxRetries int, baseDelay, maxDelay time.Duration) ClientOption {
	return func(cfg *clientConfig) {
		cfg.maxRetries = maxRetries
		cfg.baseDelay = baseDelay
		cfg.maxDelay = maxDelay
	}
}

type Client struct {
	conn    net.Conn
	scanner *bufio.Scanner
	mu      sync.Mutex
	nextID  atomic.Uint64
	timeout time.Duration
}

func NewClient(path string, opts ...ClientOption) (*Client, error) {
	cfg := &clientConfig{
		timeout:    DefaultTimeout,
		maxRetries: 0,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   5 * time.Second,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	var conn net.Conn
	var err error
	delay := cfg.baseDelay

	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		conn, err = net.DialTimeout("unix", path, cfg.timeout) //nolint:noctx // Timeout provides cancellation
		if err == nil {
			break
		}

		// Don't sleep after last attempt
		if attempt < cfg.maxRetries {
			// Add jitter (±25%) - math/rand is fine for timing jitter
			jitter := time.Duration(float64(delay) * (0.75 + rand.Float64()*0.5)) //nolint:gosec // Non-security use case
			time.Sleep(jitter)

			// Exponential backoff
			delay *= 2
			if delay > cfg.maxDelay {
				delay = cfg.maxDelay
			}
		}
	}

	if err != nil {
		return nil, formatConnectionError(path, err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // allow messages up to 4MB

	c := &Client{
		conn:    conn,
		scanner: scanner,
		timeout: cfg.timeout,
	}

	return c, nil
}

func (c *Client) Call(ctx context.Context, method string, params any) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.timeout > 0 {
		_ = c.conn.SetDeadline(time.Now().Add(c.timeout))
		defer func() { _ = c.conn.SetDeadline(time.Time{}) }()
	}

	id := strconv.FormatUint(c.nextID.Add(1), 10)

	var paramsJSON []byte
	if params != nil {
		var err error
		paramsJSON, err = encodeParams(params)
		if err != nil {
			return nil, fmt.Errorf("encode params: %w", err)
		}
	}

	req := &Request{
		ID:     id,
		Method: method,
		Params: paramsJSON,
	}

	data, err := EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				return nil, &cli.TimeoutError{Err: errors.New("timeout waiting for response")}
			}

			return nil, fmt.Errorf("read response: %w", err)
		}

		return nil, errors.New("connection closed")
	}

	resp, err := DecodeResponse(c.scanner.Bytes())
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) SetTimeout(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeout = d
}

func encodeParams(v any) ([]byte, error) {
	switch p := v.(type) {
	case []byte:
		return p, nil
	case json.RawMessage:
		return p, nil
	default:
		return json.Marshal(v)
	}
}

// formatConnectionError converts socket connection errors to user-friendly messages.
func formatConnectionError(path string, err error) error {
	// Check for specific error types
	if errors.Is(err, syscall.ECONNREFUSED) {
		return &cli.ConnectionError{
			Err: fmt.Errorf("socket not responding at %s\nThe server may have crashed. Try restarting with 'kvelmo serve'", path),
		}
	}

	if errors.Is(err, syscall.ENOENT) || os.IsNotExist(err) {
		return &cli.ConnectionError{
			Err: fmt.Errorf("socket not found at %s\nRun 'kvelmo serve' to start the server", path),
		}
	}

	if errors.Is(err, syscall.EACCES) || os.IsPermission(err) {
		return &cli.ConnectionError{
			Err: fmt.Errorf("permission denied for socket at %s\nCheck file permissions or run with appropriate privileges", path),
		}
	}

	// Check for timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &cli.TimeoutError{
			Err: fmt.Errorf("connection timed out for socket at %s\nThe server may be overloaded or not responding", path),
		}
	}

	// Default: wrap with path context
	return &cli.ConnectionError{
		Err: fmt.Errorf("connect to %s: %w", path, err),
	}
}
