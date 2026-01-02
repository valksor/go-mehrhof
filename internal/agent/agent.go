package agent

import "context"

// Agent is the interface for AI agents.
type Agent interface {
	// Name returns the agent's identifier
	Name() string

	// Run executes a prompt and returns the response
	Run(ctx context.Context, prompt string) (*Response, error)

	// RunStream executes a prompt and streams events
	RunStream(ctx context.Context, prompt string) (<-chan Event, <-chan error)

	// RunWithCallback executes with a callback for each event
	RunWithCallback(ctx context.Context, prompt string, cb StreamCallback) (*Response, error)

	// Available checks if the agent is available (binary exists, etc.)
	Available() error

	// WithEnv adds an environment variable to pass to the agent process.
	// Returns the agent for method chaining.
	WithEnv(key, value string) Agent

	// WithArgs adds CLI arguments to pass to the agent process.
	// Returns the agent for method chaining.
	WithArgs(args ...string) Agent
}

// StreamCallback is called for each streaming event.
type StreamCallback func(event Event) error

// Parser parses agent output into structured responses.
type Parser interface {
	// ParseEvent parses a single line of output
	ParseEvent(line []byte) (Event, error)

	// Parse aggregates events into a response
	Parse(events []Event) (*Response, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// Extended agent interfaces for advanced functionality
// ──────────────────────────────────────────────────────────────────────────────

// MetadataProvider returns agent metadata.
type MetadataProvider interface {
	// Metadata returns information about the agent
	Metadata() AgentMetadata
}

// AgentMetadata describes an agent's capabilities.
type AgentMetadata struct {
	Name         string      // Display name
	Version      string      // Agent/CLI version
	Description  string      // Human-readable description
	Models       []ModelInfo // Available models
	Capabilities AgentCapabilities
}

// ModelInfo describes an available model.
type ModelInfo struct {
	ID         string  // Model identifier (e.g., "claude-3-opus-20240229")
	Name       string  // Display name (e.g., "Claude 3 Opus")
	Default    bool    // Is this the default model?
	MaxTokens  int     // Maximum context tokens
	InputCost  float64 // Cost per million input tokens
	OutputCost float64 // Cost per million output tokens
}

// AgentCapabilities describes what an agent can do.
type AgentCapabilities struct {
	Streaming      bool     // Supports streaming output
	ToolUse        bool     // Supports tool use/function calling
	FileOperations bool     // Can create/modify/delete files
	CodeExecution  bool     // Can execute code/commands
	MultiTurn      bool     // Supports conversation history
	SystemPrompt   bool     // Accepts system prompts
	AllowedTools   []string // List of available tools, empty = all
}
