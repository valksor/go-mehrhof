package youtrack

import (
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{
			name:    "short scheme",
			input:   "yt:ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "full scheme",
			input:   "youtrack:ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "bare ID",
			input:   "ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "URL - youtrack.cloud",
			input:   "https://youtrack.cloud/issue/ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "URL - myjetbrains.com",
			input:   "https://company.myjetbrains.com/youtrack/issue/ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "URL with title",
			input:   "https://company.myjetbrains.com/youtrack/issue/ABC-123-some-title",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "mixed case ID",
			input:   "aBc-123",
			wantID:  "ABC-123", // Normalized to uppercase
			wantErr: false,
		},
		{
			name:    "invalid format - no dash",
			input:   "ABC123",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "numeric only project",
			input:   "123-456",
			wantID:  "123-456",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ref.ID != tt.wantID {
				t.Errorf("ParseReference() ID = %v, want %v", ref.ID, tt.wantID)
			}
		})
	}
}

func TestIsValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"ABC-123", true},
		{"PROJECT-1", true},
		{"123-456", true},
		{"A1-23", true},      // valid: number part is digits only
		{"A1-B2", false},     // invalid: letter in number part
		{"abc-123", true},    // lowercase is now valid
		{"ABC123", false},    // no dash
		{"ABC-", false},       // no number
		{"-123", false},       // no project
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsValidID(tt.id); got != tt.want {
				t.Errorf("IsValidID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestMapPriorityValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "critical",
			value:    map[string]interface{}{"name": "Critical"},
			expected: "critical", // Will map to PriorityCritical
		},
		{
			name:     "urgent",
			value:    map[string]interface{}{"name": "Urgent"},
			expected: "critical",
		},
		{
			name:     "high",
			value:    map[string]interface{}{"name": "High"},
			expected: "high",
		},
		{
			name:     "normal",
			value:    map[string]interface{}{"name": "Normal"},
			expected: "normal",
		},
		{
			name:     "low",
			value:    map[string]interface{}{"name": "Low"},
			expected: "low",
		},
		{
			name:     "unknown",
			value:    map[string]interface{}{"name": "Unknown"},
			expected: "normal",
		},
		{
			name:     "not a map",
			value:    "string",
			expected: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPriorityValue(tt.value)
			if got.String() != tt.expected {
				t.Errorf("mapPriorityValue() = %v, want %v", got.String(), tt.expected)
			}
		})
	}
}

func TestMapStatusValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "new",
			value:    map[string]interface{}{"name": "New"},
			expected: "open",
		},
		{
			name:     "in progress",
			value:    map[string]interface{}{"name": "In Progress"},
			expected: "in_progress",
		},
		{
			name:     "review",
			value:    map[string]interface{}{"name": "Review"},
			expected: "review",
		},
		{
			name:     "done",
			value:    map[string]interface{}{"name": "Done"},
			expected: "done",
		},
		{
			name:     "fixed",
			value:    map[string]interface{}{"name": "Fixed"},
			expected: "done",
		},
		{
			name:     "closed",
			value:    map[string]interface{}{"name": "Closed"},
			expected: "closed",
		},
		{
			name:     "obsolete",
			value:    map[string]interface{}{"name": "Obsolete"},
			expected: "closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStatusValue(tt.value)
			if string(got) != tt.expected {
				t.Errorf("mapStatusValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractNameFromValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "map with name",
			value: map[string]interface{}{"name": "TestValue", "id": "123"},
			want:  "TestValue",
		},
		{
			name:  "map without name",
			value: map[string]interface{}{"id": "123"},
			want:  "",
		},
		{
			name:  "string value",
			value: "DirectString",
			want:  "DirectString",
		},
		{
			name:  "nil value",
			value: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractNameFromValue(tt.value); got != tt.want {
				t.Errorf("extractNameFromValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusToYouTrackState(t *testing.T) {
	tests := []struct {
		status provider.Status
		want   string
	}{
		{provider.StatusOpen, "New"},
		{provider.StatusInProgress, "In Progress"},
		{provider.StatusReview, "Review"},
		{provider.StatusDone, "Done"},
		{provider.StatusClosed, "Obsolete"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := statusToYouTrackState(tt.status); got != tt.want {
				t.Errorf("statusToYouTrackState() = %v, want %v", got, tt.want)
			}
		})
	}
}
