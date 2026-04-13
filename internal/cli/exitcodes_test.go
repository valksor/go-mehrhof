package cli

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			expected: ExitSuccess,
		},
		{
			name:     "ConnectionError returns ExitConnection",
			err:      &ConnectionError{Err: errors.New("refused")},
			expected: ExitConnection,
		},
		{
			name:     "TimeoutError returns ExitTimeout",
			err:      &TimeoutError{Err: errors.New("timed out")},
			expected: ExitTimeout,
		},
		{
			name:     "StateError returns ExitState",
			err:      &StateError{Err: errors.New("invalid transition")},
			expected: ExitState,
		},
		{
			name:     "NotFoundError returns ExitNotFound",
			err:      &NotFoundError{Err: errors.New("task missing")},
			expected: ExitNotFound,
		},
		{
			name:     "UsageError returns ExitUsage",
			err:      &UsageError{Err: errors.New("bad args")},
			expected: ExitUsage,
		},
		{
			name:     "plain error returns ExitGeneral",
			err:      errors.New("something failed"),
			expected: ExitGeneral,
		},
		{
			name:     "wrapped ConnectionError returns ExitConnection",
			err:      fmt.Errorf("dial failed: %w", &ConnectionError{Err: errors.New("refused")}),
			expected: ExitConnection,
		},
		{
			name:     "wrapped TimeoutError returns ExitTimeout",
			err:      fmt.Errorf("call failed: %w", &TimeoutError{Err: errors.New("deadline exceeded")}),
			expected: ExitTimeout,
		},
		{
			name:     "double-wrapped StateError returns ExitState",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", &StateError{Err: errors.New("bad state")})),
			expected: ExitState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCodeFromError(tt.err)
			if got != tt.expected {
				t.Errorf("ExitCodeFromError(%v) = %d, want %d", tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	inner := errors.New("root cause")

	tests := []struct {
		name string
		err  error
	}{
		{"ConnectionError", &ConnectionError{Err: inner}},
		{"TimeoutError", &TimeoutError{Err: inner}},
		{"StateError", &StateError{Err: inner}},
		{"NotFoundError", &NotFoundError{Err: inner}},
		{"UsageError", &UsageError{Err: inner}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, inner) {
				t.Errorf("%s.Unwrap() should expose inner error", tt.name)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	inner := errors.New("something broke")

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ConnectionError", &ConnectionError{Err: inner}, "connection error: something broke"},
		{"TimeoutError", &TimeoutError{Err: inner}, "timeout: something broke"},
		{"StateError", &StateError{Err: inner}, "state error: something broke"},
		{"NotFoundError", &NotFoundError{Err: inner}, "not found: something broke"},
		{"UsageError", &UsageError{Err: inner}, "usage error: something broke"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}
