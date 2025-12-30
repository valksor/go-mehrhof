package events

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBus(t *testing.T) {
	bus := NewBus()
	if bus == nil {
		t.Fatal("NewBus returned nil")
	}
	if bus.handlers == nil {
		t.Error("handlers map not initialized")
	}
	if bus.allHandlers == nil {
		t.Error("allHandlers slice not initialized")
	}
}

func TestSubscribe(t *testing.T) {
	bus := NewBus()

	id := bus.Subscribe(TypeStateChanged, func(e Event) {})

	if id == "" {
		t.Error("Subscribe returned empty ID")
	}

	if !bus.HasSubscribers(TypeStateChanged) {
		t.Error("HasSubscribers returned false after Subscribe")
	}
}

func TestSubscribeMultiple(t *testing.T) {
	bus := NewBus()

	id1 := bus.Subscribe(TypeStateChanged, func(e Event) {})
	id2 := bus.Subscribe(TypeStateChanged, func(e Event) {})

	if id1 == id2 {
		t.Error("Subscribe returned duplicate IDs")
	}
}

func TestSubscribeAll(t *testing.T) {
	bus := NewBus()

	id := bus.SubscribeAll(func(e Event) {})

	if id == "" {
		t.Error("SubscribeAll returned empty ID")
	}

	// SubscribeAll means it subscribes to ALL types
	if !bus.HasSubscribers(TypeStateChanged) {
		t.Error("HasSubscribers returned false for TypeStateChanged after SubscribeAll")
	}
	if !bus.HasSubscribers(TypeError) {
		t.Error("HasSubscribers returned false for TypeError after SubscribeAll")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewBus()

	id := bus.Subscribe(TypeStateChanged, func(e Event) {})
	bus.Unsubscribe(id)

	if bus.HasSubscribers(TypeStateChanged) {
		t.Error("HasSubscribers returned true after Unsubscribe")
	}
}

func TestUnsubscribeAll(t *testing.T) {
	bus := NewBus()

	id := bus.SubscribeAll(func(e Event) {})
	bus.Unsubscribe(id)

	if bus.HasSubscribers(TypeStateChanged) {
		t.Error("HasSubscribers returned true after Unsubscribe of all-handler")
	}
}

func TestUnsubscribeNonexistent(t *testing.T) {
	bus := NewBus()

	// Should not panic
	bus.Unsubscribe("nonexistent")
}

func TestPublish(t *testing.T) {
	bus := NewBus()

	var received Event
	bus.Subscribe(TypeStateChanged, func(e Event) {
		received = e
	})

	event := StateChangedEvent{
		From:   "idle",
		To:     "acquiring",
		Event:  "start",
		TaskID: "test-123",
	}
	bus.Publish(event)

	if received.Type != TypeStateChanged {
		t.Errorf("received event type = %v, want %v", received.Type, TypeStateChanged)
	}
	if received.Data["from"] != "idle" {
		t.Errorf("received from = %v, want idle", received.Data["from"])
	}
	if received.Data["to"] != "acquiring" {
		t.Errorf("received to = %v, want acquiring", received.Data["to"])
	}
}

func TestPublishToMultipleSubscribers(t *testing.T) {
	bus := NewBus()

	var count atomic.Int32

	bus.Subscribe(TypeStateChanged, func(e Event) { count.Add(1) })
	bus.Subscribe(TypeStateChanged, func(e Event) { count.Add(1) })
	bus.Subscribe(TypeStateChanged, func(e Event) { count.Add(1) })

	bus.Publish(StateChangedEvent{From: "a", To: "b"})

	if count.Load() != 3 {
		t.Errorf("count = %d, want 3", count.Load())
	}
}

func TestPublishRaw(t *testing.T) {
	bus := NewBus()

	var received Event
	bus.Subscribe(TypeError, func(e Event) {
		received = e
	})

	event := Event{
		Type:      TypeError,
		Timestamp: time.Now(),
		Data:      map[string]any{"message": "test error"},
	}
	bus.PublishRaw(event)

	if received.Type != TypeError {
		t.Errorf("received event type = %v, want %v", received.Type, TypeError)
	}
	if received.Data["message"] != "test error" {
		t.Errorf("received message = %v, want 'test error'", received.Data["message"])
	}
}

func TestPublishDoesNotReachOtherTypes(t *testing.T) {
	bus := NewBus()

	var called bool
	bus.Subscribe(TypeError, func(e Event) {
		called = true
	})

	bus.Publish(StateChangedEvent{From: "a", To: "b"})

	if called {
		t.Error("handler for TypeError was called for StateChanged event")
	}
}

func TestSubscribeAllReceivesAllTypes(t *testing.T) {
	bus := NewBus()

	var events []Event
	var mu sync.Mutex
	bus.SubscribeAll(func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})

	bus.Publish(StateChangedEvent{From: "a", To: "b"})
	bus.Publish(ErrorEvent{TaskID: "test"})

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Errorf("received %d events, want 2", len(events))
	}
}

func TestClear(t *testing.T) {
	bus := NewBus()

	bus.Subscribe(TypeStateChanged, func(e Event) {})
	bus.SubscribeAll(func(e Event) {})

	bus.Clear()

	if bus.HasSubscribers(TypeStateChanged) {
		t.Error("HasSubscribers returned true after Clear")
	}
}

func TestConcurrentPublish(t *testing.T) {
	bus := NewBus()

	var count atomic.Int64
	bus.Subscribe(TypeProgress, func(e Event) {
		count.Add(1)
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			bus.Publish(ProgressEvent{TaskID: "test"})
		})
	}

	wg.Wait()

	if count.Load() != 100 {
		t.Errorf("count = %d, want 100", count.Load())
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	bus := NewBus()

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			id := bus.Subscribe(TypeProgress, func(e Event) {})
			bus.Unsubscribe(id)
		})
	}

	wg.Wait()
	// Test passes if no race conditions or panics
}

func TestPublishAsync(t *testing.T) {
	bus := NewBus()

	var received atomic.Bool
	bus.Subscribe(TypeCheckpoint, func(e Event) {
		received.Store(true)
	})

	bus.PublishAsync(CheckpointEvent{TaskID: "test", Commit: "abc123"})

	// Give async handler time to run
	time.Sleep(10 * time.Millisecond)

	if !received.Load() {
		t.Error("async handler was not called")
	}
}

func TestPublishRawAsync(t *testing.T) {
	bus := NewBus()

	var received atomic.Bool
	bus.Subscribe(TypeError, func(e Event) {
		received.Store(true)
	})

	event := Event{
		Type:      TypeError,
		Timestamp: time.Now(),
		Data:      map[string]any{"message": "async error"},
	}
	bus.PublishRawAsync(event)

	// Give async handler time to run
	time.Sleep(10 * time.Millisecond)

	if !received.Load() {
		t.Error("async handler was not called")
	}
}

func TestStateChangedEventToEvent(t *testing.T) {
	e := StateChangedEvent{
		From:   "idle",
		To:     "planning",
		Event:  "start",
		TaskID: "task-123",
	}
	event := e.ToEvent()

	if event.Type != TypeStateChanged {
		t.Errorf("Type = %v, want %v", event.Type, TypeStateChanged)
	}
	if event.Data["from"] != "idle" {
		t.Errorf("from = %v, want idle", event.Data["from"])
	}
	if event.Data["to"] != "planning" {
		t.Errorf("to = %v, want planning", event.Data["to"])
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestStateChangedEventToEvent_WithTimestamp(t *testing.T) {
	ts := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	e := StateChangedEvent{
		From:      "idle",
		To:        "planning",
		Timestamp: ts,
	}
	event := e.ToEvent()

	if event.Timestamp != ts {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, ts)
	}
}

func TestProgressEventToEvent(t *testing.T) {
	e := ProgressEvent{
		TaskID:  "task-123",
		Phase:   "implementing",
		Message: "Working...",
		Current: 5,
		Total:   10,
	}
	event := e.ToEvent()

	if event.Type != TypeProgress {
		t.Errorf("Type = %v, want %v", event.Type, TypeProgress)
	}
	if event.Data["phase"] != "implementing" {
		t.Errorf("phase = %v, want implementing", event.Data["phase"])
	}
	if event.Data["current"] != 5 {
		t.Errorf("current = %v, want 5", event.Data["current"])
	}
}

func TestErrorEventToEvent(t *testing.T) {
	e := ErrorEvent{
		TaskID: "task-123",
		Error:  nil,
		Fatal:  false,
	}
	event := e.ToEvent()

	if event.Type != TypeError {
		t.Errorf("Type = %v, want %v", event.Type, TypeError)
	}
	if event.Data["error"] != "" {
		t.Errorf("error = %v, want empty string", event.Data["error"])
	}
}

func TestErrorEventToEvent_WithError(t *testing.T) {
	e := ErrorEvent{
		TaskID: "task-123",
		Error:  &testError{msg: "test error"},
		Fatal:  true,
	}
	event := e.ToEvent()

	if event.Data["error"] != "test error" {
		t.Errorf("error = %v, want 'test error'", event.Data["error"])
	}
	if event.Data["fatal"] != true {
		t.Errorf("fatal = %v, want true", event.Data["fatal"])
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestFileChangedEventToEvent(t *testing.T) {
	e := FileChangedEvent{
		TaskID:    "task-123",
		Path:      "/path/to/file.go",
		Operation: "create",
	}
	event := e.ToEvent()

	if event.Type != TypeFileChanged {
		t.Errorf("Type = %v, want %v", event.Type, TypeFileChanged)
	}
	if event.Data["path"] != "/path/to/file.go" {
		t.Errorf("path = %v, want /path/to/file.go", event.Data["path"])
	}
	if event.Data["operation"] != "create" {
		t.Errorf("operation = %v, want create", event.Data["operation"])
	}
}

func TestCheckpointEventToEvent(t *testing.T) {
	e := CheckpointEvent{
		TaskID:  "task-123",
		Commit:  "abc123",
		Message: "checkpoint message",
	}
	event := e.ToEvent()

	if event.Type != TypeCheckpoint {
		t.Errorf("Type = %v, want %v", event.Type, TypeCheckpoint)
	}
	if event.Data["commit"] != "abc123" {
		t.Errorf("commit = %v, want abc123", event.Data["commit"])
	}
}

func TestAgentMessageEventToEvent(t *testing.T) {
	e := AgentMessageEvent{
		TaskID:  "task-123",
		Content: "Hello from agent",
		Role:    "assistant",
	}
	event := e.ToEvent()

	if event.Type != TypeAgentMessage {
		t.Errorf("Type = %v, want %v", event.Type, TypeAgentMessage)
	}
	if event.Data["content"] != "Hello from agent" {
		t.Errorf("content = %v, want 'Hello from agent'", event.Data["content"])
	}
	if event.Data["role"] != "assistant" {
		t.Errorf("role = %v, want assistant", event.Data["role"])
	}
}

func TestBlueprintReadyEventToEvent(t *testing.T) {
	e := BlueprintReadyEvent{
		TaskID:      "task-123",
		BlueprintID: "bp-456",
	}
	event := e.ToEvent()

	if event.Type != TypeBlueprintReady {
		t.Errorf("Type = %v, want %v", event.Type, TypeBlueprintReady)
	}
	if event.Data["blueprint_id"] != "bp-456" {
		t.Errorf("blueprint_id = %v, want bp-456", event.Data["blueprint_id"])
	}
}

func TestBranchCreatedEventToEvent(t *testing.T) {
	e := BranchCreatedEvent{
		TaskID: "task-123",
		Branch: "feature/task-123",
	}
	event := e.ToEvent()

	if event.Type != TypeBranchCreated {
		t.Errorf("Type = %v, want %v", event.Type, TypeBranchCreated)
	}
	if event.Data["branch"] != "feature/task-123" {
		t.Errorf("branch = %v, want feature/task-123", event.Data["branch"])
	}
}

func TestPlanCompletedEventToEvent(t *testing.T) {
	e := PlanCompletedEvent{
		TaskID:          "task-123",
		SpecificationID: 1,
	}
	event := e.ToEvent()

	if event.Type != TypePlanCompleted {
		t.Errorf("Type = %v, want %v", event.Type, TypePlanCompleted)
	}
	if event.Data["specification_id"] != 1 {
		t.Errorf("specification_id = %v, want 1", event.Data["specification_id"])
	}
}

func TestImplementDoneEventToEvent(t *testing.T) {
	e := ImplementDoneEvent{
		TaskID:   "task-123",
		DiffStat: "+10 -5",
	}
	event := e.ToEvent()

	if event.Type != TypeImplementDone {
		t.Errorf("Type = %v, want %v", event.Type, TypeImplementDone)
	}
	if event.Data["diff_stat"] != "+10 -5" {
		t.Errorf("diff_stat = %v, want '+10 -5'", event.Data["diff_stat"])
	}
}

func TestPRCreatedEventToEvent(t *testing.T) {
	e := PRCreatedEvent{
		TaskID:   "task-123",
		PRNumber: 42,
		PRURL:    "https://github.com/owner/repo/pull/42",
	}
	event := e.ToEvent()

	if event.Type != TypePRCreated {
		t.Errorf("Type = %v, want %v", event.Type, TypePRCreated)
	}
	if event.Data["pr_number"] != 42 {
		t.Errorf("pr_number = %v, want 42", event.Data["pr_number"])
	}
	if event.Data["pr_url"] != "https://github.com/owner/repo/pull/42" {
		t.Errorf("pr_url = %v, want https://github.com/owner/repo/pull/42", event.Data["pr_url"])
	}
}

func TestShutdown(t *testing.T) {
	bus := NewBus()

	var count atomic.Int32
	var started atomic.Int32
	bus.Subscribe(TypeProgress, func(e Event) {
		started.Add(1)
		// Simulate some work
		time.Sleep(5 * time.Millisecond)
		count.Add(1)
	})

	// Publish multiple async events
	for range 10 {
		bus.PublishAsync(ProgressEvent{TaskID: "test"})
	}

	// Give goroutines time to start (acquire semaphore)
	time.Sleep(20 * time.Millisecond)

	// Shutdown should wait for all active async handlers to complete
	bus.Shutdown()

	// Handlers that started should have completed
	// Note: Some may not have started if they were blocked on semaphore when context cancelled
	if count.Load() != started.Load() {
		t.Errorf("count = %d, started = %d - started handlers should complete", count.Load(), started.Load())
	}

	// At least some handlers should have run
	if count.Load() == 0 {
		t.Error("no handlers ran")
	}
}

func TestShutdownWaitsForActiveHandlers(t *testing.T) {
	bus := NewBus()

	var completed atomic.Bool
	bus.Subscribe(TypeProgress, func(e Event) {
		// Long-running handler
		time.Sleep(50 * time.Millisecond)
		completed.Store(true)
	})

	// Start an async handler
	bus.PublishAsync(ProgressEvent{TaskID: "test"})

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown should wait for the active handler
	bus.Shutdown()

	// Handler should have completed before Shutdown returned
	if !completed.Load() {
		t.Error("Shutdown returned before handler completed")
	}
}

func TestPublishAsyncSemaphoreLimiting(t *testing.T) {
	bus := NewBus()

	var concurrentCount atomic.Int32
	var maxConcurrent atomic.Int32

	bus.Subscribe(TypeProgress, func(e Event) {
		// Track max concurrent handlers
		current := concurrentCount.Add(1)
		for {
			oldMax := maxConcurrent.Load()
			if current <= oldMax {
				break
			}
			if maxConcurrent.CompareAndSwap(oldMax, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond) // Simulate work
		concurrentCount.Add(-1)
	})

	// Publish more events than the semaphore allows (maxAsyncPublishes = 100)
	for range 150 {
		bus.PublishAsync(ProgressEvent{TaskID: "test"})
	}

	// Wait for all to complete
	bus.Shutdown()

	// Max concurrent should not exceed maxAsyncPublishes (100)
	if maxConcurrent.Load() > maxAsyncPublishes {
		t.Errorf("max concurrent = %d, should not exceed %d", maxConcurrent.Load(), maxAsyncPublishes)
	}
}
