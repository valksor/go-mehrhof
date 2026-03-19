package agent

import (
	"sync"
	"time"
)

// Health tracks the health state of an agent provider using a circuit breaker
// pattern. After consecutive failures, the agent enters exponential backoff
// cooldown (30s, 60s, 120s, 240s, capped at 5 minutes). A successful call
// resets the tracker immediately.
type Health struct {
	mu            sync.Mutex
	failureCount  int
	lastFailure   time.Time
	cooldownUntil time.Time
	consecutiveOK int
	nowFunc       func() time.Time // for testing; defaults to time.Now
}

// NewHealth creates a new health tracker. The agent starts in a healthy state.
func NewHealth() *Health {
	return &Health{
		nowFunc: time.Now,
	}
}

// IsHealthy returns true if the agent is available (not in cooldown).
func (h *Health) IsHealthy() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.failureCount == 0 {
		return true
	}

	return h.now().After(h.cooldownUntil)
}

// RecordSuccess resets failure tracking and increments consecutive OK count.
func (h *Health) RecordSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failureCount = 0
	h.cooldownUntil = time.Time{}
	h.consecutiveOK++
}

// RecordFailure increments failures and sets cooldown.
// Cooldown progression: 30s, 60s, 120s, 240s, capped at 5 minutes.
func (h *Health) RecordFailure() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failureCount++
	h.consecutiveOK = 0
	now := h.now()
	h.lastFailure = now
	h.cooldownUntil = now.Add(cooldownDuration(h.failureCount))
}

// FailureCount returns the current consecutive failure count.
func (h *Health) FailureCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.failureCount
}

// CooldownRemaining returns time until the agent is available again.
// Returns zero if the agent is healthy.
func (h *Health) CooldownRemaining() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.failureCount == 0 {
		return 0
	}

	remaining := h.cooldownUntil.Sub(h.now())
	if remaining < 0 {
		return 0
	}

	return remaining
}

// Reset clears all health state, returning to a fresh healthy state.
func (h *Health) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failureCount = 0
	h.lastFailure = time.Time{}
	h.cooldownUntil = time.Time{}
	h.consecutiveOK = 0
}

// now returns the current time, using nowFunc for testability.
func (h *Health) now() time.Time {
	if h.nowFunc != nil {
		return h.nowFunc()
	}

	return time.Now()
}

// cooldownDuration calculates the cooldown duration for a given failure count.
// Progression: 30s, 60s, 120s, 240s, capped at 5 minutes.
func cooldownDuration(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}

	base := 30 * time.Second
	shift := min(failures-1, 4)
	d := base * time.Duration(1<<shift) // 30s, 60s, 120s, 240s

	if d > 5*time.Minute {
		d = 5 * time.Minute
	}

	return d
}
