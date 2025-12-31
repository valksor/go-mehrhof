package bitbucket

import (
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Reference
		wantErr bool
	}{
		{
			name:  "issue ID only",
			input: "123",
			want: &Reference{
				IssueID:    123,
				IsExplicit: false,
			},
		},
		{
			name:  "issue ID with hash",
			input: "#456",
			want: &Reference{
				IssueID:    456,
				IsExplicit: false,
			},
		},
		{
			name:  "workspace/repo#issue",
			input: "myworkspace/myrepo#789",
			want: &Reference{
				Workspace:  "myworkspace",
				RepoSlug:   "myrepo",
				IssueID:    789,
				IsExplicit: true,
			},
		},
		{
			name:  "full URL",
			input: "https://bitbucket.org/myworkspace/myrepo/issues/42",
			want: &Reference{
				Workspace:  "myworkspace",
				RepoSlug:   "myrepo",
				IssueID:    42,
				IsExplicit: true,
			},
		},
		{
			name:  "URL without https",
			input: "bitbucket.org/myworkspace/myrepo/issues/100",
			want: &Reference{
				Workspace:  "myworkspace",
				RepoSlug:   "myrepo",
				IssueID:    100,
				IsExplicit: true,
			},
		},
		{
			name:    "invalid input",
			input:   "not-valid-reference",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid URL path",
			input:   "https://bitbucket.org/workspace",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseReference(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseReference(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got.IssueID != tt.want.IssueID {
				t.Errorf("ParseReference(%q).IssueID = %d, want %d", tt.input, got.IssueID, tt.want.IssueID)
			}
			if got.Workspace != tt.want.Workspace {
				t.Errorf("ParseReference(%q).Workspace = %q, want %q", tt.input, got.Workspace, tt.want.Workspace)
			}
			if got.RepoSlug != tt.want.RepoSlug {
				t.Errorf("ParseReference(%q).RepoSlug = %q, want %q", tt.input, got.RepoSlug, tt.want.RepoSlug)
			}
			if got.IsExplicit != tt.want.IsExplicit {
				t.Errorf("ParseReference(%q).IsExplicit = %v, want %v", tt.input, got.IsExplicit, tt.want.IsExplicit)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Info().Name = %q, want %q", info.Name, ProviderName)
	}

	expectedSchemes := []string{"bitbucket", "bb"}
	if len(info.Schemes) != len(expectedSchemes) {
		t.Errorf("Info().Schemes = %v, want %v", info.Schemes, expectedSchemes)
	}
	for i, scheme := range expectedSchemes {
		if info.Schemes[i] != scheme {
			t.Errorf("Info().Schemes[%d] = %q, want %q", i, info.Schemes[i], scheme)
		}
	}

	// Check capabilities
	if !info.Capabilities.Has(provider.CapRead) {
		t.Error("Info().Capabilities should have CapRead")
	}
	if !info.Capabilities.Has(provider.CapList) {
		t.Error("Info().Capabilities should have CapList")
	}
	if !info.Capabilities.Has(provider.CapFetchComments) {
		t.Error("Info().Capabilities should have CapFetchComments")
	}
	if !info.Capabilities.Has(provider.CapComment) {
		t.Error("Info().Capabilities should have CapComment")
	}
	if !info.Capabilities.Has(provider.CapUpdateStatus) {
		t.Error("Info().Capabilities should have CapUpdateStatus")
	}
	if !info.Capabilities.Has(provider.CapSnapshot) {
		t.Error("Info().Capabilities should have CapSnapshot")
	}
	if !info.Capabilities.Has(provider.CapCreatePR) {
		t.Error("Info().Capabilities should have CapCreatePR")
	}
}

func TestGeneratePRTitle(t *testing.T) {
	tests := []struct {
		name     string
		taskWork *storage.TaskWork
		want     string
	}{
		{
			name:     "nil task work",
			taskWork: nil,
			want:     "Implementation",
		},
		{
			name:     "empty metadata",
			taskWork: &storage.TaskWork{},
			want:     "Implementation",
		},
		{
			name: "title only",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Add new feature",
				},
			},
			want: "Add new feature",
		},
		{
			name: "external key only",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "42",
				},
			},
			want: "[#42] Implementation",
		},
		{
			name: "title and external key",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title:       "Fix login bug",
					ExternalKey: "123",
				},
			},
			want: "[#123] Fix login bug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePRTitle(tt.taskWork)
			if got != tt.want {
				t.Errorf("GeneratePRTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeneratePRBody(t *testing.T) {
	tests := []struct {
		name     string
		taskWork *storage.TaskWork
		specs    []*storage.Specification
		diffStat string
		contains []string
	}{
		{
			name:     "nil task work",
			taskWork: nil,
			specs:    nil,
			diffStat: "",
			contains: []string{"## Summary", "## Test Plan"},
		},
		{
			name: "with title",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Add feature X",
				},
			},
			specs:    nil,
			diffStat: "",
			contains: []string{"## Summary", "Implementation for: Add feature X"},
		},
		{
			name: "bitbucket issue link",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "42",
				},
				Source: storage.SourceInfo{
					Type: ProviderName,
				},
			},
			specs:    nil,
			diffStat: "",
			contains: []string{"Closes #42"},
		},
		{
			name:     "with specs",
			taskWork: nil,
			specs: []*storage.Specification{
				{Title: "Spec 1", Content: "Details about spec 1"},
				{Title: "Spec 2", Content: "Details about spec 2"},
			},
			diffStat: "",
			contains: []string{"## Implementation Details", "### Spec 1", "### Spec 2"},
		},
		{
			name:     "with diff stat",
			taskWork: nil,
			specs:    nil,
			diffStat: " 3 files changed, 50 insertions(+), 10 deletions(-)",
			contains: []string{"## Changes", "3 files changed"},
		},
		{
			name:     "has footer",
			taskWork: nil,
			specs:    nil,
			diffStat: "",
			contains: []string{"Generated by [Mehrhof]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePRBody(tt.taskWork, tt.specs, tt.diffStat)
			for _, want := range tt.contains {
				if !containsString(got, want) {
					t.Errorf("GeneratePRBody() missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

func TestMapBitbucketState(t *testing.T) {
	tests := []struct {
		input string
		want  provider.Status
	}{
		{"new", provider.StatusOpen},
		{"open", provider.StatusOpen},
		{"resolved", provider.StatusClosed},
		{"on hold", provider.StatusOpen},
		{"invalid", provider.StatusClosed},
		{"duplicate", provider.StatusClosed},
		{"wontfix", provider.StatusClosed},
		{"closed", provider.StatusClosed},
		{"unknown", provider.StatusOpen},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapBitbucketState(tt.input)
			if got != tt.want {
				t.Errorf("mapBitbucketState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapBitbucketPriority(t *testing.T) {
	tests := []struct {
		input string
		want  provider.Priority
	}{
		{"trivial", provider.PriorityLow},
		{"minor", provider.PriorityNormal},
		{"major", provider.PriorityHigh},
		{"critical", provider.PriorityCritical},
		{"blocker", provider.PriorityCritical},
		{"unknown", provider.PriorityNormal},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapBitbucketPriority(tt.input)
			if got != tt.want {
				t.Errorf("mapBitbucketPriority(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		(len(haystack) > 0 && len(needle) > 0 && findString(haystack, needle)))
}

func findString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
