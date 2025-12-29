package plugin

import (
	"encoding/json"
	"time"
)

// JSON-RPC 2.0 protocol types for plugin communication.

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	Params  any    `json:"params,omitempty"`
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int64  `json:"id"`
}

// NewRequest creates a new JSON-RPC request.
func NewRequest(id int64, method string, params any) *Request {
	return &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	Error   *RPCError       `json:"error,omitempty"`
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	ID      int64           `json:"id"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *RPCError) Error() string {
	return e.Message
}

// Standard JSON-RPC error codes.
const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603

	// Custom error codes (application-specific, -32000 to -32099)
	ErrCodePluginError     = -32000
	ErrCodeNotImplemented  = -32001
	ErrCodeCapabilityError = -32002
)

// Notification represents a JSON-RPC 2.0 notification (no ID, no response expected).
type Notification struct {
	Params  any    `json:"params,omitempty"`
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
}

// StreamEvent represents a streaming event from an agent plugin.
type StreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Stream event types (aligned with agent.EventType).
const (
	StreamEventText       = "text"
	StreamEventToolUse    = "tool_use"
	StreamEventToolResult = "tool_result"
	StreamEventFile       = "file"
	StreamEventUsage      = "usage"
	StreamEventComplete   = "complete"
	StreamEventError      = "error"
)

// Provider protocol types.

// InitParams contains parameters for the init method.
type InitParams struct {
	Config map[string]any `json:"config"`
}

// InitResult contains the result of the init method.
type InitResult struct {
	Capabilities []string `json:"capabilities"`
}

// MatchParams contains parameters for provider.match.
type MatchParams struct {
	Input string `json:"input"`
}

// MatchResult contains the result of provider.match.
type MatchResult struct {
	Matches bool `json:"matches"`
}

// ParseParams contains parameters for provider.parse.
type ParseParams struct {
	Input string `json:"input"`
}

// ParseResult contains the result of provider.parse.
type ParseResult struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// FetchParams contains parameters for provider.fetch.
type FetchParams struct {
	ID string `json:"id"`
}

// WorkUnitResult represents a work unit returned by a provider plugin.
// This mirrors provider.WorkUnit but uses JSON-friendly types.
type WorkUnitResult struct {
	ID          string   `json:"id"`
	ExternalID  string   `json:"externalId,omitempty"`
	Provider    string   `json:"provider"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels,omitempty"`

	// Assignees
	Assignees []PersonResult `json:"assignees,omitempty"`

	// Comments
	Comments []CommentResult `json:"comments,omitempty"`

	// Attachments
	Attachments []AttachmentResult `json:"attachments,omitempty"`

	// Subtasks
	Subtasks []string `json:"subtasks,omitempty"`

	// Naming fields
	ExternalKey string `json:"externalKey,omitempty"`
	TaskType    string `json:"taskType,omitempty"`
	Slug        string `json:"slug,omitempty"`

	// Source info
	Source SourceInfoResult `json:"source,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`

	// Metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PersonResult represents a person in plugin responses.
type PersonResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
}

// CommentResult represents a comment in plugin responses.
type CommentResult struct {
	CreatedAt time.Time    `json:"createdAt,omitempty"`
	Author    PersonResult `json:"author,omitempty"`
	ID        string       `json:"id"`
	Body      string       `json:"body"`
}

// AttachmentResult represents an attachment in plugin responses.
type AttachmentResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	MimeType string `json:"mimeType,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// SourceInfoResult represents source information in plugin responses.
type SourceInfoResult struct {
	Reference string `json:"reference"`
	URL       string `json:"url,omitempty"`
}

// ListParams contains parameters for provider.list.
type ListParams struct {
	Options  map[string]any `json:"options,omitempty"`
	Status   string         `json:"status,omitempty"`
	Assignee string         `json:"assignee,omitempty"`
	Labels   []string       `json:"labels,omitempty"`
	Limit    int            `json:"limit,omitempty"`
	Offset   int            `json:"offset,omitempty"`
}

// AddCommentParams contains parameters for provider.addComment.
type AddCommentParams struct {
	WorkUnitID string `json:"workUnitId"`
	Body       string `json:"body"`
}

// UpdateStatusParams contains parameters for provider.updateStatus.
type UpdateStatusParams struct {
	WorkUnitID string `json:"workUnitId"`
	Status     string `json:"status"`
}

// CreatePRParams contains parameters for provider.createPR.
type CreatePRParams struct {
	WorkUnitID   string `json:"workUnitId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"sourceBranch"`
	TargetBranch string `json:"targetBranch"`
	Draft        bool   `json:"draft,omitempty"`
}

// PullRequestResult represents a pull request result.
type PullRequestResult struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Number int    `json:"number"`
}

// SnapshotParams contains parameters for provider.snapshot.
type SnapshotParams struct {
	ID string `json:"id"`
}

// SnapshotResult represents a snapshot result.
type SnapshotResult struct {
	Metadata map[string]any `json:"metadata,omitempty"`
	Content  string         `json:"content"`
}

// Agent protocol types.

// AgentInitResult contains the result of agent.init.
type AgentInitResult struct {
	Metadata     *AgentMetadataResult `json:"metadata,omitempty"`
	Capabilities []string             `json:"capabilities"`
}

// AgentMetadataResult represents agent metadata.
type AgentMetadataResult struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Models      []string `json:"models,omitempty"`
}

// AgentAvailableResult contains the result of agent.available.
type AgentAvailableResult struct {
	Error     string `json:"error,omitempty"`
	Available bool   `json:"available"`
}

// AgentRunParams contains parameters for agent.run.
type AgentRunParams struct {
	Env     map[string]string `json:"env,omitempty"`
	Options map[string]any    `json:"options,omitempty"`
	Prompt  string            `json:"prompt"`
}

// Workflow protocol types.

// WorkflowInitResult contains the result of workflow.init.
type WorkflowInitResult struct {
	Phases  []PhaseInfo  `json:"phases,omitempty"`
	Guards  []GuardInfo  `json:"guards,omitempty"`
	Effects []EffectInfo `json:"effects,omitempty"`
}

// PhaseInfo describes a custom phase from a workflow plugin.
type PhaseInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	After       string `json:"after,omitempty"`
	Before      string `json:"before,omitempty"`
}

// GuardInfo describes a custom guard from a workflow plugin.
type GuardInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// EffectInfo describes a custom effect from a workflow plugin.
type EffectInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Critical    bool   `json:"critical,omitempty"` // If true, effect failure blocks transition
}

// EvaluateGuardParams contains parameters for workflow.evaluateGuard.
type EvaluateGuardParams struct {
	WorkUnit map[string]any `json:"workUnit"`
	Name     string         `json:"name"`
}

// EvaluateGuardResult contains the result of workflow.evaluateGuard.
type EvaluateGuardResult struct {
	Reason string `json:"reason,omitempty"`
	Passed bool   `json:"passed"`
}

// ExecuteEffectParams contains parameters for workflow.executeEffect.
type ExecuteEffectParams struct {
	WorkUnit map[string]any `json:"workUnit"`
	Data     map[string]any `json:"data,omitempty"`
	Name     string         `json:"name"`
}

// ExecuteEffectResult contains the result of workflow.executeEffect.
type ExecuteEffectResult struct {
	Data    map[string]any `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
	Success bool           `json:"success"`
}
