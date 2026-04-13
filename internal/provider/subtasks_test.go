package provider

import (
	"slices"
	"testing"
)

func TestParseSubtasks(t *testing.T) {
	body := `# Task
- [ ] First task
- [x] Completed task
- [ ] Third task
`
	taskID := "owner/repo#123"
	subtasks := ParseSubtasks(taskID, body)

	if len(subtasks) != 3 {
		t.Fatalf("len(subtasks) = %d, want 3", len(subtasks))
	}
	if subtasks[0].Text != "First task" {
		t.Errorf("Text = %q, want 'First task'", subtasks[0].Text)
	}
	if subtasks[0].Completed {
		t.Error("subtasks[0] should not be completed")
	}
	if !subtasks[1].Completed {
		t.Error("subtasks[1] should be completed")
	}
	if subtasks[0].ID != "owner/repo#123-task-0" {
		t.Errorf("ID = %q, want owner/repo#123-task-0", subtasks[0].ID)
	}
	if subtasks[2].Text != "Third task" {
		t.Errorf("Text = %q, want 'Third task'", subtasks[2].Text)
	}
	if subtasks[2].Completed {
		t.Error("subtasks[2] should not be completed")
	}
	if subtasks[2].ID != "owner/repo#123-task-2" {
		t.Errorf("ID = %q, want owner/repo#123-task-2", subtasks[2].ID)
	}
}

func TestParseSubtasks_Empty(t *testing.T) {
	if subtasks := ParseSubtasks("t1", ""); subtasks != nil {
		t.Error("expected nil for empty body")
	}
	if subtasks := ParseSubtasks("t1", "no checkboxes here"); subtasks != nil {
		t.Error("expected nil for body without checkboxes")
	}
}

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{"single", "Depends on: #123", []string{"#123"}},
		{"multiple", "Depends on: #123, #456", []string{"#123", "#456"}},
		{"cross-repo", "Depends on: owner/repo#789", []string{"owner/repo#789"}},
		{"none", "No dependencies", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDependencies(tt.body)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("ParseDependencies(%q) = %v, want %v", tt.body, result, tt.expected)
			}
		})
	}
}
