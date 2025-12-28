package github

import (
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v67/github"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ──────────────────────────────────────────────────────────────────────────────
// Info tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Name = %q, want %q", info.Name, ProviderName)
	}

	if info.Description == "" {
		t.Error("Description is empty")
	}

	// Check schemes
	expectedSchemes := []string{"github", "gh"}
	if len(info.Schemes) != len(expectedSchemes) {
		t.Errorf("Schemes = %v, want %v", info.Schemes, expectedSchemes)
	}
	for i, s := range expectedSchemes {
		if info.Schemes[i] != s {
			t.Errorf("Schemes[%d] = %q, want %q", i, info.Schemes[i], s)
		}
	}

	// Check priority
	if info.Priority <= 0 {
		t.Errorf("Priority = %d, want > 0", info.Priority)
	}

	// Check capabilities
	expectedCaps := []provider.Capability{
		provider.CapRead,
		provider.CapFetchComments,
		provider.CapComment,
		provider.CapCreatePR,
		provider.CapDownloadAttachment,
		provider.CapSnapshot,
	}
	for _, cap := range expectedCaps {
		if !info.Capabilities.Has(cap) {
			t.Errorf("Capabilities missing %q", cap)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapGitHubState tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapGitHubState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  provider.Status
	}{
		{
			name:  "open state",
			state: "open",
			want:  provider.StatusOpen,
		},
		{
			name:  "closed state",
			state: "closed",
			want:  provider.StatusClosed,
		},
		{
			name:  "unknown state defaults to open",
			state: "unknown",
			want:  provider.StatusOpen,
		},
		{
			name:  "empty state defaults to open",
			state: "",
			want:  provider.StatusOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitHubState(tt.state)
			if got != tt.want {
				t.Errorf("mapGitHubState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// inferTypeFromLabels tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInferTypeFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []*gh.Label
		want   string
	}{
		{
			name:   "bug label",
			labels: []*gh.Label{{Name: ptr("bug")}},
			want:   "fix",
		},
		{
			name:   "bugfix label",
			labels: []*gh.Label{{Name: ptr("bugfix")}},
			want:   "fix",
		},
		{
			name:   "feature label",
			labels: []*gh.Label{{Name: ptr("feature")}},
			want:   "feature",
		},
		{
			name:   "enhancement label",
			labels: []*gh.Label{{Name: ptr("enhancement")}},
			want:   "feature",
		},
		{
			name:   "docs label",
			labels: []*gh.Label{{Name: ptr("docs")}},
			want:   "docs",
		},
		{
			name:   "documentation label",
			labels: []*gh.Label{{Name: ptr("documentation")}},
			want:   "docs",
		},
		{
			name:   "refactor label",
			labels: []*gh.Label{{Name: ptr("refactor")}},
			want:   "refactor",
		},
		{
			name:   "chore label",
			labels: []*gh.Label{{Name: ptr("chore")}},
			want:   "chore",
		},
		{
			name:   "test label",
			labels: []*gh.Label{{Name: ptr("test")}},
			want:   "test",
		},
		{
			name:   "ci label",
			labels: []*gh.Label{{Name: ptr("ci")}},
			want:   "ci",
		},
		{
			name:   "case insensitive - BUG",
			labels: []*gh.Label{{Name: ptr("BUG")}},
			want:   "fix",
		},
		{
			name:   "no matching label defaults to issue",
			labels: []*gh.Label{{Name: ptr("wontfix")}},
			want:   "issue",
		},
		{
			name:   "empty labels defaults to issue",
			labels: []*gh.Label{},
			want:   "issue",
		},
		{
			name:   "nil labels defaults to issue",
			labels: nil,
			want:   "issue",
		},
		{
			name:   "first matching label wins",
			labels: []*gh.Label{{Name: ptr("bug")}, {Name: ptr("feature")}},
			want:   "fix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTypeFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("inferTypeFromLabels() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// inferPriorityFromLabels tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInferPriorityFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []*gh.Label
		want   provider.Priority
	}{
		{
			name:   "critical label",
			labels: []*gh.Label{{Name: ptr("critical")}},
			want:   provider.PriorityCritical,
		},
		{
			name:   "urgent label",
			labels: []*gh.Label{{Name: ptr("urgent")}},
			want:   provider.PriorityCritical,
		},
		{
			name:   "priority:high label",
			labels: []*gh.Label{{Name: ptr("priority:high")}},
			want:   provider.PriorityHigh,
		},
		{
			name:   "high-priority label",
			labels: []*gh.Label{{Name: ptr("high-priority")}},
			want:   provider.PriorityHigh,
		},
		{
			name:   "priority:low label",
			labels: []*gh.Label{{Name: ptr("priority:low")}},
			want:   provider.PriorityLow,
		},
		{
			name:   "low-priority label",
			labels: []*gh.Label{{Name: ptr("low-priority")}},
			want:   provider.PriorityLow,
		},
		{
			name:   "case insensitive - CRITICAL",
			labels: []*gh.Label{{Name: ptr("CRITICAL")}},
			want:   provider.PriorityCritical,
		},
		{
			name:   "no priority label defaults to normal",
			labels: []*gh.Label{{Name: ptr("bug")}},
			want:   provider.PriorityNormal,
		},
		{
			name:   "empty labels defaults to normal",
			labels: []*gh.Label{},
			want:   provider.PriorityNormal,
		},
		{
			name:   "nil labels defaults to normal",
			labels: nil,
			want:   provider.PriorityNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferPriorityFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("inferPriorityFromLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// extractLabelNames tests
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractLabelNames(t *testing.T) {
	tests := []struct {
		name   string
		labels []*gh.Label
		want   []string
	}{
		{
			name:   "multiple labels",
			labels: []*gh.Label{{Name: ptr("bug")}, {Name: ptr("urgent")}, {Name: ptr("frontend")}},
			want:   []string{"bug", "urgent", "frontend"},
		},
		{
			name:   "single label",
			labels: []*gh.Label{{Name: ptr("feature")}},
			want:   []string{"feature"},
		},
		{
			name:   "empty labels",
			labels: []*gh.Label{},
			want:   []string{},
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   []string{},
		},
		{
			name:   "label with nil name",
			labels: []*gh.Label{{Name: nil}},
			want:   []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLabelNames(tt.labels)
			if len(got) != len(tt.want) {
				t.Errorf("extractLabelNames() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, name := range tt.want {
				if got[i] != name {
					t.Errorf("extractLabelNames()[%d] = %q, want %q", i, got[i], name)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapAssignees tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapAssignees(t *testing.T) {
	tests := []struct {
		name      string
		assignees []*gh.User
		want      []provider.Person
	}{
		{
			name: "multiple assignees",
			assignees: []*gh.User{
				{ID: ptr(int64(1)), Login: ptr("user1"), Email: ptr("user1@example.com")},
				{ID: ptr(int64(2)), Login: ptr("user2"), Email: ptr("user2@example.com")},
			},
			want: []provider.Person{
				{ID: "1", Name: "user1", Email: "user1@example.com"},
				{ID: "2", Name: "user2", Email: "user2@example.com"},
			},
		},
		{
			name: "assignee without email",
			assignees: []*gh.User{
				{ID: ptr(int64(123)), Login: ptr("johndoe")},
			},
			want: []provider.Person{
				{ID: "123", Name: "johndoe", Email: ""},
			},
		},
		{
			name:      "empty assignees",
			assignees: []*gh.User{},
			want:      []provider.Person{},
		},
		{
			name:      "nil assignees",
			assignees: nil,
			want:      []provider.Person{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAssignees(tt.assignees)
			if len(got) != len(tt.want) {
				t.Errorf("mapAssignees() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, p := range tt.want {
				if got[i].ID != p.ID {
					t.Errorf("mapAssignees()[%d].ID = %q, want %q", i, got[i].ID, p.ID)
				}
				if got[i].Name != p.Name {
					t.Errorf("mapAssignees()[%d].Name = %q, want %q", i, got[i].Name, p.Name)
				}
				if got[i].Email != p.Email {
					t.Errorf("mapAssignees()[%d].Email = %q, want %q", i, got[i].Email, p.Email)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapComments tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapComments(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		comments []*gh.IssueComment
		wantLen  int
	}{
		{
			name: "multiple comments",
			comments: []*gh.IssueComment{
				{
					ID:        ptr(int64(1)),
					Body:      ptr("First comment"),
					User:      &gh.User{ID: ptr(int64(10)), Login: ptr("author1")},
					CreatedAt: &gh.Timestamp{Time: now},
				},
				{
					ID:        ptr(int64(2)),
					Body:      ptr("Second comment"),
					User:      &gh.User{ID: ptr(int64(20)), Login: ptr("author2")},
					CreatedAt: &gh.Timestamp{Time: now},
				},
			},
			wantLen: 2,
		},
		{
			name:     "empty comments",
			comments: []*gh.IssueComment{},
			wantLen:  0,
		},
		{
			name:     "nil comments",
			comments: nil,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapComments(tt.comments)
			if len(got) != tt.wantLen {
				t.Errorf("mapComments() len = %d, want %d", len(got), tt.wantLen)
				return
			}

			for i, c := range tt.comments {
				if got[i].ID != "1" && got[i].ID != "2" {
					t.Errorf("mapComments()[%d].ID unexpected value: %q", i, got[i].ID)
				}
				if got[i].Body != c.GetBody() {
					t.Errorf("mapComments()[%d].Body = %q, want %q", i, got[i].Body, c.GetBody())
				}
				if got[i].Author.Name != c.GetUser().GetLogin() {
					t.Errorf("mapComments()[%d].Author.Name = %q, want %q", i, got[i].Author.Name, c.GetUser().GetLogin())
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// formatIssueMarkdown tests
// ──────────────────────────────────────────────────────────────────────────────

func TestFormatIssueMarkdown(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		issue        *gh.Issue
		wantContains []string
	}{
		{
			name: "full issue",
			issue: &gh.Issue{
				Number:    ptr(123),
				Title:     ptr("Test Issue"),
				State:     ptr("open"),
				Body:      ptr("Issue description"),
				CreatedAt: &gh.Timestamp{Time: now},
				UpdatedAt: &gh.Timestamp{Time: now},
				User:      &gh.User{Login: ptr("author")},
				Labels:    []*gh.Label{{Name: ptr("bug")}, {Name: ptr("urgent")}},
				Assignees: []*gh.User{{Login: ptr("assignee1")}},
				HTMLURL:   ptr("https://github.com/owner/repo/issues/123"),
			},
			wantContains: []string{
				"# #123: Test Issue",
				"**State:** open",
				"**Author:** @author",
				"**Labels:** bug, urgent",
				"**Assignees:** @assignee1",
				"Issue description",
			},
		},
		{
			name: "minimal issue",
			issue: &gh.Issue{
				Number:    ptr(1),
				Title:     ptr("Minimal"),
				State:     ptr("closed"),
				CreatedAt: &gh.Timestamp{Time: now},
				UpdatedAt: &gh.Timestamp{Time: now},
			},
			wantContains: []string{
				"# #1: Minimal",
				"**State:** closed",
			},
		},
		{
			name: "issue without user",
			issue: &gh.Issue{
				Number:    ptr(2),
				Title:     ptr("No Author"),
				State:     ptr("open"),
				CreatedAt: &gh.Timestamp{Time: now},
				UpdatedAt: &gh.Timestamp{Time: now},
			},
			wantContains: []string{
				"# #2: No Author",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIssueMarkdown(tt.issue)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatIssueMarkdown() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// formatCommentsMarkdown tests
// ──────────────────────────────────────────────────────────────────────────────

func TestFormatCommentsMarkdown(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		comments     []*gh.IssueComment
		wantContains []string
	}{
		{
			name: "multiple comments",
			comments: []*gh.IssueComment{
				{
					Body:      ptr("First comment body"),
					User:      &gh.User{Login: ptr("commenter1")},
					CreatedAt: &gh.Timestamp{Time: now},
				},
				{
					Body:      ptr("Second comment body"),
					User:      &gh.User{Login: ptr("commenter2")},
					CreatedAt: &gh.Timestamp{Time: now},
				},
			},
			wantContains: []string{
				"# Comments",
				"## Comment by @commenter1",
				"First comment body",
				"## Comment by @commenter2",
				"Second comment body",
			},
		},
		{
			name:     "empty comments",
			comments: []*gh.IssueComment{},
			wantContains: []string{
				"# Comments",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCommentsMarkdown(tt.comments)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatCommentsMarkdown() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Provider.Match tests
// ──────────────────────────────────────────────────────────────────────────────

func TestProvider_Match(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "github scheme",
			input: "github:123",
			want:  true,
		},
		{
			name:  "github with owner/repo",
			input: "github:owner/repo#123",
			want:  true,
		},
		{
			name:  "gh scheme",
			input: "gh:456",
			want:  true,
		},
		{
			name:  "gh with owner/repo",
			input: "gh:owner/repo#789",
			want:  true,
		},
		{
			name:  "file scheme",
			input: "file:task.md",
			want:  false,
		},
		{
			name:  "no scheme",
			input: "just-text",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "similar but not github",
			input: "github-actions:run",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Match(tt.input)
			if got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ProviderName constant test
// ──────────────────────────────────────────────────────────────────────────────

func TestProviderName(t *testing.T) {
	if ProviderName != "github" {
		t.Errorf("ProviderName = %q, want %q", ProviderName, "github")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Provider.Parse tests
// ──────────────────────────────────────────────────────────────────────────────

func TestProvider_Parse(t *testing.T) {
	tests := []struct {
		name        string
		owner       string
		repo        string
		input       string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:  "explicit owner/repo",
			owner: "",
			repo:  "",
			input: "github:owner/repo#123",
			want:  "owner/repo#123",
		},
		{
			name:  "explicit with gh scheme",
			owner: "",
			repo:  "",
			input: "gh:other/project#456",
			want:  "other/project#456",
		},
		{
			name:  "issue number only with configured repo",
			owner: "default-owner",
			repo:  "default-repo",
			input: "github:#789",
			want:  "default-owner/default-repo#789",
		},
		{
			name:  "issue number only with gh scheme",
			owner: "my-org",
			repo:  "my-repo",
			input: "gh:#42",
			want:  "my-org/my-repo#42",
		},
		{
			name:        "issue only without configured repo",
			owner:       "",
			repo:        "",
			input:       "github:#123",
			wantErr:     true,
			errContains: "repository not configured",
		},
		{
			name:        "missing owner in config",
			owner:       "",
			repo:        "some-repo",
			input:       "github:#123",
			wantErr:     true,
			errContains: "repository not configured",
		},
		{
			name:        "missing repo in config",
			owner:       "some-owner",
			repo:        "",
			input:       "github:#123",
			wantErr:     true,
			errContains: "repository not configured",
		},
		{
			name:    "invalid reference format",
			owner:   "owner",
			repo:    "repo",
			input:   "github:invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				owner:  tt.owner,
				repo:   tt.repo,
				client: NewClient("test-token", tt.owner, tt.repo),
			}

			got, err := p.Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tt.input)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Parse(%q) error = %q, want to contain %q", tt.input, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ErrRepoNotConfigured test
// ──────────────────────────────────────────────────────────────────────────────

func TestErrRepoNotConfigured(t *testing.T) {
	if ErrRepoNotConfigured == nil {
		t.Fatal("ErrRepoNotConfigured should not be nil")
	}
	if ErrRepoNotConfigured.Error() == "" {
		t.Error("ErrRepoNotConfigured.Error() should not be empty")
	}
}
