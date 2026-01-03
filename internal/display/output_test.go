package display

import (
	"strings"
	"testing"
)

func TestFormatTaskInfo(t *testing.T) {
	// Disable colors for consistent test output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name     string
		header   string
		info     TaskInfo
		opts     TaskInfoOptions
		contains []string
		excludes []string
	}{
		{
			name:   "full task info",
			header: "Task",
			info: TaskInfo{
				TaskID:      "abc123",
				Title:       "My Feature",
				ExternalKey: "FEAT-42",
				State:       "planning",
				Source:      "file:task.md",
				Branch:      "feature/abc123",
			},
			opts: DefaultTaskInfoOptions(),
			contains: []string{
				"Task: abc123",
				"Title:    My Feature",
				"Key:      FEAT-42",
				"[P] Planning", // Includes accessibility prefix
				"AI is creating specifications",
				"Source:   file:task.md",
				"Branch:   feature/abc123",
			},
		},
		{
			name:   "compact mode hides description",
			header: "Task",
			info: TaskInfo{
				TaskID: "abc123",
				State:  "planning",
			},
			opts: TaskInfoOptions{
				ShowState: true,
				Compact:   true,
			},
			contains: []string{
				"[P] Planning", // Includes accessibility prefix
			},
			excludes: []string{
				"AI is creating", // State description should not appear in compact mode
			},
		},
		{
			name:   "empty fields are hidden",
			header: "Task started",
			info: TaskInfo{
				TaskID: "xyz789",
				Title:  "Bug Fix",
				// No ExternalKey, Branch, etc.
			},
			opts: DefaultTaskInfoOptions(),
			contains: []string{
				"Task started: xyz789",
				"Title:    Bug Fix",
			},
			excludes: []string{
				"Key:",
				"Branch:",
				"Worktree:",
			},
		},
		{
			name:   "selective fields",
			header: "Task",
			info: TaskInfo{
				TaskID: "test123",
				Title:  "Test Task",
				State:  "idle",
				Source: "github:123",
			},
			opts: TaskInfoOptions{
				ShowTitle:  true,
				ShowSource: true,
				// State explicitly disabled
			},
			contains: []string{
				"Task: test123",
				"Title:    Test Task",
				"Source:   github:123",
			},
			excludes: []string{
				"State:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTaskInfo(tt.header, tt.info, tt.opts)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("FormatTaskInfo() missing expected string %q\ngot:\n%s", want, result)
				}
			}

			for _, exclude := range tt.excludes {
				if strings.Contains(result, exclude) {
					t.Errorf("FormatTaskInfo() contains unexpected string %q\ngot:\n%s", exclude, result)
				}
			}
		})
	}
}

func TestFormatNextSteps(t *testing.T) {
	// Disable colors for consistent test output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name     string
		steps    []NextStep
		contains []string
	}{
		{
			name:     "empty steps",
			steps:    []NextStep{},
			contains: []string{}, // Should return empty string
		},
		{
			name: "single step",
			steps: []NextStep{
				{Command: "mehr plan", Description: "Create specifications"},
			},
			contains: []string{
				"Next steps:",
				"mehr plan",
				"Create specifications",
			},
		},
		{
			name: "multiple steps aligned",
			steps: []NextStep{
				{Command: "mehr plan", Description: "Create specifications"},
				{Command: "mehr implement", Description: "Implement the code"},
				{Command: "mehr note", Description: "Add notes"},
			},
			contains: []string{
				"Next steps:",
				"mehr plan",
				"mehr implement",
				"mehr note",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNextSteps(tt.steps)

			if len(tt.steps) == 0 {
				if result != "" {
					t.Errorf("FormatNextSteps() with empty steps should return empty string, got %q", result)
				}

				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("FormatNextSteps() missing expected string %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

func TestFormatConfirmation(t *testing.T) {
	// Disable colors for consistent test output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name     string
		summary  string
		details  []string
		warning  string
		contains []string
	}{
		{
			name:    "summary only",
			summary: "Finish task: abc123",
			contains: []string{
				"Finish task: abc123",
			},
		},
		{
			name:    "with details",
			summary: "Finish task: abc123",
			details: []string{
				"Title: My Feature",
				"Branch: feature/abc123",
			},
			contains: []string{
				"Finish task: abc123",
				"Title: My Feature",
				"Branch: feature/abc123",
			},
		},
		{
			name:    "with warning",
			summary: "Abandon task: xyz789",
			warning: "This will delete all changes!",
			contains: []string{
				"Abandon task: xyz789",
				"This will delete all changes!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatConfirmation(tt.summary, tt.details, tt.warning)

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("FormatConfirmation() missing expected string %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}
