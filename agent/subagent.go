package agent

import (
	"log/slog"
	"sync"
	"time"

	"github.com/valksor/kvelmo/metrics"
)

// SubagentTracker tracks active subagents and generates lifecycle events.
type SubagentTracker struct {
	mu       sync.Mutex
	pending  map[string]*pendingSubagent
	eventsCh chan Event
	doneCh   <-chan struct{} // Closed when the parent connection shuts down
}

type pendingSubagent struct {
	ID          string
	Type        string
	Description string
	StartedAt   time.Time
}

// NewSubagentTracker creates a new tracker that sends events to the given channel.
// Call SetDoneChannel after the connection's done channel is available.
func NewSubagentTracker(eventsCh chan Event) *SubagentTracker {
	return &SubagentTracker{
		pending:  make(map[string]*pendingSubagent),
		eventsCh: eventsCh,
	}
}

// OnToolUse processes a tool_use event and detects subagent spawns.
// Returns true if this was a Task tool call (subagent spawn).
func (t *SubagentTracker) OnToolUse(toolCallID, toolName string, input map[string]any) bool {
	if toolName != "Task" {
		return false
	}

	// Extract subagent info from input
	subagentType, _ := input["subagent_type"].(string)
	if subagentType == "" {
		subagentType = "unknown"
	}

	description, _ := input["description"].(string)
	if description == "" {
		description, _ = input["prompt"].(string)
		runes := []rune(description)
		if len(runes) > 50 {
			description = string(runes[:47]) + "..."
		}
	}

	now := time.Now()

	t.mu.Lock()
	t.pending[toolCallID] = &pendingSubagent{
		ID:          toolCallID,
		Type:        subagentType,
		Description: description,
		StartedAt:   now,
	}
	evCh := t.eventsCh // Capture under lock to avoid race with SetEventChannel
	doneCh := t.doneCh
	t.mu.Unlock()

	// Emit started event (non-blocking to avoid deadlock if channel is full)
	trySendEventTo(evCh, doneCh, Event{
		Type:      EventSubagent,
		Timestamp: now,
		Subagent: &SubagentEvent{
			ID:          toolCallID,
			Type:        subagentType,
			Description: description,
			Status:      SubagentStarted,
			StartedAt:   now,
		},
	})

	return true
}

// OnToolResult processes a tool_result event and detects subagent completion.
// Returns true if this was a Task tool result (subagent completion).
func (t *SubagentTracker) OnToolResult(toolCallID string, success bool, errorMsg string) bool {
	t.mu.Lock()
	pending, ok := t.pending[toolCallID]
	if ok {
		delete(t.pending, toolCallID)
	}
	evCh := t.eventsCh // Capture under lock to avoid race with SetEventChannel
	doneCh := t.doneCh
	t.mu.Unlock()

	if !ok {
		return false
	}

	now := time.Now()
	duration := now.Sub(pending.StartedAt)

	status := SubagentCompleted
	exitReason := ""
	if !success {
		status = SubagentFailed
		exitReason = errorMsg
	}

	// Emit completion event (blocking with timeout to guarantee delivery)
	trySendEventTo(evCh, doneCh, Event{
		Type:      EventSubagent,
		Timestamp: now,
		Subagent: &SubagentEvent{
			ID:          pending.ID,
			Type:        pending.Type,
			Description: pending.Description,
			Status:      status,
			StartedAt:   pending.StartedAt,
			CompletedAt: now,
			Duration:    duration.Milliseconds(),
			ExitReason:  exitReason,
		},
	})

	return true
}

// trySendEventTo sends an event to the given channel.
// Completion events use a blocking send with timeout to guarantee delivery
// (matching trySendEvent policy in websocket.go). Started events use a
// non-blocking send and are dropped if the channel is full.
//
//nolint:unparam // Return value reserved for future logging/metrics
func trySendEventTo(ch chan Event, done <-chan struct{}, event Event) bool {
	isCompletion := event.Subagent != nil &&
		(event.Subagent.Status == SubagentCompleted || event.Subagent.Status == SubagentFailed)

	if isCompletion {
		select {
		case ch <- event:
			return true
		case <-done:
			return false
		case <-time.After(30 * time.Second):
			slog.Error("subagent: completion event delivery timed out",
				"subagent_id", event.Subagent.ID,
				"status", string(event.Subagent.Status))

			return false
		}
	}

	select {
	case ch <- event:
		return true
	case <-done:
		return false
	default:
		metrics.Global().RecordEventDropped()

		return false
	}
}

// ActiveCount returns the number of currently running subagents.
func (t *SubagentTracker) ActiveCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.pending)
}

// Clear removes all pending subagents (e.g., on disconnect).
func (t *SubagentTracker) Clear() {
	t.mu.Lock()
	t.pending = make(map[string]*pendingSubagent)
	t.mu.Unlock()
}

// SetEventChannel updates the event channel.
// Call this when the event channel is replaced (e.g., in SendPrompt).
func (t *SubagentTracker) SetEventChannel(ch chan Event) {
	t.mu.Lock()
	t.eventsCh = ch
	t.mu.Unlock()
}

// SetDoneChannel updates the done channel used for backpressure timeouts.
func (t *SubagentTracker) SetDoneChannel(done <-chan struct{}) {
	t.mu.Lock()
	t.doneCh = done
	t.mu.Unlock()
}
