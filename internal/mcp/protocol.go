// Package mcp implements a Model Context Protocol (MCP) server over stdio.
// The server exposes kvelmo orchestration primitives as MCP tools so that
// an interactive Claude Code session (launched by the claudemcp adapter)
// can drive a kvelmo workflow autonomously without leaving the Max
// subscription billing path.
package mcp

import (
	"encoding/json"
	"errors"
	"sync"
)

// ProtocolVersion is the MCP protocol version supported by this server.
const ProtocolVersion = "2025-06-18"

// ErrDisabled is returned when MCP is not available in the current build.
var ErrDisabled = errors.New("mcp: server is disabled in this build")

// Request represents a JSON-RPC 2.0 request (MCP compatible).
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response (MCP compatible).
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Standard JSON-RPC error codes.
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP-specific method names.
const (
	MethodInitialize    = "initialize"
	MethodToolsList     = "tools/list"
	MethodToolsCall     = "tools/call"
	MethodResourcesList = "resources/list"
	MethodResourcesRead = "resources/read"
	MethodPromptsList   = "prompts/list"
	MethodPromptsGet    = "prompts/get"
	MethodShutdown      = "shutdown"
	MethodNotifications = "notifications/"
)

// ServerInfo describes the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities describes server capabilities.
type ServerCapabilities struct {
	Tools     *ToolsCapabilities     `json:"tools,omitempty"`
	Resources *ResourcesCapabilities `json:"resources,omitempty"`
	Prompts   *PromptsCapabilities   `json:"prompts,omitempty"`
}

// ToolsCapabilities describes the tools capability.
type ToolsCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapabilities describes the resources capability.
type ResourcesCapabilities struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapabilities describes the prompts capability.
type PromptsCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ClientCapabilities describes client-side capabilities.
type ClientCapabilities struct {
	Experimental map[string]any   `json:"experimental,omitempty"`
	Roots        *RootsCapability `json:"roots,omitempty"`
	Sampling     *SamplingCap     `json:"sampling,omitempty"`
}

// RootsCapability describes the client's roots capability.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCap describes the client's sampling capability.
type SamplingCap struct{}

// InitializeParams contains initialization parameters from the client.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientInfo describes the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult contains the initialization response.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// Tool describes an available MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolsListParams contains parameters for tools/list.
type ToolsListParams struct {
	Cursor string `json:"cursor,omitempty"`
}

// ToolsListResult contains the response for tools/list.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams contains parameters for tools/call.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolCallResult contains the response for tools/call.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a block of content in a tool result.
type ContentBlock struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	Data     string         `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Content types.
const (
	ContentTypeText     = "text"
	ContentTypeImage    = "image"
	ContentTypeResource = "resource"
)

// EmptyResult is an empty result body (used for shutdown, etc.).
type EmptyResult struct{}

// RendezvousEvent is the JSON payload the MCP server pushes to the adapter
// via the rendezvous Unix socket when a phase signals completion or
// failure. The adapter watches that socket to know when to gracefully stop
// the spawned claude TUI.
//
// This is a transport-level concept owned by the MCP package because the
// adapter (agent/claudemcp) and the MCP server tool implementations
// (internal/mcp/tools.go) both need to agree on the wire format.
type RendezvousEvent struct {
	Type      string `json:"type"`
	Phase     string `json:"phase,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

// rendezvousSink is set by the `kvelmo mcp` subcommand at startup if the
// KVELMO_ADAPTER_SOCKET env var is present. notifyRendezvous is a no-op when
// no sink is wired (e.g. unit tests).
var (
	rendezvousSink   func(RendezvousEvent)
	rendezvousSinkMu sync.RWMutex
)

// SetRendezvousSink installs a function that receives rendezvous events.
// Called once by the `kvelmo mcp` subcommand wiring.
func SetRendezvousSink(fn func(RendezvousEvent)) {
	rendezvousSinkMu.Lock()
	defer rendezvousSinkMu.Unlock()
	rendezvousSink = fn
}

func notifyRendezvous(evt RendezvousEvent) {
	rendezvousSinkMu.RLock()
	fn := rendezvousSink
	rendezvousSinkMu.RUnlock()
	if fn != nil {
		fn(evt)
	}
}
