package jira

import (
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ──────────────────────────────────────────────────────────────────────────────
// ParseReference tests
// ──────────────────────────────────────────────────────────────────────────────

func TestParseReference(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantIssueKey   string
		wantProjectKey string
		wantNumber     int
		wantURL        string
		wantBaseURL    string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "jira scheme with issue key",
			input:          "jira:JIRA-123",
			wantIssueKey:   "JIRA-123",
			wantProjectKey: "JIRA",
			wantNumber:     123,
		},
		{
			name:           "j short scheme with issue key",
			input:          "j:PROJ-456",
			wantIssueKey:   "PROJ-456",
			wantProjectKey: "PROJ",
			wantNumber:     456,
		},
		{
			name:           "jira scheme with Cloud URL",
			input:          "jira:https://domain.atlassian.net/browse/JIRA-123",
			wantIssueKey:   "JIRA-123",
			wantProjectKey: "JIRA",
			wantNumber:     123,
			wantURL:        "https://domain.atlassian.net/browse/JIRA-123",
			wantBaseURL:    "https://domain.atlassian.net",
		},
		{
			name:           "j short scheme with Cloud URL",
			input:          "j:https://company.atlassian.net/browse/PROJ-456",
			wantIssueKey:   "PROJ-456",
			wantProjectKey: "PROJ",
			wantNumber:     456,
			wantURL:        "https://company.atlassian.net/browse/PROJ-456",
			wantBaseURL:    "https://company.atlassian.net",
		},
		{
			name:           "jira scheme with Server URL",
			input:          "jira:https://jira.example.com/browse/JIRA-789",
			wantIssueKey:   "JIRA-789",
			wantProjectKey: "JIRA",
			wantNumber:     789,
			wantURL:        "https://jira.example.com/browse/JIRA-789",
			wantBaseURL:    "https://jira.example.com",
		},
		{
			name:           "bare Cloud URL without scheme",
			input:          "https://domain.atlassian.net/browse/PROJ-123",
			wantIssueKey:   "PROJ-123",
			wantProjectKey: "PROJ",
			wantNumber:     123,
			wantURL:        "https://domain.atlassian.net/browse/PROJ-123",
			wantBaseURL:    "https://domain.atlassian.net",
		},
		{
			name:           "bare Server URL without scheme",
			input:          "https://jira.example.com/browse/ABC-456",
			wantIssueKey:   "ABC-456",
			wantProjectKey: "ABC",
			wantNumber:     456,
			wantURL:        "https://jira.example.com/browse/ABC-456",
			wantBaseURL:    "https://jira.example.com",
		},
		{
			name:           "bare issue key format",
			input:          "JIRA-123",
			wantIssueKey:   "JIRA-123",
			wantProjectKey: "JIRA",
			wantNumber:     123,
		},
		{
			name:           "issue key with 2 char project",
			input:          "AB-1",
			wantIssueKey:   "AB-1",
			wantProjectKey: "AB",
			wantNumber:     1,
		},
		{
			name:           "issue key with 10 char project",
			input:          "ABCDEFGHIJ-123",
			wantIssueKey:   "ABCDEFGHIJ-123",
			wantProjectKey: "ABCDEFGHIJ",
			wantNumber:     123,
		},
		{
			name:           "issue key with numbers in project",
			input:          "PROJ123-456",
			wantIssueKey:   "PROJ123-456",
			wantProjectKey: "PROJ123",
			wantNumber:     456,
		},
		{
			name:           "large issue number",
			input:          "JIRA-999999",
			wantIssueKey:   "JIRA-999999",
			wantProjectKey: "JIRA",
			wantNumber:     999999,
		},
		{
			name:           "URL with query parameters",
			input:          "https://jira.example.com/browse/JIRA-123?param=value",
			wantIssueKey:   "JIRA-123",
			wantProjectKey: "JIRA",
			wantNumber:     123,
			wantURL:        "https://jira.example.com/browse/JIRA-123?param=value",
			wantBaseURL:    "https://jira.example.com",
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
			input:       "JIRA123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - lowercase project key",
			input:       "jira-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - no number",
			input:       "JIRA-",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid format - no project key",
			input:       "-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "project key too short - 1 char",
			input:       "A-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "project key too long - 11 chars",
			input:       "ABCDEFGHIJK-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a jira reference - file scheme",
			input:       "file:task.md",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a jira reference - github scheme",
			input:       "github:123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "not a jira reference - linear scheme",
			input:       "linear:ENG-123",
			wantErr:     true,
			errContains: "unrecognized",
		},
		{
			name:        "invalid URL - different domain",
			input:       "https://example.com/page",
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

			if got.IssueKey != tt.wantIssueKey {
				t.Errorf("ParseReference(%q).IssueKey = %q, want %q", tt.input, got.IssueKey, tt.wantIssueKey)
			}

			if got.ProjectKey != tt.wantProjectKey {
				t.Errorf("ParseReference(%q).ProjectKey = %q, want %q", tt.input, got.ProjectKey, tt.wantProjectKey)
			}

			if got.Number != tt.wantNumber {
				t.Errorf("ParseReference(%q).Number = %d, want %d", tt.input, got.Number, tt.wantNumber)
			}

			if got.URL != tt.wantURL {
				t.Errorf("ParseReference(%q).URL = %q, want %q", tt.input, got.URL, tt.wantURL)
			}

			if got.BaseURL != tt.wantBaseURL {
				t.Errorf("ParseReference(%q).BaseURL = %q, want %q", tt.input, got.BaseURL, tt.wantBaseURL)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ExtractIssueKey tests
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractIssueKey(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "valid Cloud URL",
			url:  "https://domain.atlassian.net/browse/JIRA-123",
			want: "JIRA-123",
		},
		{
			name: "valid Server URL",
			url:  "https://jira.example.com/browse/PROJ-456",
			want: "PROJ-456",
		},
		{
			name: "URL with query parameters",
			url:  "https://jira.example.com/browse/JIRA-789?param=value",
			want: "JIRA-789",
		},
		{
			name: "URL with fragment",
			url:  "https://jira.example.com/browse/JIRA-123#section",
			want: "JIRA-123",
		},
		{
			name: "not a Jira URL - different domain",
			url:  "https://example.com/page",
			want: "",
		},
		{
			name: "not a Jira URL - missing browse path",
			url:  "https://jira.example.com/",
			want: "",
		},
		{
			name: "not a Jira URL - missing issue key",
			url:  "https://jira.example.com/browse/",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractIssueKey(tt.url)
			if got != tt.want {
				t.Errorf("ExtractIssueKey(%q) = %q, want %q", tt.url, got, tt.want)
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
				IssueKey:   "JIRA-123",
				ProjectKey: "JIRA",
				Number:     123,
				URL:        "https://jira.example.com/browse/JIRA-123",
			},
			want: "https://jira.example.com/browse/JIRA-123",
		},
		{
			name: "without URL",
			ref: Ref{
				IssueKey:   "PROJ-456",
				ProjectKey: "PROJ",
				Number:     456,
				URL:        "",
			},
			want: "PROJ-456",
		},
		{
			name: "empty ref",
			ref: Ref{
				IssueKey:   "",
				ProjectKey: "",
				Number:     0,
				URL:        "",
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

// ──────────────────────────────────────────────────────────────────────────────
// extractBaseURL tests
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractBaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "Cloud URL",
			url:  "https://domain.atlassian.net/browse/JIRA-123",
			want: "https://domain.atlassian.net",
		},
		{
			name: "Server URL",
			url:  "https://jira.example.com/browse/PROJ-456",
			want: "https://jira.example.com",
		},
		{
			name: "URL with query parameters",
			url:  "https://jira.example.com/browse/JIRA-123?param=value",
			want: "https://jira.example.com",
		},
		{
			name: "URL with fragment",
			url:  "https://jira.example.com/browse/JIRA-123#section",
			want: "https://jira.example.com",
		},
		{
			name: "URL with port",
			url:  "https://jira.example.com:8443/browse/PROJ-456",
			want: "https://jira.example.com:8443",
		},
		{
			name: "not a valid browse URL",
			url:  "https://example.com/page",
			want: "",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBaseURL(tt.url)
			if got != tt.want {
				t.Errorf("extractBaseURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// looksLikeProjectKey tests
// ──────────────────────────────────────────────────────────────────────────────

func TestLooksLikeProjectKey(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "valid 2 char uppercase",
			s:    "AB",
			want: true,
		},
		{
			name: "valid 10 char uppercase",
			s:    "ABCDEFGHIJ",
			want: true,
		},
		{
			name: "valid uppercase with numbers",
			s:    "PROJ123",
			want: true,
		},
		{
			name: "valid all numbers",
			s:    "12345",
			want: true,
		},
		{
			name: "too short - 1 char",
			s:    "A",
			want: false,
		},
		{
			name: "too long - 11 chars",
			s:    "ABCDEFGHIJK",
			want: false,
		},
		{
			name: "lowercase not valid",
			s:    "Ab",
			want: false,
		},
		{
			name: "special chars not valid",
			s:    "AB-CD",
			want: false,
		},
		{
			name: "empty string",
			s:    "",
			want: false,
		},
		{
			name: "single number",
			s:    "1",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeProjectKey(tt.s)
			if got != tt.want {
				t.Errorf("looksLikeProjectKey(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapJiraStatus tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapJiraStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "To Do maps to open",
			status:   "To Do",
			expected: "open",
		},
		{
			name:     "Backlog maps to open",
			status:   "Backlog",
			expected: "open",
		},
		{
			name:     "Open maps to open",
			status:   "Open",
			expected: "open",
		},
		{
			name:     "New maps to open",
			status:   "New",
			expected: "open",
		},
		{
			name:     "In Progress maps to in_progress",
			status:   "In Progress",
			expected: "in_progress",
		},
		{
			name:     "Started maps to in_progress",
			status:   "Started",
			expected: "in_progress",
		},
		{
			name:     "In Development maps to in_progress",
			status:   "In Development",
			expected: "in_progress",
		},
		{
			name:     "In Review maps to review",
			status:   "In Review",
			expected: "review",
		},
		{
			name:     "Code Review maps to review",
			status:   "Code Review",
			expected: "review",
		},
		{
			name:     "Under Review maps to review",
			status:   "Under Review",
			expected: "review",
		},
		{
			name:     "Done maps to done",
			status:   "Done",
			expected: "done",
		},
		{
			name:     "Closed maps to done",
			status:   "Closed",
			expected: "done",
		},
		{
			name:     "Resolved maps to done",
			status:   "Resolved",
			expected: "done",
		},
		{
			name:     "Complete maps to done",
			status:   "Complete",
			expected: "done",
		},
		{
			name:     "Finished maps to done",
			status:   "Finished",
			expected: "done",
		},
		{
			name:     "Won't Fix maps to closed",
			status:   "Won't Fix",
			expected: "closed",
		},
		{
			name:     "Cancelled maps to closed",
			status:   "Cancelled",
			expected: "closed",
		},
		{
			name:     "Obsolete maps to closed",
			status:   "Obsolete",
			expected: "closed",
		},
		{
			name:     "unknown status defaults to open",
			status:   "CustomStatus",
			expected: "open",
		},
		{
			name:     "empty status defaults to open",
			status:   "",
			expected: "open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapJiraStatus(tt.status)
			if string(got) != tt.expected {
				t.Errorf("mapJiraStatus(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapJiraPriority tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapJiraPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority *Priority
		expected string
	}{
		{
			name:     "Highest maps to critical",
			priority: &Priority{Name: "Highest"},
			expected: "critical",
		},
		{
			name:     "Critical maps to critical",
			priority: &Priority{Name: "Critical"},
			expected: "critical",
		},
		{
			name:     "High maps to high",
			priority: &Priority{Name: "High"},
			expected: "high",
		},
		{
			name:     "Low maps to low",
			priority: &Priority{Name: "Low"},
			expected: "low",
		},
		{
			name:     "Lowest maps to low",
			priority: &Priority{Name: "Lowest"},
			expected: "low",
		},
		{
			name:     "Medium maps to normal",
			priority: &Priority{Name: "Medium"},
			expected: "normal",
		},
		{
			name:     "Normal maps to normal",
			priority: &Priority{Name: "Normal"},
			expected: "normal",
		},
		{
			name:     "Default maps to normal",
			priority: &Priority{Name: "Default"},
			expected: "normal",
		},
		{
			name:     "nil priority defaults to normal",
			priority: nil,
			expected: "normal",
		},
		{
			name:     "unknown priority defaults to normal",
			priority: &Priority{Name: "Custom"},
			expected: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapJiraPriority(tt.priority)
			if got.String() != tt.expected {
				t.Errorf("mapJiraPriority(%v) = %q, want %q", tt.priority, got.String(), tt.expected)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapProviderPriorityToJira tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapProviderPriorityToJira(t *testing.T) {
	tests := []struct {
		name     string
		priority provider.Priority
		expected string
	}{
		{
			name:     "critical maps to Highest",
			priority: provider.PriorityCritical,
			expected: "Highest",
		},
		{
			name:     "high maps to High",
			priority: provider.PriorityHigh,
			expected: "High",
		},
		{
			name:     "normal maps to Medium",
			priority: provider.PriorityNormal,
			expected: "Medium",
		},
		{
			name:     "low maps to Low",
			priority: provider.PriorityLow,
			expected: "Low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapProviderPriorityToJira(tt.priority)
			if got != tt.expected {
				t.Errorf("mapProviderPriorityToJira(%v) = %q, want %q", tt.priority, got, tt.expected)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// inferTaskTypeFromLabels tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInferTaskTypeFromLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected string
	}{
		{
			name:     "bug label maps to fix",
			labels:   []string{"bug", "backend"},
			expected: "fix",
		},
		{
			name:     "bugfix label maps to fix",
			labels:   []string{"bugfix"},
			expected: "fix",
		},
		{
			name:     "fix label maps to fix",
			labels:   []string{"fix"},
			expected: "fix",
		},
		{
			name:     "feature label maps to feature",
			labels:   []string{"feature"},
			expected: "feature",
		},
		{
			name:     "enhancement label maps to feature",
			labels:   []string{"enhancement"},
			expected: "feature",
		},
		{
			name:     "docs label maps to docs",
			labels:   []string{"docs"},
			expected: "docs",
		},
		{
			name:     "documentation label maps to docs",
			labels:   []string{"documentation"},
			expected: "docs",
		},
		{
			name:     "refactor label maps to refactor",
			labels:   []string{"refactor"},
			expected: "refactor",
		},
		{
			name:     "chore label maps to chore",
			labels:   []string{"chore"},
			expected: "chore",
		},
		{
			name:     "test label maps to test",
			labels:   []string{"test"},
			expected: "test",
		},
		{
			name:     "ci label maps to ci",
			labels:   []string{"ci"},
			expected: "ci",
		},
		{
			name:     "no recognized labels defaults to issue",
			labels:   []string{"random", "labels"},
			expected: "issue",
		},
		{
			name:     "empty labels defaults to issue",
			labels:   []string{},
			expected: "issue",
		},
		{
			name:     "first recognized label wins",
			labels:   []string{"random", "bug", "feature"},
			expected: "fix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTaskTypeFromLabels(tt.labels)
			if got != tt.expected {
				t.Errorf("inferTaskTypeFromLabels(%v) = %q, want %q", tt.labels, got, tt.expected)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// buildJQL tests
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildJQL(t *testing.T) {
	tests := []struct {
		name       string
		projectKey string
		opts       provider.ListOptions
		expected   string
	}{
		{
			name:       "basic project filter",
			projectKey: "PROJ",
			opts:       provider.ListOptions{},
			expected:   `project = PROJ ORDER BY created DESC`,
		},
		{
			name:       "with status filter",
			projectKey: "PROJ",
			opts: provider.ListOptions{
				Status: "In Progress",
			},
			expected: `project = PROJ AND status = "In Progress" ORDER BY created DESC`,
		},
		{
			name:       "with label filter",
			projectKey: "PROJ",
			opts: provider.ListOptions{
				Labels: []string{"bug", "urgent"},
			},
			expected: `project = PROJ AND labels in ("bug", "urgent") ORDER BY created DESC`,
		},
		{
			name:       "with status and label filters",
			projectKey: "PROJ",
			opts: provider.ListOptions{
				Status: "Done",
				Labels: []string{"backend"},
			},
			expected: `project = PROJ AND status = "Done" AND labels in ("backend") ORDER BY created DESC`,
		},
		{
			name:       "with custom ordering",
			projectKey: "PROJ",
			opts: provider.ListOptions{
				OrderBy: "priority",
			},
			expected: `project = PROJ ORDER BY priority DESC`,
		},
		{
			name:       "with ascending order",
			projectKey: "PROJ",
			opts: provider.ListOptions{
				OrderDir: "asc",
			},
			expected: `project = PROJ ORDER BY created ASC`,
		},
		{
			name:       "with custom ordering and direction",
			projectKey: "PROJ",
			opts: provider.ListOptions{
				OrderBy:  "updated",
				OrderDir: "asc",
			},
			expected: `project = PROJ ORDER BY updated ASC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildJQL(tt.projectKey, tt.opts)
			if got != tt.expected {
				t.Errorf("buildJQL(%q, opts) = %q, want %q", tt.projectKey, got, tt.expected)
			}
		})
	}
}
