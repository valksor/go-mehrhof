package codex

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent"
)

// handleNotification processes incoming JSON-RPC notifications.
func (c *CLIConnection) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "item/agentMessage/delta":
		// Streaming text delta
		var delta struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params, &delta); err == nil && delta.Text != "" {
			c.emitEvent(agent.Event{
				Type:      agent.EventStream,
				Content:   delta.Text,
				Timestamp: time.Now(),
			})
		}

	case "item/started":
		// Item started (command execution, file change, etc.)
		var item struct {
			ItemID string `json:"itemId"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(params, &item); err == nil {
			switch item.Type {
			case "commandExecution":
				c.emitEvent(agent.Event{
					Type:      agent.EventToolUse,
					Content:   "Bash",
					Timestamp: time.Now(),
				})
			case "fileChange":
				c.emitEvent(agent.Event{
					Type:      agent.EventToolUse,
					Content:   "Edit",
					Timestamp: time.Now(),
				})
			}
		}

	case "item/completed":
		// Item completed
		var item struct {
			ItemID string `json:"itemId"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(params, &item); err == nil {
			c.emitEvent(agent.Event{
				Type:      agent.EventToolResult,
				Content:   item.Type + " completed",
				Timestamp: time.Now(),
			})
		}

	case "turn/completed":
		// Turn completed - signal completion
		c.turnActive.Store(false)
		c.emitEvent(agent.Event{
			Type:      agent.EventComplete,
			Timestamp: time.Now(),
		})

	case "turn/failed":
		// Turn failed
		c.turnActive.Store(false)
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(params, &failure)
		c.emitEvent(agent.Event{
			Type:      agent.EventError,
			Error:     failure.Error,
			Timestamp: time.Now(),
		})
	}
}

// handleRequest processes incoming JSON-RPC requests (need response).
func (c *CLIConnection) handleRequest(method string, id int64, params json.RawMessage) {
	switch method {
	case "item/commandExecution/requestApproval":
		c.handleCommandApproval(id, params)
	case "item/fileChange/requestApproval":
		c.handleFileChangeApproval(id, params)
	case "item/mcpToolCall/requestApproval":
		c.handleMcpApproval(id, params)
	default:
		// Unknown request - auto-approve
		_ = c.transport.Respond(id, map[string]any{"decision": "accept"})
	}
}

func (c *CLIConnection) handleCommandApproval(id int64, params json.RawMessage) {
	var req struct {
		ItemID  string   `json:"itemId"`
		Command []string `json:"command"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		slog.Warn("rejecting malformed command approval request", "error", err)
		_ = c.transport.Respond(id, map[string]any{"decision": "reject"})

		return
	}

	// Create permission request
	requestID := uuid.NewString()
	c.pendingApprovalsMu.Lock()
	c.pendingApprovals[requestID] = id
	c.pendingApprovalsMu.Unlock()

	command := ""
	if len(req.Command) > 0 {
		command = req.Command[0]
	}

	// Check with permission handler
	permReq := agent.PermissionRequest{
		ID:    requestID,
		Tool:  "Bash",
		Input: map[string]any{"command": command},
	}

	if c.config.PermissionHandler != nil {
		approved := c.config.PermissionHandler(permReq)
		_ = c.HandlePermission(requestID, approved)
	} else {
		// Emit event for external handling
		c.emitEvent(agent.Event{
			Type:              agent.EventPermission,
			PermissionRequest: &permReq,
			Timestamp:         time.Now(),
		})
	}
}

func (c *CLIConnection) handleFileChangeApproval(id int64, params json.RawMessage) {
	var req struct {
		ItemID  string `json:"itemId"`
		Changes []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		slog.Warn("rejecting malformed file change approval request", "error", err)
		_ = c.transport.Respond(id, map[string]any{"decision": "reject"})

		return
	}

	// Create permission request
	requestID := uuid.NewString()
	c.pendingApprovalsMu.Lock()
	c.pendingApprovals[requestID] = id
	c.pendingApprovalsMu.Unlock()

	paths := make([]string, len(req.Changes))
	for i, ch := range req.Changes {
		paths[i] = ch.Path
	}

	permReq := agent.PermissionRequest{
		ID:    requestID,
		Tool:  "Edit",
		Input: map[string]any{"paths": paths},
	}

	if c.config.PermissionHandler != nil {
		approved := c.config.PermissionHandler(permReq)
		_ = c.HandlePermission(requestID, approved)
	} else {
		c.emitEvent(agent.Event{
			Type:              agent.EventPermission,
			PermissionRequest: &permReq,
			Timestamp:         time.Now(),
		})
	}
}

func (c *CLIConnection) handleMcpApproval(id int64, _ json.RawMessage) {
	// Auto-approve MCP tool calls
	_ = c.transport.Respond(id, map[string]any{"decision": "accept"})
}

func (c *CLIConnection) emitEvent(event agent.Event) {
	c.eventsMu.Lock()
	ch := c.events
	c.eventsMu.Unlock()

	select {
	case ch <- event:
	default:
		// Channel full, drop event
	}
}

// readErrors reads stderr for error messages.
func (c *CLIConnection) readErrors() {
	buf := make([]byte, 4096)
	for {
		n, err := c.stderr.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			slog.Debug("codex stderr", "output", string(buf[:n]))
		}
	}
}
