package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent"
)

// trySendEvent sends an event to the events channel.
// Terminal events (EventComplete, EventError) use a blocking send with timeout
// to guarantee delivery — dropping them would hang SendPrompt goroutines.
// Non-terminal events use a non-blocking send and are dropped if the channel is full.
func (w *WebSocketConnection) trySendEvent(event agent.Event) {
	if w.closed.Load() {
		return
	}
	if event.Type == agent.EventComplete || event.Type == agent.EventError {
		select {
		case w.events <- event:
		case <-w.done:
		case <-time.After(30 * time.Second):
			slog.Error("claude websocket: terminal event delivery timed out", "type", string(event.Type))
		}

		return
	}
	select {
	case w.events <- event:
	case <-w.done:
	default:
		slog.Warn("claude websocket: event dropped, channel full", "type", string(event.Type))
	}
}

// trySendOutgoing sends a message on the outgoing channel.
// Control responses (permission decisions) use a blocking send with timeout
// because dropping them would leave Claude CLI waiting indefinitely.
// Other messages use a non-blocking send.
func (w *WebSocketConnection) trySendOutgoing(msg outgoingMessage) error {
	if msg.Type == "control_response" {
		select {
		case w.outgoing <- msg:
			return nil
		case <-w.lifecycleCtx.Done():
			return errors.New("connection closed")
		case <-time.After(30 * time.Second):
			slog.Error("claude websocket: control_response delivery timed out")

			return errors.New("control response delivery timed out")
		}
	}
	select {
	case w.outgoing <- msg:
		return nil
	case <-w.lifecycleCtx.Done():
		return errors.New("connection closed")
	default:
		slog.Warn("claude websocket: outgoing channel full, dropping message", "type", msg.Type)

		return errors.New("outgoing channel full")
	}
}

// sendLoop sends outgoing messages to Claude CLI.
func (w *WebSocketConnection) sendLoop(ctx context.Context) {
	for {
		select {
		case msg := <-w.outgoing:
			w.connMu.Lock()
			if w.conn != nil {
				data, err := json.Marshal(msg)
				if err == nil {
					slog.Debug("claude websocket: sending message", "type", msg.Type, "len", len(data), "data", string(data))
					data = append(data, '\n')
					_ = w.conn.Write(ctx, websocket.MessageText, data)
				}
			}
			w.connMu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// handleIncomingMessage processes messages from Claude CLI.
func (w *WebSocketConnection) handleIncomingMessage(msg incomingMessage) {
	switch msg.Type {
	case "system/init", "system":
		// Handle both old "system/init" and new "system" message formats
		if msg.SessionID != "" && w.sessionID == "" {
			slog.Info("claude websocket: session initialized", "session_id", msg.SessionID, "type", msg.Type)
			w.sessionID = msg.SessionID
			w.connected.Store(true)
			// Signal that session is ready
			w.sessionOnce.Do(func() {
				close(w.sessionReady)
			})
			w.trySendEvent(agent.Event{
				Type:      agent.EventInit,
				Content:   "Session initialized: " + w.sessionID,
				Timestamp: time.Now(),
			})
		}

	case "stream_event":
		content := msg.Content
		if content == "" {
			content = msg.Delta
		}
		w.trySendEvent(agent.Event{
			Type:      agent.EventStream,
			Content:   content,
			Timestamp: time.Now(),
		})

	case "assistant":
		if msg.Message != nil {
			content := extractTextContent(msg.Message.Content)
			if content != "" {
				w.trySendEvent(agent.Event{
					Type:      agent.EventAssistant,
					Content:   content,
					Timestamp: time.Now(),
				})
			}
		}

	case "control_request":
		if msg.Request != nil && msg.RequestID != "" {
			var input map[string]any
			_ = json.Unmarshal(msg.Request.Input, &input)

			// Store pending request for later response
			w.pendingRequestsMu.Lock()
			w.pendingRequests[msg.RequestID] = pendingRequest{
				Input:     input,
				ToolUseID: msg.Request.ToolUseID,
			}
			w.pendingRequestsMu.Unlock()

			req := agent.PermissionRequest{
				ID:     msg.RequestID,
				Tool:   msg.Request.ToolName,
				Input:  input,
				Action: msg.Request.Subtype,
			}
			slog.Info("claude websocket: control_request received", "id", req.ID, "tool", req.Tool)

			// Auto-handle with permission handler
			if w.config.PermissionHandler != nil {
				approved := w.config.PermissionHandler(req)
				_ = w.HandlePermission(req.ID, approved)
			} else {
				// Send event for external handling
				w.trySendEvent(agent.Event{
					Type:              agent.EventPermission,
					PermissionRequest: &req,
					Timestamp:         time.Now(),
				})
			}
		}

	case "result":
		// Result uses subtype:"success" for success, or is_error:true for errors
		if msg.Subtype == "success" && !msg.IsError {
			w.trySendEvent(agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
			})
		} else {
			w.trySendEvent(agent.Event{
				Type:      agent.EventError,
				Error:     msg.Error,
				Timestamp: time.Now(),
			})
		}

	case "keep_alive":
		// Heartbeat - no action needed

	case "tool_use":
		var input map[string]any
		if msg.Input != nil {
			_ = json.Unmarshal(msg.Input, &input)
		}

		toolCallID := msg.ID
		if toolCallID != "" {
			w.subagents.OnToolUse(toolCallID, msg.Name, input)
		}

		w.trySendEvent(agent.Event{
			Type:      agent.EventToolUse,
			Content:   msg.Name,
			Data:      input,
			Timestamp: time.Now(),
		})

	case "tool_result":
		toolCallID := msg.ToolUseID
		if toolCallID != "" {
			w.subagents.OnToolResult(toolCallID, !msg.IsError, msg.Error)
		}

		w.trySendEvent(agent.Event{
			Type:      agent.EventToolResult,
			Content:   msg.Result,
			Timestamp: time.Now(),
		})

	case "tool_progress":
		// Tool execution heartbeat - shows elapsed time for long-running tools
		w.trySendEvent(agent.Event{
			Type:      agent.EventToolProgress,
			Timestamp: time.Now(),
			Data: map[string]any{
				"tool_use_id":     msg.ToolUseID,
				"tool_name":       msg.ToolName,
				"elapsed_seconds": msg.ElapsedTimeSeconds,
			},
		})

	case "streamlined_text":
		// Simplified text output mode (Claude v2.1.81+)
		content := msg.Content
		if content == "" {
			content = msg.Delta
		}
		if content != "" {
			w.trySendEvent(agent.Event{
				Type:      agent.EventStream,
				Content:   content,
				Timestamp: time.Now(),
			})
		}

	case "streamlined_tool_use_summary":
		// Compact tool usage summary (e.g., "Read 2 files, wrote 1 file")
		w.trySendEvent(agent.Event{
			Type:      agent.EventToolUse,
			Content:   msg.Content,
			Timestamp: time.Now(),
		})

	case "prompt_suggestion":
		w.trySendEvent(agent.Event{
			Type:      agent.EventPromptSuggestion,
			Timestamp: time.Now(),
			Data: map[string]any{
				"suggestions": msg.Suggestions,
			},
		})

	default:
		slog.Debug("claude websocket: unhandled message type", "type", msg.Type)
	}
}

// extractTextContent extracts text content from Claude's message content field.
// Content can be either a simple string or an array of content blocks.
func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}

	// Try array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var texts []string
		for _, block := range blocks {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}

		return strings.Join(texts, "\n")
	}

	return ""
}

// SendPrompt sends a user prompt and returns the event stream.
func (w *WebSocketConnection) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	if !w.promptInFlight.CompareAndSwap(false, true) {
		return nil, errors.New("another prompt is already in flight")
	}

	if w.sessionID == "" {
		w.promptInFlight.Store(false)

		return nil, errors.New("not connected (no session)")
	}

	if err := w.trySendOutgoing(outgoingMessage{
		Type:      "user",
		SessionID: w.sessionID,
		Message: &struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			Role:    "user",
			Content: prompt,
		},
	}); err != nil {
		w.promptInFlight.Store(false)

		return nil, fmt.Errorf("send prompt: %w", err)
	}

	// Return filtered event stream
	filtered := make(chan agent.Event, 100)
	go func() {
		defer close(filtered)
		defer w.promptInFlight.Store(false)

		// drainStale discards buffered events so the next SendPrompt starts clean.
		drainStale := func() {
			for {
				select {
				case _, ok := <-w.events:
					if !ok {
						return
					}
				default:
					return
				}
			}
		}

		for {
			select {
			case event, ok := <-w.events:
				if !ok {
					return
				}
				select {
				case filtered <- event:
				case <-ctx.Done():
					drainStale()

					return
				}
				if event.Type == agent.EventComplete || event.Type == agent.EventError {
					return
				}
			case <-ctx.Done():
				drainStale()

				return
			}
		}
	}()

	return filtered, nil
}

// HandlePermission sends a permission response.
func (w *WebSocketConnection) HandlePermission(requestID string, approved bool) error {
	// Get the stored request info
	w.pendingRequestsMu.Lock()
	pending, ok := w.pendingRequests[requestID]
	if ok {
		delete(w.pendingRequests, requestID)
	}
	w.pendingRequestsMu.Unlock()

	var inner *controlResponseInner
	if approved {
		inner = &controlResponseInner{
			Behavior:     "allow",
			UpdatedInput: pending.Input, // Pass through original input
			ToolUseID:    pending.ToolUseID,
		}
	} else {
		inner = &controlResponseInner{
			Behavior:  "deny",
			Message:   "Permission denied by kvelmo",
			Interrupt: false,
			ToolUseID: pending.ToolUseID,
		}
	}

	slog.Debug("claude websocket: sending control_response", "request_id", requestID, "behavior", inner.Behavior)

	return w.trySendOutgoing(outgoingMessage{
		Type: "control_response",
		Response: &controlResponsePayload{
			Subtype:   "success",
			RequestID: requestID,
			Response:  inner,
		},
	})
}

// Interrupt sends an interrupt control request to abort the current agent turn.
func (w *WebSocketConnection) Interrupt() error {
	if !w.connected.Load() {
		return nil // Not connected, nothing to interrupt
	}

	requestID := uuid.NewString()
	slog.Debug("claude websocket: sending interrupt", "request_id", requestID)

	if err := w.trySendOutgoing(outgoingMessage{
		Type:      "control_request",
		RequestID: requestID,
		Request: &controlRequestPayload{
			Subtype: "interrupt",
		},
	}); err != nil {
		return fmt.Errorf("send interrupt: %w", err)
	}

	// Emit interrupted event
	w.trySendEvent(agent.Event{
		Type:      agent.EventInterrupted,
		Content:   "Agent turn interrupted",
		Timestamp: time.Now(),
	})

	return nil
}
