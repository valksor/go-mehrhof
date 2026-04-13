package provider

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerStateString(t *testing.T) {
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

func TestCircuitBreakerDefaults(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	if cb.maxFailures != 5 {
		t.Errorf("default maxFailures = %d, want 5", cb.maxFailures)
	}
	if cb.resetTimeout != 30*time.Second {
		t.Errorf("default resetTimeout = %v, want 30s", cb.resetTimeout)
	}
}

func TestCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	if cb.State() != CircuitClosed {
		t.Fatalf("initial state = %v, want closed", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow() on closed circuit returned error: %v", err)
	}
}

func TestCircuitBreakerOpensAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	// First two failures keep circuit closed
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Fatalf("state after 2 failures = %v, want closed", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow() after 2 failures returned error: %v", err)
	}

	// Third failure opens circuit
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("state after 3 failures = %v, want open", cb.State())
	}
	if err := cb.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Allow() on open circuit = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("state = %v, want open", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(15 * time.Millisecond)

	// Should transition to half-open and allow one probe
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow() after timeout returned error: %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("state after timeout = %v, want half-open", cb.State())
	}

	// Second request while half-open should be blocked
	if err := cb.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Allow() in half-open (second call) = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreakerHalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for reset timeout and trigger half-open
	time.Sleep(15 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}

	// Successful probe closes circuit
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Fatalf("state after success in half-open = %v, want closed", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow() after recovery returned error: %v", err)
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for reset timeout and trigger half-open
	time.Sleep(15 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}

	// Failed probe reopens circuit (failures >= maxFailures since count carried over + 1 more)
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("state after failure in half-open = %v, want open", cb.State())
	}
	if err := cb.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Allow() after reopening = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreakerRecordSuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	// Accumulate some failures
	cb.RecordFailure()
	cb.RecordFailure()

	// Success resets the count
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Fatalf("state after success = %v, want closed", cb.State())
	}

	// Should need 3 more failures to open again
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Fatalf("state after 2 new failures = %v, want closed", cb.State())
	}

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("state after 3 new failures = %v, want open", cb.State())
	}
}
