package provider

import (
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

// RetryConfig controls HTTP retry behavior.
type RetryConfig struct {
	MaxRetries     int             // Maximum retry attempts (0 = no retries)
	BaseDelay      time.Duration   // Initial delay between retries (doubled each attempt)
	MaxDelay       time.Duration   // Maximum delay cap
	CircuitBreaker *CircuitBreaker // Optional circuit breaker
}

// DefaultRetryConfig provides sensible defaults for API calls.
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	BaseDelay:  100 * time.Millisecond,
	MaxDelay:   5 * time.Second,
}

// NoRetryConfig disables retries for non-idempotent operations (POST, etc.).
var NoRetryConfig = RetryConfig{
	MaxRetries: 0,
}

// DoWithRetry executes an HTTP request with exponential backoff retry.
// Retries on 5xx errors, 429 (rate limit), and network errors.
// Does not retry on 4xx client errors (except 429).
func DoWithRetry(client *http.Client, req *http.Request, cfg RetryConfig) (*http.Response, error) {
	// Check circuit breaker before attempting
	if cfg.CircuitBreaker != nil {
		if err := cfg.CircuitBreaker.Allow(); err != nil {
			return nil, err
		}
	}

	if cfg.MaxRetries == 0 {
		resp, err := client.Do(req)
		if cfg.CircuitBreaker != nil {
			if err != nil || resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
				cfg.CircuitBreaker.RecordFailure()
			} else {
				cfg.CircuitBreaker.RecordSuccess()
			}
		}

		return resp, err
	}

	delay := cfg.BaseDelay
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Clone request for retry (body may have been consumed)
		reqClone := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				// Close any previous response body before returning error
				if lastResp != nil && lastResp.Body != nil {
					_ = lastResp.Body.Close()
				}

				return nil, err
			}
			reqClone.Body = body
		}

		resp, err := client.Do(reqClone)

		// Success - return immediately
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			// Close any previous response body before returning success
			if lastResp != nil && lastResp.Body != nil {
				_ = lastResp.Body.Close()
			}

			if cfg.CircuitBreaker != nil {
				cfg.CircuitBreaker.RecordSuccess()
			}

			return resp, nil
		}

		// Close previous response body if we're retrying
		if lastResp != nil && lastResp.Body != nil {
			_ = lastResp.Body.Close()
		}

		lastResp = resp
		lastErr = err

		// Determine if we should retry
		shouldRetry := false
		if err != nil {
			// Network error - retry
			shouldRetry = true
			slog.Debug("http retry: network error", "attempt", attempt+1, "error", err)
		} else if resp.StatusCode >= 500 {
			// Server error - retry
			shouldRetry = true
			slog.Debug("http retry: server error", "attempt", attempt+1, "status", resp.StatusCode)
		} else if resp.StatusCode == http.StatusTooManyRequests {
			// Rate limited - retry
			shouldRetry = true
			slog.Debug("http retry: rate limited", "attempt", attempt+1)
		}

		if !shouldRetry {
			return resp, err
		}

		// Don't sleep after last attempt
		if attempt < cfg.MaxRetries {
			// Add jitter (±25%)
			jitter := time.Duration(float64(delay) * (0.75 + rand.Float64()*0.5)) //nolint:gosec // Non-security timing jitter

			// Respect context cancellation during sleep
			select {
			case <-req.Context().Done():
				if lastResp != nil && lastResp.Body != nil {
					_ = lastResp.Body.Close()
				}

				return nil, req.Context().Err()
			case <-time.After(jitter):
			}

			// Exponential backoff
			delay *= 2
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	// All retries exhausted - record failure
	if cfg.CircuitBreaker != nil {
		cfg.CircuitBreaker.RecordFailure()
	}

	// Return last response/error after exhausting retries
	if lastErr != nil {
		return nil, lastErr
	}

	return lastResp, nil
}
