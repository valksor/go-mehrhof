package conductor

import (
	"strings"
	"testing"
)

func TestExtractDecisions_Empty(t *testing.T) {
	decisions := ExtractDecisions(nil)
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions, got %d", len(decisions))
	}

	decisions = ExtractDecisions([]map[string]any{})
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions from empty slice, got %d", len(decisions))
	}
}

func TestExtractDecisions_WithToolUse(t *testing.T) {
	records := []map[string]any{
		{
			"type":      "tool_use",
			"tool":      "write_file",
			"action":    "create",
			"file":      "pkg/foo/bar.go",
			"reasoning": "Adding new handler for task creation",
		},
		{
			"type":    "text",
			"content": "Thinking about the implementation...",
		},
		{
			"type":      "tool_use",
			"tool":      "edit_file",
			"action":    "modify",
			"file":      "pkg/foo/baz.go",
			"reasoning": "Fix import ordering",
		},
		{
			"type": "tool_use",
			// No tool or action - should be skipped.
		},
	}

	decisions := ExtractDecisions(records)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}

	if decisions[0].Tool != "write_file" {
		t.Errorf("expected tool 'write_file', got %q", decisions[0].Tool)
	}

	if decisions[0].File != "pkg/foo/bar.go" {
		t.Errorf("expected file 'pkg/foo/bar.go', got %q", decisions[0].File)
	}

	if decisions[1].Tool != "edit_file" {
		t.Errorf("expected tool 'edit_file', got %q", decisions[1].Tool)
	}
}

func TestExtractDecisions_LimitTo20(t *testing.T) {
	records := make([]map[string]any, 30)
	for i := range records {
		records[i] = map[string]any{
			"type": "tool_use",
			"tool": "write_file",
		}
	}

	decisions := ExtractDecisions(records)
	if len(decisions) != 20 {
		t.Fatalf("expected max 20 decisions, got %d", len(decisions))
	}
}

func TestFormatDecisionsMarkdown(t *testing.T) {
	decisions := []KeyDecision{
		{
			Tool:      "write_file",
			Action:    "create",
			File:      "pkg/handler.go",
			Reasoning: "New endpoint needed",
		},
		{
			Tool:   "run_command",
			Action: "test",
		},
	}

	result := FormatDecisionsMarkdown(decisions, "3 files changed, 120 insertions(+), 5 deletions(-)")

	if !strings.Contains(result, "<details>") {
		t.Error("expected collapsible details tag")
	}

	if !strings.Contains(result, "AI Agent Decisions") {
		t.Error("expected summary heading")
	}

	if !strings.Contains(result, "**write_file** - create") {
		t.Error("expected first decision")
	}

	if !strings.Contains(result, "`pkg/handler.go`") {
		t.Error("expected file reference")
	}

	if !strings.Contains(result, "3 files changed") {
		t.Error("expected diff stat")
	}

	// Test empty decisions.
	empty := FormatDecisionsMarkdown(nil, "")
	if !strings.Contains(empty, "No key decisions recorded") {
		t.Error("expected no decisions message for empty input")
	}
}
