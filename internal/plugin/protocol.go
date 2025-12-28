package plugin

import (
	"encoding/json"
	"time"
)

// JSON-RPC 2.0 protocol types for plugin communication.

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
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
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
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
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
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
	ID        string       `json:"id"`
	Body      string       `json:"body"`
	Author    PersonResult `json:"author,omitempty"`
	CreatedAt time.Time    `json:"createdAt,omitempty"`
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
	Status   string         `json:"status,omitempty"`
	Labels   []string       `json:"labels,omitempty"`
	Assignee string         `json:"assignee,omitempty"`
	Limit    int            `json:"limit,omitempty"`
	Offset   int            `json:"offset,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
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
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// SnapshotParams contains parameters for provider.snapshot.
type SnapshotParams struct {
	ID string `json:"id"`
}

// SnapshotResult represents a snapshot result.
type SnapshotResult struct {
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Agent protocol types.

// AgentInitResult contains the result of agent.init.
type AgentInitResult struct {
	Capabilities []string             `json:"capabilities"`
	Metadata     *AgentMetadataResult `json:"metadata,omitempty"`
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
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// AgentRunParams contains parameters for agent.run.
type AgentRunParams struct {
	Prompt  string            `json:"prompt"`
	Env     map[string]string `json:"env,omitempty"`
	Options map[string]any    `json:"options,omitempty"`
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
	Name     string         `json:"name"`
	WorkUnit map[string]any `json:"workUnit"`
}

// EvaluateGuardResult contains the result of workflow.evaluateGuard.
type EvaluateGuardResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// ExecuteEffectParams contains parameters for workflow.executeEffect.
type ExecuteEffectParams struct {
	Name     string         `json:"name"`
	WorkUnit map[string]any `json:"workUnit"`
	Data     map[string]any `json:"data,omitempty"`
}

// ExecuteEffectResult contains the result of workflow.executeEffect.
type ExecuteEffectResult struct {
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}
