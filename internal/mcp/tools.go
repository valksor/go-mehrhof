package mcp

import (
	"context"
	"fmt"
)

// RegisterTools installs the kvelmo MCP tool surface on the registry, backed
// by the given client. Each tool is intentionally narrow and dispatched
// straight to a single worktree RPC.
//
// The tool naming convention is `kvelmo_<verb>_<noun>` so that namespacing is
// obvious to the LLM when multiple MCP servers are mounted.
func RegisterTools(reg *ToolRegistry, client *Client) {
	reg.Register(toolGetTask(), execGetTask(client))
	reg.Register(toolGetSpecifications(), execGetSpecifications(client))
	reg.Register(toolReadFile(), execReadFile(client))
	reg.Register(toolSaveArtifact(), execSaveArtifact(client))
	reg.Register(toolCreateCheckpoint(), execCreateCheckpoint(client))
	reg.Register(toolSignalComplete(), execSignalComplete(client))
	reg.Register(toolSignalFailure(), execSignalFailure(client))
}

// --- Tool definitions ---

func toolGetTask() Tool {
	return Tool{
		Name:        "kvelmo_get_task",
		Description: "Return the current kvelmo task (id, title, description, state). Call this first to learn the current phase.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
}

func toolGetSpecifications() Tool {
	return Tool{
		Name:        "kvelmo_get_specifications",
		Description: "Return the specification files written so far for this task. Set include_content=true to also return their contents (large output).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"include_content": map[string]any{
					"type":        "boolean",
					"description": "If true, include file contents (default: false).",
				},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
}

func toolReadFile() Tool {
	return Tool{
		Name:        "kvelmo_read_file",
		Description: "Read a single file from the worktree. Returns content as text. Maximum 10 MiB. Use relative paths or absolute paths within the worktree.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to the worktree root, or absolute path inside it.",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	}
}

func toolSaveArtifact() Tool {
	return Tool{
		Name:        "kvelmo_save_artifact",
		Description: "Persist an artifact produced during the current phase. kind must be one of: spec, plan, implementation_summary. Returns the saved path.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind": map[string]any{
					"type":        "string",
					"description": "Artifact type: spec | plan | implementation_summary.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Markdown content of the artifact.",
				},
			},
			"required":             []string{"kind", "content"},
			"additionalProperties": false,
		},
	}
}

func toolCreateCheckpoint() Tool {
	return Tool{
		Name:        "kvelmo_create_checkpoint",
		Description: "Create a git checkpoint commit with the given message. Returns the new commit SHA.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Short imperative checkpoint message.",
				},
			},
			"required":             []string{"message"},
			"additionalProperties": false,
		},
	}
}

func toolSignalComplete() Tool {
	return Tool{
		Name:        "kvelmo_signal_complete",
		Description: "Signal that the current phase is complete. Finalises the phase (detects spec files, copies to repo, creates the completion checkpoint). MUST be called before exiting the session.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"phase": map[string]any{
					"type":        "string",
					"description": "Phase that just completed: plan | implement | simplify | optimize | review.",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "Optional human-readable summary of what was accomplished.",
				},
			},
			"required":             []string{"phase"},
			"additionalProperties": false,
		},
	}
}

func toolSignalFailure() Tool {
	return Tool{
		Name:        "kvelmo_signal_failure",
		Description: "Signal that the current phase has hit an unrecoverable error. The conductor will record the failure and surface it to the user.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"phase": map[string]any{
					"type":        "string",
					"description": "Phase that failed: plan | implement | simplify | optimize | review.",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Short, human-readable failure reason.",
				},
				"retryable": map[string]any{
					"type":        "boolean",
					"description": "Whether the failure might succeed on retry (default: false).",
				},
			},
			"required":             []string{"phase", "reason"},
			"additionalProperties": false,
		},
	}
}

// --- Executors ---

type taskGetResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Phase       string `json:"phase"`
	ExternalID  string `json:"external_id,omitempty"`
}

func execGetTask(c *Client) Executor {
	return func(ctx context.Context, _ map[string]any) (*ToolCallResult, error) {
		var out taskGetResult
		if errResult, ok := c.CallInto(ctx, "mcp.task.get", nil, &out); !ok {
			return errResult, nil
		}

		return jsonResult(out), nil
	}
}

type specificationsResult struct {
	Specifications []struct {
		Path    string `json:"path"`
		Content string `json:"content,omitempty"`
	} `json:"specifications"`
}

func execGetSpecifications(c *Client) Executor {
	return func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		include := argBool(args, "include_content", false)
		var out specificationsResult
		if errResult, ok := c.CallInto(ctx, "mcp.task.specifications",
			map[string]any{"include_content": include}, &out); !ok {
			return errResult, nil
		}

		return jsonResult(out), nil
	}
}

type fileReadResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

func execReadFile(c *Client) Executor {
	return func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		path := argString(args, "path")
		if path == "" {
			return missing("path")
		}
		var out fileReadResult
		if errResult, ok := c.CallInto(ctx, "mcp.files.read",
			map[string]any{"path": path}, &out); !ok {
			return errResult, nil
		}

		return jsonResult(out), nil
	}
}

type artifactSaveResult struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

func execSaveArtifact(c *Client) Executor {
	return func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		kind := argString(args, "kind")
		content := argString(args, "content")
		if kind == "" {
			return missing("kind")
		}
		if content == "" {
			return missing("content")
		}
		var out artifactSaveResult
		if errResult, ok := c.CallInto(ctx, "mcp.artifacts.save",
			map[string]any{"kind": kind, "content": content}, &out); !ok {
			return errResult, nil
		}

		return jsonResult(out), nil
	}
}

type checkpointResult struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

func execCreateCheckpoint(c *Client) Executor {
	return func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		msg := argString(args, "message")
		if msg == "" {
			return missing("message")
		}
		var out checkpointResult
		if errResult, ok := c.CallInto(ctx, "mcp.checkpoints.create",
			map[string]any{"message": msg}, &out); !ok {
			return errResult, nil
		}

		return jsonResult(out), nil
	}
}

type signalResult struct {
	OK        bool   `json:"ok"`
	State     string `json:"state"`
	Finalized bool   `json:"finalized"`
}

func execSignalComplete(c *Client) Executor {
	return func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		phase := argString(args, "phase")
		summary := argString(args, "summary")
		if phase == "" {
			return missing("phase")
		}
		params := map[string]any{"phase": phase}
		if summary != "" {
			params["summary"] = summary
		}
		var out signalResult
		if errResult, ok := c.CallInto(ctx, "mcp.signal.complete", params, &out); !ok {
			return errResult, nil
		}
		notifyRendezvous(RendezvousEvent{Type: "complete", Phase: phase, Summary: summary})

		return textResult(fmt.Sprintf("phase %s complete (state=%s, finalized=%v)", phase, out.State, out.Finalized)), nil
	}
}

func execSignalFailure(c *Client) Executor {
	return func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		phase := argString(args, "phase")
		reason := argString(args, "reason")
		retryable := argBool(args, "retryable", false)
		if phase == "" {
			return missing("phase")
		}
		if reason == "" {
			return missing("reason")
		}
		params := map[string]any{"phase": phase, "reason": reason, "retryable": retryable}
		var out signalResult
		if errResult, ok := c.CallInto(ctx, "mcp.signal.failure", params, &out); !ok {
			return errResult, nil
		}
		notifyRendezvous(RendezvousEvent{Type: "failure", Phase: phase, Reason: reason, Retryable: retryable})

		return textResult(fmt.Sprintf("phase %s failed: %s (retryable=%v)", phase, reason, retryable)), nil
	}
}

// (RendezvousEvent and its sink live in protocol.go since they are a
// transport-level concern, not a tool implementation detail.)
