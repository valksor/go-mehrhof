package apiagent

// Role represents the role of a message participant.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a conversation message in provider-neutral format.
// Each Provider converts these to its wire format in BuildRequest.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall  // For assistant messages that invoke tools
	ToolResult *ToolResult // For tool role messages returning results
}

// ToolCall represents a tool invocation requested by the assistant.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// ToolDef defines a tool available to the model.
// Each Provider converts this to its function-calling schema.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema for parameters
}
