package conductor

import (
	"context"
	"testing"
)

// Additional guard function tests extending coverage in extra_test.go.
// These tests focus on edge cases not covered by the existing suite.

func TestGuardHasSource_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "Source with Reference but no Provider still valid",
			wu:   &WorkUnit{Source: &Source{Provider: "", Reference: "some-ref"}},
			want: true,
		},
		{
			name: "Source with only URL and no Reference",
			wu:   &WorkUnit{Source: &Source{Provider: "github", URL: "https://example.com", Reference: ""}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guardHasSource(context.Background(), tt.wu)
			if got != tt.want {
				t.Errorf("guardHasSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardHasDescription_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			// guard only checks non-empty string, not blank-only
			name: "whitespace-only is truthy",
			wu:   &WorkUnit{Description: "   "},
			want: true,
		},
		{
			name: "multi-line description",
			wu:   &WorkUnit{Description: "line 1\nline 2\nline 3"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guardHasDescription(context.Background(), tt.wu)
			if got != tt.want {
				t.Errorf("guardHasDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardHasSpecifications_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "multiple specification files",
			wu:   &WorkUnit{Specifications: []string{"/path/spec-1.md", "/path/spec-2.md", "/path/spec-3.md"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guardHasSpecifications(context.Background(), tt.wu)
			if got != tt.want {
				t.Errorf("guardHasSpecifications() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanUndo_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "multiple checkpoints",
			wu:   &WorkUnit{Checkpoints: []string{"aaa111", "bbb222", "ccc333"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guardCanUndo(context.Background(), tt.wu)
			if got != tt.want {
				t.Errorf("guardCanUndo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanRedo_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "multiple entries in redo stack",
			wu:   &WorkUnit{RedoStack: []string{"aaa111", "bbb222"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guardCanRedo(context.Background(), tt.wu)
			if got != tt.want {
				t.Errorf("guardCanRedo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardCanSubmit_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		wu   *WorkUnit
		want bool
	}{
		{
			name: "gitlab provider is valid",
			wu:   &WorkUnit{Source: &Source{Provider: "gitlab", Reference: "group/project!42"}},
			want: true,
		},
		{
			name: "wrike provider is valid",
			wu:   &WorkUnit{Source: &Source{Provider: "wrike", Reference: "TASK-123"}},
			want: true,
		},
		{
			name: "file provider is valid (has non-empty Provider)",
			wu:   &WorkUnit{Source: &Source{Provider: "file", Reference: "task.md"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guardCanSubmit(context.Background(), tt.wu)
			if got != tt.want {
				t.Errorf("guardCanSubmit() = %v, want %v", got, tt.want)
			}
		})
	}
}
