package recorder

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildScratchpad(t *testing.T) {
	now := time.Now()

	records := []Record{
		{Timestamp: now, Direction: Inbound, Type: "prompt", Event: json.RawMessage(`"plan the feature"`)},
		{Timestamp: now.Add(1 * time.Second), Direction: Outbound, Type: "assistant", Event: json.RawMessage(`{"content": "Let me analyze the requirements"}`)},
		{Timestamp: now.Add(2 * time.Second), Direction: Outbound, Type: "tool_use", Event: json.RawMessage(`{"name": "ReadFile", "input": "src/main.go"}`)},
		{Timestamp: now.Add(3 * time.Second), Direction: Outbound, Type: "tool_result", Event: json.RawMessage(`{"content": "package main\nfunc main() {}"}`)},
		{Timestamp: now.Add(4 * time.Second), Direction: Outbound, Type: "complete", Event: json.RawMessage(`{"content": "Analysis complete"}`)},
	}

	pad := BuildScratchpad("job-1", "plan", records)

	if pad.JobID != "job-1" {
		t.Errorf("expected job-1, got %s", pad.JobID)
	}
	if pad.Phase != "plan" {
		t.Errorf("expected plan, got %s", pad.Phase)
	}
	if len(pad.Thoughts) != 4 {
		t.Fatalf("expected 4 thoughts (skipping inbound), got %d", len(pad.Thoughts))
	}

	// Verify thought types.
	expectedTypes := []string{"reasoning", "tool_call", "observation", "decision"}
	for i, want := range expectedTypes {
		if pad.Thoughts[i].Type != want {
			t.Errorf("thought %d: expected type %q, got %q", i, want, pad.Thoughts[i].Type)
		}
	}

	// Verify tool call.
	if pad.Thoughts[1].ToolName != "ReadFile" {
		t.Errorf("expected tool name ReadFile, got %q", pad.Thoughts[1].ToolName)
	}

	// Verify sequences are sequential.
	for i, th := range pad.Thoughts {
		if th.Sequence != i+1 {
			t.Errorf("thought %d: expected sequence %d, got %d", i, i+1, th.Sequence)
		}
	}
}

func TestBuildScratchpadEmptyRecords(t *testing.T) {
	pad := BuildScratchpad("job-2", "implement", nil)
	if len(pad.Thoughts) != 0 {
		t.Errorf("expected 0 thoughts, got %d", len(pad.Thoughts))
	}
}

func TestBuildScratchpadSkipsInbound(t *testing.T) {
	records := []Record{
		{Direction: Inbound, Type: "prompt", Event: json.RawMessage(`"hello"`)},
	}
	pad := BuildScratchpad("job-3", "plan", records)
	if len(pad.Thoughts) != 0 {
		t.Errorf("expected 0 thoughts (inbound only), got %d", len(pad.Thoughts))
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"string", json.RawMessage(`"hello"`), "hello"},
		{"object with content", json.RawMessage(`{"content": "world"}`), "world"},
		{"object with message", json.RawMessage(`{"message": "info"}`), "info"},
		{"empty", json.RawMessage(``), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContent(tt.raw)
			if got != tt.want {
				t.Errorf("extractContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
