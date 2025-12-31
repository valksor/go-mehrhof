package quality

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}

	// Should have 3 standard linters registered
	if len(r.linters) != 3 {
		t.Errorf("expected 3 linters, got %d", len(r.linters))
	}
}

func TestRegistryRegister(t *testing.T) {
	r := &Registry{}
	r.Register(NewGolangCI())
	r.Register(NewESLint())

	if len(r.linters) != 2 {
		t.Errorf("expected 2 linters, got %d", len(r.linters))
	}
}

func TestGolangCIName(t *testing.T) {
	g := NewGolangCI()
	if g.Name() != "golangci-lint" {
		t.Errorf("expected golangci-lint, got %s", g.Name())
	}
}

func TestESLintName(t *testing.T) {
	e := NewESLint()
	if e.Name() != "eslint" {
		t.Errorf("expected eslint, got %s", e.Name())
	}
}

func TestRuffName(t *testing.T) {
	r := NewRuff()
	if r.Name() != "ruff" {
		t.Errorf("expected ruff, got %s", r.Name())
	}
}

func TestDetectForProject(t *testing.T) {
	// Create temp directory with go.mod
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	detected := r.DetectForProject(tmpDir)

	// Should detect golangci-lint if available
	var hasGolangCI bool
	for _, l := range detected {
		if l.Name() == "golangci-lint" {
			hasGolangCI = true
		}
	}

	// Only check if golangci-lint is available on system
	g := NewGolangCI()
	if g.Available() && !hasGolangCI {
		t.Error("expected golangci-lint to be detected for Go project")
	}
}

func TestDetectForProjectJS(t *testing.T) {
	// Create temp directory with package.json
	tmpDir := t.TempDir()
	pkgJSON := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgJSON, []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	detected := r.DetectForProject(tmpDir)

	// Should detect eslint if available
	var hasESLint bool
	for _, l := range detected {
		if l.Name() == "eslint" {
			hasESLint = true
		}
	}

	// Only check if eslint is available on system
	e := NewESLint()
	if e.Available() && !hasESLint {
		t.Error("expected eslint to be detected for JS project")
	}
}

func TestDetectForProjectPython(t *testing.T) {
	// Create temp directory with pyproject.toml
	tmpDir := t.TempDir()
	pyproject := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyproject, []byte("[project]\nname = \"test\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	detected := r.DetectForProject(tmpDir)

	// Should detect ruff if available
	var hasRuff bool
	for _, l := range detected {
		if l.Name() == "ruff" {
			hasRuff = true
		}
	}

	// Only check if ruff is available on system
	ru := NewRuff()
	if ru.Available() && !hasRuff {
		t.Error("expected ruff to be detected for Python project")
	}
}

func TestGolangCIParseOutput(t *testing.T) {
	g := NewGolangCI()

	tests := []struct {
		name       string
		output     string
		wantIssues int
		wantPassed bool
	}{
		{
			name:       "empty",
			output:     "",
			wantIssues: 0,
			wantPassed: true,
		},
		{
			name: "with issues",
			output: `{
				"Issues": [
					{
						"FromLinter": "errcheck",
						"Text": "Error return value not checked",
						"Pos": {"Filename": "main.go", "Line": 10, "Column": 5}
					}
				]
			}`,
			wantIssues: 1,
			wantPassed: true, // warnings only
		},
		{
			name: "with error severity",
			output: `{
				"Issues": [
					{
						"FromLinter": "typecheck",
						"Text": "undeclared name: foo",
						"Severity": "error",
						"Pos": {"Filename": "main.go", "Line": 5, "Column": 10}
					}
				]
			}`,
			wantIssues: 1,
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.parseOutput([]byte(tt.output))
			if err != nil {
				t.Fatalf("parseOutput error: %v", err)
			}
			if len(result.Issues) != tt.wantIssues {
				t.Errorf("got %d issues, want %d", len(result.Issues), tt.wantIssues)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("got passed=%v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestESLintParseOutput(t *testing.T) {
	e := NewESLint()

	tests := []struct {
		name       string
		output     string
		wantIssues int
		wantPassed bool
	}{
		{
			name:       "empty array",
			output:     "[]",
			wantIssues: 0,
			wantPassed: true,
		},
		{
			name: "with warning",
			output: `[{
				"filePath": "/app/src/index.js",
				"messages": [{"ruleId": "no-unused-vars", "severity": 1, "message": "unused var", "line": 5, "column": 10}],
				"errorCount": 0,
				"warningCount": 1
			}]`,
			wantIssues: 1,
			wantPassed: true,
		},
		{
			name: "with error",
			output: `[{
				"filePath": "/app/src/index.js",
				"messages": [{"ruleId": "no-undef", "severity": 2, "message": "undefined variable", "line": 10, "column": 5}],
				"errorCount": 1,
				"warningCount": 0
			}]`,
			wantIssues: 1,
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.parseOutput([]byte(tt.output))
			if err != nil {
				t.Fatalf("parseOutput error: %v", err)
			}
			if len(result.Issues) != tt.wantIssues {
				t.Errorf("got %d issues, want %d", len(result.Issues), tt.wantIssues)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("got passed=%v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestRuffParseOutput(t *testing.T) {
	r := NewRuff()

	tests := []struct {
		name       string
		output     string
		wantIssues int
		wantPassed bool
	}{
		{
			name:       "empty array",
			output:     "[]",
			wantIssues: 0,
			wantPassed: true,
		},
		{
			name: "with warning",
			output: `[{
				"code": "W503",
				"message": "line break before binary operator",
				"location": {"file": "main.py", "row": 10, "column": 5}
			}]`,
			wantIssues: 1,
			wantPassed: true,
		},
		{
			name: "with error",
			output: `[{
				"code": "E501",
				"message": "line too long",
				"location": {"file": "main.py", "row": 100, "column": 80}
			}]`,
			wantIssues: 1,
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.parseOutput([]byte(tt.output))
			if err != nil {
				t.Fatalf("parseOutput error: %v", err)
			}
			if len(result.Issues) != tt.wantIssues {
				t.Errorf("got %d issues, want %d", len(result.Issues), tt.wantIssues)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("got passed=%v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestFormatResults(t *testing.T) {
	results := []*Result{
		{
			Linter: "golangci-lint",
			Issues: []Issue{
				{Path: "main.go", Line: 10, Message: "unused variable", Severity: SeverityWarning, Rule: "unused"},
			},
			Passed:  true,
			Summary: "1 issue found",
		},
	}

	output := FormatResults(results)

	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "golangci-lint") {
		t.Error("expected linter name in output")
	}
	if !contains(output, "main.go:10") {
		t.Error("expected file location in output")
	}
	if !contains(output, "unused variable") {
		t.Error("expected issue message in output")
	}
}

func TestFormatResultsEmpty(t *testing.T) {
	output := FormatResults(nil)
	if output != "" {
		t.Error("expected empty output for nil results")
	}

	output = FormatResults([]*Result{})
	if output != "" {
		t.Error("expected empty output for empty results")
	}
}

func TestFormatResultsWithError(t *testing.T) {
	results := []*Result{
		{
			Linter: "eslint",
			Error:  context.DeadlineExceeded,
		},
	}

	output := FormatResults(results)
	if !contains(output, "error") {
		t.Error("expected error indication in output")
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !fileExists(existingFile) {
		t.Error("expected fileExists to return true for existing file")
	}

	if fileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("expected fileExists to return false for non-existing file")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
