package linear

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// ParseReference tests
// ──────────────────────────────────────────────────────────────────────────────

func TestParseReference(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantIssueID string
		wantTeamKey string
		wantNumber  int
		wantURL     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "linear scheme with issue ID",
			input:       "linear:ENG-123",
			wantIssueID: "ENG-123",
			wantTeamKey: "ENG",
			wantNumber:  123,
		},
		{
			name:        "linear scheme with different team",
			input:       "linear:PROD-456",
			wantIssueID: "PROD-456",
			wantTeamKey: "PROD",
			wantNumber:  456,
		},
		{
			name:        "ln short scheme with issue ID",
			input:       "ln:ENG-123",
			wantIssueID: "ENG-123",
			wantTeamKey: "ENG",
			wantNumber:  123,
		},
		{
			name:        "ln short scheme with number team key",
			input:       "ln:TEAM1-999",
			wantIssueID: "TEAM1-999",
			wantTeamKey: "TEAM1",
			wantNumber:  999,
		},
		{
			name:        "linear app URL with team and issue",
			input:       "https://linear.app/myteam/issue/ENG-123-title",
			wantIssueID: "ENG-123",
			wantTeamKey: "ENG",
			wantNumber:  123,
			wantURL:     "https://linear.app/myteam/issue/ENG-123-title",
		},
		{
			name:        "linear app URL without team path",
			input:       "https://linear.app/issue/ENG-123-some-title",
			wantIssueID: "ENG-123",
			wantTeamKey: "ENG",
			wantNumber:  123,
			wantURL:     "https://linear.app/issue/ENG-123-some-title",
		},
		{
			name:        "linear app URL with underscores in team key",
			input:       "https://linear.app/my_team/issue/ABC-1-title",
			wantIssueID: "ABC-1",
			wantTeamKey: "ABC",
			wantNumber:  1,
			wantURL:     "https://linear.app/my_team/issue/ABC-1-title",
		},
		{
			name:        "bare issue ID format",
			input:       "ENG-123",
			wantIssueID: "ENG-123",
			wantTeamKey: "ENG",
			wantNumber:  123,
		},
		{
			name:        "bare issue ID with numeric team key",
			input:       "TEAM1-456",
			wantIssueID: "TEAM1-456",
			wantTeamKey: "TEAM1",
			wantNumber:  456,
		},
		{
			name:        "single char team key",
			input:       "A-1",
			wantIssueID: "A-1",
			wantTeamKey: "A",
			wantNumber:  1,
		},
		{
			name:        "large issue number",
			input:       "ENG-999999",
			wantIssueID: "ENG-999999",
			wantTeamKey: "ENG",
			wantNumber:  999999,
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
			name:        "invalid format - no dash",
			input:       "ENG123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - lowercase team key",
			input:       "eng-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - no number",
			input:       "ENG-",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - no team key",
			input:       "-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a linear reference - file scheme",
			input:       "file:task.md",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a linear reference - github scheme",
			input:       "github:123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a linear reference - wrike scheme",
			input:       "wrike:1234567890",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid URL - different domain",
			input:       "https://example.com/issue/ENG-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "just text",
			input:       "not a reference",
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

			if got.IssueID != tt.wantIssueID {
				t.Errorf("ParseReference(%q).IssueID = %q, want %q", tt.input, got.IssueID, tt.wantIssueID)
			}

			if got.TeamKey != tt.wantTeamKey {
				t.Errorf("ParseReference(%q).TeamKey = %q, want %q", tt.input, got.TeamKey, tt.wantTeamKey)
			}

			if got.Number != tt.wantNumber {
				t.Errorf("ParseReference(%q).Number = %d, want %d", tt.input, got.Number, tt.wantNumber)
			}

			if got.URL != tt.wantURL {
				t.Errorf("ParseReference(%q).URL = %q, want %q", tt.input, got.URL, tt.wantURL)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ExtractIssueID tests
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractIssueID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "valid Linear URL with team path",
			url:  "https://linear.app/myteam/issue/ENG-123-title",
			want: "ENG-123",
		},
		{
			name: "valid Linear URL without team path",
			url:  "https://linear.app/issue/ENG-456-some-title",
			want: "ENG-456",
		},
		{
			name: "valid Linear URL with underscores in team",
			url:  "https://linear.app/my_team/issue/PROD-1-description",
			want: "PROD-1",
		},
		{
			name: "not a Linear URL - different domain",
			url:  "https://example.com/page",
			want: "",
		},
		{
			name: "not a Linear URL - missing issue path",
			url:  "https://linear.app/",
			want: "",
		},
		{
			name: "not a Linear URL - missing issue identifier",
			url:  "https://linear.app/team/issue/",
			want: "",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
		{
			name: "just text",
			url:  "not a url",
			want: "",
		},
		{
			name: "Linear URL with query params",
			url:  "https://linear.app/team/issue/ENG-789-title?param=value",
			want: "ENG-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractIssueID(tt.url)
			if got != tt.want {
				t.Errorf("ExtractIssueID(%q) = %q, want %q", tt.url, got, tt.want)
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
			name: "with URL",
			ref: Ref{
				IssueID: "ENG-123",
				TeamKey: "ENG",
				Number:  123,
				URL:     "https://linear.app/team/issue/ENG-123-title",
			},
			want: "https://linear.app/team/issue/ENG-123-title",
		},
		{
			name: "without URL",
			ref: Ref{
				IssueID: "PROD-456",
				TeamKey: "PROD",
				Number:  456,
				URL:     "",
			},
			want: "PROD-456",
		},
		{
			name: "empty ref",
			ref: Ref{
				IssueID: "",
				TeamKey: "",
				Number:  0,
				URL:     "",
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
