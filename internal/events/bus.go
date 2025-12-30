package events

import (
	"context"
	"fmt"
	"sync"
)

const (
	// maxAsyncPublishes limits concurrent goroutines in PublishAsync
	maxAsyncPublishes = 100
)

// Handler processes events
type Handler func(Event)

// Subscription tracks a handler registration
type Subscription struct {
	ID      string
	Type    Type
	Handler Handler
}

// Bus manages event pub/sub
type Bus struct {
	mu          sync.RWMutex
	handlers    map[Type][]Subscription
	allHandlers []Subscription
	nextID      int
	// semaphore limits concurrent goroutines in PublishAsync
	semaphore chan struct{}
	// wg tracks active async publishes for graceful shutdown
	wg sync.WaitGroup
	// ctx is used for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBus creates a new event bus
func NewBus() *Bus {
	ctx, cancel := context.WithCancel(context.Background())
	return &Bus{
		handlers:    make(map[Type][]Subscription),
		allHandlers: make([]Subscription, 0),
		semaphore:   make(chan struct{}, maxAsyncPublishes),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Subscribe registers a handler for a specific event type
// Returns subscription ID for later unsubscription
func (b *Bus) Subscribe(eventType Type, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := fmt.Sprintf("sub_%d", b.nextID)

	sub := Subscription{
		ID:      id,
		Type:    eventType,
		Handler: handler,
	}

	b.handlers[eventType] = append(b.handlers[eventType], sub)
	return id
}

// SubscribeAll registers a handler for all events
// Returns subscription ID for later unsubscription
func (b *Bus) SubscribeAll(handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := fmt.Sprintf("sub_%d", b.nextID)

	sub := Subscription{
		ID:      id,
		Handler: handler,
	}

	b.allHandlers = append(b.allHandlers, sub)
	return id
}

// Unsubscribe removes a handler by ID
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Remove from type-specific handlers
	for eventType, subs := range b.handlers {
		for i, sub := range subs {
			if sub.ID == id {
				b.handlers[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}

	// Remove from all-handlers
	for i, sub := range b.allHandlers {
		if sub.ID == id {
			b.allHandlers = append(b.allHandlers[:i], b.allHandlers[i+1:]...)
			return
		}
	}
}

// Publish sends a typed event to all registered handlers
func (b *Bus) Publish(e Eventer) {
	event := e.ToEvent()
	b.PublishRaw(event)
}

// PublishRaw sends a raw event to all registered handlers
func (b *Bus) PublishRaw(event Event) {
	b.mu.RLock()
	// Pre-allocate capacity to avoid reallocations
	capacity := len(b.allHandlers)
	if subs, ok := b.handlers[event.Type]; ok {
		capacity += len(subs)
	}
	handlers := make([]Handler, 0, capacity)

	// Type-specific handlers
	if subs, ok := b.handlers[event.Type]; ok {
		for _, sub := range subs {
			handlers = append(handlers, sub.Handler)
		}
	}

	// All-event handlers
	for _, sub := range b.allHandlers {
		handlers = append(handlers, sub.Handler)
	}
	b.mu.RUnlock()

	// Call handlers outside lock to prevent deadlocks
	for _, handler := range handlers {
		handler(event)
	}
}

// PublishAsync sends an event asynchronously
func (b *Bus) PublishAsync(e Eventer) {
	b.PublishRawAsync(e.ToEvent())
}

// PublishRawAsync sends a raw event asynchronously
func (b *Bus) PublishRawAsync(event Event) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		// Acquire semaphore slot or exit if context cancelled
		select {
		case b.semaphore <- struct{}{}:
			defer func() { <-b.semaphore }()
		case <-b.ctx.Done():
			return
		}
		b.PublishRaw(event)
	}()
}

// HasSubscribers returns true if there are any subscribers for the given type
func (b *Bus) HasSubscribers(eventType Type) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.allHandlers) > 0 {
		return true
	}
	return len(b.handlers[eventType]) > 0
}

// Clear removes all subscriptions
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers = make(map[Type][]Subscription)
	b.allHandlers = make([]Subscription, 0)
}

// Shutdown gracefully shuts down the event bus, waiting for async publishes to complete.
func (b *Bus) Shutdown() {
	b.cancel()
	// Wait for all active async publishes to complete
	b.wg.Wait()
}
