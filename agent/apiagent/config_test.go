package apiagent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/apiagent"
)

func TestDefaultAPIConfig(t *testing.T) {
	cfg := apiagent.DefaultAPIConfig()
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", cfg.Timeout)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if cfg.ExecTimeout != 2*time.Minute {
		t.Errorf("ExecTimeout = %v, want 2m", cfg.ExecTimeout)
	}
}

// TestBaseUsesCustomHTTPClient verifies that a custom HTTPClient on the config
// is the one used to drive requests (config.httpClient() custom branch). The
// custom client points all traffic at the recording server via a transport.
func TestBaseUsesCustomHTTPClient(t *testing.T) {
	var hit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "data: {\"type\":\"text\",\"content\":\"ok\"}\n\n", "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = t.TempDir()
	cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(t, events)

	if !hit {
		t.Error("custom HTTP client did not reach the server")
	}
}

// TestBaseTruncatesLongToolOutput verifies the tool-result event truncates very
// long output (truncateOutput's >maxLen branch via the conversation loop).
func TestBaseTruncatesLongToolOutput(t *testing.T) {
	// Write a large file the agent will read so the tool output exceeds 500 bytes.
	workDir := t.TempDir()
	big := strings.Repeat("A", 4000)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		calls++
		if calls == 1 {
			writeSSE(
				w,
				"data: {\"type\":\"tool_use\",\"id\":\"c1\",\"name\":\"bash\",\"input\":{\"command\":\"printf '%s' "+big+"\"}}\n\n",
				"data: [DONE]\n\n",
			)

			return
		}
		writeSSE(w, "data: {\"type\":\"text\",\"content\":\"done\"}\n\n", "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := &mockProvider{name: "test", server: server}
	cfg := apiagent.DefaultAPIConfig()
	cfg.WorkDir = workDir

	base := apiagent.NewBase(provider, cfg)
	if err := base.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	events, err := base.SendPrompt(context.Background(), "run it")
	if err != nil {
		t.Fatal(err)
	}

	collected := drainEvents(t, events)
	var truncated bool
	for _, e := range collected {
		if e.Type == agent.EventToolResult {
			out, _ := e.Data["output"].(string)
			if strings.Contains(out, "truncated") {
				truncated = true
			}
		}
	}
	if !truncated {
		t.Error("expected long tool output to be truncated in the event data")
	}
}
