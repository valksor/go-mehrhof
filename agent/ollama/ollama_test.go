package ollama_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/agent/apiagent"
	"github.com/valksor/kvelmo/agent/ollama"
)

//nolint:errcheck // Test helper; write errors to httptest are not actionable
func writeSSE(w http.ResponseWriter, lines ...string) {
	for _, line := range lines {
		fmt.Fprint(w, line)
	}
}

func TestParseStreamNativeText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		writeSSE(w,
			"{\"message\":{\"role\":\"assistant\",\"content\":\"Hi!\"},\"done\":false}\n",
			"{\"message\":{\"role\":\"assistant\",\"content\":\"\"},\"done\":true,\"done_reason\":\"stop\"}\n",
		)
	}))
	defer server.Close()

	provider := ollama.NewProvider(ollama.Config{BaseURL: server.URL})
	cfg := &apiagent.APIConfig{BaseURL: server.URL, Model: "llama3.1"}

	req, err := provider.BuildRequest(context.Background(), cfg, []apiagent.Message{
		{Role: apiagent.RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // Body ownership transferred to ParseStream
	if err != nil {
		t.Fatal(err)
	}

	chunks, err := provider.ParseStream(context.Background(), resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var text string
	var textSb strings.Builder
	for c := range chunks {
		if c.Type == apiagent.ChunkText {
			textSb.WriteString(c.Text)
		}
	}
	text = textSb.String()

	if text != "Hi!" {
		t.Errorf("expected 'Hi!', got %q", text)
	}
}

func TestParseStreamNativeToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		writeSSE(w,
			"{\"message\":{\"role\":\"assistant\",\"content\":\"\",\"tool_calls\":[{\"id\":\"call_1\",\"function\":{\"name\":\"read_file\",\"arguments\":{\"path\":\"/etc/hostname\"}}}]},\"done\":false}\n",
			"{\"message\":{\"role\":\"assistant\",\"content\":\"\"},\"done\":true,\"done_reason\":\"stop\"}\n",
		)
	}))
	defer server.Close()

	provider := ollama.NewProvider(ollama.Config{BaseURL: server.URL})
	cfg := &apiagent.APIConfig{BaseURL: server.URL, Model: "llama3.1"}

	req, err := provider.BuildRequest(context.Background(), cfg, []apiagent.Message{
		{Role: apiagent.RoleUser, Content: "read hostname"},
	}, apiagent.KvelmoTools())
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // Body ownership transferred to ParseStream
	if err != nil {
		t.Fatal(err)
	}

	chunks, err := provider.ParseStream(context.Background(), resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var toolUse *apiagent.ToolUseChunk
	for c := range chunks {
		if c.Type == apiagent.ChunkToolUse && c.ToolUse != nil {
			toolUse = c.ToolUse
		}
	}

	if toolUse == nil {
		t.Fatal("expected a tool use chunk")
	}

	if toolUse.Name != "read_file" {
		t.Errorf("expected name 'read_file', got %q", toolUse.Name)
	}

	path, ok := toolUse.Input["path"].(string)
	if !ok || path != "/etc/hostname" {
		t.Errorf("expected path '/etc/hostname', got %v", toolUse.Input)
	}
}

func TestProviderName(t *testing.T) {
	p := ollama.NewProvider(ollama.Config{})
	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got %q", p.Name())
	}
}

func TestProviderAvailableServerDown(t *testing.T) {
	p := ollama.NewProvider(ollama.Config{BaseURL: "http://127.0.0.1:1"})
	if err := p.Available(); err == nil {
		t.Error("expected error when server not reachable")
	}
}

func TestProviderAvailableServerUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			writeSSE(w, `{"models":[]}`)
		}
	}))
	defer server.Close()

	p := ollama.NewProvider(ollama.Config{BaseURL: server.URL})
	if err := p.Available(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnectModelExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/show":
			w.WriteHeader(http.StatusOK)
			writeSSE(w, `{"modelfile":"..."}`)
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
			writeSSE(w, `{"models":[]}`)
		}
	}))
	defer server.Close()

	provider := ollama.NewProvider(ollama.Config{BaseURL: server.URL, Model: "llama3.1"})
	cfg := &apiagent.APIConfig{Model: "llama3.1"}

	if err := provider.Connect(context.Background(), cfg); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

func TestConnectModelPulled(t *testing.T) {
	pullCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/show":
			w.WriteHeader(http.StatusNotFound)
		case "/api/pull":
			pullCalled = true
			w.WriteHeader(http.StatusOK)
			writeSSE(w,
				"{\"status\":\"pulling manifest\"}\n",
				"{\"status\":\"success\"}\n",
			)
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
			writeSSE(w, `{"models":[]}`)
		}
	}))
	defer server.Close()

	provider := ollama.NewProvider(ollama.Config{BaseURL: server.URL, Model: "llama3.1"})
	cfg := &apiagent.APIConfig{Model: "llama3.1"}

	if err := provider.Connect(context.Background(), cfg); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !pullCalled {
		t.Error("expected /api/pull to be called for missing model")
	}
}

func TestConnectPullError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/show":
			w.WriteHeader(http.StatusNotFound)
		case "/api/pull":
			w.WriteHeader(http.StatusOK)
			writeSSE(w, "{\"error\":\"model not found in registry\"}\n")
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
			writeSSE(w, `{"models":[]}`)
		}
	}))
	defer server.Close()

	provider := ollama.NewProvider(ollama.Config{BaseURL: server.URL, Model: "nonexistent"})
	cfg := &apiagent.APIConfig{Model: "nonexistent"}

	err := provider.Connect(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for failed pull")
	}

	if !strings.Contains(err.Error(), "pull failed") {
		t.Errorf("expected 'pull failed' in error, got: %v", err)
	}
}

func TestBuildRequestUsesNativeEndpoint(t *testing.T) {
	provider := ollama.NewProvider(ollama.Config{BaseURL: "http://localhost:11434"})
	cfg := &apiagent.APIConfig{BaseURL: "http://localhost:11434", Model: "llama3.1"}

	req, err := provider.BuildRequest(context.Background(), cfg, []apiagent.Message{
		{Role: apiagent.RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(req.URL.Path, "/api/chat") {
		t.Errorf("expected /api/chat endpoint, got %s", req.URL.Path)
	}
}
