// Package httpclient provides shared HTTP utilities for provider clients.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// Default configuration values used across providers.
const (
	DefaultTimeout    = 30 * time.Second
	DefaultMaxRetries = 3
	DefaultBackoff    = 1 * time.Second
	MaxBackoff        = 30 * time.Second
	BackoffMultiplier = 2
)

// HTTPError represents an HTTP error with status code.
// This type implements the HTTPStatusCode() interface expected by providererrors.
type HTTPError struct {
	Message string
	Code    int
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("HTTP %d", e.Code)
}

// HTTPStatusCode returns the HTTP status code.
func (e *HTTPError) HTTPStatusCode() int {
	return e.Code
}

// NewHTTPError creates a new HTTPError with the given code and message.
func NewHTTPError(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}

// RetryConfig controls retry behavior.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     DefaultMaxRetries,
		InitialBackoff: DefaultBackoff,
		MaxBackoff:     MaxBackoff,
		Multiplier:     BackoffMultiplier,
	}
}

// ShouldRetry determines if an error is retryable.
// Returns true for rate limiting (429), service unavailable (503), and network errors.
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check for wrapped provider errors
	if errors.Is(err, providererrors.ErrRateLimited) {
		return true
	}
	if errors.Is(err, providererrors.ErrNetworkError) {
		return true
	}

	// Check for HTTP status codes directly
	var httpErr interface{ HTTPStatusCode() int }
	if errors.As(err, &httpErr) {
		code := httpErr.HTTPStatusCode()
		return code == http.StatusTooManyRequests ||
			code == http.StatusServiceUnavailable ||
			code == http.StatusGatewayTimeout ||
			code == http.StatusBadGateway
	}

	return false
}

// RetryFunc is a function that performs an operation that may need retrying.
type RetryFunc func() error

// WithRetry executes the given function with exponential backoff retry.
// It respects context cancellation and stops when the context is done.
func WithRetry(ctx context.Context, config RetryConfig, fn RetryFunc) error {
	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !ShouldRetry(err) {
			return err
		}

		// Don't wait after last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Wait with exponential backoff
		select {
		case <-time.After(backoff):
			backoff = time.Duration(float64(backoff) * config.Multiplier)
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// NewHTTPClient creates a new http.Client with the default timeout.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

// NewHTTPClientWithTimeout creates a new http.Client with a custom timeout.
func NewHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
