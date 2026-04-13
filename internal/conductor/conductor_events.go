package conductor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/valksor/kvelmo/metrics"
)

// Internal methods
// Events returns the event channel.
func (c *Conductor) Events() <-chan ConductorEvent {
	return c.events
}

// AddListener adds an event listener.
func (c *Conductor) AddListener(listener EventListener) {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()
	c.listeners = append(c.listeners, listener)
}

// Close cleans up conductor resources.
// Safe to call multiple times.
func (c *Conductor) Close() error {
	c.closeOnce.Do(func() {
		// Acquire eventsMu to serialize with emit() and prevent TOCTOU race
		c.eventsMu.Lock()
		c.closed.Store(true)
		close(c.events)
		c.eventsMu.Unlock()

		// Cancel lifecycle context to stop background goroutines
		if c.lifecycleCancel != nil {
			c.lifecycleCancel()
		}
	})

	return nil
}

func (c *Conductor) onStateChanged(from, to State, event Event, wu *WorkUnit) {
	c.emit(ConductorEvent{
		Type:    "state_changed",
		State:   to,
		Message: fmt.Sprintf("State changed: %s -> %s (event: %s)", from, to, event),
	})
}

// emitEnrichedError emits a user-friendly error event with fix instructions.
// The enriched error data is included in the event's Data field as JSON.
func (c *Conductor) emitEnrichedError(err error, phase string) {
	ue := EnrichError(err, phase)

	data, marshalErr := json.Marshal(ue)
	if marshalErr != nil {
		slog.Warn("failed to marshal enriched error", "error", marshalErr)
		data = nil
	}

	c.emit(ConductorEvent{
		Type:    "error",
		Error:   ue.Message,
		Message: ue.Fix,
		Data:    data,
	})
}

func (c *Conductor) emit(e ConductorEvent) {
	// Acquire eventsMu to prevent race with Close() closing the channel.
	// This serializes the closed-check + send, eliminating the TOCTOU window.
	c.eventsMu.Lock()
	if c.closed.Load() {
		c.eventsMu.Unlock()

		return
	}
	e.Timestamp = time.Now()
	// Send to channel (non-blocking). The events channel is an optional
	// subscription API for callers that prefer channel semantics over
	// AddListener; in deployments where no one reads c.events the channel
	// saturates quickly. Drops are counted via metrics but not logged —
	// listeners still receive every event, so a full channel is not a fault.
	select {
	case c.events <- e:
	default:
		metrics.Global().RecordEventDropped()
	}
	c.eventsMu.Unlock()

	// Notify listeners under separate lock to avoid deadlock with c.mu
	c.listenersMu.RLock()
	listeners := make([]EventListener, len(c.listeners))
	copy(listeners, c.listeners)
	c.listenersMu.RUnlock()

	for _, listener := range listeners {
		go listener(e)
	}
}
