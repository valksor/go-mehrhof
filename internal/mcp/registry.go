package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Executor is the function signature for a hand-written MCP tool.
type Executor func(ctx context.Context, args map[string]any) (*ToolCallResult, error)

// ToolRegistry holds hand-curated MCP tools.
//
// Unlike the prototype's auto-mapping from Cobra commands, kvelmo's MCP server
// exposes a narrow, curated set of orchestration primitives. Each tool is
// registered explicitly with a JSON Schema input and an Executor.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]registered
}

type registered struct {
	tool     Tool
	executor Executor
}

// NewToolRegistry constructs an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]registered)}
}

// Register adds a tool. Re-registering the same name replaces the prior entry.
func (r *ToolRegistry) Register(tool Tool, executor Executor) {
	if tool.Name == "" {
		slog.Warn("mcp: tool registered without name; ignoring")

		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[tool.Name]; exists {
		slog.Warn("mcp: replacing existing tool", "tool", tool.Name)
	}
	r.tools[tool.Name] = registered{tool: tool, executor: executor}
}

// ListTools returns all registered tools.
func (r *ToolRegistry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, entry := range r.tools {
		out = append(out, entry.tool)
	}

	return out
}

// CallTool executes a tool by name. A 5-minute default timeout is applied
// when the parent context has no deadline.
func (r *ToolRegistry) CallTool(ctx context.Context, name string, args map[string]any) (*ToolCallResult, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	r.mu.RLock()
	entry, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	if err := validateArgs(entry.tool.InputSchema, args); err != nil {
		return &ToolCallResult{
			Content: []ContentBlock{{Type: ContentTypeText, Text: err.Error()}},
			IsError: true,
		}, nil
	}

	slog.Info("mcp tool call", "tool", name)
	result, err := entry.executor(ctx, args)
	if err != nil {
		slog.Error("mcp tool failed", "tool", name, "error", err)

		return nil, err
	}

	return result, nil
}

// validateArgs enforces required fields and rejects unknown properties when
// the schema sets additionalProperties:false. Numeric range checks (minimum/
// maximum) are also enforced.
func validateArgs(schema map[string]any, args map[string]any) error {
	if schema == nil {
		return nil
	}
	properties, _ := schema["properties"].(map[string]any)

	if required, ok := schema["required"].([]string); ok {
		for _, key := range required {
			if _, present := args[key]; !present {
				return fmt.Errorf("required argument missing: %s", key)
			}
		}
	} else if requiredAny, ok := schema["required"].([]any); ok {
		for _, key := range requiredAny {
			s, _ := key.(string)
			if s == "" {
				continue
			}
			if _, present := args[s]; !present {
				return fmt.Errorf("required argument missing: %s", s)
			}
		}
	}

	additional, hasAdditional := schema["additionalProperties"]
	allowExtra := true
	if hasAdditional {
		if b, ok := additional.(bool); ok {
			allowExtra = b
		}
	}
	if !allowExtra && properties != nil {
		for arg := range args {
			if _, defined := properties[arg]; !defined {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
		}
	}

	if properties == nil {
		return nil
	}
	for arg, value := range args {
		propRaw, ok := properties[arg]
		if !ok {
			continue
		}
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		propType, _ := prop["type"].(string)
		if propType != "integer" && propType != "number" {
			continue
		}
		num, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("argument %q: expected number, got %T", arg, value)
		}
		if lo, ok := toFloat64(prop["minimum"]); ok && num < lo {
			return fmt.Errorf("argument %q: value %v below minimum %v", arg, num, lo)
		}
		if hi, ok := toFloat64(prop["maximum"]); ok && num > hi {
			return fmt.Errorf("argument %q: value %v above maximum %v", arg, num, hi)
		}
	}

	return nil
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()

		return f, err == nil
	default:
		return 0, false
	}
}
