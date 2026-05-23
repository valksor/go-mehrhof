package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRegister_EmptyNameIgnored(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(Tool{Name: ""}, func(context.Context, map[string]any) (*ToolCallResult, error) {
		// Executor body is never invoked: an empty-name tool is ignored at
		// registration. Return a non-nil result to satisfy the nilnil linter.
		return textResult("unreachable"), nil
	})
	if len(reg.ListTools()) != 0 {
		t.Errorf("empty-name tool should be ignored, got %d tools", len(reg.ListTools()))
	}
}

func TestRegister_ReplaceExisting(t *testing.T) {
	reg := NewToolRegistry()
	exec1 := func(context.Context, map[string]any) (*ToolCallResult, error) { return textResult("v1"), nil }
	exec2 := func(context.Context, map[string]any) (*ToolCallResult, error) { return textResult("v2"), nil }
	reg.Register(Tool{Name: "dup"}, exec1)
	reg.Register(Tool{Name: "dup"}, exec2)

	if len(reg.ListTools()) != 1 {
		t.Fatalf("expected 1 tool after replace, got %d", len(reg.ListTools()))
	}
	res, err := reg.CallTool(context.Background(), "dup", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Content[0].Text != "v2" {
		t.Errorf("expected replacement executor, got %q", res.Content[0].Text)
	}
}

func TestCallTool_NotFound(t *testing.T) {
	reg := NewToolRegistry()
	_, err := reg.CallTool(context.Background(), "ghost", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestCallTool_ExecutorError(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(Tool{Name: "boom"}, func(context.Context, map[string]any) (*ToolCallResult, error) {
		return nil, errors.New("executor failed")
	})
	_, err := reg.CallTool(context.Background(), "boom", nil)
	if err == nil {
		t.Fatal("expected executor error to propagate")
	}
}

func TestCallTool_AppliesDefaultDeadline(t *testing.T) {
	reg := NewToolRegistry()
	var sawDeadline bool
	reg.Register(Tool{Name: "checkdeadline"}, func(ctx context.Context, _ map[string]any) (*ToolCallResult, error) {
		_, sawDeadline = ctx.Deadline()

		return textResult("ok"), nil
	})
	if _, err := reg.CallTool(context.Background(), "checkdeadline", nil); err != nil {
		t.Fatal(err)
	}
	if !sawDeadline {
		t.Error("CallTool should attach a default deadline when none present")
	}
}

func TestCallTool_RespectsExistingDeadline(t *testing.T) {
	reg := NewToolRegistry()
	var deadline time.Time
	reg.Register(Tool{Name: "t"}, func(ctx context.Context, _ map[string]any) (*ToolCallResult, error) {
		deadline, _ = ctx.Deadline()

		return textResult("ok"), nil
	})
	want := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), want)
	defer cancel()
	if _, err := reg.CallTool(ctx, "t", nil); err != nil {
		t.Fatal(err)
	}
	if !deadline.Equal(want) {
		t.Errorf("existing deadline replaced: got %v want %v", deadline, want)
	}
}

func TestCallTool_ValidationFailureReturnsErrorResult(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(Tool{
		Name: "needs_arg",
		InputSchema: map[string]any{
			schemaKeyType:     schemaTypeObject,
			schemaKeyProps:    map[string]any{"a": map[string]any{schemaKeyType: schemaTypeString}},
			schemaKeyRequired: []string{"a"},
		},
	}, func(context.Context, map[string]any) (*ToolCallResult, error) {
		return textResult("should not run"), nil
	})

	res, err := reg.CallTool(context.Background(), "needs_arg", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	if !res.IsError {
		t.Error("expected validation error result")
	}
}

func TestValidateArgs_NilSchema(t *testing.T) {
	if err := validateArgs(nil, map[string]any{"x": 1}); err != nil {
		t.Errorf("nil schema should accept anything: %v", err)
	}
}

func TestValidateArgs_RequiredAsAnySlice(t *testing.T) {
	// JSON unmarshalling produces []any, not []string.
	schema := map[string]any{
		schemaKeyType:     schemaTypeObject,
		schemaKeyProps:    map[string]any{"name": map[string]any{schemaKeyType: schemaTypeString}},
		schemaKeyRequired: []any{"name"},
	}
	if err := validateArgs(schema, map[string]any{}); err == nil {
		t.Error("expected missing required error")
	}
	if err := validateArgs(schema, map[string]any{"name": "x"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateArgs_NumericRange(t *testing.T) {
	schema := map[string]any{
		schemaKeyType: schemaTypeObject,
		schemaKeyProps: map[string]any{
			"count": map[string]any{
				schemaKeyType: "integer",
				"minimum":     float64(1),
				"maximum":     float64(10),
			},
		},
	}
	tests := []struct {
		name    string
		val     any
		wantErr bool
	}{
		{"in range", float64(5), false},
		{"at minimum", float64(1), false},
		{"at maximum", float64(10), false},
		{"below minimum", float64(0), true},
		{"above maximum", float64(11), true},
		{"int kind", 5, false},
		{"json number", json.Number("7"), false},
		{"not a number", "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgs(schema, map[string]any{"count": tt.val})
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArgs(%v) error = %v, wantErr %v", tt.val, err, tt.wantErr)
			}
		})
	}
}

func TestValidateArgs_AdditionalPropertiesNotBool(t *testing.T) {
	// additionalProperties set to a non-bool (e.g. a schema) means allowExtra stays true.
	schema := map[string]any{
		schemaKeyType:      schemaTypeObject,
		schemaKeyProps:     map[string]any{"a": map[string]any{schemaKeyType: schemaTypeString}},
		schemaKeyAddlProps: map[string]any{schemaKeyType: schemaTypeString},
	}
	if err := validateArgs(schema, map[string]any{"a": "x", "extra": "y"}); err != nil {
		t.Errorf("non-bool additionalProperties should allow extras: %v", err)
	}
}

func TestValidateArgs_IgnoresNonObjectProperty(t *testing.T) {
	// A property whose definition isn't a map should be skipped silently.
	schema := map[string]any{
		schemaKeyType:  schemaTypeObject,
		schemaKeyProps: map[string]any{"weird": "not-a-map"},
	}
	if err := validateArgs(schema, map[string]any{"weird": 1}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestToFloat64_AllKinds(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{"float64", float64(1.5), 1.5, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(3), 3, true},
		{"int32", int32(4), 4, true},
		{"int64", int64(5), 5, true},
		{"uint", uint(6), 6, true},
		{"uint32", uint32(7), 7, true},
		{"uint64", uint64(8), 8, true},
		{"json.Number", json.Number("9.25"), 9.25, true},
		{"bad json.Number", json.Number("notanumber"), 0, false},
		{"string", "x", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.in)
			if ok != tt.ok {
				t.Fatalf("toFloat64(%v) ok = %v, want %v", tt.in, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
