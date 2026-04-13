// Package replay implements an agent that replays recorded sessions for deterministic testing.
package replay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/recorder"
)

// Agent replays a recorded agent session deterministically.
type Agent struct {
	path      string
	records   []recorder.Record
	header    recorder.Header
	connected atomic.Bool
}

// New creates a replay agent from a recording file path.
func New(path string) (*Agent, error) {
	records, err := recorder.ReadAll(path)
	if err != nil {
		return nil, fmt.Errorf("read recording: %w", err)
	}

	// Best-effort header extraction — first record may not be a valid header,
	// so unmarshal errors are intentionally ignored.
	var header recorder.Header
	if len(records) > 0 {
		if err := json.Unmarshal(records[0].Event, &header); err != nil {
			slog.Debug("replay: unmarshal failed", "type", "header", "error", err)
		}
	}

	return &Agent{
		path:    path,
		records: records,
		header:  header,
	}, nil
}

// Name returns "replay".
func (a *Agent) Name() string {
	return "replay"
}

// Available always returns nil (replays don't need external services).
func (a *Agent) Available() error {
	return nil
}

// Connect marks the agent as connected.
func (a *Agent) Connect(_ context.Context) error {
	a.connected.Store(true)

	return nil
}

// Connected returns true if connected.
func (a *Agent) Connected() bool {
	return a.connected.Load()
}

// SendPrompt replays outbound events from the recording.
func (a *Agent) SendPrompt(_ context.Context, _ string) (<-chan agent.Event, error) {
	if !a.connected.Load() {
		return nil, errors.New("replay: not connected")
	}

	events := make(chan agent.Event, 32)

	go func() {
		defer close(events)

		// Emit init event.
		events <- agent.Event{
			Type:      agent.EventInit,
			Timestamp: time.Now(),
			Data:      map[string]any{"agent": "replay", "recording": a.path},
		}

		// Replay outbound events.
		for _, rec := range a.records {
			if rec.Direction != recorder.Outbound {
				continue
			}

			evt := recordToEvent(rec)
			events <- evt

			// Stop after terminal events.
			if evt.Type == agent.EventComplete || evt.Type == agent.EventError {
				return
			}
		}

		// If no terminal event was found, emit complete.
		events <- agent.Event{
			Type:      agent.EventComplete,
			Timestamp: time.Now(),
		}
	}()

	return events, nil
}

// HandlePermission is a no-op for replay agents.
func (a *Agent) HandlePermission(_ string, _ bool) error {
	return nil
}

// Interrupt is a no-op for replay agents.
func (a *Agent) Interrupt() error {
	return nil
}

// Close disconnects the agent.
func (a *Agent) Close() error {
	a.connected.Store(false)

	return nil
}

// WithEnv returns the same agent (replays ignore env vars).
func (a *Agent) WithEnv(_, _ string) agent.Agent { return a }

// WithArgs returns the same agent (replays ignore args).
func (a *Agent) WithArgs(_ ...string) agent.Agent { return a }

// WithWorkDir returns the same agent (replays ignore work dir).
func (a *Agent) WithWorkDir(_ string) agent.Agent { return a }

// WithTimeout returns the same agent (replays ignore timeout).
func (a *Agent) WithTimeout(_ time.Duration) agent.Agent { return a }

// recordToEvent converts a recorder.Record to an agent.Event.
func recordToEvent(rec recorder.Record) agent.Event {
	evt := agent.Event{
		Timestamp: rec.Timestamp,
	}

	switch rec.Type {
	case "stream":
		evt.Type = agent.EventStream
		var content struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(rec.Event, &content); err != nil {
			slog.Debug("replay: unmarshal failed", "type", rec.Type, "error", err)
		}
		evt.Content = content.Content

	case "assistant":
		evt.Type = agent.EventAssistant
		var content struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(rec.Event, &content); err != nil {
			slog.Debug("replay: unmarshal failed", "type", rec.Type, "error", err)
		}
		evt.Content = content.Content

	case "tool_use":
		evt.Type = agent.EventToolUse
		var data map[string]any
		if err := json.Unmarshal(rec.Event, &data); err != nil {
			slog.Debug("replay: unmarshal failed", "type", rec.Type, "error", err)
		}
		evt.Data = data

	case "tool_result":
		evt.Type = agent.EventToolResult
		var data map[string]any
		if err := json.Unmarshal(rec.Event, &data); err != nil {
			slog.Debug("replay: unmarshal failed", "type", rec.Type, "error", err)
		}
		evt.Data = data

	case "complete":
		evt.Type = agent.EventComplete

	case "error":
		evt.Type = agent.EventError
		var content struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rec.Event, &content); err != nil {
			slog.Debug("replay: unmarshal failed", "type", rec.Type, "error", err)
		}
		evt.Error = content.Error

	default:
		// Unknown types become stream events with the raw content.
		evt.Type = agent.EventStream
		evt.Content = string(rec.Event)
	}

	return evt
}
