package claudesdk

import (
	"encoding/json"
	"slices"
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

func TestLocalAcceptOptions_OriginPatterns(t *testing.T) {
	patterns := localAcceptOptions.OriginPatterns

	// Must include localhost and 127.0.0.1 patterns
	expectedPatterns := []string{"localhost:*", "127.0.0.1:*", "[::1]:*"}
	for _, expected := range expectedPatterns {
		if !slices.Contains(patterns, expected) {
			t.Errorf("localAcceptOptions.OriginPatterns missing %q, got %v", expected, patterns)
		}
	}

	// Must not include wildcard (security)
	if slices.Contains(patterns, "*") {
		t.Error("localAcceptOptions.OriginPatterns should not allow all origins")
	}
}
