package replay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/recorder"
)

func writeTestRecording(t *testing.T, dir string, records []recorder.Record) string {
	t.Helper()

	path := filepath.Join(dir, "test_replay_001.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	// Write header first.
	header := recorder.Header{JobID: "test-job", Agent: "test-agent"}
	if err := enc.Encode(header); err != nil {
		t.Fatal(err)
	}

	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}

	return path
}

func TestReplayBasic(t *testing.T) {
	dir := t.TempDir()

	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`{"content":"hello"}`)},
		{Direction: recorder.Outbound, Type: "assistant", Event: json.RawMessage(`{"content":"hello world"}`)},
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}

	path := writeTestRecording(t, dir, records)

	ra, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if ra.Name() != "replay" {
		t.Errorf("Name() = %q, want %q", ra.Name(), "replay")
	}

	if err := ra.Available(); err != nil {
		t.Errorf("Available() error: %v", err)
	}

	if err := ra.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	if !ra.Connected() {
		t.Error("Connected() = false after Connect()")
	}

	events, err := ra.SendPrompt(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("SendPrompt() error: %v", err)
	}

	var types []agent.EventType
	for evt := range events {
		types = append(types, evt.Type)
	}

	// Expect: init, stream, assistant, complete
	if len(types) != 4 {
		t.Fatalf("got %d events, want 4: %v", len(types), types)
	}
	if types[0] != agent.EventInit {
		t.Errorf("types[0] = %q, want %q", types[0], agent.EventInit)
	}
	if types[1] != agent.EventStream {
		t.Errorf("types[1] = %q, want %q", types[1], agent.EventStream)
	}
	if types[2] != agent.EventAssistant {
		t.Errorf("types[2] = %q, want %q", types[2], agent.EventAssistant)
	}
	if types[3] != agent.EventComplete {
		t.Errorf("types[3] = %q, want %q", types[3], agent.EventComplete)
	}
}

func TestReplaySkipsInbound(t *testing.T) {
	dir := t.TempDir()

	records := []recorder.Record{
		{Direction: recorder.Inbound, Type: "prompt", Event: json.RawMessage(`{"content":"ignored"}`)},
		{Direction: recorder.Outbound, Type: "stream", Event: json.RawMessage(`{"content":"output"}`)},
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}

	path := writeTestRecording(t, dir, records)
	ra, _ := New(path)
	_ = ra.Connect(context.Background())

	events, _ := ra.SendPrompt(context.Background(), "test")

	var streamCount int
	for evt := range events {
		if evt.Type == agent.EventStream {
			streamCount++
		}
	}

	if streamCount != 1 {
		t.Errorf("got %d stream events, want 1 (inbound should be skipped)", streamCount)
	}
}

func TestReplayNotConnected(t *testing.T) {
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}
	path := writeTestRecording(t, dir, records)
	ra, _ := New(path)

	_, err := ra.SendPrompt(context.Background(), "test")
	if err == nil {
		t.Error("SendPrompt() should fail when not connected")
	}
}

func TestReplayBuilderPattern(t *testing.T) {
	dir := t.TempDir()
	records := []recorder.Record{
		{Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}
	path := writeTestRecording(t, dir, records)
	ra, _ := New(path)

	// Builder methods should return the same agent (no-op for replay).
	if ra.WithEnv("K", "V") != ra {
		t.Error("WithEnv should return same agent")
	}
	if ra.WithArgs("a") != ra {
		t.Error("WithArgs should return same agent")
	}
	if ra.WithWorkDir("/tmp") != ra {
		t.Error("WithWorkDir should return same agent")
	}
	if ra.WithTimeout(time.Second) != ra {
		t.Error("WithTimeout should return same agent")
	}
}
