package quality

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
)

func TestSlopCheckerName(t *testing.T) {
	c := NewSlopChecker()
	if c.Name() != "slop-detector" {
		t.Errorf("Name() = %q, want %q", c.Name(), "slop-detector")
	}
}

func TestSlopScanFileAIBoilerplate(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		rule    string
	}{
		{
			name:    "Here's phrase",
			content: "// Here's the implementation\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "Let me phrase",
			content: "// Let me explain this function\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "I'll phrase",
			content: "// I'll add error handling here\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "As an AI phrase",
			content: "// As an AI, I suggest using this pattern\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "Sure comma phrase",
			content: "// Sure, this works well\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "Certainly phrase",
			content: "// Certainly this is the right approach\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "I've implemented phrase",
			content: "// I've implemented the feature\nfunc main() {}\n",
			want:    1,
			rule:    "slop-ai-boilerplate",
		},
		{
			name:    "no boilerplate",
			content: "// handleRequest processes incoming HTTP requests.\nfunc handleRequest() {}\n",
			want:    0,
			rule:    "",
		},
		{
			name:    "not a comment",
			content: "fmt.Println(\"Here's a string\")\n",
			want:    0,
			rule:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "test.go")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			checker := NewSlopChecker()
			ff, err := checker.scanFile("test.go", file)
			if err != nil {
				t.Fatal(err)
			}

			matched := filterByRule(ff, tt.rule)
			if len(matched) != tt.want {
				t.Errorf("got %d findings for rule %q, want %d; findings: %+v", len(matched), tt.rule, tt.want, ff)
			}

			for _, f := range matched {
				if f.Category != findings.CategoryQuality {
					t.Errorf("category = %v, want quality", f.Category)
				}
				if f.Source != "slop-detector" {
					t.Errorf("source = %q, want %q", f.Source, "slop-detector")
				}
				if f.Severity != findings.SeverityMedium {
					t.Errorf("severity = %v, want medium", f.Severity)
				}
			}
		})
	}
}

func TestSlopScanFileExcessiveComments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name: "over 40 percent comments",
			// 6 comment lines, 5 code lines = 11 total, ~55% comments
			content: "// comment 1\n// comment 2\n// comment 3\n// comment 4\n// comment 5\n// comment 6\n" +
				"func a() {}\nfunc b() {}\nfunc c() {}\nfunc d() {}\nfunc e() {}\n",
			want: 1,
		},
		{
			name: "under 40 percent comments",
			// 2 comment lines, 10 code lines = 12 total, ~17% comments
			content: "// comment 1\n// comment 2\n" +
				"func a() {}\nfunc b() {}\nfunc c() {}\nfunc d() {}\nfunc e() {}\n" +
				"func f() {}\nfunc g() {}\nfunc h() {}\nfunc i() {}\nfunc j() {}\n",
			want: 0,
		},
		{
			name:    "too few lines to flag",
			content: "// comment\n// comment\nfunc a() {}\n",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "test.go")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			checker := NewSlopChecker()
			ff, err := checker.scanFile("test.go", file)
			if err != nil {
				t.Fatal(err)
			}

			matched := filterByRule(ff, "slop-excessive-comments")
			if len(matched) != tt.want {
				t.Errorf("got %d findings, want %d; findings: %+v", len(matched), tt.want, ff)
			}

			for _, f := range matched {
				if f.Severity != findings.SeverityLow {
					t.Errorf("severity = %v, want low", f.Severity)
				}
			}
		})
	}
}

func TestSlopScanFileRedundantErrorWrap(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "error prefix",
			content: "package main\n\nfunc f() error {\n\treturn fmt.Errorf(\"error: %w\", err)\n}\n",
			want:    1,
		},
		{
			name:    "err prefix",
			content: "package main\n\nfunc f() error {\n\treturn fmt.Errorf(\"err: %w\", err)\n}\n",
			want:    1,
		},
		{
			name:    "failed to fail",
			content: "package main\n\nfunc f() error {\n\treturn fmt.Errorf(\"failed to fail: %w\", err)\n}\n",
			want:    1,
		},
		{
			name:    "good error wrap",
			content: "package main\n\nfunc f() error {\n\treturn fmt.Errorf(\"opening config: %w\", err)\n}\n",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "test.go")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			checker := NewSlopChecker()
			ff, err := checker.scanFile("test.go", file)
			if err != nil {
				t.Fatal(err)
			}

			matched := filterByRule(ff, "slop-redundant-error-wrap")
			if len(matched) != tt.want {
				t.Errorf("got %d findings, want %d; findings: %+v", len(matched), tt.want, ff)
			}

			for _, f := range matched {
				if f.Severity != findings.SeverityLow {
					t.Errorf("severity = %v, want low", f.Severity)
				}
			}
		})
	}
}

func TestSlopScanFileCommentedOutCode(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "commented fmt.Println",
			content: "package main\n\n// fmt.Println(\"debug\")\nfunc main() {}\n",
			want:    1,
		},
		{
			name:    "commented log.Printf",
			content: "package main\n\n// log.Printf(\"debug %v\", x)\nfunc main() {}\n",
			want:    1,
		},
		{
			name:    "normal comment",
			content: "package main\n\n// This function handles requests.\nfunc main() {}\n",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "test.go")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			checker := NewSlopChecker()
			ff, err := checker.scanFile("test.go", file)
			if err != nil {
				t.Fatal(err)
			}

			matched := filterByRule(ff, "slop-commented-out-code")
			if len(matched) != tt.want {
				t.Errorf("got %d findings, want %d; findings: %+v", len(matched), tt.want, ff)
			}
		})
	}
}

func TestSlopScanFileIgnoredError(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "ignored error no justification",
			content: "package main\n\nfunc f() {\n\t_ = err\n}\n",
			want:    1,
		},
		{
			name:    "ignored error with justification",
			content: "package main\n\nfunc f() {\n\t_ = err // safe to ignore: Close on read-only file\n}\n",
			want:    0,
		},
		{
			name:    "not an error ignore",
			content: "package main\n\nfunc f() {\n\tresult := doWork()\n}\n",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "test.go")
			if err := os.WriteFile(file, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			checker := NewSlopChecker()
			ff, err := checker.scanFile("test.go", file)
			if err != nil {
				t.Fatal(err)
			}

			matched := filterByRule(ff, "slop-ignored-error")
			if len(matched) != tt.want {
				t.Errorf("got %d findings, want %d; findings: %+v", len(matched), tt.want, ff)
			}

			for _, f := range matched {
				if f.Severity != findings.SeverityMedium {
					t.Errorf("severity = %v, want medium", f.Severity)
				}
			}
		})
	}
}

func TestSlopScanFileNonexistent(t *testing.T) {
	checker := NewSlopChecker()
	_, err := checker.scanFile("missing.go", "/nonexistent/missing.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSlopCheckerImplementsChecker(t *testing.T) {
	// Compile-time check that SlopChecker implements Checker.
	var _ Checker = (*SlopChecker)(nil)
}

// filterByRule returns findings matching the given rule. If rule is empty,
// returns all findings.
func filterByRule(ff []findings.Finding, rule string) []findings.Finding {
	if rule == "" {
		return ff
	}

	var matched []findings.Finding
	for _, f := range ff {
		if f.Rule == rule {
			matched = append(matched, f)
		}
	}

	return matched
}
