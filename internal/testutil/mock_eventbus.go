// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"fmt"
	"sync"

	"github.com/valksor/go-mehrhof/internal/events"
)

// MockEventBus is a mock implementation of events.Bus for testing.
type MockEventBus struct {
	mu            sync.Mutex
	events        []events.Event
	subscriptions map[string][]events.Handler // eventType -> handlers
	allHandlers   []events.Handler
	publishedRaw  []events.Event
	closed        bool
	closeError    error
}

// NewMockEventBus creates a new mock event bus.
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events:        make([]events.Event, 0),
		subscriptions: make(map[string][]events.Handler),
		allHandlers:   make([]events.Handler, 0),
		publishedRaw:  make([]events.Event, 0),
	}
}

// Subscribe subscribes to events of a specific type.
func (m *MockEventBus) Subscribe(eventType events.Type, handler events.Handler) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store handler by string event type
	m.subscriptions[string(eventType)] = append(m.subscriptions[string(eventType)], handler)

	return "mock_id"
}

// SubscribeAll subscribes to all events.
func (m *MockEventBus) SubscribeAll(handler events.Handler) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.allHandlers = append(m.allHandlers, handler)

	return "mock_id"
}

// Unsubscribe removes a handler by ID (mock implementation - no-op).
func (m *MockEventBus) Unsubscribe(id string) {
	// Mock implementation - no-op since we don't track IDs
}

// PublishRaw publishes a raw event.
func (m *MockEventBus) PublishRaw(event events.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	m.events = append(m.events, event)
	m.publishedRaw = append(m.publishedRaw, event)

	// Notify type-specific subscribers
	eventTypeStr := string(event.Type)
	if handlers, ok := m.subscriptions[eventTypeStr]; ok {
		for _, handler := range handlers {
			handler(event)
		}
	}

	// Notify global subscribers
	for _, handler := range m.allHandlers {
		handler(event)
	}
}

// Publish publishes a typed event (converts to Event and calls PublishRaw).
func (m *MockEventBus) Publish(eventer events.Eventer) {
	event := eventer.ToEvent()
	m.PublishRaw(event)
}

// Close closes the event bus.
func (m *MockEventBus) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true

	return m.closeError
}

// Events returns all captured events.
func (m *MockEventBus) Events() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to avoid race conditions
	eventsCopy := make([]events.Event, len(m.events))
	copy(eventsCopy, m.events)

	return eventsCopy
}

// PublishedRaw returns all raw published events.
func (m *MockEventBus) PublishedRaw() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	eventsCopy := make([]events.Event, len(m.publishedRaw))
	copy(eventsCopy, m.publishedRaw)

	return eventsCopy
}

// Clear clears all captured events.
func (m *MockEventBus) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = make([]events.Event, 0)
	m.publishedRaw = make([]events.Event, 0)
}

// Count returns the count of captured events.
func (m *MockEventBus) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.events)
}

// CountByType returns the count of events of a specific type.
func (m *MockEventBus) CountByType(eventType events.Type) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, e := range m.events {
		if e.Type == eventType {
			count++
		}
	}

	return count
}

// FindByType returns events of a specific type.
func (m *MockEventBus) FindByType(eventType events.Type) []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := make([]events.Event, 0)
	for _, e := range m.events {
		if e.Type == eventType {
			found = append(found, e)
		}
	}

	return found
}

// LastEvent returns the last captured event.
func (m *MockEventBus) LastEvent() *events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.events) == 0 {
		return nil
	}
	last := m.events[len(m.events)-1]

	return &last
}

// HasEventType checks if an event of the given type was published.
func (m *MockEventBus) HasEventType(eventType events.Type) bool {
	return m.CountByType(eventType) > 0
}

// AssertEventType asserts that an event of the given type was published.
func (m *MockEventBus) AssertEventType(t TestingT, eventType events.Type) bool {
	t.Helper()
	if !m.HasEventType(eventType) {
		t.Errorf("expected event type %q, but none was published. Got events: %v", eventType, m.events)

		return false
	}

	return true
}

// AssertEventCount asserts the number of events of a type.
func (m *MockEventBus) AssertEventCount(t TestingT, eventType events.Type, expected int) bool {
	t.Helper()
	count := m.CountByType(eventType)
	if count != expected {
		t.Errorf("expected %d events of type %q, got %d", expected, eventType, count)

		return false
	}

	return true
}

// AssertMinEventCount asserts at least N events of a type.
func (m *MockEventBus) AssertMinEventCount(t TestingT, eventType events.Type, minimum int) bool {
	t.Helper()
	count := m.CountByType(eventType)
	if count < minimum {
		t.Errorf("expected at least %d events of type %q, got %d", minimum, eventType, count)

		return false
	}

	return true
}

// SetCloseError sets an error to return when Close is called.
func (m *MockEventBus) SetCloseError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeError = err
}

// SubscriberCount returns the number of subscribers for an event type.
func (m *MockEventBus) SubscriberCount(eventType events.Type) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.subscriptions[string(eventType)])
}

// AllSubscriberCount returns the total number of global subscribers.
func (m *MockEventBus) AllSubscriberCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.allHandlers)
}

// TestingT is the interface expected by testing.T (for helper functions).
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertHelper is an implementation of TestingT for use in tests.
type AssertHelper struct {
	t      interface{}
	Errors []string
	failed bool
}

// NewAssertHelper creates a new assert helper.
func NewAssertHelper(t interface{}) *AssertHelper {
	return &AssertHelper{t: t}
}

// Helper marks this as a test helper.
func (a *AssertHelper) Helper() {}

// Errorf records an error.
func (a *AssertHelper) Errorf(format string, args ...any) {
	a.Errors = append(a.Errors, fmt.Sprintf(format, args...))
	a.failed = true
}

// Failed returns true if any assertions failed.
func (a *AssertHelper) Failed() bool {
	return a.failed
}

// GetErrors returns all recorded errors.
func (a *AssertHelper) GetErrors() []string {
	return a.Errors
}

// AssertEventTypeWithoutT is like AssertEventType but without requiring testing.T.
func (m *MockEventBus) AssertEventTypeWithoutT(eventType events.Type) *AssertHelper {
	h := &AssertHelper{}
	m.AssertEventType(h, eventType)

	return h
}
