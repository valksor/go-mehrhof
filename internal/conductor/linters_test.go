package conductor

import (
	"context"
	"testing"

	"github.com/valksor/go-mehrhof/internal/quality"
)

// mockLinter is a minimal mock of quality.Linter for testing
type mockLinter struct {
	name string
}

func (m *mockLinter) Name() string {
	return m.name
}

func (m *mockLinter) Available() bool {
	return true
}

func (m *mockLinter) Run(ctx context.Context, workDir string, files []string) (*quality.Result, error) {
	return nil, nil
}

// Test linterNames utility function
func TestLinterNames(t *testing.T) {
	tests := []struct {
		name    string
		linters []quality.Linter
		want    string
	}{
		{
			name:    "empty slice",
			linters: []quality.Linter{},
			want:    "",
		},
		{
			name:    "nil slice",
			linters: nil,
			want:    "",
		},
		{
			name: "single linter",
			linters: []quality.Linter{
				&mockLinter{name: "golangci-lint"},
			},
			want: "golangci-lint",
		},
		{
			name: "multiple linters",
			linters: []quality.Linter{
				&mockLinter{name: "golangci-lint"},
				&mockLinter{name: "eslint"},
				&mockLinter{name: "ruff"},
			},
			want: "golangci-lint, eslint, ruff",
		},
		{
			name: "linters with special characters",
			linters: []quality.Linter{
				&mockLinter{name: "golangci-lint"},
				&mockLinter{name: "pylint"},
				&mockLinter{name: "shellcheck"},
			},
			want: "golangci-lint, pylint, shellcheck",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linterNames(tt.linters)
			if got != tt.want {
				t.Errorf("linterNames() = %q, want %q", got, tt.want)
			}
		})
	}
}
