package retry

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and rejecting calls.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation, tracking failures
	CircuitOpen                         // Rejecting calls, waiting for cooldown
	CircuitHalfOpen                     // Testing recovery with a single call
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern to fail fast when an
// external service is unavailable. It tracks consecutive failures and opens
// the circuit when the threshold is exceeded, avoiding wasted retry cycles.
type CircuitBreaker struct {
	name             string
	threshold        int           // consecutive failures before opening
	cooldown         time.Duration // time before transitioning to half-open
	state            CircuitState
	consecutiveFails int
	lastFailure      time.Time
	mu               sync.Mutex
}

// NewCircuitBreaker creates a circuit breaker with the given name, failure
// threshold, and cooldown duration.
func NewCircuitBreaker(name string, threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}

	return &CircuitBreaker{
		name:      name,
		threshold: threshold,
		cooldown:  cooldown,
		state:     CircuitClosed,
	}
}

// Allow returns true if the circuit breaker permits a request.
// In Open state, it checks whether the cooldown has elapsed and transitions
// to HalfOpen if so.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitHalfOpen:
		return false // Only one probe at a time; already admitted
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.cooldown {
			cb.state = CircuitHalfOpen

			return true
		}

		return false
	default:
		return true
	}
}

// RecordSuccess records a successful call and resets the circuit to closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails = 0
	cb.state = CircuitClosed
}

// RecordFailure records a failed call. If consecutive failures reach the
// threshold, the circuit opens.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.lastFailure = time.Now()

	if cb.consecutiveFails >= cb.threshold || cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.state
}

// Name returns the circuit breaker's name.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// WithCircuitBreaker adds a circuit breaker to RetryableOp. When the circuit
// is open, RetryableOp returns ErrCircuitOpen immediately without executing fn.
func WithCircuitBreaker(cb *CircuitBreaker) Option {
	return func(c *config) {
		c.circuitBreaker = cb
	}
}

// circuitOpenError wraps ErrCircuitOpen with the breaker name for diagnostics.
func circuitOpenError(name string) error {
	return fmt.Errorf("%w: %s", ErrCircuitOpen, name)
}
