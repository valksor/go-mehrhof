package wrike

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// ParseReference tests
// ──────────────────────────────────────────────────────────────────────────────

func TestParseReference(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantTaskID    string
		wantPermalink string
		wantErr       bool
		errContains   string
	}{
		{
			name:       "wrike scheme with numeric ID",
			input:      "wrike:1234567890",
			wantTaskID: "1234567890",
		},
		{
			name:       "wrike scheme with API ID",
			input:      "wrike:IEAAJXXXXXXXX",
			wantTaskID: "IEAAJXXXXXXXX",
		},
		{
			name:       "wk short scheme with numeric ID",
			input:      "wk:1234567890",
			wantTaskID: "1234567890",
		},
		{
			name:       "wk short scheme with API ID",
			input:      "wk:IEAAJXXXXXXXX",
			wantTaskID: "IEAAJXXXXXXXX",
		},
		{
			name:          "permalink URL",
			input:         "https://www.wrike.com/open.htm?id=1234567890",
			wantTaskID:    "1234567890",
			wantPermalink: "https://www.wrike.com/open.htm?id=1234567890",
		},
		{
			name:          "permalink with more digits",
			input:         "https://www.wrike.com/open.htm?id=12345678901234",
			wantTaskID:    "12345678901234",
			wantPermalink: "https://www.wrike.com/open.htm?id=12345678901234",
		},
		{
			name:       "bare API ID format",
			input:      "IEAAJXXXXXXXX",
			wantTaskID: "IEAAJXXXXXXXX",
		},
		{
			name:       "bare API ID with numbers",
			input:      "IEAAJ123456789",
			wantTaskID: "IEAAJ123456789",
		},
		{
			name:       "bare numeric ID (10 digits)",
			input:      "1234567890",
			wantTaskID: "1234567890",
		},
		{
			name:       "bare numeric ID (more than 10 digits)",
			input:      "123456789012345",
			wantTaskID: "123456789012345",
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "whitespace only",
			input:       "   ",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "invalid format - too short",
			input:       "wrike:123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - letters only",
			input:       "wrike:abcdef",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - mix that doesn't match",
			input:       "wrike:123abc",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a wrike reference - file scheme",
			input:       "file:task.md",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a wrike reference - github scheme",
			input:       "github:123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "numeric ID with 9 digits (too short)",
			input:       "123456789",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "API ID with lowercase",
			input:       "ieaajxxxxxxxx",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "API ID starting with wrong prefix",
			input:       "XXAAJXXXXXXXX",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "API ID too short",
			input:       "IEA",
			wantErr:     true,
			errContains: "unrecognized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseReference(%q) expected error, got nil", tt.input)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseReference(%q) error = %q, want to contain %q", tt.input, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseReference(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got.TaskID != tt.wantTaskID {
				t.Errorf("ParseReference(%q).TaskID = %q, want %q", tt.input, got.TaskID, tt.wantTaskID)
			}

			if got.Permalink != tt.wantPermalink {
				t.Errorf("ParseReference(%q).Permalink = %q, want %q", tt.input, got.Permalink, tt.wantPermalink)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ExtractNumericID tests
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractNumericID(t *testing.T) {
	tests := []struct {
		name      string
		permalink string
		want      string
	}{
		{
			name:      "valid permalink with 10 digits",
			permalink: "https://www.wrike.com/open.htm?id=1234567890",
			want:      "1234567890",
		},
		{
			name:      "valid permalink with more digits",
			permalink: "https://www.wrike.com/open.htm?id=12345678901234",
			want:      "12345678901234",
		},
		{
			name:      "permalink with additional query params",
			permalink: "https://www.wrike.com/open.htm?id=1234567890&other=value",
			want:      "1234567890",
		},
		{
			name:      "not a permalink - regular URL",
			permalink: "https://example.com/page",
			want:      "",
		},
		{
			name:      "not a permalink - missing query param",
			permalink: "https://www.wrike.com/open.htm",
			want:      "",
		},
		{
			name:      "not a permalink - wrong query param name",
			permalink: "https://www.wrike.com/open.htm?task=1234567890",
			want:      "",
		},
		{
			name:      "empty string",
			permalink: "",
			want:      "",
		},
		{
			name:      "just text",
			permalink: "not a url",
			want:      "",
		},
		{
			name:      "ID in middle of URL",
			permalink: "https://www.wrike.com/open.htm?id=1234567890&extra=true",
			want:      "1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractNumericID(tt.permalink)
			if got != tt.want {
				t.Errorf("ExtractNumericID(%q) = %q, want %q", tt.permalink, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Ref.String tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  Ref
		want string
	}{
		{
			name: "with permalink",
			ref: Ref{
				TaskID:    "1234567890",
				Permalink: "https://www.wrike.com/open.htm?id=1234567890",
			},
			want: "https://www.wrike.com/open.htm?id=1234567890",
		},
		{
			name: "without permalink",
			ref: Ref{
				TaskID:    "IEAAJXXXXXXXX",
				Permalink: "",
			},
			want: "IEAAJXXXXXXXX",
		},
		{
			name: "empty ref",
			ref: Ref{
				TaskID:    "",
				Permalink: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("Ref.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
