package provider

import (
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestLinearProvider(t *testing.T) {
	lp := NewLinearProvider("", "")

	if lp.Name() != "linear" {
		t.Errorf("Name() = %s, want linear", lp.Name())
	}
}

func TestLinearProvider_ImplementsInterfaces(t *testing.T) {
	lp := NewLinearProvider("test-token", "ENG")

	// Provider interface
	var _ Provider = lp

	// HierarchyProvider interface
	var _ HierarchyProvider = lp

	// CommentProvider interface
	var _ CommentProvider = lp

	// LabelProvider interface
	var _ LabelProvider = lp

	// ListProvider interface
	var _ ListProvider = lp

	// CreateProvider interface
	var _ CreateProvider = lp

	// AttachmentProvider interface
	var _ AttachmentProvider = lp
}

func TestLinearProvider_FetchTask_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.FetchTask(nil, "ENG-123") //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("FetchTask should fail without token")
	}
	if err.Error() != "LINEAR_TOKEN not set" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLinearProvider_UpdateStatus_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	err := lp.UpdateStatus(nil, "ENG-123", "done") //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("UpdateStatus should fail without token")
	}
}

func TestLinearProvider_FetchParent_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.FetchParent(nil, &Task{}) //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("FetchParent should fail without token")
	}
}

func TestLinearProvider_FetchSiblings_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.FetchSiblings(nil, &Task{}) //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("FetchSiblings should fail without token")
	}
}

func TestLinearProvider_FetchComments_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.FetchComments(nil, "ENG-123") //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("FetchComments should fail without token")
	}
}

func TestLinearProvider_AddComment_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	err := lp.AddComment(nil, "ENG-123", "test comment") //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("AddComment should fail without token")
	}
}

func TestLinearProvider_AddLabels_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	err := lp.AddLabels(nil, "ENG-123", []string{"bug"}) //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("AddLabels should fail without token")
	}
}

func TestLinearProvider_RemoveLabels_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	err := lp.RemoveLabels(nil, "ENG-123", []string{"bug"}) //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("RemoveLabels should fail without token")
	}
}

func TestLinearProvider_ListTasks_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.ListTasks(nil, ListOptions{}) //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("ListTasks should fail without token")
	}
}

func TestLinearProvider_CreateTask_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.CreateTask(nil, CreateTaskOptions{Title: "Test"}) //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("CreateTask should fail without token")
	}
}

func TestLinearProvider_DownloadAttachment_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")

	_, err := lp.DownloadAttachment(nil, "https://example.com/attachment") //nolint:staticcheck // nil context is intentional for test
	if err == nil {
		t.Error("DownloadAttachment should fail without token")
	}
}

func TestLinearPriorityConversion(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{1, "critical"},
		{2, "high"},
		{3, "normal"},
		{4, "low"},
		{0, "normal"},
		{5, "normal"},
	}

	for _, tt := range tests {
		got := linearPriorityToString(tt.priority)
		if got != tt.want {
			t.Errorf("linearPriorityToString(%d) = %s, want %s", tt.priority, got, tt.want)
		}
	}
}

func TestLinearPriorityFromString(t *testing.T) {
	tests := []struct {
		priority string
		want     int
	}{
		{"critical", 1},
		{"urgent", 1},
		{"high", 2},
		{"normal", 3},
		{"medium", 3},
		{"low", 4},
		{"unknown", 0},
	}

	for _, tt := range tests {
		got := linearPriorityFromString(tt.priority)
		if got != tt.want {
			t.Errorf("linearPriorityFromString(%q) = %d, want %d", tt.priority, got, tt.want)
		}
	}
}

func TestLinearProvider_IssueToTask(t *testing.T) {
	lp := NewLinearProvider("test-token", "ENG")

	issue := &linearIssue{
		ID:          "abc123",
		Identifier:  "ENG-456",
		Title:       "Fix bug in login",
		Description: "The login button is broken",
		URL:         "https://linear.app/team/issue/ENG-456",
		Priority:    2,
		State: &linearState{
			ID:   "state-1",
			Name: "In Progress",
			Type: "started",
		},
		Team: &linearTeam{
			ID:  "team-1",
			Key: "ENG",
		},
		Parent: &linearParent{
			ID:         "parent-123",
			Identifier: "ENG-100",
		},
		Labels: &linearLabels{
			Nodes: []linearLabel{
				{ID: "label-1", Name: "bug"},
				{ID: "label-2", Name: "priority"},
			},
		},
		Assignee: &linearUser{
			ID:   "user-1",
			Name: "John Doe",
		},
	}

	task := lp.issueToTask(issue)

	// Check basic fields
	if task.ID != "ENG-456" {
		t.Errorf("ID = %s, want ENG-456", task.ID)
	}
	if task.Title != "Fix bug in login" {
		t.Errorf("Title = %s, want 'Fix bug in login'", task.Title)
	}
	if task.Description != "The login button is broken" {
		t.Errorf("Description mismatch")
	}
	if task.URL != "https://linear.app/team/issue/ENG-456" {
		t.Errorf("URL mismatch")
	}
	if task.Source != "linear" {
		t.Errorf("Source = %s, want linear", task.Source)
	}

	// Check priority (from Linear, not inferred)
	if task.Priority != "high" {
		t.Errorf("Priority = %s, want high", task.Priority)
	}

	// Check labels (includes state name)
	expectedLabels := []string{"bug", "priority", "In Progress"}
	if len(task.Labels) != len(expectedLabels) {
		t.Errorf("Labels count = %d, want %d", len(task.Labels), len(expectedLabels))
	}

	// Check metadata
	if task.Metadata("linear_id") != "abc123" {
		t.Errorf("linear_id = %s, want abc123", task.Metadata("linear_id"))
	}
	if task.Metadata("linear_identifier") != "ENG-456" {
		t.Errorf("linear_identifier = %s, want ENG-456", task.Metadata("linear_identifier"))
	}
	if task.Metadata("linear_team_key") != "ENG" {
		t.Errorf("linear_team_key = %s, want ENG", task.Metadata("linear_team_key"))
	}
	if task.Metadata("linear_parent_id") != "parent-123" {
		t.Errorf("linear_parent_id = %s, want parent-123", task.Metadata("linear_parent_id"))
	}
	if task.Metadata("linear_assignee") != "John Doe" {
		t.Errorf("linear_assignee = %s, want John Doe", task.Metadata("linear_assignee"))
	}
}

func TestRegistryHasLinearProvider(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	linear, err := r.Get("linear")
	if err != nil {
		t.Errorf("Get(linear) error = %v", err)
	}
	if linear == nil {
		t.Error("linear provider should not be nil")
	}
	if linear.Name() != "linear" {
		t.Errorf("Name() = %s, want linear", linear.Name())
	}
}
