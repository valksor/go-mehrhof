package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestServerInitializeListCall(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(Tool{
		Name:        "echo",
		Description: "echo input back",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"msg": map[string]any{"type": "string"}},
			"required":             []string{"msg"},
			"additionalProperties": false,
		},
	}, func(_ context.Context, args map[string]any) (*ToolCallResult, error) {
		msg, ok := args["msg"].(string)
		if !ok {
			return &ToolCallResult{IsError: true, Content: []ContentBlock{{Type: ContentTypeText, Text: "msg must be string"}}}, nil
		}

		return &ToolCallResult{Content: []ContentBlock{{Type: ContentTypeText, Text: msg}}}, nil
	})

	srv := NewServer(reg)

	in, inW := io.Pipe()
	outR, out := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.ServeReadWriter(ctx, in, out) }()

	writeJSON := func(v any) {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := inW.Write(append(data, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	readJSON := func() Response {
		t.Helper()
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 1024)
		for {
			n, err := outR.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				if line, _, ok := bytes.Cut(buf, []byte{'\n'}); ok {
					var resp Response
					if err := json.Unmarshal(line, &resp); err != nil {
						t.Fatalf("unmarshal response: %v (%s)", err, string(buf))
					}

					return resp
				}
			}
			if err != nil {
				t.Fatalf("read: %v", err)
			}
		}
	}

	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  MethodInitialize,
		"params": InitializeParams{
			ProtocolVersion: ProtocolVersion,
			ClientInfo:      ClientInfo{Name: "test-client", Version: "0.1"},
		},
	})
	if resp := readJSON(); resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}

	writeJSON(map[string]any{"jsonrpc": "2.0", "id": 2, "method": MethodToolsList})
	resp := readJSON()
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
	var list ToolsListResult
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		t.Fatalf("unmarshal tools list: %v", err)
	}
	if len(list.Tools) != 1 || list.Tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", list.Tools)
	}

	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  MethodToolsCall,
		"params":  ToolCallParams{Name: "echo", Arguments: map[string]any{"msg": "hi"}},
	})
	resp = readJSON()
	if resp.Error != nil {
		t.Fatalf("tools/call error: %+v", resp.Error)
	}
	var call ToolCallResult
	if err := json.Unmarshal(resp.Result, &call); err != nil {
		t.Fatalf("unmarshal call: %v", err)
	}
	if call.IsError || len(call.Content) != 1 || call.Content[0].Text != "hi" {
		t.Fatalf("unexpected call result: %+v", call)
	}

	_ = inW.Close()
	cancel()
	<-serverErr
}

func TestValidateArgsRejectsExtra(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"a": map[string]any{"type": "string"}},
		"required":             []string{"a"},
		"additionalProperties": false,
	}
	if err := validateArgs(schema, map[string]any{"a": "x", "b": "y"}); err == nil {
		t.Fatal("expected error on extra arg")
	}
	if err := validateArgs(schema, map[string]any{}); err == nil {
		t.Fatal("expected error on missing required")
	}
	if err := validateArgs(schema, map[string]any{"a": "x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
