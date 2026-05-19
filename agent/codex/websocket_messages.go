package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/agent"
)

// handleNotification processes incoming notifications.
func (w *WebSocketConnection) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "item/agentMessage/delta":
		var delta struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params, &delta); err == nil && delta.Text != "" {
			w.emitEvent(agent.Event{
				Type:      agent.EventStream,
				Content:   delta.Text,
				Timestamp: time.Now(),
			})
		}

	case "item/started":
		var item struct {
			ItemID string `json:"itemId"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(params, &item); err == nil {
			toolName := item.Type
			switch item.Type {
			case "commandExecution":
				toolName = "Bash"
			case "fileChange":
				toolName = "Edit"
			}
			w.emitEvent(agent.Event{
				Type:      agent.EventToolUse,
				Content:   toolName,
				Timestamp: time.Now(),
			})
		}

	case "item/completed":
		var item struct {
			ItemID string `json:"itemId"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(params, &item); err == nil {
			w.emitEvent(agent.Event{
				Type:      agent.EventToolResult,
				Content:   item.Type + " completed",
				Timestamp: time.Now(),
			})
		}

	case "turn/completed":
		w.turnActive.Store(false)
		w.emitEvent(agent.Event{
			Type:      agent.EventComplete,
			Timestamp: time.Now(),
		})

	case "turn/failed":
		w.turnActive.Store(false)
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(params, &failure)
		w.emitEvent(agent.Event{
			Type:      agent.EventError,
			Error:     failure.Error,
			Timestamp: time.Now(),
		})
	}
}

// handleRequest processes incoming requests.
func (w *WebSocketConnection) handleRequest(method string, id int64, params json.RawMessage) {
	switch method {
	case "item/commandExecution/requestApproval":
		w.handleCommandApproval(id, params)
	case "item/fileChange/requestApproval":
		w.handleFileChangeApproval(id, params)
	case "item/mcpToolCall/requestApproval":
		_ = w.transport.Respond(context.Background(), id, map[string]any{keyDecision: decisionAccept})
	default:
		_ = w.transport.Respond(context.Background(), id, map[string]any{keyDecision: decisionAccept})
	}
}

func (w *WebSocketConnection) handleCommandApproval(id int64, params json.RawMessage) {
	var req struct {
		ItemID  string   `json:"itemId"`
		Command []string `json:"command"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		slog.Warn("rejecting malformed command approval request", "error", err)
		_ = w.transport.Respond(context.Background(), id, map[string]any{keyDecision: decisionReject})

		return
	}

	requestID := uuid.NewString()
	w.pendingApprovalsMu.Lock()
	w.pendingApprovals[requestID] = id
	w.pendingApprovalsMu.Unlock()

	command := ""
	if len(req.Command) > 0 {
		command = req.Command[0]
	}

	permReq := agent.PermissionRequest{
		ID:    requestID,
		Tool:  "Bash",
		Input: map[string]any{"command": command},
	}

	if w.config.PermissionHandler != nil {
		approved := w.config.PermissionHandler(permReq)
		_ = w.HandlePermission(requestID, approved)
	} else {
		w.emitEvent(agent.Event{
			Type:              agent.EventPermission,
			PermissionRequest: &permReq,
			Timestamp:         time.Now(),
		})
	}
}

func (w *WebSocketConnection) handleFileChangeApproval(id int64, params json.RawMessage) {
	var req struct {
		ItemID  string `json:"itemId"`
		Changes []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		slog.Warn("rejecting malformed file change approval request", "error", err)
		_ = w.transport.Respond(context.Background(), id, map[string]any{keyDecision: decisionReject})

		return
	}

	requestID := uuid.NewString()
	w.pendingApprovalsMu.Lock()
	w.pendingApprovals[requestID] = id
	w.pendingApprovalsMu.Unlock()

	paths := make([]string, len(req.Changes))
	for i, ch := range req.Changes {
		paths[i] = ch.Path
	}

	permReq := agent.PermissionRequest{
		ID:    requestID,
		Tool:  "Edit",
		Input: map[string]any{"paths": paths},
	}

	if w.config.PermissionHandler != nil {
		approved := w.config.PermissionHandler(permReq)
		_ = w.HandlePermission(requestID, approved)
	} else {
		w.emitEvent(agent.Event{
			Type:              agent.EventPermission,
			PermissionRequest: &permReq,
			Timestamp:         time.Now(),
		})
	}
}

func (w *WebSocketConnection) emitEvent(event agent.Event) {
	w.eventsMu.Lock()
	ch := w.events
	w.eventsMu.Unlock()

	select {
	case ch <- event:
	default:
	}
}

// SendPrompt sends a prompt via JSON-RPC.
func (w *WebSocketConnection) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	if !w.connected.Load() {
		return nil, errors.New("not connected")
	}

	if w.threadID == "" {
		return nil, errors.New("no thread started")
	}

	// Create new event channel
	w.eventsMu.Lock()
	w.events = make(chan agent.Event, 100)
	ch := w.events
	w.eventsMu.Unlock()
	w.subagents.SetEventChannel(ch)

	// Start turn
	w.turnActive.Store(true)
	_, err := w.transport.Call(ctx, "turn/start", map[string]any{
		"threadId": w.threadID,
		"message": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	})
	if err != nil {
		w.turnActive.Store(false)

		return nil, fmt.Errorf("turn/start: %w", err)
	}

	// Return filtered channel
	filtered := make(chan agent.Event, 100)
	go func() {
		defer close(filtered)
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				filtered <- event
				if event.Type == agent.EventComplete || event.Type == agent.EventError {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return filtered, nil
}

// HandlePermission responds to a permission request.
func (w *WebSocketConnection) HandlePermission(requestID string, approved bool) error {
	w.pendingApprovalsMu.Lock()
	rpcID, ok := w.pendingApprovals[requestID]
	if ok {
		delete(w.pendingApprovals, requestID)
	}
	w.pendingApprovalsMu.Unlock()

	if !ok {
		return fmt.Errorf("no pending approval for %s", requestID)
	}

	decision := decisionAccept
	if !approved {
		decision = decisionReject
	}

	return w.transport.Respond(context.Background(), rpcID, map[string]any{keyDecision: decision})
}

// Interrupt aborts the current agent turn via turn/interrupt JSON-RPC call.
func (w *WebSocketConnection) Interrupt() error {
	if !w.connected.Load() {
		return nil // Not connected, nothing to interrupt
	}

	if !w.turnActive.Load() {
		return nil // No active turn to interrupt
	}

	if w.threadID == "" {
		return nil // No thread started
	}

	slog.Info("codex websocket: sending turn/interrupt", "threadId", w.threadID)

	// Call turn/interrupt - this is fire-and-forget
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := w.transport.Call(ctx, "turn/interrupt", map[string]any{
		"threadId": w.threadID,
	})
	if err != nil {
		slog.Warn("codex websocket: turn/interrupt failed", "error", err)
		// Don't return error - interrupt is best effort
	}

	w.turnActive.Store(false)

	// Emit interrupted event
	w.emitEvent(agent.Event{
		Type:      agent.EventInterrupted,
		Content:   "Agent turn interrupted",
		Timestamp: time.Now(),
	})

	return nil
}
