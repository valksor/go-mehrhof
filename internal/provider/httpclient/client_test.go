package httpclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name         string
		code         int
		message      string
		wantContains string
		wantCode     int
	}{
		{
			name:         "with message",
			code:         404,
			message:      "not found",
			wantContains: "HTTP 404: not found",
			wantCode:     404,
		},
		{
			name:         "without message",
			code:         500,
			message:      "",
			wantContains: "HTTP 500",
			wantCode:     500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewHTTPError(tt.code, tt.message)

			if got := err.Error(); got != tt.wantContains {
				t.Errorf("HTTPError.Error() = %q, want %q", got, tt.wantContains)
			}

			if got := err.HTTPStatusCode(); got != tt.wantCode {
				t.Errorf("HTTPError.HTTPStatusCode() = %d, want %d", got, tt.wantCode)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "rate limited error",
			err:  providererrors.ErrRateLimited,
			want: true,
		},
		{
			name: "network error",
			err:  providererrors.ErrNetworkError,
			want: true,
		},
		{
			name: "HTTP 429 Too Many Requests",
			err:  NewHTTPError(http.StatusTooManyRequests, "rate limit"),
			want: true,
		},
		{
			name: "HTTP 503 Service Unavailable",
			err:  NewHTTPError(http.StatusServiceUnavailable, "unavailable"),
			want: true,
		},
		{
			name: "HTTP 504 Gateway Timeout",
			err:  NewHTTPError(http.StatusGatewayTimeout, "timeout"),
			want: true,
		},
		{
			name: "HTTP 502 Bad Gateway",
			err:  NewHTTPError(http.StatusBadGateway, "bad gateway"),
			want: true,
		},
		{
			name: "HTTP 401 Unauthorized",
			err:  NewHTTPError(http.StatusUnauthorized, "unauthorized"),
			want: false,
		},
		{
			name: "HTTP 404 Not Found",
			err:  NewHTTPError(http.StatusNotFound, "not found"),
			want: false,
		},
		{
			name: "HTTP 500 Internal Server Error",
			err:  NewHTTPError(http.StatusInternalServerError, "server error"),
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("generic error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRetry(tt.err); got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithRetry(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		attempts := 0
		err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
			attempts++
			return nil
		})
		if err != nil {
			t.Errorf("WithRetry() error = %v, want nil", err)
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("retries on retryable error", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2,
		}

		err := WithRetry(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return NewHTTPError(http.StatusTooManyRequests, "rate limit")
			}
			return nil
		})
		if err != nil {
			t.Errorf("WithRetry() error = %v, want nil", err)
		}
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("fails after max retries", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			Multiplier:     2,
		}

		err := WithRetry(context.Background(), config, func() error {
			attempts++
			return NewHTTPError(http.StatusTooManyRequests, "rate limit")
		})

		if err == nil {
			t.Error("WithRetry() error = nil, want error")
		}
		if attempts != 3 { // Initial + 2 retries
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("does not retry non-retryable error", func(t *testing.T) {
		attempts := 0
		err := WithRetry(context.Background(), DefaultRetryConfig(), func() error {
			attempts++
			return NewHTTPError(http.StatusNotFound, "not found")
		})

		if err == nil {
			t.Error("WithRetry() error = nil, want error")
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		config := RetryConfig{
			MaxRetries:     10,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     1 * time.Second,
			Multiplier:     2,
		}

		// Cancel after short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := WithRetry(ctx, config, func() error {
			attempts++
			return NewHTTPError(http.StatusTooManyRequests, "rate limit")
		})

		if !errors.Is(err, context.Canceled) {
			t.Errorf("WithRetry() error = %v, want context.Canceled", err)
		}
		// Should have attempted at least once before cancellation
		if attempts < 1 {
			t.Errorf("attempts = %d, want >= 1", attempts)
		}
	})
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", config.MaxRetries, DefaultMaxRetries)
	}
	if config.InitialBackoff != DefaultBackoff {
		t.Errorf("InitialBackoff = %v, want %v", config.InitialBackoff, DefaultBackoff)
	}
	if config.MaxBackoff != MaxBackoff {
		t.Errorf("MaxBackoff = %v, want %v", config.MaxBackoff, MaxBackoff)
	}
	if config.Multiplier != BackoffMultiplier {
		t.Errorf("Multiplier = %v, want %v", config.Multiplier, BackoffMultiplier)
	}
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient()
	if client == nil {
		t.Fatal("NewHTTPClient() returned nil")
	}
	if client.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, DefaultTimeout)
	}
}

func TestNewHTTPClientWithTimeout(t *testing.T) {
	timeout := 60 * time.Second
	client := NewHTTPClientWithTimeout(timeout)
	if client == nil {
		t.Fatal("NewHTTPClientWithTimeout() returned nil")
	}
	if client.Timeout != timeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, timeout)
	}
}
