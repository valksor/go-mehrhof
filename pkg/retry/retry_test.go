package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryableOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		maxRetries  int
		baseDelay   time.Duration
		fn          func(calls *int) func() error
		opts        []Option
		wantErr     bool
		wantErrMsg  string
		wantCalls   int
		maxDuration time.Duration // upper bound for the test to complete
	}{
		{
			name:       "succeeds on first try",
			maxRetries: 3,
			baseDelay:  time.Millisecond,
			fn: func(calls *int) func() error {
				return func() error {
					*calls++
					return nil
				}
			},
			wantCalls: 1,
		},
		{
			name:       "succeeds after 2 retries",
			maxRetries: 3,
			baseDelay:  time.Millisecond,
			fn: func(calls *int) func() error {
				return func() error {
					*calls++
					if *calls < 3 {
						return errors.New("Unable to create '/tmp/test.lock': File exists")
					}
					return nil
				}
			},
			wantCalls: 3,
		},
		{
			name:       "fails after max retries exhausted",
			maxRetries: 2,
			baseDelay:  time.Millisecond,
			fn: func(calls *int) func() error {
				return func() error {
					*calls++
					return errors.New("could not lock ref 'refs/heads/main'")
				}
			},
			wantErr:    true,
			wantErrMsg: "operation failed after 3 attempts",
			wantCalls:  3,
		},
		{
			name:       "non-retryable error stops immediately",
			maxRetries: 5,
			baseDelay:  time.Millisecond,
			fn: func(calls *int) func() error {
				return func() error {
					*calls++
					return errors.New("permission denied")
				}
			},
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:       "custom retry check function",
			maxRetries: 3,
			baseDelay:  time.Millisecond,
			fn: func(calls *int) func() error {
				return func() error {
					*calls++
					if *calls < 3 {
						return errors.New("custom-transient-error")
					}
					return nil
				}
			},
			opts: []Option{
				WithRetryCheck(func(err error) bool {
					return err.Error() == "custom-transient-error"
				}),
			},
			wantCalls: 3,
		},
		{
			name:       "zero max retries means single attempt",
			maxRetries: 0,
			baseDelay:  time.Millisecond,
			fn: func(calls *int) func() error {
				return func() error {
					*calls++
					return errors.New("Unable to create lock file")
				}
			},
			wantErr:    true,
			wantErrMsg: "operation failed after 1 attempts",
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int
			fn := tt.fn(&calls)

			err := RetryableOp(context.Background(), tt.maxRetries, tt.baseDelay, fn, tt.opts...)

			if (err != nil) != tt.wantErr {
				t.Fatalf("RetryableOp() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErrMsg != "" && err != nil {
				if got := err.Error(); !contains(got, tt.wantErrMsg) {
					t.Errorf("error message %q does not contain %q", got, tt.wantErrMsg)
				}
			}
			if calls != tt.wantCalls {
				t.Errorf("expected %d calls, got %d", tt.wantCalls, calls)
			}
		})
	}
}

func TestRetryableOp_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	var calls int
	fn := func() error {
		calls++
		if calls == 1 {
			cancel() // cancel after first attempt
		}
		return errors.New("Unable to create lock file")
	}

	err := RetryableOp(ctx, 5, 10*time.Millisecond, fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in error chain, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call before cancellation, got %d", calls)
	}
}

func TestBackoff_Increases(t *testing.T) {
	t.Parallel()

	base := 10 * time.Millisecond
	prev := time.Duration(0)

	for attempt := range 5 {
		d := backoff(base, attempt)
		// With jitter the delay might not be strictly increasing for adjacent
		// attempts, but the midpoint should grow. We check that the raw
		// exponential part (without jitter) grows.
		raw := base << uint(attempt)
		if attempt > 0 && raw <= prev {
			t.Errorf("attempt %d: raw delay %v should be > previous %v", attempt, raw, prev)
		}
		prev = raw

		// Verify jitter bounds: delay should be within [0.8*raw, 1.2*raw].
		lower := time.Duration(float64(raw) * 0.8)
		upper := time.Duration(float64(raw) * 1.2)
		if d < lower || d > upper {
			t.Errorf("attempt %d: delay %v outside jitter bounds [%v, %v]", attempt, d, lower, upper)
		}
	}
}

func TestBackoff_CappedAtMax(t *testing.T) {
	t.Parallel()

	d := backoff(time.Minute, 20) // 1min * 2^20 would be huge without cap
	// With jitter the max is maxBackoff * 1.2
	if d > time.Duration(float64(maxBackoff)*1.2)+time.Millisecond {
		t.Errorf("delay %v exceeds cap %v (with jitter)", d, maxBackoff)
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("Unable to create '/tmp/index.lock': File exists"), true},
		{errors.New("could not lock ref 'refs/heads/main'"), true},
		{errors.New("Cannot lock ref 'refs/heads/feature'"), true},
		{errors.New("resource temporarily unavailable"), true},
		{errors.New("permission denied"), false},
		{errors.New("not a git repository"), false},
	}

	for _, tt := range tests {
		name := "nil"
		if tt.err != nil {
			name = tt.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
