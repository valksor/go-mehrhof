package wrike

import (
	"strings"
	"testing"
	"time"

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
	expectedSchemes := []string{"wrike", "wk"}
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
// mapStatus tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   provider.Status
	}{
		{
			name:   "active status",
			status: "Active",
			want:   provider.StatusOpen,
		},
		{
			name:   "new status",
			status: "New",
			want:   provider.StatusOpen,
		},
		{
			name:   "in progress status",
			status: "In Progress",
			want:   provider.StatusOpen,
		},
		{
			name:   "inprogress status (no space)",
			status: "Inprogress",
			want:   provider.StatusOpen,
		},
		{
			name:   "draft status",
			status: "Draft",
			want:   provider.StatusOpen,
		},
		{
			name:   "completed status",
			status: "Completed",
			want:   provider.StatusDone,
		},
		{
			name:   "done status",
			status: "Done",
			want:   provider.StatusDone,
		},
		{
			name:   "closed status",
			status: "Closed",
			want:   provider.StatusClosed,
		},
		{
			name:   "cancelled status",
			status: "Cancelled",
			want:   provider.StatusClosed,
		},
		{
			name:   "canceled status (single l)",
			status: "Canceled",
			want:   provider.StatusClosed,
		},
		{
			name:   "deferred status",
			status: "Deferred",
			want:   provider.StatusClosed,
		},
		{
			name:   "review status",
			status: "Review",
			want:   provider.StatusReview,
		},
		{
			name:   "unknown status defaults to open",
			status: "Unknown",
			want:   provider.StatusOpen,
		},
		{
			name:   "empty status defaults to open",
			status: "",
			want:   provider.StatusOpen,
		},
		{
			name:   "case insensitive - ACTIVE",
			status: "ACTIVE",
			want:   provider.StatusOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStatus(tt.status)
			if got != tt.want {
				t.Errorf("mapStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapPriority tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		want     provider.Priority
	}{
		{
			name:     "critical priority",
			priority: "Critical",
			want:     provider.PriorityCritical,
		},
		{
			name:     "urgent priority",
			priority: "Urgent",
			want:     provider.PriorityCritical,
		},
		{
			name:     "high priority",
			priority: "High",
			want:     provider.PriorityHigh,
		},
		{
			name:     "low priority",
			priority: "Low",
			want:     provider.PriorityLow,
		},
		{
			name:     "normal priority",
			priority: "Normal",
			want:     provider.PriorityNormal,
		},
		{
			name:     "empty priority defaults to normal",
			priority: "",
			want:     provider.PriorityNormal,
		},
		{
			name:     "unknown priority defaults to normal",
			priority: "Unknown",
			want:     provider.PriorityNormal,
		},
		{
			name:     "case insensitive - CRITICAL",
			priority: "CRITICAL",
			want:     provider.PriorityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPriority(tt.priority)
			if got != tt.want {
				t.Errorf("mapPriority(%q) = %v, want %v", tt.priority, got, tt.want)
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
		comments []Comment
		wantLen  int
	}{
		{
			name: "multiple comments",
			comments: []Comment{
				{
					ID:          "IEAAJ1",
					Text:        "First comment",
					AuthorID:    "USER1",
					AuthorName:  "Author One",
					CreatedDate: now,
					UpdatedDate: now,
				},
				{
					ID:          "IEAAJ2",
					Text:        "Second comment",
					AuthorID:    "USER2",
					AuthorName:  "Author Two",
					CreatedDate: now,
					UpdatedDate: now,
				},
			},
			wantLen: 2,
		},
		{
			name:     "empty comments",
			comments: []Comment{},
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
				if got[i].ID != c.ID {
					t.Errorf("mapComments()[%d].ID = %q, want %q", i, got[i].ID, c.ID)
				}
				if got[i].Body != c.Text {
					t.Errorf("mapComments()[%d].Body = %q, want %q", i, got[i].Body, c.Text)
				}
				if got[i].Author.Name != c.AuthorName {
					t.Errorf("mapComments()[%d].Author.Name = %q, want %q", i, got[i].Author.Name, c.AuthorName)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// buildMetadata tests
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildMetadata(t *testing.T) {
	now := time.Now()

	task := &Task{
		ID:          "IEAAJTASK",
		Title:       "Test Task",
		Description: "Test description",
		Status:      "Active",
		Priority:    "High",
		Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
		SubTaskIDs:  []string{"IEAAJ1", "IEAAJ2"},
		CreatedDate: now,
		UpdatedDate: now,
	}

	subtasks := []SubtaskInfo{
		{ID: "IEAAJ1", Title: "Subtask 1", Status: "Active"},
		{ID: "IEAAJ2", Title: "Subtask 2", Status: "Completed"},
	}

	metadata := buildMetadata(task, subtasks)

	// Check basic metadata
	if metadata["permalink"] != task.Permalink {
		t.Errorf("permalink = %q, want %q", metadata["permalink"], task.Permalink)
	}
	if metadata["api_id"] != task.ID {
		t.Errorf("api_id = %q, want %q", metadata["api_id"], task.ID)
	}
	if metadata["wrike_status"] != task.Status {
		t.Errorf("wrike_status = %q, want %q", metadata["wrike_status"], task.Status)
	}
	if metadata["wrike_priority"] != task.Priority {
		t.Errorf("wrike_priority = %q, want %q", metadata["wrike_priority"], task.Priority)
	}

	// Check subtask metadata
	if metadata["subtask_count"] != 2 {
		t.Errorf("subtask_count = %v, want 2", metadata["subtask_count"])
	}
	subtaskList, ok := metadata["subtasks"].([]map[string]string)
	if !ok {
		t.Fatal("subtasks is not of type []map[string]string")
	}
	if len(subtaskList) != 2 {
		t.Errorf("len(subtasks) = %d, want 2", len(subtaskList))
	}
}

func TestBuildMetadataWithNoSubtasks(t *testing.T) {
	now := time.Now()

	task := &Task{
		ID:          "IEAAJTASK",
		Title:       "Test Task",
		Description: "Test description",
		Status:      "Active",
		Priority:    "High",
		Permalink:   "https://www.wrike.com/open.htm?id=1234567890",
		SubTaskIDs:  []string{},
		CreatedDate: now,
		UpdatedDate: now,
	}

	metadata := buildMetadata(task, nil)

	// Check basic metadata exists
	if metadata["permalink"] != task.Permalink {
		t.Errorf("permalink = %q, want %q", metadata["permalink"], task.Permalink)
	}

	// Check subtasks are not present
	if _, ok := metadata["subtasks"]; ok {
		t.Error("subtasks should not be present when there are none")
	}
	if _, ok := metadata["subtask_count"]; ok {
		t.Error("subtask_count should not be present when there are none")
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
			name:  "wrike scheme with numeric ID",
			input: "wrike:1234567890",
			want:  true,
		},
		{
			name:  "wrike scheme with API ID",
			input: "wrike:IEAAJXXXXXXXX",
			want:  true,
		},
		{
			name:  "wk scheme with numeric ID",
			input: "wk:1234567890",
			want:  true,
		},
		{
			name:  "wk scheme with API ID",
			input: "wk:IEAAJXXXXXXXX",
			want:  true,
		},
		{
			name:  "permalink URL",
			input: "https://www.wrike.com/open.htm?id=1234567890",
			want:  true,
		},
		{
			name:  "bare API ID",
			input: "IEAAJXXXXXXXX",
			want:  true,
		},
		{
			name:  "bare numeric ID (10 digits)",
			input: "1234567890",
			want:  true,
		},
		{
			name:  "bare numeric ID (more than 10 digits)",
			input: "12345678901234",
			want:  true,
		},
		{
			name:  "file scheme",
			input: "file:task.md",
			want:  false,
		},
		{
			name:  "github scheme",
			input: "github:123",
			want:  false,
		},
		{
			name:  "no scheme - short text",
			input: "just-text",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "similar but not wrike",
			input: "wrike-actions:run",
			want:  false,
		},
		{
			name:  "numeric ID less than 10 digits",
			input: "123456789",
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
// Provider.Parse tests
// ──────────────────────────────────────────────────────────────────────────────

func TestProvider_Parse(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name        string
		input       string
		want        string
		errContains string
		wantErr     bool
	}{
		{
			name:  "wrike scheme with numeric ID",
			input: "wrike:1234567890",
			want:  "1234567890",
		},
		{
			name:  "wrike scheme with API ID",
			input: "wrike:IEAAJXXXXXXXX",
			want:  "IEAAJXXXXXXXX",
		},
		{
			name:  "wk scheme with numeric ID",
			input: "wk:1234567890",
			want:  "1234567890",
		},
		{
			name:  "wk scheme with API ID",
			input: "wk:IEAAJXXXXXXXX",
			want:  "IEAAJXXXXXXXX",
		},
		{
			name:  "permalink - extracts numeric ID",
			input: "https://www.wrike.com/open.htm?id=1234567890",
			want:  "1234567890",
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "invalid format - not a valid ID",
			input:       "wrike:abc",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "invalid format - not enough digits",
			input:       "wrike:123",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "not a wrike reference",
			input:       "github:123",
			wantErr:     true,
			errContains: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
// ProviderName constant test
// ──────────────────────────────────────────────────────────────────────────────

func TestProviderName(t *testing.T) {
	if ProviderName != "wrike" {
		t.Errorf("ProviderName = %q, want %q", ProviderName, "wrike")
	}
}
