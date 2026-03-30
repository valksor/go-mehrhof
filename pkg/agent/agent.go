// Package agent provides an interface and registry for AI agents.
// Based on flow_v2.md design: supports Claude, Codex, and custom agents
// with WebSocket (primary) and CLI (fallback) connection modes.
package agent

import (
	"context"
	"maps"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/agent/permission"
	"github.com/valksor/kvelmo/pkg/metrics"
)

// Agent is the interface for AI agents (Claude, Codex, custom).
//
//nolint:interfacebloat // All methods are required for agent lifecycle management
type Agent interface {
	// Name returns the agent's identifier (e.g., "claude", "codex")
	Name() string

	// Available checks if the agent is available (binary exists, API reachable)
	Available() error

	// Connect establishes connection (WebSocket or process)
	Connect(ctx context.Context) error

	// Connected returns true if the agent is connected
	Connected() bool

	// SendPrompt sends a prompt and returns streaming events
	SendPrompt(ctx context.Context, prompt string) (<-chan Event, error)

	// HandlePermission responds to a permission request
	HandlePermission(requestID string, approved bool) error

	// Interrupt aborts the current agent turn
	// Returns nil if successful or not connected
	Interrupt() error

	// Close closes the connection
	Close() error

	// WithEnv adds environment variable (returns new Agent for chaining)
	WithEnv(key, value string) Agent

	// WithArgs adds CLI arguments (returns new Agent for chaining)
	WithArgs(args ...string) Agent

	// WithWorkDir sets the working directory (returns new Agent for chaining)
	WithWorkDir(dir string) Agent

	// WithTimeout sets the execution timeout (returns new Agent for chaining)
	WithTimeout(d time.Duration) Agent
}

// EventType identifies the type of streaming event.
type EventType string

const (
	EventStream           EventType = "stream"            // Token-by-token output
	EventAssistant        EventType = "assistant"         // Full assistant message
	EventToolUse          EventType = "tool_use"          // Tool call initiated
	EventToolResult       EventType = "tool_result"       // Tool call completed
	EventPermission       EventType = "permission"        // Permission request
	EventComplete         EventType = "complete"          // Job completed successfully
	EventError            EventType = "error"             // Error occurred
	EventInit             EventType = "init"              // Session initialized
	EventKeepAlive        EventType = "keep_alive"        // Heartbeat
	EventSubagent         EventType = "subagent"          // Subagent lifecycle event
	EventProgress         EventType = "progress"          // Partial completion update
	EventToolProgress     EventType = "tool_progress"     // Tool execution heartbeat (elapsed time)
	EventInterrupted      EventType = "interrupted"       // Agent turn was interrupted
	EventPromptSuggestion EventType = "prompt_suggestion" // Predicted follow-up prompts
)

// Event represents a streaming event from an agent.
type Event struct {
	Type      EventType      `json:"type"`
	Content   string         `json:"content,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitzero"`

	// For EventPermission
	PermissionRequest *PermissionRequest `json:"permission_request,omitempty"`

	// For EventError
	Error string `json:"error,omitempty"`

	// For EventSubagent
	Subagent *SubagentEvent `json:"subagent,omitempty"`

	// For EventProgress
	Progress *ProgressInfo `json:"progress,omitempty"`
}

// ProgressInfo reports partial completion of an agent operation.
type ProgressInfo struct {
	Percentage float64 `json:"percentage,omitempty"` // 0-100
	Message    string  `json:"message,omitempty"`
	Phase      string  `json:"phase,omitempty"` // Current sub-phase
}

// SubagentStatus indicates the lifecycle state of a subagent.
type SubagentStatus string

const (
	SubagentStarted   SubagentStatus = "started"
	SubagentCompleted SubagentStatus = "completed"
	SubagentFailed    SubagentStatus = "failed"
)

// SubagentEvent represents a subagent lifecycle event.
type SubagentEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`        // "Explore", "Plan", etc.
	Description string         `json:"description"` // Short description from agent
	Status      SubagentStatus `json:"status"`
	StartedAt   time.Time      `json:"started_at,omitzero"`
	CompletedAt time.Time      `json:"completed_at,omitzero"`
	Duration    int64          `json:"duration,omitempty"`    // milliseconds
	ExitReason  string         `json:"exit_reason,omitempty"` // For failed status
}

// PermissionRequest represents a tool permission request from the agent.
type PermissionRequest struct {
	ID     string         `json:"id"`
	Tool   string         `json:"tool"`
	Input  map[string]any `json:"input,omitempty"`
	Action string         `json:"action,omitempty"`

	// DangerLevel indicates how risky this operation is.
	// Set by danger detection when evaluating the request.
	DangerLevel permission.DangerLevel `json:"danger_level,omitempty"`
	// DangerReason explains why the operation is flagged.
	DangerReason string `json:"danger_reason,omitempty"`
}

// PermissionHandler evaluates permission requests.
// Returns true to approve, false to deny.
type PermissionHandler func(req PermissionRequest) bool

// PermissionResult holds the evaluation result with danger info.
type PermissionResult struct {
	Approved     bool
	DangerLevel  permission.DangerLevel
	DangerReason string
}

// EvaluatePermission evaluates a permission request with danger detection.
// Returns the result including danger level and reason.
func EvaluatePermission(req PermissionRequest) PermissionResult {
	// Check for dangerous operations first
	danger := permission.DetectDanger(req.Tool, req.Input)

	// Dangerous operations are always denied
	if danger.Level == permission.Dangerous {
		metrics.Global().RecordPermissionDenied()

		return PermissionResult{
			Approved:     false,
			DangerLevel:  danger.Level,
			DangerReason: danger.Reason,
		}
	}

	// Check if tool is in safe list (case-insensitive)
	approved := isSafeTool(req.Tool)

	if approved {
		metrics.Global().RecordPermissionApproved()
	} else {
		metrics.Global().RecordPermissionDenied()
	}

	return PermissionResult{
		Approved:     approved,
		DangerLevel:  danger.Level,
		DangerReason: danger.Reason,
	}
}

// EvaluatePermissionWithConfig evaluates a permission request using additional safe tools from config.
func EvaluatePermissionWithConfig(req PermissionRequest, cfg *Config) PermissionResult {
	// Check for dangerous operations first
	danger := permission.DetectDanger(req.Tool, req.Input)
	if danger.Level == permission.Dangerous {
		metrics.Global().RecordPermissionDenied()

		return PermissionResult{
			Approved:     false,
			DangerLevel:  danger.Level,
			DangerReason: danger.Reason,
		}
	}

	// Check global safe tools
	approved := isSafeTool(req.Tool)

	// Check config-level safe tools
	if !approved && cfg != nil {
		toolLower := strings.ToLower(req.Tool)
		for _, safe := range cfg.SafeTools {
			if strings.ToLower(safe) == toolLower {
				approved = true

				break
			}
		}
	}

	if approved {
		metrics.Global().RecordPermissionApproved()
	} else {
		metrics.Global().RecordPermissionDenied()
	}

	return PermissionResult{
		Approved:     approved,
		DangerLevel:  danger.Level,
		DangerReason: danger.Reason,
	}
}

// safeTools are tools that can be auto-approved.
// Keys are lowercase; use isSafeTool for case-insensitive lookup.
// Includes aliases for PascalCase tool names (e.g., "ReadFile" → "readfile").
var safeTools = map[string]bool{
	"read_file": true,
	"readfile":  true, // PascalCase alias
	"read":      true,
	"glob":      true,
	"grep":      true,
	"list_dir":  true,
	"listdir":   true, // PascalCase alias
	"ls":        true,
	"search":    true,
}

// isSafeTool checks if a tool is in the safe list (case-insensitive).
func isSafeTool(name string) bool {
	return safeTools[strings.ToLower(name)]
}

// DefaultPermissionHandler auto-approves safe read-only tools.
// Denies dangerous operations regardless of tool type.
func DefaultPermissionHandler(req PermissionRequest) bool {
	result := EvaluatePermission(req)

	return result.Approved
}

// KvelmoPermissionHandler is a more permissive handler for kvelmo's internal jobs.
// It allows Write/Edit/Bash tools that kvelmo needs for planning and implementation.
// Dangerous operations are still denied.
func KvelmoPermissionHandler(req PermissionRequest) bool {
	// Check for dangerous operations first
	danger := permission.DetectDanger(req.Tool, req.Input)
	if danger.Level == permission.Dangerous {
		return false
	}

	// Allow kvelmo-specific tools in addition to safe tools
	toolLower := strings.ToLower(req.Tool)
	if isSafeTool(toolLower) {
		return true
	}

	// Allow Write, Edit, Bash for kvelmo operations
	switch toolLower {
	case "write", "edit", "bash":
		return true
	}

	return false
}

// ConnectionMode indicates how the agent is connected.
type ConnectionMode string

const (
	ModeWebSocket ConnectionMode = "websocket" // WebSocket server mode
	ModeCLI       ConnectionMode = "cli"       // CLI subprocess mode
	ModeAPI       ConnectionMode = "api"       // Direct API mode
)

// Config holds common agent configuration.
type Config struct {
	// Connection preferences
	PreferWebSocket bool              // Try WebSocket first (default: true)
	WebSocketPort   int               // Port for WebSocket server (default: 0 = auto)
	Command         []string          // CLI command (e.g., ["claude"], ["codex"])
	Args            []string          // Additional CLI arguments
	Environment     map[string]string // Environment variables

	// Execution settings
	WorkDir    string        // Working directory
	Timeout    time.Duration // Execution timeout (default: 30m)
	RetryCount int           // Retry attempts (default: 3)
	RetryDelay time.Duration // Delay between retries (default: 1s)

	// Permission handling
	PermissionHandler PermissionHandler // Custom permission handler
	SafeTools         []string          // Additional tools to auto-approve (case-insensitive)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		PreferWebSocket:   true,
		WebSocketPort:     0, // Auto-assign
		Timeout:           30 * time.Minute,
		RetryCount:        3,
		RetryDelay:        time.Second,
		Environment:       make(map[string]string),
		PermissionHandler: DefaultPermissionHandler,
	}
}

// Merge merges another config into this one (other takes precedence).
func (c Config) Merge(other Config) Config {
	if other.Command != nil {
		c.Command = other.Command
	}
	if other.Args != nil {
		c.Args = append(c.Args, other.Args...)
	}
	if other.WorkDir != "" {
		c.WorkDir = other.WorkDir
	}
	if other.Timeout > 0 {
		c.Timeout = other.Timeout
	}
	if other.Environment != nil {
		if c.Environment == nil {
			c.Environment = make(map[string]string)
		}
		maps.Copy(c.Environment, other.Environment)
	}
	if other.PermissionHandler != nil {
		c.PermissionHandler = other.PermissionHandler
	}

	return c
}
