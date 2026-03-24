// Package apiagent provides a shared base for API-based AI agents.
//
// To add a new API agent, implement the Provider interface (3 methods)
// and pass it to NewBase. The Base struct handles the full Agent interface,
// conversation loop, tool execution, permission checking, and event streaming.
package apiagent

import (
	"context"
	"io"
	"net/http"
)

// Provider defines how a specific API backend communicates.
// Implementing this interface is all that's needed to add a new API agent.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic", "ollama").
	Name() string

	// Available checks if the provider is reachable (API key set, endpoint responding).
	Available() error

	// BuildRequest constructs an HTTP request for the given messages and tools.
	BuildRequest(ctx context.Context, cfg *APIConfig, messages []Message, tools []ToolDef) (*http.Request, error)

	// ParseStream reads the HTTP response body and emits chunks.
	// The channel is closed when the stream ends.
	ParseStream(ctx context.Context, body io.ReadCloser) (<-chan Chunk, error)
}

// Connector is an optional interface that providers can implement
// to perform setup during Connect (e.g., downloading models).
// If a provider implements this, Base.Connect calls it before marking connected.
type Connector interface {
	Connect(ctx context.Context, cfg *APIConfig) error
}

// ChunkType identifies the type of streaming chunk from the API.
type ChunkType int

const (
	ChunkText    ChunkType = iota // Streaming text token
	ChunkToolUse                  // Tool call request from the model
	ChunkDone                     // Message complete (stop reason)
	ChunkError                    // API error
)

// Chunk represents a single piece of a streaming API response.
type Chunk struct {
	Type    ChunkType
	Text    string        // For ChunkText
	ToolUse *ToolUseChunk // For ChunkToolUse
	Error   string        // For ChunkError
}

// ToolUseChunk represents a tool call requested by the model.
type ToolUseChunk struct {
	ID    string
	Name  string
	Input map[string]any
}
