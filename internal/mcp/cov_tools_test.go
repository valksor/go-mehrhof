package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// recordingRPC is a configurable WorktreeRPC fake that records the last call
// and returns a programmable response keyed by method.
type recordingRPC struct {
	responses  map[string]json.RawMessage
	err        error
	lastMethod string
	lastParams any
}

func (r *recordingRPC) Call(_ context.Context, method string, params any) (*socket.Response, error) {
	r.lastMethod = method
	r.lastParams = params
	if r.err != nil {
		return nil, r.err
	}
	res := r.responses[method]
	if res == nil {
		res = json.RawMessage(`{}`)
	}

	return &socket.Response{Result: res}, nil
}

func (r *recordingRPC) Close() error { return nil }

func newRegisteredServer(t *testing.T, responses map[string]json.RawMessage) (*ToolRegistry, *recordingRPC) {
	t.Helper()
	rpc := &recordingRPC{responses: responses}
	client := NewClient(rpc, "task-1", "/wt", "/sock")
	reg := NewToolRegistry()
	RegisterTools(reg, client)

	return reg, rpc
}

func TestRegisterTools_AllToolsPresent(t *testing.T) {
	reg, _ := newRegisteredServer(t, nil)
	tools := reg.ListTools()

	want := map[string]bool{
		"kvelmo_get_task":           false,
		"kvelmo_get_specifications": false,
		"kvelmo_read_file":          false,
		"kvelmo_save_artifact":      false,
		"kvelmo_create_checkpoint":  false,
		"kvelmo_signal_complete":    false,
		"kvelmo_signal_failure":     false,
	}
	for _, tool := range tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
		// Every tool must declare an object input schema.
		if tool.InputSchema[schemaKeyType] != schemaTypeObject {
			t.Errorf("tool %q schema type = %v, want object", tool.Name, tool.InputSchema[schemaKeyType])
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
	if len(tools) != 7 {
		t.Errorf("registered %d tools, want 7", len(tools))
	}
}

func TestExecGetTask_Success(t *testing.T) {
	reg, rpc := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.task.get": json.RawMessage(`{"id":"t1","title":"Title","state":"planning","phase":"plan"}`),
	})

	res, err := reg.CallTool(context.Background(), "kvelmo_get_task", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	if rpc.lastMethod != "mcp.task.get" {
		t.Errorf("method = %q, want mcp.task.get", rpc.lastMethod)
	}
	// The text block should contain the marshalled task.
	var out taskGetResult
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("decode result text: %v", err)
	}
	if out.ID != "t1" || out.State != "planning" {
		t.Errorf("decoded result = %+v", out)
	}
}

func TestExecGetTask_RPCError(t *testing.T) {
	rpc := &recordingRPC{err: errors.New("socket closed")}
	reg := NewToolRegistry()
	RegisterTools(reg, NewClient(rpc, "t", "w", "s"))

	res, err := reg.CallTool(context.Background(), "kvelmo_get_task", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true on rpc failure")
	}
}

func TestExecGetSpecifications_IncludeContent(t *testing.T) {
	reg, rpc := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.task.specifications": json.RawMessage(`{"specifications":[{"path":"spec.md","content":"hi"}]}`),
	})

	res, err := reg.CallTool(context.Background(), "kvelmo_get_specifications", map[string]any{"include_content": true})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	params, _ := rpc.lastParams.(map[string]any)
	if params["include_content"] != true {
		t.Errorf("include_content param = %v, want true", params["include_content"])
	}
}

func TestExecReadFile_MissingPath(t *testing.T) {
	reg, _ := newRegisteredServer(t, nil)

	res, err := reg.CallTool(context.Background(), "kvelmo_read_file", map[string]any{"path": ""})
	// validateArgs requires "path"; empty present satisfies presence but exec rejects empty.
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for empty path")
	}
}

func TestExecReadFile_Success(t *testing.T) {
	reg, rpc := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.files.read": json.RawMessage(`{"path":"a.go","content":"package a","size":9}`),
	})

	res, err := reg.CallTool(context.Background(), "kvelmo_read_file", map[string]any{"path": "a.go"})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	params, _ := rpc.lastParams.(map[string]any)
	if params["path"] != "a.go" {
		t.Errorf("path param = %v", params["path"])
	}
}

func TestExecSaveArtifact_Validation(t *testing.T) {
	reg, _ := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.artifacts.save": json.RawMessage(`{"path":"out/spec.md","kind":"spec"}`),
	})

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{"missing kind", map[string]any{"kind": "", "content": "x"}, true},
		{"missing content", map[string]any{"kind": "spec", "content": ""}, true},
		{"valid", map[string]any{"kind": "spec", "content": "body"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := reg.CallTool(context.Background(), "kvelmo_save_artifact", tt.args)
			if err != nil {
				t.Fatalf("CallTool error = %v", err)
			}
			if res.IsError != tt.wantErr {
				t.Errorf("IsError = %v, want %v (%+v)", res.IsError, tt.wantErr, res)
			}
		})
	}
}

func TestExecCreateCheckpoint(t *testing.T) {
	reg, rpc := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.checkpoints.create": json.RawMessage(`{"sha":"abc123","message":"chk"}`),
	})

	// Missing message.
	res, _ := reg.CallTool(context.Background(), "kvelmo_create_checkpoint", map[string]any{"message": ""})
	if !res.IsError {
		t.Error("expected error for empty message")
	}

	// Valid message.
	res, err := reg.CallTool(context.Background(), "kvelmo_create_checkpoint", map[string]any{"message": "chk"})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	if rpc.lastMethod != "mcp.checkpoints.create" {
		t.Errorf("method = %q", rpc.lastMethod)
	}
}

func TestExecSignalComplete(t *testing.T) {
	var captured []RendezvousEvent
	SetRendezvousSink(func(e RendezvousEvent) { captured = append(captured, e) })
	defer SetRendezvousSink(nil)

	reg, rpc := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.signal.complete": json.RawMessage(`{"ok":true,"state":"planned","finalized":true}`),
	})

	// Missing phase.
	res, _ := reg.CallTool(context.Background(), "kvelmo_signal_complete", map[string]any{"phase": "", "summary": "x"})
	if !res.IsError {
		t.Error("expected error for missing phase")
	}

	// Valid with summary.
	res, err := reg.CallTool(context.Background(), "kvelmo_signal_complete", map[string]any{"phase": "plan", "summary": "done"})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	params, _ := rpc.lastParams.(map[string]any)
	if params[schemaKeyPhase] != "plan" || params["summary"] != "done" {
		t.Errorf("params = %v", params)
	}
	if len(captured) != 1 || captured[0].Type != "complete" || captured[0].Phase != "plan" {
		t.Errorf("rendezvous events = %+v", captured)
	}
}

func TestExecSignalFailure(t *testing.T) {
	var captured []RendezvousEvent
	SetRendezvousSink(func(e RendezvousEvent) { captured = append(captured, e) })
	defer SetRendezvousSink(nil)

	reg, rpc := newRegisteredServer(t, map[string]json.RawMessage{
		"mcp.signal.failure": json.RawMessage(`{"ok":true,"state":"failed","finalized":false}`),
	})

	// Missing phase.
	if res, _ := reg.CallTool(context.Background(), "kvelmo_signal_failure", map[string]any{"phase": "", "reason": "boom"}); !res.IsError {
		t.Error("expected error for missing phase")
	}
	// Missing reason.
	if res, _ := reg.CallTool(context.Background(), "kvelmo_signal_failure", map[string]any{"phase": "implement", "reason": ""}); !res.IsError {
		t.Error("expected error for missing reason")
	}

	res, err := reg.CallTool(context.Background(), "kvelmo_signal_failure", map[string]any{
		"phase": "implement", "reason": "compile error", "retryable": true,
	})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	params, _ := rpc.lastParams.(map[string]any)
	if params["retryable"] != true || params["reason"] != "compile error" {
		t.Errorf("params = %v", params)
	}
	if len(captured) != 1 || captured[0].Type != "failure" || !captured[0].Retryable {
		t.Errorf("rendezvous events = %+v", captured)
	}
}
