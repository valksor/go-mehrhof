package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/valksor/kvelmo/internal/socket"
)

// WorktreeRPC is the minimal interface the MCP tools need from the underlying
// transport. The real implementation is a *socket.Client connected to a
// per-worktree kvelmo socket; tests substitute a fake.
type WorktreeRPC interface {
	Call(ctx context.Context, method string, params any) (*socket.Response, error)
	Close() error
}

// Client wraps a worktree socket client with MCP-friendly helpers: it
// unmarshals results into typed values and translates RPC errors into MCP
// ToolCallResult{IsError: true} blocks so the LLM sees them as tool failures
// rather than transport errors.
type Client struct {
	rpc        WorktreeRPC
	TaskID     string
	Worktree   string
	HostSocket string // path to the worktree socket (for diagnostics)
}

// NewClient constructs a Client around an already-connected socket.
func NewClient(rpc WorktreeRPC, taskID, worktree, hostSocket string) *Client {
	return &Client{rpc: rpc, TaskID: taskID, Worktree: worktree, HostSocket: hostSocket}
}

// Close closes the underlying transport.
func (c *Client) Close() error {
	if c.rpc == nil {
		return nil
	}

	return c.rpc.Close()
}

// CallInto invokes method with params and unmarshals the JSON-RPC result into
// out. Returns an MCP-shaped error result if the RPC failed, so the caller can
// pass it straight back to the LLM as a tool result. The bool indicates
// whether the call succeeded; on false, the returned *ToolCallResult is
// already populated.
func (c *Client) CallInto(ctx context.Context, method string, params any, out any) (*ToolCallResult, bool) {
	if c.rpc == nil {
		return errorResult(fmt.Sprintf("mcp client not connected (method %s)", method)), false
	}
	resp, err := c.rpc.Call(ctx, method, params)
	if err != nil {
		return errorResult(fmt.Sprintf("rpc %s failed: %v", method, err)), false
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return errorResult(fmt.Sprintf("decode %s response: %v", method, err)), false
		}
	}

	return nil, true
}

// errorResult builds a ToolCallResult with IsError=true and the given text.
func errorResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentBlock{{Type: ContentTypeText, Text: text}},
		IsError: true,
	}
}

// textResult builds a successful ToolCallResult with a single text block.
func textResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentBlock{{Type: ContentTypeText, Text: text}},
	}
}

// jsonResult builds a successful ToolCallResult by JSON-marshalling v.
func jsonResult(v any) *ToolCallResult {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("marshal result: %v", err))
	}

	return &ToolCallResult{
		Content: []ContentBlock{{Type: ContentTypeText, Text: string(data)}},
	}
}

// missing returns an MCP error result for a missing required argument.
func missing(name string) (*ToolCallResult, error) {
	return errorResult("missing required argument: " + name), nil
}

// argString extracts a string argument or empty string if absent/wrong type.
func argString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// argBool extracts a bool argument with a default value.
func argBool(args map[string]any, key string, def bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}

	return def
}

// ErrMissingEnv is returned when a required env var is empty.
var ErrMissingEnv = errors.New("required env var is empty")
