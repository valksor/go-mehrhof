package apiagent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/agent/apiagent"
)

func TestParseSSE(t *testing.T) {
	input := "event: message\ndata: hello world\n\nevent: done\ndata: goodbye\n\n"
	reader := strings.NewReader(input)

	events := apiagent.ParseSSE(context.Background(), reader)

	var collected []apiagent.SSEEvent
	for e := range events {
		collected = append(collected, e)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 events, got %d", len(collected))
	}

	if collected[0].Event != "message" || collected[0].Data != "hello world" {
		t.Errorf("event 0: got event=%q data=%q", collected[0].Event, collected[0].Data)
	}

	if collected[1].Event != "done" || collected[1].Data != "goodbye" {
		t.Errorf("event 1: got event=%q data=%q", collected[1].Event, collected[1].Data)
	}
}

func TestParseSSEComments(t *testing.T) {
	input := ": this is a comment\ndata: actual data\n\n"
	events := apiagent.ParseSSE(context.Background(), strings.NewReader(input))

	var collected []apiagent.SSEEvent
	for e := range events {
		collected = append(collected, e)
	}

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	if collected[0].Data != "actual data" {
		t.Errorf("expected 'actual data', got %q", collected[0].Data)
	}
}

func TestParseSSEMultilineData(t *testing.T) {
	input := "data: line one\ndata: line two\n\n"
	events := apiagent.ParseSSE(context.Background(), strings.NewReader(input))

	var collected []apiagent.SSEEvent
	for e := range events {
		collected = append(collected, e)
	}

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	if collected[0].Data != "line one\nline two" {
		t.Errorf("expected multiline data, got %q", collected[0].Data)
	}
}

func TestParseSSEContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Use a reader that would block forever
	events := apiagent.ParseSSE(ctx, strings.NewReader("data: test\n\n"))

	// Should drain without blocking
	for range events {
		// Consume all events
	}
}

func TestParseNDJSON(t *testing.T) {
	input := `{"type":"text","content":"hello"}
{"type":"done"}
`
	reader := strings.NewReader(input)

	messages := apiagent.ParseNDJSON(context.Background(), reader)

	var collected []json.RawMessage
	for m := range messages {
		collected = append(collected, m)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(collected))
	}

	var first map[string]string
	if err := json.Unmarshal(collected[0], &first); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}

	if first["type"] != "text" {
		t.Errorf("expected type=text, got %q", first["type"])
	}
}

func TestParseNDJSONSkipsBlankLines(t *testing.T) {
	input := `{"a":1}

{"b":2}
`
	messages := apiagent.ParseNDJSON(context.Background(), strings.NewReader(input))

	var collected []json.RawMessage
	for m := range messages {
		collected = append(collected, m)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(collected))
	}
}
