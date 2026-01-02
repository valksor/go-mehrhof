package azuredevops

import (
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Reference
		wantErr bool
	}{
		{
			name:  "work item ID only",
			input: "123",
			want: &Reference{
				WorkItemID: 123,
				IsExplicit: false,
			},
		},
		{
			name:  "org/project#ID format",
			input: "myorg/myproject#456",
			want: &Reference{
				Organization: "myorg",
				Project:      "myproject",
				WorkItemID:   456,
				IsExplicit:   true,
			},
		},
		{
			name:  "dev.azure.com URL",
			input: "https://dev.azure.com/myorg/myproject/_workitems/edit/789",
			want: &Reference{
				Organization: "myorg",
				Project:      "myproject",
				WorkItemID:   789,
				IsExplicit:   true,
			},
		},
		{
			name:  "dev.azure.com URL without https",
			input: "dev.azure.com/myorg/myproject/_workitems/edit/101",
			want: &Reference{
				Organization: "myorg",
				Project:      "myproject",
				WorkItemID:   101,
				IsExplicit:   true,
			},
		},
		{
			name:  "visualstudio.com URL",
			input: "https://myorg.visualstudio.com/myproject/_workitems/edit/202",
			want: &Reference{
				Organization: "myorg",
				Project:      "myproject",
				WorkItemID:   202,
				IsExplicit:   true,
			},
		},
		{
			name:  "with azdo prefix",
			input: "azdo:303",
			want: &Reference{
				WorkItemID: 303,
				IsExplicit: false,
			},
		},
		{
			name:  "with azure prefix",
			input: "azure:404",
			want: &Reference{
				WorkItemID: 404,
				IsExplicit: false,
			},
		},
		{
			name:    "invalid input - not a number",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid input - negative number",
			input:   "-1",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
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

			if got.WorkItemID != tt.want.WorkItemID {
				t.Errorf("ParseReference(%q).WorkItemID = %d, want %d", tt.input, got.WorkItemID, tt.want.WorkItemID)
			}
			if got.Organization != tt.want.Organization {
				t.Errorf("ParseReference(%q).Organization = %q, want %q", tt.input, got.Organization, tt.want.Organization)
			}
			if got.Project != tt.want.Project {
				t.Errorf("ParseReference(%q).Project = %q, want %q", tt.input, got.Project, tt.want.Project)
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

	expectedSchemes := []string{"azdo", "azure"}
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
	if !info.Capabilities.Has(provider.CapLinkBranch) {
		t.Error("Info().Capabilities should have CapLinkBranch")
	}
}

func TestMapAzureState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  provider.Status
	}{
		{
			name:  "New state",
			state: "New",
			want:  provider.StatusOpen,
		},
		{
			name:  "To Do state",
			state: "To Do",
			want:  provider.StatusOpen,
		},
		{
			name:  "Active state",
			state: "Active",
			want:  provider.StatusInProgress,
		},
		{
			name:  "In Progress state",
			state: "In Progress",
			want:  provider.StatusInProgress,
		},
		{
			name:  "Resolved state",
			state: "Resolved",
			want:  provider.StatusClosed,
		},
		{
			name:  "Done state",
			state: "Done",
			want:  provider.StatusClosed,
		},
		{
			name:  "Closed state",
			state: "Closed",
			want:  provider.StatusClosed,
		},
		{
			name:  "In Review state",
			state: "In Review",
			want:  provider.StatusReview,
		},
		{
			name:  "Unknown state",
			state: "Something Else",
			want:  provider.StatusOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAzureState(tt.state)
			if got != tt.want {
				t.Errorf("mapAzureState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestMapAzurePriority(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		want     provider.Priority
	}{
		{
			name:     "priority 1 (critical)",
			priority: 1,
			want:     provider.PriorityCritical,
		},
		{
			name:     "priority 2 (high)",
			priority: 2,
			want:     provider.PriorityHigh,
		},
		{
			name:     "priority 3 (normal)",
			priority: 3,
			want:     provider.PriorityNormal,
		},
		{
			name:     "priority 4 (low)",
			priority: 4,
			want:     provider.PriorityLow,
		},
		{
			name:     "priority 0 (default)",
			priority: 0,
			want:     provider.PriorityNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAzurePriority(tt.priority)
			if got != tt.want {
				t.Errorf("mapAzurePriority(%d) = %v, want %v", tt.priority, got, tt.want)
			}
		})
	}
}

func TestMapWorkItemType(t *testing.T) {
	tests := []struct {
		name   string
		wiType string
		want   string
	}{
		{
			name:   "Bug type",
			wiType: "Bug",
			want:   "fix",
		},
		{
			name:   "Feature type",
			wiType: "Feature",
			want:   "feature",
		},
		{
			name:   "User Story type",
			wiType: "User Story",
			want:   "feature",
		},
		{
			name:   "Task type",
			wiType: "Task",
			want:   "task",
		},
		{
			name:   "Epic type",
			wiType: "Epic",
			want:   "epic",
		},
		{
			name:   "Issue type",
			wiType: "Issue",
			want:   "issue",
		},
		{
			name:   "Unknown type",
			wiType: "Unknown",
			want:   "task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapWorkItemType(tt.wiType)
			if got != tt.want {
				t.Errorf("mapWorkItemType(%q) = %q, want %q", tt.wiType, got, tt.want)
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		name string
		tags string
		want []string
	}{
		{
			name: "empty tags",
			tags: "",
			want: nil,
		},
		{
			name: "single tag",
			tags: "bug",
			want: []string{"bug"},
		},
		{
			name: "multiple tags",
			tags: "bug; feature; urgent",
			want: []string{"bug", "feature", "urgent"},
		},
		{
			name: "tags with extra spaces",
			tags: "  bug  ;  feature  ",
			want: []string{"bug", "feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTags(tt.tags)
			if len(got) != len(tt.want) {
				t.Errorf("parseTags(%q) returned %d tags, want %d", tt.tags, len(got), len(tt.want))

				return
			}
			for i, tag := range got {
				if tag != tt.want[i] {
					t.Errorf("parseTags(%q)[%d] = %q, want %q", tt.tags, i, tag, tt.want[i])
				}
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Fix login bug", "fix-login-bug"},
		{"API  --  Integration", "api-integration"},
		{"Test!!!123", "test123"},
		{"Very Long Title That Should Be Truncated After Fifty Characters For Branch Names", "very-long-title-that-should-be-truncated-after-fif"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractWorkItemIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		{
			name:  "AB# format",
			input: "Fixed AB#123 and AB#456",
			want:  []int{123, 456},
		},
		{
			name:  "# format",
			input: "Related to #789",
			want:  []int{789},
		},
		{
			name:  "mixed formats",
			input: "See AB#100 and #200 for details",
			want:  []int{100, 200},
		},
		{
			name:  "no work items",
			input: "Just regular text",
			want:  nil,
		},
		{
			name:  "duplicates",
			input: "AB#123 is related to AB#123",
			want:  []int{123},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractWorkItemIDs(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractWorkItemIDs() returned %d IDs, want %d", len(got), len(tt.want))

				return
			}
			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("ExtractWorkItemIDs()[%d] = %d, want %d", i, id, tt.want[i])
				}
			}
		})
	}
}

func TestReferenceString(t *testing.T) {
	tests := []struct {
		ref  Reference
		want string
	}{
		{
			ref:  Reference{WorkItemID: 123},
			want: "123",
		},
		{
			ref:  Reference{Organization: "org", Project: "proj", WorkItemID: 456},
			want: "org/proj#456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("Reference.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAzureTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		empty bool
	}{
		{
			name:  "empty string",
			input: "",
			empty: true,
		},
		{
			name:  "RFC3339",
			input: "2024-01-01T00:00:00Z",
			empty: false,
		},
		{
			name:  "ISO 8601 without ms",
			input: "2024-01-15T10:30:00Z",
			empty: false,
		},
		{
			name:  "invalid timestamp",
			input: "not-a-timestamp",
			empty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAzureTime(tt.input)
			if tt.empty && !got.IsZero() {
				t.Errorf("parseAzureTime(%q) = %v, want zero time", tt.input, got)
			}
			if !tt.empty && got.IsZero() {
				t.Errorf("parseAzureTime(%q) returned zero time, expected non-zero", tt.input)
			}
		})
	}
}
