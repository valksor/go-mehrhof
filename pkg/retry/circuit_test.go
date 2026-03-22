package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_StartsClosedAndAllows(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 3, time.Second)

	if cb.State() != CircuitClosed {
		t.Fatalf("expected closed, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("closed circuit should allow requests")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 3, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitClosed {
		t.Fatal("should still be closed after 2 failures")
	}

	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected open after 3 failures, got %s", cb.State())
	}
	if cb.Allow() {
		t.Fatal("open circuit should reject requests")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 3, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Fatal("success should reset to closed")
	}

	// Need full threshold again to open
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitClosed {
		t.Fatal("should still be closed after 2 failures post-reset")
	}
}

func TestCircuitBreaker_HalfOpenOnCooldown(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)

	cb.RecordFailure() // Opens circuit

	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}

	time.Sleep(15 * time.Millisecond)

	// After cooldown, Allow() should transition to half-open and return true
	if !cb.Allow() {
		t.Fatal("expected half-open to allow one probe")
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("expected half-open, got %s", cb.State())
	}

	// Second Allow() in half-open should be rejected
	if cb.Allow() {
		t.Fatal("half-open should only allow one probe")
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)

	cb.RecordFailure()

	time.Sleep(15 * time.Millisecond)

	cb.Allow() // Transitions to half-open
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Fatal("success in half-open should close circuit")
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)

	cb.RecordFailure()

	time.Sleep(15 * time.Millisecond)

	cb.Allow() // Transitions to half-open
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatal("failure in half-open should reopen circuit")
	}
}

func TestCircuitBreaker_DefaultValues(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("defaults", 0, 0)

	if cb.threshold != 5 {
		t.Errorf("expected default threshold 5, got %d", cb.threshold)
	}
	if cb.cooldown != 30*time.Second {
		t.Errorf("expected default cooldown 30s, got %v", cb.cooldown)
	}
}

func TestCircuitBreaker_Name(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("github-api", 3, time.Second)

	if cb.Name() != "github-api" {
		t.Errorf("expected name github-api, got %s", cb.Name())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestRetryableOp_WithCircuitBreaker(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test-service", 2, time.Minute)

	// Two failures should open the circuit
	calls := 0
	_ = RetryableOp(context.Background(), 0, time.Millisecond, func() error {
		calls++

		return errors.New("temporarily unavailable")
	}, WithCircuitBreaker(cb))

	_ = RetryableOp(context.Background(), 0, time.Millisecond, func() error {
		calls++

		return errors.New("temporarily unavailable")
	}, WithCircuitBreaker(cb))

	if cb.State() != CircuitOpen {
		t.Fatalf("expected open after 2 failures, got %s", cb.State())
	}

	// Next call should fail immediately with ErrCircuitOpen
	err := RetryableOp(context.Background(), 3, time.Millisecond, func() error {
		calls++

		return errors.New("should not be called")
	}, WithCircuitBreaker(cb))

	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 total calls (fn should not execute when open), got %d", calls)
	}
}

func TestRetryableOp_CircuitBreakerRecordsSuccess(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker("test-service", 5, time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()

	err := RetryableOp(context.Background(), 0, time.Millisecond, func() error {
		return nil
	}, WithCircuitBreaker(cb))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.State() != CircuitClosed {
		t.Fatal("success should keep circuit closed")
	}
}
