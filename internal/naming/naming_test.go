package naming

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		maxLen int
		want   string
	}{
		{
			name:   "simple title",
			title:  "Add user authentication",
			maxLen: 50,
			want:   "add-user-authentication",
		},
		{
			name:   "with special characters",
			title:  "Fix bug #123: login fails!",
			maxLen: 50,
			want:   "fix-bug-123-login-fails",
		},
		{
			name:   "with diacritics",
			title:  "Résumé parsing für Änderungen",
			maxLen: 50,
			want:   "resume-parsing-fur-anderungen",
		},
		{
			name:   "truncate at word boundary",
			title:  "This is a very long title that should be truncated",
			maxLen: 20,
			want:   "this-is-a-very-long",
		},
		{
			name:   "truncate mid-word when necessary",
			title:  "Supercalifragilisticexpialidocious",
			maxLen: 15,
			want:   "supercalifragil",
		},
		{
			name:   "empty string",
			title:  "",
			maxLen: 50,
			want:   "",
		},
		{
			name:   "only special chars",
			title:  "!@#$%^&*()",
			maxLen: 50,
			want:   "",
		},
		{
			name:   "underscores to hyphens",
			title:  "user_authentication_module",
			maxLen: 50,
			want:   "user-authentication-module",
		},
		{
			name:   "multiple spaces",
			title:  "Add    multiple   spaces",
			maxLen: 50,
			want:   "add-multiple-spaces",
		},
		{
			name:   "no max length",
			title:  "No limit on length",
			maxLen: 0,
			want:   "no-limit-on-length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.title, tt.maxLen)
			if got != tt.want {
				t.Errorf("Slugify(%q, %d) = %q, want %q", tt.title, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExpandTemplate(t *testing.T) {
	vars := TemplateVars{
		Key:    "FEATURE-123",
		TaskID: "a1b2c3d4",
		Type:   "feature",
		Slug:   "add-user-auth",
		Title:  "Add user authentication",
	}

	tests := []struct {
		name    string
		pattern string
		vars    TemplateVars
		want    string
	}{
		{
			name:    "default branch pattern",
			pattern: "{type}/{key}--{slug}",
			vars:    vars,
			want:    "feature/FEATURE-123--add-user-auth",
		},
		{
			name:    "default commit prefix",
			pattern: "[{key}]",
			vars:    vars,
			want:    "[FEATURE-123]",
		},
		{
			name:    "task_id pattern",
			pattern: "task/{task_id}",
			vars:    vars,
			want:    "task/a1b2c3d4",
		},
		{
			name:    "all variables",
			pattern: "{type}/{key}/{task_id}/{slug}/{title}",
			vars:    vars,
			want:    "feature/FEATURE-123/a1b2c3d4/add-user-auth/Add user authentication",
		},
		{
			name:    "unknown variable preserved",
			pattern: "{type}/{unknown}",
			vars:    vars,
			want:    "feature/{unknown}",
		},
		{
			name:    "no variables",
			pattern: "static/branch/name",
			vars:    vars,
			want:    "static/branch/name",
		},
		{
			name:    "empty pattern",
			pattern: "",
			vars:    vars,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandTemplate(tt.pattern, tt.vars)
			if got != tt.want {
				t.Errorf("ExpandTemplate(%q, vars) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{
			name:    "valid pattern",
			pattern: "{type}/{key}--{slug}",
			want:    nil,
		},
		{
			name:    "unknown variable",
			pattern: "{type}/{unknown}",
			want:    []string{"unknown"},
		},
		{
			name:    "multiple unknown",
			pattern: "{foo}/{bar}/{key}",
			want:    []string{"foo", "bar"},
		},
		{
			name:    "no variables",
			pattern: "static/path",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidatePattern(tt.pattern)
			if len(got) != len(tt.want) {
				t.Errorf("ValidatePattern(%q) = %v, want %v", tt.pattern, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("ValidatePattern(%q)[%d] = %q, want %q", tt.pattern, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestTaskTypeFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "ticket pattern FEATURE",
			filename: "FEATURE-123.md",
			want:     "feature",
		},
		{
			name:     "ticket pattern FIX",
			filename: "FIX-456.md",
			want:     "fix",
		},
		{
			name:     "ticket pattern BUG",
			filename: "BUG-789.md",
			want:     "fix", // aliased to fix
		},
		{
			name:     "type prefix feature",
			filename: "feature-user-auth.md",
			want:     "feature",
		},
		{
			name:     "type prefix feat",
			filename: "feat-user-auth.md",
			want:     "feature", // aliased to feature
		},
		{
			name:     "type prefix fix",
			filename: "fix-login-bug.md",
			want:     "fix",
		},
		{
			name:     "type prefix docs",
			filename: "docs-api-reference.md",
			want:     "docs",
		},
		{
			name:     "type prefix chore",
			filename: "chore-update-deps.md",
			want:     "chore",
		},
		{
			name:     "unknown prefix defaults to task",
			filename: "my-custom-task.md",
			want:     "task",
		},
		{
			name:     "simple filename defaults to task",
			filename: "task.md",
			want:     "task",
		},
		{
			name:     "with full path",
			filename: "/path/to/tasks/FEATURE-123.md",
			want:     "feature",
		},
		{
			name:     "unknown ticket prefix",
			filename: "JIRA-123.md",
			want:     "task", // JIRA is not a known type
		},
		{
			name:     "empty filename",
			filename: "",
			want:     "task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskTypeFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("TaskTypeFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestKeyFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "ticket ID",
			filename: "FEATURE-123.md",
			want:     "FEATURE-123",
		},
		{
			name:     "hyphenated name",
			filename: "fix-login-bug.md",
			want:     "fix-login-bug",
		},
		{
			name:     "simple name",
			filename: "task.md",
			want:     "task",
		},
		{
			name:     "with path",
			filename: "/path/to/ABC-1.md",
			want:     "ABC-1",
		},
		{
			name:     "no extension",
			filename: "FEATURE-123",
			want:     "FEATURE-123",
		},
		{
			name:     "multiple dots",
			filename: "feature.test.md",
			want:     "feature.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeyFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("KeyFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestKeyFromDirectory(t *testing.T) {
	tests := []struct {
		name    string
		dirPath string
		want    string
	}{
		{
			name:    "simple directory",
			dirPath: "/path/to/FEATURE-123",
			want:    "FEATURE-123",
		},
		{
			name:    "relative path",
			dirPath: "./tasks/my-feature",
			want:    "my-feature",
		},
		{
			name:    "just directory name",
			dirPath: "my-task",
			want:    "my-task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeyFromDirectory(tt.dirPath)
			if got != tt.want {
				t.Errorf("KeyFromDirectory(%q) = %q, want %q", tt.dirPath, got, tt.want)
			}
		})
	}
}

func TestParseTicketID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID string
		wantOK bool
	}{
		{
			name:   "standard ticket",
			input:  "FEATURE-123",
			wantID: "FEATURE-123",
			wantOK: true,
		},
		{
			name:   "short ticket",
			input:  "ABC-1",
			wantID: "ABC-1",
			wantOK: true,
		},
		{
			name:   "ticket with suffix",
			input:  "JIRA-456-extra-stuff",
			wantID: "JIRA-456",
			wantOK: true,
		},
		{
			name:   "lowercase not matched",
			input:  "feature-123",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "no number",
			input:  "FEATURE-abc",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "no hyphen",
			input:  "FEATURE123",
			wantID: "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := ParseTicketID(tt.input)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf("ParseTicketID(%q) = (%q, %v), want (%q, %v)",
					tt.input, gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestCleanBranchName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid branch",
			input: "feature/FEATURE-123--add-auth",
			want:  "feature/FEATURE-123--add-auth",
		},
		{
			name:  "double slashes",
			input: "feature//FEATURE-123",
			want:  "feature/FEATURE-123",
		},
		{
			name:  "trailing slash",
			input: "feature/name/",
			want:  "feature/name",
		},
		{
			name:  "trailing hyphen",
			input: "feature/name-",
			want:  "feature/name",
		},
		{
			name:  "leading slash",
			input: "/feature/name",
			want:  "feature/name",
		},
		{
			name:  "triple hyphens become double",
			input: "feature/name---test",
			want:  "feature/name--test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanBranchName(tt.input)
			if got != tt.want {
				t.Errorf("CleanBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
