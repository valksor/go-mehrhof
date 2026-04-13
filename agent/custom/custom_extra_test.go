package custom

import (
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// readEvent reads from ch with a short timeout to avoid test hangs.
func readEvent(t *testing.T, ch chan agent.Event) (agent.Event, bool) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		return ev, ok
	case <-time.After(100 * time.Millisecond):
		return agent.Event{}, false
	}
}

// newTestAgent creates an Agent ready for parseJSONLine tests.
// It creates an events channel of size 10 on the returned agent.
func newTestAgent() *Agent {
	a := NewWithConfig(DefaultConfig("test", []string{"echo"}))
	a.events = make(chan agent.Event, 10)

	return a
}

// ============================================================
// Stream / text / delta types
// ============================================================

func TestParseJSONLine_StreamType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"stream","content":"hello"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event from channel, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "hello" {
		t.Errorf("Content = %q, want hello", ev.Content)
	}
}

func TestParseJSONLine_TextType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"text","text":"hi"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "hi" {
		t.Errorf("Content = %q, want hi", ev.Content)
	}
}

func TestParseJSONLine_DeltaType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"delta","delta":"d"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "d" {
		t.Errorf("Content = %q, want d", ev.Content)
	}
}

func TestParseJSONLine_ContentType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"content","content":"value"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "value" {
		t.Errorf("Content = %q, want value", ev.Content)
	}
}

func TestParseJSONLine_AssistantType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"assistant","content":"assistant response"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "assistant response" {
		t.Errorf("Content = %q, want 'assistant response'", ev.Content)
	}
}

// ============================================================
// Error type
// ============================================================

func TestParseJSONLine_ErrorType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"error","error":"oops"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventError {
		t.Errorf("Type = %q, want EventError", ev.Type)
	}
	if ev.Error != "oops" {
		t.Errorf("Error = %q, want oops", ev.Error)
	}
}

// ============================================================
// Complete / done / result types
// ============================================================

func TestParseJSONLine_CompleteType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"complete"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventComplete {
		t.Errorf("Type = %q, want EventComplete", ev.Type)
	}
}

func TestParseJSONLine_DoneType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"done"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventComplete {
		t.Errorf("Type = %q, want EventComplete", ev.Type)
	}
}

func TestParseJSONLine_ResultTypeSuccess(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"result","success":true}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventComplete {
		t.Errorf("Type = %q, want EventComplete", ev.Type)
	}
}

func TestParseJSONLine_ResultTypeFailure(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"result","success":false}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventError {
		t.Errorf("Type = %q, want EventError", ev.Type)
	}
}

// ============================================================
// Tool use type
// ============================================================

func TestParseJSONLine_ToolUseType(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"tool_use","tool":"Read","input":{}}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventToolUse {
		t.Errorf("Type = %q, want EventToolUse", ev.Type)
	}
	if ev.Content != "Read" {
		t.Errorf("Content = %q, want Read", ev.Content)
	}
	if ev.Data == nil {
		t.Error("Data should not be nil for tool_use with input")
	}
}

func TestParseJSONLine_ToolUseWithInput(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"tool_use","tool":"Write","input":{"file_path":"/tmp/x.go","content":"package main"}}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventToolUse {
		t.Errorf("Type = %q, want EventToolUse", ev.Type)
	}
	if ev.Content != "Write" {
		t.Errorf("Content = %q, want Write", ev.Content)
	}
	if fp, ok := ev.Data["file_path"]; !ok || fp != "/tmp/x.go" {
		t.Errorf("Data[file_path] = %v, want /tmp/x.go", ev.Data["file_path"])
	}
}

// ============================================================
// Non-JSON line (plain text)
// ============================================================

func TestParseJSONLine_NonJSON(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine("plain text output")

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "plain text output" {
		t.Errorf("Content = %q, want 'plain text output'", ev.Content)
	}
}

func TestParseJSONLine_EmptyJSON(t *testing.T) {
	a := newTestAgent()
	// Empty JSON object — unknown type with no content → nothing emitted
	a.parseJSONLine(`{}`)

	_, ok := readEvent(t, a.events)
	if ok {
		// An event was emitted — acceptable only if it is a stream with empty content.
		// The default case emits nothing when content == "".
		// If something is emitted here it indicates a behavior change; fail to catch regressions.
		t.Error("expected no event for {} (unknown type, no content)")
	}
}

// ============================================================
// Unknown type with content → EventStream
// ============================================================

func TestParseJSONLine_UnknownTypeWithContent(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"unknown_future_type","content":"some data"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event for unknown type with content, got none")
	}
	if ev.Type != agent.EventStream {
		t.Errorf("Type = %q, want EventStream", ev.Type)
	}
	if ev.Content != "some data" {
		t.Errorf("Content = %q, want 'some data'", ev.Content)
	}
}

// ============================================================
// Unknown type with no content → nothing emitted
// ============================================================

func TestParseJSONLine_UnknownTypeNoContent(t *testing.T) {
	a := newTestAgent()
	a.parseJSONLine(`{"type":"no_content_type"}`)

	_, ok := readEvent(t, a.events)
	if ok {
		t.Error("expected no event for unknown type without content")
	}
}

// ============================================================
// Timestamp is set
// ============================================================

func TestParseJSONLine_TimestampSet(t *testing.T) {
	before := time.Now()
	a := newTestAgent()
	a.parseJSONLine(`{"type":"stream","content":"ts check"}`)

	ev, ok := readEvent(t, a.events)
	if !ok {
		t.Fatal("expected event, got none")
	}
	after := time.Now()

	if ev.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if ev.Timestamp.Before(before) || ev.Timestamp.After(after) {
		t.Errorf("Timestamp %v is outside range [%v, %v]", ev.Timestamp, before, after)
	}
}
