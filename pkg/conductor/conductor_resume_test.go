package conductor

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "short string under limit",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string exactly at limit",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "string over limit",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
		{
			name:   "maxLen zero",
			input:  "hello",
			maxLen: 0,
			want:   "...",
		},
		{
			name:   "unicode CJK within byte limit but rune count matches",
			input:  "你好世界测试",
			maxLen: 6,
			want:   "你好世界测试",
		},
		{
			name:   "unicode CJK over rune limit",
			input:  "你好世界测试",
			maxLen: 3,
			want:   "你好世...",
		},
		{
			name:   "emoji string over rune limit",
			input:  "😀😁😂🤣😃😄",
			maxLen: 3,
			want:   "😀😁😂...",
		},
		{
			name:   "emoji string under rune limit",
			input:  "😀😁",
			maxLen: 5,
			want:   "😀😁",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestBuildPriorContext_nonexistentPath(t *testing.T) {
	t.Parallel()

	got := buildPriorContext("/nonexistent/path/recording.jsonl", "implement")
	if got != "" {
		t.Errorf("expected empty string for nonexistent path, got %q", got)
	}
}

func TestBuildPriorContext_emptyPath(t *testing.T) {
	t.Parallel()

	got := buildPriorContext("", "plan")
	if got != "" {
		t.Errorf("expected empty string for empty path, got %q", got)
	}
}
