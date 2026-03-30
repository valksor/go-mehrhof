package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNotifier_SendAndDispatch(t *testing.T) {
	var (
		mu       sync.Mutex
		received []Payload
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		mu.Lock()
		received = append(received, p)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	endpoints := []WebhookEndpoint{
		{URL: srv.URL, Format: FormatGeneric},
	}

	n := New(endpoints, false)

	go n.Start(t.Context())

	payload := Payload{
		Event:     "state_changed",
		Timestamp: time.Now(),
		TaskID:    "task-123",
		TaskTitle: "Implement feature X",
		State:     "implementing",
		Message:   "Task moved to implementing",
	}

	n.Send(payload)

	// Wait for dispatch.
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook dispatch")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(received))
	}
	if received[0].TaskID != "task-123" {
		t.Errorf("expected task_id=task-123, got %s", received[0].TaskID)
	}
	if received[0].Event != "state_changed" {
		t.Errorf("expected event=state_changed, got %s", received[0].Event)
	}
}

func TestNotifier_EventFilter(t *testing.T) {
	var (
		mu       sync.Mutex
		received []Payload
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		mu.Lock()
		received = append(received, p)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	endpoints := []WebhookEndpoint{
		{URL: srv.URL, Format: FormatGeneric, Events: []string{"error"}},
	}

	n := New(endpoints, false)

	go n.Start(t.Context())

	// Send state_changed — should be filtered out.
	n.Send(Payload{
		Event:  "state_changed",
		TaskID: "task-1",
	})

	// Send error — should be dispatched.
	n.Send(Payload{
		Event:  "error",
		TaskID: "task-2",
		Error:  "something broke",
	})

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook dispatch")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Small grace period to catch any extra dispatches.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 dispatched payload, got %d", len(received))
	}
	if received[0].TaskID != "task-2" {
		t.Errorf("expected task_id=task-2, got %s", received[0].TaskID)
	}
}

func TestNotifier_OnFailureOverride(t *testing.T) {
	var (
		mu       sync.Mutex
		received []Payload
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		mu.Lock()
		received = append(received, p)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Endpoint only wants state_changed, but onFailure=true should override for errors.
	endpoints := []WebhookEndpoint{
		{URL: srv.URL, Format: FormatGeneric, Events: []string{"state_changed"}},
	}

	n := New(endpoints, true)

	go n.Start(t.Context())

	n.Send(Payload{
		Event:  "error",
		TaskID: "task-err",
		Error:  "agent crashed",
	})

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook dispatch")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 dispatched payload, got %d", len(received))
	}
	if received[0].Event != "error" {
		t.Errorf("expected event=error, got %s", received[0].Event)
	}
}

func TestNotifier_SlackFormat(t *testing.T) {
	p := Payload{
		Event:         "state_changed",
		TaskID:        "task-456",
		TaskTitle:     "Deploy to production",
		State:         "implementing",
		PreviousState: "planned",
		ProjectPath:   "/home/user/myproject",
		Error:         "build failed",
	}

	result := FormatSlackPayload(p)

	blocks, ok := result["blocks"].([]map[string]any)
	if !ok {
		t.Fatal("expected blocks key with slice of maps")
	}

	if len(blocks) < 1 {
		t.Fatal("expected at least 1 block")
	}

	// First block should be a section.
	if blocks[0]["type"] != "section" {
		t.Errorf("expected first block type=section, got %v", blocks[0]["type"])
	}

	text, ok := blocks[0]["text"].(map[string]any)
	if !ok {
		t.Fatal("expected text field in section block")
	}
	if text["text"] != "*Deploy to production*" {
		t.Errorf("expected bold title, got %v", text["text"])
	}

	fields, ok := blocks[0]["fields"].([]map[string]any)
	if !ok {
		t.Fatal("expected fields in section block")
	}

	// Should have State, Previous State, and Project fields.
	if len(fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(fields))
	}

	// Should have a context block for the error.
	if len(blocks) < 2 {
		t.Fatal("expected context block for error")
	}
	if blocks[1]["type"] != "context" {
		t.Errorf("expected second block type=context, got %v", blocks[1]["type"])
	}

	elements, ok := blocks[1]["elements"].([]map[string]any)
	if !ok {
		t.Fatal("expected elements in context block")
	}
	if len(elements) == 0 {
		t.Fatal("expected at least one element in context block")
	}
	if elements[0]["text"] != ":warning: build failed" {
		t.Errorf("expected warning text, got %v", elements[0]["text"])
	}
}

func TestNotifier_SlackFormat_NoError(t *testing.T) {
	p := Payload{
		Event:     "state_changed",
		TaskTitle: "Simple task",
		State:     "planned",
	}

	result := FormatSlackPayload(p)

	blocks, ok := result["blocks"].([]map[string]any)
	if !ok {
		t.Fatal("expected blocks key")
	}

	// No error means no context block — only the section.
	if len(blocks) != 1 {
		t.Errorf("expected 1 block (no error context), got %d", len(blocks))
	}
}

func TestNotifier_NonBlocking(t *testing.T) {
	endpoints := []WebhookEndpoint{
		{URL: "http://localhost:1/nope", Format: FormatGeneric},
	}

	n := New(endpoints, false)
	// Do not start the consumer — channel will fill up.

	// Fill the channel.
	for i := range channelCapacity {
		n.Send(Payload{
			Event:  "state_changed",
			TaskID: "fill-" + string(rune('0'+i%10)),
		})
	}

	// This extra send must not block.
	done := make(chan struct{})
	go func() {
		n.Send(Payload{
			Event:  "state_changed",
			TaskID: "overflow",
		})
		close(done)
	}()

	select {
	case <-done:
		// Success — Send returned without blocking.
	case <-time.After(1 * time.Second):
		t.Fatal("Send blocked on full channel")
	}
}
