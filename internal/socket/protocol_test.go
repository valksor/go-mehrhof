package socket

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := &Request{
		JSONRPC: "2.0",
		ID:      "test-1",
		Method:  "ping",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", parsed["jsonrpc"])
	}
	if parsed["id"] != "test-1" {
		t.Errorf("id = %v, want test-1", parsed["id"])
	}
	if parsed["method"] != "ping" {
		t.Errorf("method = %v, want ping", parsed["method"])
	}
}

func TestRequestWithParams(t *testing.T) {
	params := map[string]any{"path": "/test/path"}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal params error = %v", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		ID:      "test-2",
		Method:  "projects.register",
		Params:  paramsJSON,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	var parsedParams map[string]any
	if err := json.Unmarshal(parsed.Params, &parsedParams); err != nil {
		t.Fatalf("Params unmarshal error = %v", err)
	}

	if parsedParams["path"] != "/test/path" {
		t.Errorf("params.path = %v, want /test/path", parsedParams["path"])
	}
}

func TestResponseMarshal(t *testing.T) {
	result := map[string]any{"status": "ok"}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal result error = %v", err)
	}

	resp := &Response{
		JSONRPC: "2.0",
		ID:      "test-3",
		Result:  resultJSON,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", parsed["jsonrpc"])
	}
	if parsed["id"] != "test-3" {
		t.Errorf("id = %v, want test-3", parsed["id"])
	}
}

func TestErrorResponseMarshal(t *testing.T) {
	resp := NewErrorResponse("test-4", -32600, "Invalid request")

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	errObj, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatal("error should be an object")
	}

	code, ok := errObj["code"].(float64)
	if !ok {
		t.Fatal("error.code should be a number")
	}
	if code != -32600 {
		t.Errorf("error.code = %v, want -32600", errObj["code"])
	}
	if errObj["message"] != "Invalid request" {
		t.Errorf("error.message = %v, want 'Invalid request'", errObj["message"])
	}
}

func TestNewResultResponse(t *testing.T) {
	result := map[string]string{"key": "value"}
	resp, err := NewResultResponse("test-5", result)
	if err != nil {
		t.Fatalf("NewResultResponse error = %v", err)
	}

	if resp.ID != "test-5" {
		t.Errorf("ID = %v, want test-5", resp.ID)
	}
	if resp.Error != nil {
		t.Error("Error should be nil")
	}

	var parsed map[string]string
	if err := json.Unmarshal(resp.Result, &parsed); err != nil {
		t.Fatalf("Result unmarshal error = %v", err)
	}

	if parsed["key"] != "value" {
		t.Errorf("result.key = %v, want value", parsed["key"])
	}
}

func TestRequestIDTypes(t *testing.T) {
	// String ID
	req1 := &Request{JSONRPC: "2.0", ID: "string-id", Method: "test"}
	data1, err := json.Marshal(req1)
	if err != nil {
		t.Fatalf("Marshal req1 error = %v", err)
	}
	var parsed1 Request
	if err := json.Unmarshal(data1, &parsed1); err != nil {
		t.Fatalf("Unmarshal parsed1 error = %v", err)
	}
	if parsed1.ID != "string-id" {
		t.Errorf("string ID = %v, want string-id", parsed1.ID)
	}

	// UUID style ID
	req2 := &Request{JSONRPC: "2.0", ID: "550e8400-e29b-41d4-a716-446655440000", Method: "test"}
	data2, err := json.Marshal(req2)
	if err != nil {
		t.Fatalf("Marshal req2 error = %v", err)
	}
	var parsed2 Request
	if err := json.Unmarshal(data2, &parsed2); err != nil {
		t.Fatalf("Unmarshal parsed2 error = %v", err)
	}
	if parsed2.ID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("UUID ID = %v, want 550e8400-e29b-41d4-a716-446655440000", parsed2.ID)
	}
}

func TestErrorCodes(t *testing.T) {
	codes := map[int]string{
		-32700: "parse error",
		-32600: "invalid request",
		-32601: "method not found",
		-32602: "invalid params",
		-32603: "internal error",
	}

	for code, desc := range codes {
		resp := NewErrorResponse("test", code, desc)
		if resp.Error.Code != code {
			t.Errorf("code %d: Error.Code = %d", code, resp.Error.Code)
		}
	}
}

func TestRPCError_Error(t *testing.T) {
	e := &RPCError{Code: -32600, Message: "Invalid request"}
	got := e.Error()
	if got != "rpc error -32600: Invalid request" {
		t.Errorf("RPCError.Error() = %q, want \"rpc error -32600: Invalid request\"", got)
	}
}

func TestEncodeEvent_Success(t *testing.T) {
	event := map[string]string{"type": "ping"}
	data, err := encodeEvent(event)
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("encodeEvent() returned empty data")
	}
	// Should end with newline
	if data[len(data)-1] != '\n' {
		t.Error("encodeEvent() result should end with newline")
	}
}

func TestServer_ActiveConnections_Zero(t *testing.T) {
	s := NewServer("/tmp/kvelmo-test-server.sock")
	if got := s.ActiveConnections(); got != 0 {
		t.Errorf("ActiveConnections() on new server = %d, want 0", got)
	}
}

func TestServer_Path(t *testing.T) {
	s := NewServer("/tmp/kvelmo-test-path.sock")
	if got := s.Path(); got != "/tmp/kvelmo-test-path.sock" {
		t.Errorf("Path() = %q, want /tmp/kvelmo-test-path.sock", got)
	}
}
