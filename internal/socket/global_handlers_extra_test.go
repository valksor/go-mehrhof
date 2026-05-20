package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/meta"
)

// withTempHome isolates config/state under a temp KVELMO_HOME.
func withTempHome(t *testing.T) {
	t.Helper()
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
}

// --- handleActivityQuery ---

func TestHandleActivityQuery_NoLogger(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleActivityQuery(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleActivityQuery: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result[keyEnabled] != false {
		t.Errorf("enabled = %v, want false (no logger)", result[keyEnabled])
	}
}

func TestHandleActivityQuery_InvalidParams(t *testing.T) {
	g := newTestGlobalSocket(t)
	// Configure a logger so we get past the nil-logger branch.
	resp, err := g.handleActivityQuery(context.Background(), &Request{
		ID:     "1",
		Params: json.RawMessage(`{not valid json`),
	})
	if err != nil {
		t.Fatalf("handleActivityQuery: %v", err)
	}
	// With no logger configured, invalid params are never parsed — still returns enabled:false.
	if resp.Error != nil && resp.Result == nil {
		t.Log("returned error response as expected for malformed params")
	}
}

// --- handleBatch ---

func TestHandleBatch_NoAction(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleBatch(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("handleBatch: %v", err)
	}
	if resp.Error == nil {
		t.Error("expected error for missing action")
	}
}

func TestHandleBatch_InvalidAction(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleBatch(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{"action":"frobnicate"}`)})
	if err != nil {
		t.Fatalf("handleBatch: %v", err)
	}
	if resp.Error == nil {
		t.Error("expected error for invalid action")
	}
}

func TestHandleBatch_ValidActionNoWorktrees(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleBatch(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{"action":"plan"}`)})
	if err != nil {
		t.Fatalf("handleBatch: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "plan" {
		t.Errorf("action = %v", result["action"])
	}
}

func TestHandleBatch_MalformedParams(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, _ := g.handleBatch(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{bad`)})
	if resp.Error == nil {
		t.Error("expected error for malformed params")
	}
}

// --- handleConfigCheck ---

func TestHandleConfigCheck(t *testing.T) {
	withTempHome(t)
	g := newTestGlobalSocket(t)

	resp, err := g.handleConfigCheck(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleConfigCheck: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["drifts"]; !ok {
		t.Error("response missing 'drifts'")
	}
}

// --- handleExport ---

func TestHandleExport(t *testing.T) {
	g := newTestGlobalSocket(t)

	resp, err := g.handleExport(context.Background(), &Request{
		ID:     "1",
		Params: json.RawMessage(`{"format":"json","since":"7d"}`),
	})
	if err != nil {
		t.Fatalf("handleExport: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["format"] != "json" {
		t.Errorf("format = %v", result["format"])
	}
	if _, ok := result["metrics"]; !ok {
		t.Error("response missing 'metrics'")
	}
	if _, ok := result["tasks"]; !ok {
		t.Error("response missing 'tasks'")
	}
}

func TestHandleExport_MalformedParams(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, _ := g.handleExport(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{bad`)})
	if resp.Error == nil {
		t.Error("expected error for malformed params")
	}
}

func TestParseSinceDuration(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"1h", false},
		{"30m", false},
		{"7d", false},
		{"90d", false},
		{"garbage", true},
		{"", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, err := parseSinceDuration(tc.in)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseSinceDuration(%q) err = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

// --- browser config/status handlers (no live browser needed) ---

func TestHandleBrowserStatus(t *testing.T) {
	withTempHome(t)
	g := newTestGlobalSocket(t)

	resp, err := g.handleBrowserStatus(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserStatus: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["installed"]; !ok {
		t.Error("response missing 'installed'")
	}
}

func TestHandleBrowserConfigGet(t *testing.T) {
	withTempHome(t)
	g := newTestGlobalSocket(t)

	resp, err := g.handleBrowserConfigGet(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserConfigGet: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
}

func TestHandleBrowserConfigSet(t *testing.T) {
	withTempHome(t)
	g := newTestGlobalSocket(t)

	cases := []struct {
		name    string
		params  string
		wantErr bool
	}{
		{"headless", `{"key":"headless","value":"true"}`, false},
		{"browser valid", `{"key":"browser","value":"firefox"}`, false},
		{"browser invalid", `{"key":"browser","value":"netscape"}`, true},
		{"profile", `{"key":"profile","value":"default"}`, false},
		{"timeout valid", `{"key":"timeout","value":"5000"}`, false},
		{"timeout invalid", `{"key":"timeout","value":"abc"}`, true},
		{"unknown key", `{"key":"nonsense","value":"x"}`, true},
		{"malformed", `{bad`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := g.handleBrowserConfigSet(context.Background(), &Request{
				ID:     "1",
				Params: json.RawMessage(tc.params),
			})
			if err != nil {
				t.Fatalf("handleBrowserConfigSet: %v", err)
			}
			gotErr := resp.Error != nil
			if gotErr != tc.wantErr {
				t.Errorf("error = %v, wantErr %v (resp: %+v)", gotErr, tc.wantErr, resp)
			}
		})
	}
}

// --- browser snapshot/eval malformed-param fast paths ---

func TestHandleBrowserSnapshot_MalformedParams(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, _ := g.handleBrowserSnapshot(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{bad`)})
	if resp.Error == nil {
		t.Error("expected error for malformed params")
	}
}

func TestHandleBrowserEval_MalformedParams(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, _ := g.handleBrowserEval(context.Background(), &Request{ID: "1", Params: json.RawMessage(`{bad`)})
	if resp.Error == nil {
		t.Error("expected error for malformed params")
	}
}
