package github

import (
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/storage"
)

func TestIsLikelyFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"go file", "main.go", true},
		{"python file", "script.py", true},
		{"javascript file", "app.js", true},
		{"typescript file", "component.ts", true},
		{"tsx file", "Component.tsx", true},
		{"jsx file", "Component.jsx", true},
		{"java file", "Main.java", true},
		{"kotlin file", "Main.kt", true},
		{"rust file", "lib.rs", true},
		{"ruby file", "app.rb", true},
		{"php file", "index.php", true},
		{"c file", "main.c", true},
		{"cpp file", "main.cpp", true},
		{"header file", "header.h", true},
		{"hpp file", "header.hpp", true},
		{"csharp file", "Program.cs", true},
		{"markdown file", "README.md", true},
		{"yaml file", "config.yaml", true},
		{"yml file", "config.yml", true},
		{"json file", "package.json", true},
		{"toml file", "Cargo.toml", true},
		{"xml file", "pom.xml", true},
		{"html file", "index.html", true},
		{"css file", "styles.css", true},
		{"sql file", "schema.sql", true},
		{"shell file", "script.sh", true},
		{"bash file", "script.bash", true},
		{"zsh file", "script.zsh", true},
		{"powershell file", "script.ps1", true},
		{"batch file", "script.bat", true},
		{"path with directory", "internal/pkg/file.go", true},
		{"uppercase extension", "FILE.GO", true},
		{"mixed case", "File.Py", true},
		{"no extension", "Makefile", false},
		{"unknown extension", "file.xyz", false},
		{"empty string", "", false},
		{"just dot", ".", false},
		{"hidden file no ext", ".gitignore", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("isLikelyFilePath(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPlannedFiles(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "backtick file references",
			content:  "We need to modify `main.go` and `config.yaml`",
			expected: []string{"main.go", "config.yaml"},
		},
		{
			name:     "create/modify patterns",
			content:  "create `handler.go` and modify `server.go`",
			expected: []string{"handler.go", "server.go"},
		},
		{
			name:     "list items",
			content:  "Files:\n- `api.go`\n- `types.go`",
			expected: []string{"api.go", "types.go"},
		},
		{
			name:     "path with directory",
			content:  "Update `internal/pkg/handler.go`",
			expected: []string{"internal/pkg/handler.go"},
		},
		{
			name:     "no duplicates",
			content:  "Modify `main.go` and update `main.go` again",
			expected: []string{"main.go"},
		},
		{
			name:     "empty content",
			content:  "",
			expected: nil,
		},
		{
			name:     "no files",
			content:  "This is just text without any file references",
			expected: nil,
		},
		{
			name:     "mixed valid and invalid",
			content:  "Edit `main.go` and `unknown.xyz`",
			expected: []string{"main.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPlannedFiles(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("extractPlannedFiles() returned %d files, want %d", len(result), len(tt.expected))
				t.Errorf("got: %v, want: %v", result, tt.expected)
				return
			}
			for i, f := range result {
				if f != tt.expected[i] {
					t.Errorf("extractPlannedFiles()[%d] = %q, want %q", i, f, tt.expected[i])
				}
			}
		})
	}
}

func TestExtractApproachSummary(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "approach heading",
			content:  "## Approach\nThis is the approach summary.\n\n## Next Section",
			expected: "This is the approach summary.",
		},
		{
			name:     "strategy heading",
			content:  "## Strategy\nUse a modular design.\n\n## Implementation",
			expected: "Use a modular design.",
		},
		{
			name:     "implementation approach heading",
			content:  "## Implementation Approach\nBuild incrementally.\n\n## Details",
			expected: "Build incrementally.",
		},
		{
			name:     "solution heading",
			content:  "## Solution\nRefactor the existing code.\n\n## Testing",
			expected: "Refactor the existing code.",
		},
		{
			name:     "case insensitive",
			content:  "## APPROACH\nUppercase heading.\n\n## Next",
			expected: "Uppercase heading.",
		},
		{
			name:     "no approach section",
			content:  "## Overview\nSome content.\n\n## Details",
			expected: "",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractApproachSummary(tt.content)
			if result != tt.expected {
				t.Errorf("extractApproachSummary() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseDiffStat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single file",
			input:    " main.go | 10 +++++++---",
			expected: "main.go | 10 +++++++---\n",
		},
		{
			name:     "multiple files",
			input:    " main.go | 10 +++++++---\n config.yaml | 5 +++++",
			expected: "main.go | 10 +++++++---\nconfig.yaml | 5 +++++\n",
		},
		{
			name:     "with summary line",
			input:    " main.go | 10 +++++++---\n 2 files changed, 15 insertions(+), 3 deletions(-)",
			expected: "main.go | 10 +++++++---\n2 files changed, 15 insertions(+), 3 deletions(-)\n",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \n   \n   ",
			expected: "",
		},
		{
			name:     "with leading/trailing whitespace",
			input:    "\n  main.go | 5 +++++  \n\n",
			expected: "main.go | 5 +++++\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDiffStat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseDiffStat() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateChangeSummary(t *testing.T) {
	tests := []struct {
		name      string
		exchanges []storage.Exchange
		expected  string
	}{
		{
			name: "single file change",
			exchanges: []storage.Exchange{
				{
					FilesChanged: []storage.FileChange{
						{Path: "main.go", Operation: "modified"},
					},
				},
			},
			expected: "- `main.go` (modified)\n",
		},
		{
			name: "multiple file changes",
			exchanges: []storage.Exchange{
				{
					FilesChanged: []storage.FileChange{
						{Path: "main.go", Operation: "modified"},
						{Path: "config.yaml", Operation: "created"},
					},
				},
			},
			expected: "- `main.go` (modified)\n- `config.yaml` (created)\n",
		},
		{
			name: "multiple exchanges with duplicates",
			exchanges: []storage.Exchange{
				{
					FilesChanged: []storage.FileChange{
						{Path: "main.go", Operation: "modified"},
					},
				},
				{
					FilesChanged: []storage.FileChange{
						{Path: "main.go", Operation: "modified"},
						{Path: "test.go", Operation: "created"},
					},
				},
			},
			expected: "- `main.go` (modified)\n- `test.go` (created)\n",
		},
		{
			name:      "empty exchanges",
			exchanges: []storage.Exchange{},
			expected:  "",
		},
		{
			name: "exchanges with no file changes",
			exchanges: []storage.Exchange{
				{FilesChanged: nil},
				{FilesChanged: []storage.FileChange{}},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateChangeSummary(tt.exchanges)
			if result != tt.expected {
				t.Errorf("GenerateChangeSummary() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCommentTimestamp(t *testing.T) {
	result := CommentTimestamp()

	// Check format: should be "YYYY-MM-DD HH:MM:SS UTC"
	if len(result) != 23 {
		t.Errorf("CommentTimestamp() length = %d, want 23", len(result))
	}

	if !strings.HasSuffix(result, "UTC") {
		t.Errorf("CommentTimestamp() should end with 'UTC', got %q", result)
	}

	// Check it contains expected separators
	if !strings.Contains(result, "-") || !strings.Contains(result, ":") {
		t.Errorf("CommentTimestamp() should contain date/time separators, got %q", result)
	}
}

func TestCommentGenerator_GenerateBranchCreatedComment(t *testing.T) {
	gen := &CommentGenerator{}

	tests := []struct {
		name       string
		branchName string
		expected   string
	}{
		{
			name:       "simple branch",
			branchName: "feature/add-login",
			expected:   "Started working on this issue.\nBranch: `feature/add-login`",
		},
		{
			name:       "task branch",
			branchName: "task/issue-123",
			expected:   "Started working on this issue.\nBranch: `task/issue-123`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.GenerateBranchCreatedComment(tt.branchName)
			if result != tt.expected {
				t.Errorf("GenerateBranchCreatedComment() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCommentGenerator_GeneratePlanComment(t *testing.T) {
	gen := &CommentGenerator{}

	tests := []struct {
		name     string
		specs    []*storage.Specification
		contains []string
	}{
		{
			name:     "empty specs",
			specs:    []*storage.Specification{},
			contains: []string{"Planning complete."},
		},
		{
			name:     "nil specs",
			specs:    nil,
			contains: []string{"Planning complete."},
		},
		{
			name: "spec with files",
			specs: []*storage.Specification{
				{Content: "Create `main.go` and `config.yaml`"},
			},
			contains: []string{"Implementation Plan", "main.go", "config.yaml"},
		},
		{
			name: "spec with approach",
			specs: []*storage.Specification{
				{Content: "## Approach\nUse modular design.\n\n## Files"},
			},
			contains: []string{"Implementation Plan", "Approach", "Use modular design"},
		},
		{
			name: "multiple specs uses latest",
			specs: []*storage.Specification{
				{Content: "Old spec with `old.go`"},
				{Content: "New spec with `new.go`"},
			},
			contains: []string{"new.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.GeneratePlanComment(tt.specs)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("GeneratePlanComment() should contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestCommentGenerator_GenerateImplementComment(t *testing.T) {
	gen := &CommentGenerator{}

	tests := []struct {
		name     string
		diffStat string
		summary  string
		contains []string
	}{
		{
			name:     "with diff and summary",
			diffStat: "main.go | 10 ++++",
			summary:  "Added new feature",
			contains: []string{"Implementation Complete", "Added new feature", "main.go | 10 ++++", "Ready for review"},
		},
		{
			name:     "diff only",
			diffStat: "main.go | 5 ++",
			summary:  "",
			contains: []string{"Implementation Complete", "main.go | 5 ++", "Ready for review"},
		},
		{
			name:     "summary only",
			diffStat: "",
			summary:  "Fixed bug",
			contains: []string{"Implementation Complete", "Fixed bug", "Ready for review"},
		},
		{
			name:     "empty both",
			diffStat: "",
			summary:  "",
			contains: []string{"Implementation Complete", "Ready for review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.GenerateImplementComment(tt.diffStat, tt.summary)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("GenerateImplementComment() should contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestCommentGenerator_GeneratePRCreatedComment(t *testing.T) {
	gen := &CommentGenerator{}

	result := gen.GeneratePRCreatedComment(42, "https://github.com/owner/repo/pull/42")
	expected := "Pull request created: #42\nhttps://github.com/owner/repo/pull/42"

	if result != expected {
		t.Errorf("GeneratePRCreatedComment() = %q, want %q", result, expected)
	}
}

func TestNewCommentGenerator(t *testing.T) {
	p := &Provider{}
	gen := NewCommentGenerator(p)

	if gen == nil {
		t.Fatal("NewCommentGenerator() returned nil")
	}
	if gen.provider != p {
		t.Error("NewCommentGenerator() did not set provider correctly")
	}
}
