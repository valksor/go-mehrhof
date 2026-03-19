package agent

import (
	"sync"
	"time"
)

// HealthStatus is a point-in-time snapshot of an agent's health state.
type HealthStatus struct {
	Healthy           bool       `json:"healthy"`
	FailureCount      int        `json:"failure_count"`
	CooldownRemaining string     `json:"cooldown_remaining,omitempty"`
	LastFailure       *time.Time `json:"last_failure,omitempty"`
}

// HealthRegistry tracks health for multiple agents. It is safe for concurrent
// use. Trackers are created lazily on first access, so callers never need to
// pre-register agent names.
type HealthRegistry struct {
	mu     sync.RWMutex
	agents map[string]*Health
}

// NewHealthRegistry creates a new health registry.
func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{
		agents: make(map[string]*Health),
	}
}

// Get returns the health tracker for an agent, creating one if needed.
func (r *HealthRegistry) Get(name string) *Health {
	// Fast path: read lock
	r.mu.RLock()
	h, ok := r.agents[name]
	r.mu.RUnlock()

	if ok {
		return h
	}

	// Slow path: write lock, double-check
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, ok = r.agents[name]; ok {
		return h
	}

	h = NewHealth()
	r.agents[name] = h

	return h
}

// SelectHealthy returns the first healthy agent from the given list.
// Returns the name and true if found, empty string and false if all are
// unhealthy or the list is empty.
func (r *HealthRegistry) SelectHealthy(names []string) (string, bool) {
	for _, name := range names {
		if r.Get(name).IsHealthy() {
			return name, true
		}
	}

	return "", false
}

// Status returns a snapshot of all tracked agent health states.
func (r *HealthRegistry) Status() map[string]HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]HealthStatus, len(r.agents))

	for name, h := range r.agents {
		h.mu.Lock()

		status := HealthStatus{
			Healthy:      h.failureCount == 0 || h.now().After(h.cooldownUntil),
			FailureCount: h.failureCount,
		}

		if h.failureCount > 0 {
			lf := h.lastFailure
			status.LastFailure = &lf

			remaining := h.cooldownUntil.Sub(h.now())
			if remaining > 0 {
				status.CooldownRemaining = remaining.Truncate(time.Second).String()
			}
		}

		h.mu.Unlock()

		result[name] = status
	}

	return result
}
