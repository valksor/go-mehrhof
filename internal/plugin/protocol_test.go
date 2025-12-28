package plugin

import (
	"encoding/json"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// NewRequest tests
// ──────────────────────────────────────────────────────────────────────────────

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name     string
		id       int64
		method   string
		params   any
		wantJSON string
	}{
		{
			name:   "basic request with params",
			id:     1,
			method: "provider.fetch",
			params: map[string]string{"id": "123"},
		},
		{
			name:   "request without params",
			id:     42,
			method: "shutdown",
			params: nil,
		},
		{
			name:   "request with struct params",
			id:     100,
			method: "agent.run",
			params: AgentRunParams{Prompt: "test prompt"},
		},
		{
			name:   "request with zero ID",
			id:     0,
			method: "stream",
			params: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.id, tt.method, tt.params)

			if req == nil {
				t.Fatal("NewRequest returned nil")
			}

			if req.JSONRPC != "2.0" {
				t.Errorf("JSONRPC = %q, want %q", req.JSONRPC, "2.0")
			}

			if req.ID != tt.id {
				t.Errorf("ID = %d, want %d", req.ID, tt.id)
			}

			if req.Method != tt.method {
				t.Errorf("Method = %q, want %q", req.Method, tt.method)
			}

			// Params comparison depends on type
			if tt.params != nil {
				if req.Params == nil {
					t.Error("Params is nil, want non-nil")
				}
			}

			// Ensure it can be marshaled to JSON
			data, err := json.Marshal(req)
			if err != nil {
				t.Errorf("json.Marshal error = %v", err)
			}
			if len(data) == 0 {
				t.Error("json.Marshal returned empty data")
			}
		})
	}
}

func TestNewRequest_JSONMarshal(t *testing.T) {
	req := NewRequest(1, "test.method", map[string]int{"value": 42})

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	// Unmarshal back to verify structure
	var unmarshaled map[string]any
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if unmarshaled["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", unmarshaled["jsonrpc"])
	}

	if id, ok := unmarshaled["id"].(float64); !ok || int64(id) != 1 {
		t.Errorf("id = %v, want 1", unmarshaled["id"])
	}

	if unmarshaled["method"] != "test.method" {
		t.Errorf("method = %v, want test.method", unmarshaled["method"])
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RPCError tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRPCError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *RPCError
		wantMsg string
	}{
		{
			name:    "simple error message",
			err:     &RPCError{Code: ErrCodeInternalError, Message: "internal error"},
			wantMsg: "internal error",
		},
		{
			name:    "error with data",
			err:     &RPCError{Code: ErrCodePluginError, Message: "plugin failed", Data: map[string]string{"detail": "test"}},
			wantMsg: "plugin failed",
		},
		{
			name:    "empty message",
			err:     &RPCError{Code: ErrCodeParseError, Message: ""},
			wantMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestRPCError_ImplementsError(t *testing.T) {
	var _ error = &RPCError{}
}

// ──────────────────────────────────────────────────────────────────────────────
// Error code constants tests
// ──────────────────────────────────────────────────────────────────────────────

func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		value int
	}{
		{"ErrCodeParseError", ErrCodeParseError, -32700},
		{"ErrCodeInvalidRequest", ErrCodeInvalidRequest, -32600},
		{"ErrCodeMethodNotFound", ErrCodeMethodNotFound, -32601},
		{"ErrCodeInvalidParams", ErrCodeInvalidParams, -32602},
		{"ErrCodeInternalError", ErrCodeInternalError, -32603},
		{"ErrCodePluginError", ErrCodePluginError, -32000},
		{"ErrCodeNotImplemented", ErrCodeNotImplemented, -32001},
		{"ErrCodeCapabilityError", ErrCodeCapabilityError, -32002},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.value {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.value)
			}
		})
	}

	// Verify JSON-RPC standard codes are in correct range (-32700 to -32600)
	standardCodes := []int{ErrCodeParseError, ErrCodeInvalidRequest, ErrCodeMethodNotFound, ErrCodeInvalidParams, ErrCodeInternalError}
	for _, code := range standardCodes {
		if code > -32600 || code < -32700 {
			t.Errorf("standard code %d is out of range [-32700, -32600]", code)
		}
	}

	// Verify custom codes are in correct range (-32000 to -32099)
	customCodes := []int{ErrCodePluginError, ErrCodeNotImplemented, ErrCodeCapabilityError}
	for _, code := range customCodes {
		if code > -32000 || code < -32099 {
			t.Errorf("custom code %d is out of range [-32099, -32000]", code)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Stream event type constants tests
// ──────────────────────────────────────────────────────────────────────────────

func TestStreamEventTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"StreamEventText", StreamEventText, "text"},
		{"StreamEventToolUse", StreamEventToolUse, "tool_use"},
		{"StreamEventToolResult", StreamEventToolResult, "tool_result"},
		{"StreamEventFile", StreamEventFile, "file"},
		{"StreamEventUsage", StreamEventUsage, "usage"},
		{"StreamEventComplete", StreamEventComplete, "complete"},
		{"StreamEventError", StreamEventError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Response type tests
// ──────────────────────────────────────────────────────────────────────────────

func TestResponse_JSONMarshal(t *testing.T) {
	tests := []struct {
		name string
		resp Response
	}{
		{
			name: "successful response",
			resp: Response{
				JSONRPC: "2.0",
				ID:      1,
				Result:  json.RawMessage(`{"status": "ok"}`),
			},
		},
		{
			name: "error response",
			resp: Response{
				JSONRPC: "2.0",
				ID:      2,
				Error: &RPCError{
					Code:    ErrCodeInternalError,
					Message: "something failed",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("json.Marshal error = %v", err)
			}

			var unmarshaled Response
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("json.Unmarshal error = %v", err)
			}

			if unmarshaled.JSONRPC != tt.resp.JSONRPC {
				t.Errorf("JSONRPC = %q, want %q", unmarshaled.JSONRPC, tt.resp.JSONRPC)
			}
			if unmarshaled.ID != tt.resp.ID {
				t.Errorf("ID = %d, want %d", unmarshaled.ID, tt.resp.ID)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Notification type tests
// ──────────────────────────────────────────────────────────────────────────────

func TestNotification_JSONMarshal(t *testing.T) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  "stream",
		Params:  map[string]string{"type": "text", "data": "hello"},
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	var unmarshaled map[string]any
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	// Notifications should not have an ID field
	if _, hasID := unmarshaled["id"]; hasID {
		t.Error("Notification should not have id field")
	}

	if unmarshaled["method"] != "stream" {
		t.Errorf("method = %v, want stream", unmarshaled["method"])
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// StreamEvent type tests
// ──────────────────────────────────────────────────────────────────────────────

func TestStreamEvent_JSONMarshal(t *testing.T) {
	tests := []struct {
		name  string
		event StreamEvent
	}{
		{
			name: "text event",
			event: StreamEvent{
				Type: StreamEventText,
				Data: json.RawMessage(`"hello world"`),
			},
		},
		{
			name: "file event",
			event: StreamEvent{
				Type: StreamEventFile,
				Data: json.RawMessage(`{"path": "test.go", "content": "package main"}`),
			},
		},
		{
			name: "complete event without data",
			event: StreamEvent{
				Type: StreamEventComplete,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("json.Marshal error = %v", err)
			}

			var unmarshaled StreamEvent
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("json.Unmarshal error = %v", err)
			}

			if unmarshaled.Type != tt.event.Type {
				t.Errorf("Type = %q, want %q", unmarshaled.Type, tt.event.Type)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Provider protocol types tests
// ──────────────────────────────────────────────────────────────────────────────

func TestProviderProtocolTypes_JSONRoundtrip(t *testing.T) {
	t.Run("InitParams", func(t *testing.T) {
		original := InitParams{
			Config: map[string]any{"key": "value", "num": float64(42)},
		}
		testJSONRoundtrip(t, original)
	})

	t.Run("MatchParams", func(t *testing.T) {
		original := MatchParams{Input: "github:123"}
		testJSONRoundtrip(t, original)
	})

	t.Run("MatchResult", func(t *testing.T) {
		original := MatchResult{Matches: true}
		testJSONRoundtrip(t, original)
	})

	t.Run("ParseParams", func(t *testing.T) {
		original := ParseParams{Input: "gh:owner/repo#123"}
		testJSONRoundtrip(t, original)
	})

	t.Run("ParseResult", func(t *testing.T) {
		original := ParseResult{ID: "123", Error: ""}
		testJSONRoundtrip(t, original)
	})

	t.Run("FetchParams", func(t *testing.T) {
		original := FetchParams{ID: "issue-123"}
		testJSONRoundtrip(t, original)
	})

	t.Run("AddCommentParams", func(t *testing.T) {
		original := AddCommentParams{WorkUnitID: "123", Body: "Test comment"}
		testJSONRoundtrip(t, original)
	})

	t.Run("UpdateStatusParams", func(t *testing.T) {
		original := UpdateStatusParams{WorkUnitID: "123", Status: "closed"}
		testJSONRoundtrip(t, original)
	})

	t.Run("CreatePRParams", func(t *testing.T) {
		original := CreatePRParams{
			WorkUnitID:   "123",
			Title:        "Test PR",
			Description:  "PR description",
			SourceBranch: "feature/test",
			TargetBranch: "main",
			Draft:        true,
		}
		testJSONRoundtrip(t, original)
	})

	t.Run("SnapshotParams", func(t *testing.T) {
		original := SnapshotParams{ID: "123"}
		testJSONRoundtrip(t, original)
	})

	t.Run("SnapshotResult", func(t *testing.T) {
		original := SnapshotResult{
			Content:  "# Task\n\nDescription here",
			Metadata: map[string]any{"key": "value"},
		}
		testJSONRoundtrip(t, original)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Agent protocol types tests
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentProtocolTypes_JSONRoundtrip(t *testing.T) {
	t.Run("AgentRunParams", func(t *testing.T) {
		original := AgentRunParams{
			Prompt:  "Test prompt",
			Env:     map[string]string{"API_KEY": "secret"},
			Options: map[string]any{"temperature": float64(0.7)},
		}
		testJSONRoundtrip(t, original)
	})

	t.Run("AgentAvailableResult", func(t *testing.T) {
		original := AgentAvailableResult{Available: true, Error: ""}
		testJSONRoundtrip(t, original)
	})

	t.Run("AgentInitResult", func(t *testing.T) {
		original := AgentInitResult{
			Capabilities: []string{"streaming", "tools"},
			Metadata: &AgentMetadataResult{
				Name:        "test-agent",
				Version:     "1.0.0",
				Description: "A test agent",
				Models:      []string{"model-a", "model-b"},
			},
		}
		testJSONRoundtrip(t, original)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Workflow protocol types tests
// ──────────────────────────────────────────────────────────────────────────────

func TestWorkflowProtocolTypes_JSONRoundtrip(t *testing.T) {
	t.Run("WorkflowInitResult", func(t *testing.T) {
		original := WorkflowInitResult{
			Phases: []PhaseInfo{
				{Name: "custom-phase", Description: "A custom phase", After: "planning"},
			},
			Guards: []GuardInfo{
				{Name: "custom-guard", Description: "A custom guard"},
			},
			Effects: []EffectInfo{
				{Name: "custom-effect", Description: "A custom effect"},
			},
		}
		testJSONRoundtrip(t, original)
	})

	t.Run("EvaluateGuardParams", func(t *testing.T) {
		original := EvaluateGuardParams{
			Name:     "my-guard",
			WorkUnit: map[string]any{"id": "123", "status": "open"},
		}
		testJSONRoundtrip(t, original)
	})

	t.Run("EvaluateGuardResult", func(t *testing.T) {
		original := EvaluateGuardResult{Passed: false, Reason: "Condition not met"}
		testJSONRoundtrip(t, original)
	})

	t.Run("ExecuteEffectParams", func(t *testing.T) {
		original := ExecuteEffectParams{
			Name:     "my-effect",
			WorkUnit: map[string]any{"id": "123"},
			Data:     map[string]any{"extra": "data"},
		}
		testJSONRoundtrip(t, original)
	})

	t.Run("ExecuteEffectResult", func(t *testing.T) {
		original := ExecuteEffectResult{
			Success: true,
			Error:   "",
			Data:    map[string]any{"result": "ok"},
		}
		testJSONRoundtrip(t, original)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// WorkUnitResult tests
// ──────────────────────────────────────────────────────────────────────────────

func TestWorkUnitResult_JSONRoundtrip(t *testing.T) {
	original := WorkUnitResult{
		ID:          "123",
		ExternalID:  "EXT-123",
		Provider:    "github",
		Title:       "Test Issue",
		Description: "Issue description",
		Status:      "open",
		Priority:    1,
		Labels:      []string{"bug", "urgent"},
		Assignees: []PersonResult{
			{ID: "user1", Name: "User One", Email: "user1@test.com"},
		},
		Comments: []CommentResult{
			{ID: "c1", Body: "First comment", Author: PersonResult{Name: "Author"}},
		},
		Attachments: []AttachmentResult{
			{ID: "a1", Name: "file.txt", URL: "http://example.com/file.txt"},
		},
		Subtasks:    []string{"task1", "task2"},
		ExternalKey: "GH-123",
		TaskType:    "issue",
		Slug:        "test-issue",
		Source: SourceInfoResult{
			Reference: "github:123",
			URL:       "https://github.com/owner/repo/issues/123",
		},
		Metadata: map[string]any{"custom": "data"},
	}

	testJSONRoundtrip(t, original)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────────────────────────────────────

// testJSONRoundtrip tests that a value can be marshaled and unmarshaled
func testJSONRoundtrip[T any](t *testing.T, original T) {
	t.Helper()

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	var unmarshaled T
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	// Re-marshal to compare (handles field ordering differences)
	data1, _ := json.Marshal(original)
	data2, _ := json.Marshal(unmarshaled)

	if string(data1) != string(data2) {
		t.Errorf("roundtrip mismatch:\noriginal:    %s\nunmarshaled: %s", data1, data2)
	}
}
