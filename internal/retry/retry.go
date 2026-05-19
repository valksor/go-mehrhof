package retry

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// maxBackoff is the upper bound for the computed delay between retries.
const maxBackoff = 5 * time.Minute

// Option configures retry behaviour.
type Option func(*config)

type config struct {
	retryCheck     func(error) bool
	circuitBreaker *CircuitBreaker
}

// WithRetryCheck supplies a custom function that decides whether a given error
// is eligible for retry. When set, this check is used instead of the default
// IsRetryable.
func WithRetryCheck(check func(error) bool) Option {
	return func(c *config) {
		c.retryCheck = check
	}
}

// RetryableOp executes fn, retrying up to maxRetries times on retryable errors.
// Between attempts it sleeps with exponential backoff and jitter. The operation
// respects context cancellation between retries.
//
// If fn returns nil the result is nil. If all attempts fail the last error is
// returned wrapped with attempt information.
func RetryableOp(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error, opts ...Option) error {
	cfg := config{
		retryCheck: IsRetryable,
	}
	for _, o := range opts {
		o(&cfg)
	}

	var lastErr error
	for attempt := range maxRetries + 1 {
		// Check circuit breaker before attempting.
		if cb := cfg.circuitBreaker; cb != nil {
			if !cb.Allow() {
				return circuitOpenError(cb.Name())
			}
		}

		lastErr = fn()
		if lastErr == nil {
			if cb := cfg.circuitBreaker; cb != nil {
				cb.RecordSuccess()
			}

			return nil
		}

		if cb := cfg.circuitBreaker; cb != nil {
			cb.RecordFailure()
		}

		// Don't retry if this is the last attempt.
		if attempt == maxRetries {
			break
		}

		// Don't retry non-retryable errors.
		if !cfg.retryCheck(lastErr) {
			return lastErr
		}

		delay := backoff(baseDelay, attempt)
		slog.Debug(
			"retry: retrying operation",
			"attempt", attempt+1,
			"max", maxRetries+1,
			"delay", delay,
			"error", lastErr,
		)

		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()

			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries+1, lastErr)
}

// backoff computes the delay for a given attempt using exponential backoff with
// jitter: min(baseDelay * 2^attempt, maxBackoff) * random(0.8, 1.2).
func backoff(base time.Duration, attempt int) time.Duration {
	// Cap the shift to prevent int64 overflow for large attempt values.
	shift := min(uint(attempt), 62)
	delay := base << shift // base * 2^attempt
	if delay <= 0 || delay > maxBackoff {
		delay = maxBackoff
	}

	// Apply jitter: multiply by a random factor in [0.8, 1.2).
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	randVal := float64(binary.LittleEndian.Uint64(buf[:])) / float64(^uint64(0)) //nolint:mnd // normalize to [0,1)
	jitter := 0.8 + randVal*0.4                                                  //nolint:mnd // jitter range [0.8, 1.2) is a well-known backoff constant

	return time.Duration(float64(delay) * jitter)
}

// retryableSubstrings are error message fragments that indicate transient,
// retryable failures.
var retryableSubstrings = []string{
	"Unable to create",
	".lock",
	"File exists",
	"lock file",
	"could not lock",
	"Cannot lock ref",
	"temporarily unavailable",
	"try again",
	"resource temporarily",
}

// IsRetryable returns true if the error message contains patterns known to
// represent transient failures (lock contention, temporary unavailability).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	for _, sub := range retryableSubstrings {
		if strings.Contains(msg, sub) {
			return true
		}
	}

	return false
}
