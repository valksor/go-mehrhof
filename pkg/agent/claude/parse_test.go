package claude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"nil input", nil, ""},
		{"empty input", json.RawMessage(""), ""},
		{"simple string", json.RawMessage(`"hello world"`), "hello world"},
		{"empty string", json.RawMessage(`""`), ""},
		{"single text block", json.RawMessage(`[{"type":"text","text":"hello"}]`), "hello"},
		{"multiple text blocks", json.RawMessage(`[{"type":"text","text":"hello"},{"type":"text","text":"world"}]`), "hello\nworld"},
		{"mixed block types", json.RawMessage(`[{"type":"text","text":"hello"},{"type":"tool_use","text":"ignored"},{"type":"text","text":"world"}]`), "hello\nworld"},
		{"no text blocks", json.RawMessage(`[{"type":"tool_use","id":"123"}]`), ""},
		{"empty text in block", json.RawMessage(`[{"type":"text","text":""}]`), ""},
		{"invalid json", json.RawMessage(`not json`), ""},
		{"number (not string or array)", json.RawMessage(`42`), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextContent(tt.raw)
			if got != tt.want {
				t.Errorf("extractTextContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckLocalOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"no origin header", "", true},
		{"localhost with port", "http://localhost:3000", true},
		{"localhost no port", "http://localhost", true},
		{"127.0.0.1 with port", "http://127.0.0.1:8080", true},
		{"127.0.0.1 no port", "http://127.0.0.1", true},
		{"external origin", "https://example.com", false},
		{"https localhost", "https://localhost:3000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			got := checkLocalOrigin(req)
			if got != tt.want {
				t.Errorf("checkLocalOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}
