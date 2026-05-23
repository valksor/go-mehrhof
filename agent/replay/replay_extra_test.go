package replay

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/recorder"
)

func TestNew_ReadError(t *testing.T) {
	_, err := New("/nonexistent/path/recording.jsonl")
	if err == nil {
		t.Fatal("New() should fail for missing recording file")
	}
}

func TestNew_HeaderExtraction(t *testing.T) {
	// recorder.ReadAll consumes the file header line, so New() parses the FIRST
	// data record's event as the header (best-effort). Encode header-shaped fields
	// in that first record so the extraction observably populates a.header.
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{"job_id":"embedded-job","agent":"embedded-agent"}`)},
	}
	path := writeTestRecording(t, dir, records)

	a, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if a.header.JobID != "embedded-job" {
		t.Errorf("header.JobID = %q, want embedded-job", a.header.JobID)
	}
	if a.header.Agent != "embedded-agent" {
		t.Errorf("header.Agent = %q, want embedded-agent", a.header.Agent)
	}
}

func TestNew_HeaderExtractionMalformed(t *testing.T) {
	// First record event is not header-shaped JSON; unmarshal error is ignored and
	// the header stays zero-value.
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`"just a string"`)},
	}
	path := writeTestRecording(t, dir, records)

	a, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if a.header.JobID != "" {
		t.Errorf("header.JobID = %q, want empty for non-object event", a.header.JobID)
	}
}

func TestHandlePermission_Noop(t *testing.T) {
	a := &Agent{}
	if err := a.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() = %v, want nil", err)
	}
}

func TestInterrupt_Noop(t *testing.T) {
	a := &Agent{}
	if err := a.Interrupt(); err != nil {
		t.Errorf("Interrupt() = %v, want nil", err)
	}
}

func TestClose_DisconnectsAgent(t *testing.T) {
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}
	path := writeTestRecording(t, dir, records)
	a, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_ = a.Connect(context.Background())
	if !a.Connected() {
		t.Fatal("expected connected after Connect")
	}

	if err := a.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	if a.Connected() {
		t.Error("expected disconnected after Close")
	}
}

// TestRecordToEvent_AllTypes drives every branch of recordToEvent directly.
func TestRecordToEvent_AllTypes(t *testing.T) {
	ts := time.Now()

	tests := []struct {
		name        string
		rec         recorder.Record
		wantType    agent.EventType
		wantContent string
		wantError   string
		checkData   func(t *testing.T, data map[string]any)
	}{
		{
			name:        "stream",
			rec:         recorder.Record{Timestamp: ts, Type: "stream", Event: json.RawMessage(`{"content":"streamed text"}`)},
			wantType:    agent.EventStream,
			wantContent: "streamed text",
		},
		{
			name:        "assistant",
			rec:         recorder.Record{Timestamp: ts, Type: "assistant", Event: json.RawMessage(`{"content":"final answer"}`)},
			wantType:    agent.EventAssistant,
			wantContent: "final answer",
		},
		{
			name:     "tool_use",
			rec:      recorder.Record{Timestamp: ts, Type: "tool_use", Event: json.RawMessage(`{"tool":"Read","input":{"path":"x"}}`)},
			wantType: agent.EventToolUse,
			checkData: func(t *testing.T, data map[string]any) {
				t.Helper()
				if data["tool"] != "Read" {
					t.Errorf("data[tool] = %v, want Read", data["tool"])
				}
			},
		},
		{
			name:     "tool_result",
			rec:      recorder.Record{Timestamp: ts, Type: "tool_result", Event: json.RawMessage(`{"is_error":false,"output":"ok"}`)},
			wantType: agent.EventToolResult,
			checkData: func(t *testing.T, data map[string]any) {
				t.Helper()
				if data["output"] != "ok" {
					t.Errorf("data[output] = %v, want ok", data["output"])
				}
			},
		},
		{
			name:     "complete",
			rec:      recorder.Record{Timestamp: ts, Type: "complete", Event: json.RawMessage(`{}`)},
			wantType: agent.EventComplete,
		},
		{
			name:      "error",
			rec:       recorder.Record{Timestamp: ts, Type: "error", Event: json.RawMessage(`{"error":"boom"}`)},
			wantType:  agent.EventError,
			wantError: "boom",
		},
		{
			name:        "unknown type becomes stream",
			rec:         recorder.Record{Timestamp: ts, Type: "mystery", Event: json.RawMessage(`raw bytes here`)},
			wantType:    agent.EventStream,
			wantContent: "raw bytes here",
		},
		{
			name:        "malformed stream json tolerated",
			rec:         recorder.Record{Timestamp: ts, Type: "stream", Event: json.RawMessage(`{not valid json`)},
			wantType:    agent.EventStream,
			wantContent: "",
		},
		{
			name:      "malformed error json tolerated",
			rec:       recorder.Record{Timestamp: ts, Type: "error", Event: json.RawMessage(`{bad`)},
			wantType:  agent.EventError,
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := recordToEvent(tt.rec)

			if evt.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", evt.Type, tt.wantType)
			}
			if tt.wantContent != "" && evt.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", evt.Content, tt.wantContent)
			}
			if tt.wantError != "" && evt.Error != tt.wantError {
				t.Errorf("Error = %q, want %q", evt.Error, tt.wantError)
			}
			if !evt.Timestamp.Equal(ts) {
				t.Errorf("Timestamp = %v, want %v", evt.Timestamp, ts)
			}
			if tt.checkData != nil {
				tt.checkData(t, evt.Data)
			}
		})
	}
}

// TestSendPrompt_StopsOnErrorTerminal verifies the loop stops at an error event
// and does not append a synthetic complete event.
func TestSendPrompt_StopsOnErrorTerminal(t *testing.T) {
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`{"content":"working"}`)},
		{Direction: recorder.Outbound, Type: "error", Event: json.RawMessage(`{"error":"failed"}`)},
		// This trailing record must never be replayed since error is terminal.
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`{"content":"unreachable"}`)},
	}
	path := writeTestRecording(t, dir, records)
	a, _ := New(path)
	_ = a.Connect(context.Background())

	events, _ := a.SendPrompt(context.Background(), "go")

	var types []agent.EventType
	var sawUnreachable bool
	for evt := range events {
		types = append(types, evt.Type)
		if evt.Content == "unreachable" {
			sawUnreachable = true
		}
	}

	if sawUnreachable {
		t.Error("replay continued past terminal error event")
	}
	if types[len(types)-1] != agent.EventError {
		t.Errorf("last event = %q, want error", types[len(types)-1])
	}
}

// TestSendPrompt_SyntheticComplete verifies that when no terminal event exists,
// a complete event is appended.
func TestSendPrompt_SyntheticComplete(t *testing.T) {
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`{"content":"a"}`)},
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`{"content":"b"}`)},
	}
	path := writeTestRecording(t, dir, records)
	a, _ := New(path)
	_ = a.Connect(context.Background())

	events, _ := a.SendPrompt(context.Background(), "go")

	var types []agent.EventType
	for evt := range events {
		types = append(types, evt.Type)
	}

	// Expect init, two stream events, then a synthetic complete.
	if len(types) != 4 {
		t.Fatalf("got %d events, want 4: %v", len(types), types)
	}
	if types[len(types)-1] != agent.EventComplete {
		t.Errorf("last event = %q, want synthetic complete", types[len(types)-1])
	}
}

func TestNew_EmptyRecordsNoHeader(t *testing.T) {
	// A file that exists but only has a header line. recorder.ReadAll returns the
	// records after the header, so records may be empty — header stays zero-value.
	dir := t.TempDir()
	path := writeTestRecording(t, dir, nil)

	a, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if a.path != path {
		t.Errorf("path = %q, want %q", a.path, path)
	}
}

func TestReplay_InitEventCarriesRecordingPath(t *testing.T) {
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}
	path := writeTestRecording(t, dir, records)
	a, _ := New(path)
	_ = a.Connect(context.Background())

	events, _ := a.SendPrompt(context.Background(), "go")

	first := <-events
	if first.Type != agent.EventInit {
		t.Fatalf("first event = %q, want init", first.Type)
	}
	if first.Data["recording"] != path {
		t.Errorf("init recording = %v, want %q", first.Data["recording"], path)
	}
	if first.Data["agent"] != "replay" {
		t.Errorf("init agent = %v, want replay", first.Data["agent"])
	}

	// Drain the rest.
	for range events {
	}

	// Sanity: the recording file is in the temp dir we created.
	if filepath.Dir(path) != dir {
		t.Errorf("recording path %q not under temp dir %q", path, dir)
	}
}
