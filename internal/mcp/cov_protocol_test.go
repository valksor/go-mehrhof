package mcp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestError_ErrorMethod(t *testing.T) {
	e := &Error{Code: InvalidParams, Message: "bad params"}
	if e.Error() != "bad params" {
		t.Errorf("Error() = %q, want %q", e.Error(), "bad params")
	}
}

func TestRequestRoundTrip(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`42`),
		Method:  MethodToolsCall,
		Params:  json.RawMessage(`{"name":"x"}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.JSONRPC != req.JSONRPC || got.Method != req.Method {
		t.Errorf("round trip mismatch: %+v vs %+v", got, req)
	}
	if string(got.ID) != "42" {
		t.Errorf("ID = %s", got.ID)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  json.RawMessage(`{"ok":true}`),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	// Error must be omitted when nil.
	if got := string(data); got == "" || containsKey(t, data, "error") {
		t.Errorf("error key should be omitted: %s", got)
	}
	var back Response
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if string(back.Result) != `{"ok":true}` {
		t.Errorf("result = %s", back.Result)
	}
}

func TestErrorResponseRoundTrip(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`7`),
		Error:   &Error{Code: MethodNotFound, Message: "nope", Data: json.RawMessage(`{"hint":"x"}`)},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	// Result must be omitted when empty.
	if containsKey(t, data, "result") {
		t.Errorf("result key should be omitted: %s", data)
	}
	var back Response
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Error == nil || back.Error.Code != MethodNotFound {
		t.Errorf("error = %+v", back.Error)
	}
}

func TestInitializeParamsRoundTrip(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ClientCapabilities{
			Roots:        &RootsCapability{ListChanged: true},
			Sampling:     &SamplingCap{},
			Experimental: map[string]any{"feature": true},
		},
		ClientInfo: ClientInfo{Name: "claude", Version: "2.1"},
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var got InitializeParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ClientInfo.Name != "claude" || got.ProtocolVersion != ProtocolVersion {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if got.Capabilities.Roots == nil || !got.Capabilities.Roots.ListChanged {
		t.Error("roots capability lost in round trip")
	}
}

func TestInitializeResultRoundTrip(t *testing.T) {
	res := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapabilities{ListChanged: false},
			Resources: &ResourcesCapabilities{Subscribe: true},
			Prompts:   &PromptsCapabilities{ListChanged: true},
		},
		ServerInfo: ServerInfo{Name: "kvelmo-mcp", Version: "1.0.0"},
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var got InitializeResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ServerInfo.Name != "kvelmo-mcp" {
		t.Errorf("server info lost: %+v", got.ServerInfo)
	}
	if got.Capabilities.Resources == nil || !got.Capabilities.Resources.Subscribe {
		t.Error("resources capability lost")
	}
}

func TestToolCallRoundTrips(t *testing.T) {
	params := ToolCallParams{Name: "kvelmo_get_task", Arguments: map[string]any{"x": "y"}}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var gotParams ToolCallParams
	if err := json.Unmarshal(data, &gotParams); err != nil {
		t.Fatal(err)
	}
	if gotParams.Name != params.Name || !reflect.DeepEqual(gotParams.Arguments, params.Arguments) {
		t.Errorf("params round trip mismatch: %+v", gotParams)
	}

	result := ToolCallResult{
		Content: []ContentBlock{{Type: ContentTypeText, Text: "hello"}},
		IsError: true,
	}
	rdata, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var gotResult ToolCallResult
	if err := json.Unmarshal(rdata, &gotResult); err != nil {
		t.Fatal(err)
	}
	if !gotResult.IsError || gotResult.Content[0].Text != "hello" {
		t.Errorf("result round trip mismatch: %+v", gotResult)
	}
}

func TestRendezvousEventRoundTrip(t *testing.T) {
	evt := RendezvousEvent{Type: "failure", Phase: "implement", Reason: "boom", Retryable: true}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var got RendezvousEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, evt) {
		t.Errorf("round trip mismatch: %+v vs %+v", got, evt)
	}
}

func TestSetRendezvousSink_NilIsNoop(t *testing.T) {
	SetRendezvousSink(nil)
	// Must not panic with a nil sink.
	notifyRendezvous(RendezvousEvent{Type: "complete"})

	got := make(chan RendezvousEvent, 1)
	SetRendezvousSink(func(e RendezvousEvent) { got <- e })
	defer SetRendezvousSink(nil)

	notifyRendezvous(RendezvousEvent{Type: "complete", Phase: "plan"})
	select {
	case e := <-got:
		if e.Phase != "plan" {
			t.Errorf("got event %+v", e)
		}
	default:
		t.Fatal("sink was not invoked")
	}
}

// containsKey reports whether the top-level JSON object has the given key.
func containsKey(t *testing.T, data []byte, key string) bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	_, ok := m[key]

	return ok
}
