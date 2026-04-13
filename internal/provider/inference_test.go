package provider

import "testing"

func TestInferPriority(t *testing.T) {
	tests := []struct {
		labels   []string
		expected string
	}{
		{[]string{"p0"}, "p0"},
		{[]string{"priority:critical"}, "p0"},
		{[]string{"p1"}, "p1"},
		{[]string{"priority:high"}, "p1"},
		{[]string{"bug", "p1"}, "p1"},
		{[]string{}, ""},
		{[]string{"bug"}, ""},
	}
	for _, tt := range tests {
		result := InferPriority(tt.labels)
		if result != tt.expected {
			t.Errorf("InferPriority(%v) = %q, want %q", tt.labels, result, tt.expected)
		}
	}
}

func TestInferType(t *testing.T) {
	tests := []struct {
		labels   []string
		expected string
	}{
		{[]string{"bug"}, "bug"},
		{[]string{"defect"}, "bug"},
		{[]string{"fix"}, "bug"},
		{[]string{"feature"}, "feature"},
		{[]string{"enhancement"}, "feature"},
		{[]string{"feat"}, "feature"},
		{[]string{"chore"}, "chore"},
		{[]string{"maintenance"}, "chore"},
		{[]string{"tech-debt"}, "chore"},
		{[]string{"docs"}, "docs"},
		{[]string{"documentation"}, "docs"},
		{[]string{"p1", "bug"}, "bug"},
		{[]string{}, ""},
		{[]string{"help-wanted"}, ""},
	}
	for _, tt := range tests {
		result := InferType(tt.labels)
		if result != tt.expected {
			t.Errorf("InferType(%v) = %q, want %q", tt.labels, result, tt.expected)
		}
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"Fix login bug", "fix-login-bug"},
		{"Add user authentication", "add-user-authentication"},
		{"Update README.md", "update-readme-md"},
		{"Fix: API endpoint returns 500", "fix-api-endpoint-returns-500"},
		{"  spaces around  ", "spaces-around"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"UPPERCASE", "uppercase"},
		{"special@#$%chars!", "special-chars"},
		{"", ""},
		{"a very long title that exceeds the maximum slug length limit and should be truncated properly", "a-very-long-title-that-exceeds-the-maximum-slug-le"},
		{"truncate at word boundary-", "truncate-at-word-boundary"},
	}
	for _, tt := range tests {
		result := GenerateSlug(tt.title)
		if result != tt.expected {
			t.Errorf("GenerateSlug(%q) = %q, want %q", tt.title, result, tt.expected)
		}
	}
}

func TestInferAll(t *testing.T) {
	tests := []struct {
		title            string
		labels           []string
		expectedPriority string
		expectedType     string
		expectedSlug     string
	}{
		{
			title:            "Fix authentication bug",
			labels:           []string{"bug", "p1"},
			expectedPriority: "p1",
			expectedType:     "bug",
			expectedSlug:     "fix-authentication-bug",
		},
		{
			title:            "Add user dashboard",
			labels:           []string{"feature", "priority:high"},
			expectedPriority: "p1",
			expectedType:     "feature",
			expectedSlug:     "add-user-dashboard",
		},
		{
			title:            "Update dependencies",
			labels:           []string{"chore"},
			expectedPriority: "",
			expectedType:     "chore",
			expectedSlug:     "update-dependencies",
		},
		{
			title:            "",
			labels:           []string{},
			expectedPriority: "",
			expectedType:     "",
			expectedSlug:     "",
		},
	}
	for _, tt := range tests {
		priority, taskType, slug := InferAll(tt.title, tt.labels)
		if priority != tt.expectedPriority {
			t.Errorf("InferAll(%q, %v) priority = %q, want %q", tt.title, tt.labels, priority, tt.expectedPriority)
		}
		if taskType != tt.expectedType {
			t.Errorf("InferAll(%q, %v) type = %q, want %q", tt.title, tt.labels, taskType, tt.expectedType)
		}
		if slug != tt.expectedSlug {
			t.Errorf("InferAll(%q, %v) slug = %q, want %q", tt.title, tt.labels, slug, tt.expectedSlug)
		}
	}
}
