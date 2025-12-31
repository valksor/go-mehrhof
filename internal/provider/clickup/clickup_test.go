package clickup

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
			name:  "task ID only",
			input: "abc1234",
			want: &Reference{
				TaskID:     "abc1234",
				IsExplicit: false,
			},
		},
		{
			name:  "custom task ID",
			input: "PROJ-123",
			want: &Reference{
				CustomID:   "PROJ-123",
				IsExplicit: false,
			},
		},
		{
			name:  "app URL with team",
			input: "https://app.clickup.com/t/12345678/abc1234",
			want: &Reference{
				TaskID:     "abc1234",
				TeamID:     "12345678",
				IsExplicit: true,
			},
		},
		{
			name:  "app URL without team",
			input: "https://app.clickup.com/t/abc1234",
			want: &Reference{
				TaskID:     "abc1234",
				IsExplicit: true,
			},
		},
		{
			name:  "app URL without https",
			input: "app.clickup.com/t/12345678/xyz9876",
			want: &Reference{
				TaskID:     "xyz9876",
				TeamID:     "12345678",
				IsExplicit: true,
			},
		},
		{
			name:  "share URL",
			input: "https://sharing.clickup.com/12345/t/h/abc1234/somehash",
			want: &Reference{
				TaskID:     "abc1234",
				IsExplicit: true,
			},
		},
		{
			name:  "with clickup prefix",
			input: "clickup:abc1234",
			want: &Reference{
				TaskID:     "abc1234",
				IsExplicit: false,
			},
		},
		{
			name:  "with cu prefix",
			input: "cu:PROJ-456",
			want: &Reference{
				CustomID:   "PROJ-456",
				IsExplicit: false,
			},
		},
		{
			name:    "invalid input - too short",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid input - random string",
			input:   "hello-world",
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

			if got.TaskID != tt.want.TaskID {
				t.Errorf("ParseReference(%q).TaskID = %q, want %q", tt.input, got.TaskID, tt.want.TaskID)
			}
			if got.CustomID != tt.want.CustomID {
				t.Errorf("ParseReference(%q).CustomID = %q, want %q", tt.input, got.CustomID, tt.want.CustomID)
			}
			if got.TeamID != tt.want.TeamID {
				t.Errorf("ParseReference(%q).TeamID = %q, want %q", tt.input, got.TeamID, tt.want.TeamID)
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

	expectedSchemes := []string{"clickup", "cu"}
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
}

func TestMapClickUpStatus(t *testing.T) {
	tests := []struct {
		name string
		task *Task
		want provider.Status
	}{
		{
			name: "nil status",
			task: &Task{Status: nil},
			want: provider.StatusOpen,
		},
		{
			name: "closed type",
			task: &Task{Status: &Status{Type: "closed"}},
			want: provider.StatusClosed,
		},
		{
			name: "done type",
			task: &Task{Status: &Status{Type: "done"}},
			want: provider.StatusClosed,
		},
		{
			name: "open type",
			task: &Task{Status: &Status{Type: "open"}},
			want: provider.StatusOpen,
		},
		{
			name: "in progress status name",
			task: &Task{Status: &Status{Status: "In Progress"}},
			want: provider.StatusInProgress,
		},
		{
			name: "in review status name",
			task: &Task{Status: &Status{Status: "In Review"}},
			want: provider.StatusReview,
		},
		{
			name: "complete status name",
			task: &Task{Status: &Status{Status: "Complete"}},
			want: provider.StatusClosed,
		},
		{
			name: "todo status name",
			task: &Task{Status: &Status{Status: "To Do"}},
			want: provider.StatusOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapClickUpStatus(tt.task)
			if got != tt.want {
				t.Errorf("mapClickUpStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapClickUpPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority *Priority
		want     provider.Priority
	}{
		{
			name:     "nil priority",
			priority: nil,
			want:     provider.PriorityNormal,
		},
		{
			name:     "urgent",
			priority: &Priority{Priority: "urgent"},
			want:     provider.PriorityCritical,
		},
		{
			name:     "high",
			priority: &Priority{Priority: "high"},
			want:     provider.PriorityHigh,
		},
		{
			name:     "normal",
			priority: &Priority{Priority: "normal"},
			want:     provider.PriorityNormal,
		},
		{
			name:     "low",
			priority: &Priority{Priority: "low"},
			want:     provider.PriorityLow,
		},
		{
			name:     "priority level 1",
			priority: &Priority{Priority: "1"},
			want:     provider.PriorityCritical,
		},
		{
			name:     "priority level 4",
			priority: &Priority{Priority: "4"},
			want:     provider.PriorityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapClickUpPriority(tt.priority)
			if got != tt.want {
				t.Errorf("mapClickUpPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapTaskType(t *testing.T) {
	tests := []struct {
		name string
		task *Task
		want string
	}{
		{
			name: "regular task",
			task: &Task{},
			want: "task",
		},
		{
			name: "bug tag",
			task: &Task{
				Tags: []Tag{{Name: "Bug"}},
			},
			want: "fix",
		},
		{
			name: "feature tag",
			task: &Task{
				Tags: []Tag{{Name: "Feature"}},
			},
			want: "feature",
		},
		{
			name: "enhancement tag",
			task: &Task{
				Tags: []Tag{{Name: "Enhancement"}},
			},
			want: "feature",
		},
		{
			name: "chore tag",
			task: &Task{
				Tags: []Tag{{Name: "Chore"}},
			},
			want: "task",
		},
		{
			name: "docs tag",
			task: &Task{
				Tags: []Tag{{Name: "Documentation"}},
			},
			want: "docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapTaskType(tt.task)
			if got != tt.want {
				t.Errorf("mapTaskType() = %q, want %q", got, tt.want)
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

func TestExtractTaskIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single task URL",
			input: "Check out https://app.clickup.com/t/abc1234",
			want:  []string{"abc1234"},
		},
		{
			name:  "multiple task URLs",
			input: "Related: app.clickup.com/t/abc1234 and app.clickup.com/t/xyz5678",
			want:  []string{"abc1234", "xyz5678"},
		},
		{
			name:  "task URL with team",
			input: "See https://app.clickup.com/t/12345/abc1234",
			want:  []string{"abc1234"},
		},
		{
			name:  "no task URLs",
			input: "This is just regular text",
			want:  nil,
		},
		{
			name:  "duplicate task URLs",
			input: "See app.clickup.com/t/abc1234 and again app.clickup.com/t/abc1234",
			want:  []string{"abc1234"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTaskIDs(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractTaskIDs() returned %d IDs, want %d", len(got), len(tt.want))
				return
			}
			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("ExtractTaskIDs()[%d] = %q, want %q", i, id, tt.want[i])
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
			ref:  Reference{TaskID: "abc1234"},
			want: "abc1234",
		},
		{
			ref:  Reference{CustomID: "PROJ-123"},
			want: "PROJ-123",
		},
		{
			ref:  Reference{TaskID: "abc1234", CustomID: "PROJ-456"},
			want: "PROJ-456",
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

func TestParseTimestamp(t *testing.T) {
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
			name:  "millisecond timestamp",
			input: "1704067200000",
			empty: false,
		},
		{
			name:  "RFC3339",
			input: "2024-01-01T00:00:00Z",
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
			got := parseTimestamp(tt.input)
			if tt.empty && !got.IsZero() {
				t.Errorf("parseTimestamp(%q) = %v, want zero time", tt.input, got)
			}
			if !tt.empty && got.IsZero() {
				t.Errorf("parseTimestamp(%q) returned zero time, expected non-zero", tt.input)
			}
		})
	}
}
