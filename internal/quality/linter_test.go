package quality

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGolangCILintName(t *testing.T) {
	g := NewGolangCILint()
	if g.Name() != "golangci-lint" {
		t.Errorf("Name() = %s, want golangci-lint", g.Name())
	}
}

func TestGolangCILintWithConfig(t *testing.T) {
	g := NewGolangCILint().WithConfig("/path/to/config.yml")
	if g.configPath != "/path/to/config.yml" {
		t.Errorf("configPath = %s, want /path/to/config.yml", g.configPath)
	}
}

func TestGolangCILintNotGoProject(t *testing.T) {
	tmpDir := t.TempDir()
	// No go.mod, so should return empty report

	g := NewGolangCILint()
	ctx := context.Background()

	report, err := g.Lint(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	// Should have no issues (not a Go project)
	hasToolMissing := false
	for _, issue := range report.Issues {
		if issue.Rule == "tool-missing" {
			hasToolMissing = true

			break
		}
	}

	// Either tool is missing or no issues (not a Go project)
	if !hasToolMissing && len(report.Issues) > 0 {
		t.Errorf("Expected no issues for non-Go project, got %d", len(report.Issues))
	}
}

func TestESLintName(t *testing.T) {
	e := NewESLint()
	if e.Name() != "eslint" {
		t.Errorf("Name() = %s, want eslint", e.Name())
	}
}

func TestESLintWithConfig(t *testing.T) {
	e := NewESLint().WithConfig("/path/to/eslint.config.js")
	if e.configPath != "/path/to/eslint.config.js" {
		t.Errorf("configPath = %s, want /path/to/eslint.config.js", e.configPath)
	}
}

func TestESLintNotJSProject(t *testing.T) {
	tmpDir := t.TempDir()
	// No package.json, so should return empty report

	e := NewESLint()
	ctx := context.Background()

	report, err := e.Lint(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	// Should have no issues or tool-missing (not a JS project)
	for _, issue := range report.Issues {
		if issue.Rule != "tool-missing" {
			t.Errorf("Unexpected issue for non-JS project: %v", issue)
		}
	}
}

func TestGoVetName(t *testing.T) {
	g := NewGoVet()
	if g.Name() != "go-vet" {
		t.Errorf("Name() = %s, want go-vet", g.Name())
	}
}

func TestGoVetAvailable(t *testing.T) {
	g := NewGoVet()
	// go should typically be available in test environment
	if !g.Available() {
		t.Skip("go not available, skipping test")
	}
}

func TestGoVetNotGoProject(t *testing.T) {
	tmpDir := t.TempDir()

	g := NewGoVet()
	ctx := context.Background()

	report, err := g.Lint(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if len(report.Issues) != 0 {
		t.Errorf("Expected 0 issues for non-Go project, got %d", len(report.Issues))
	}
}

func TestRunnerMultipleLinters(t *testing.T) {
	r := NewRunner()

	if len(r.linters) < 2 {
		t.Errorf("Expected at least 2 default linters, got %d", len(r.linters))
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	reports, err := r.Run(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(reports) < 2 {
		t.Errorf("Expected at least 2 reports, got %d", len(reports))
	}
}

func TestRunnerAddLinter(t *testing.T) {
	r := NewRunner()
	initialCount := len(r.linters)

	r.AddLinter(NewGoVet())

	if len(r.linters) != initialCount+1 {
		t.Errorf("Expected %d linters after add, got %d", initialCount+1, len(r.linters))
	}
}

func TestRunnerContextCancellation(t *testing.T) {
	r := NewRunner()
	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Run(ctx, tmpDir)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected []string
	}{
		{
			name:     "go project",
			files:    []string{"go.mod"},
			expected: []string{"go"},
		},
		{
			name:     "js project",
			files:    []string{"package.json"},
			expected: []string{"javascript"},
		},
		{
			name:     "ts project",
			files:    []string{"package.json", "tsconfig.json"},
			expected: []string{"javascript", "typescript"},
		},
		{
			name:     "python project",
			files:    []string{"requirements.txt"},
			expected: []string{"python"},
		},
		{
			name:     "mixed project",
			files:    []string{"go.mod", "package.json"},
			expected: []string{"go", "javascript"},
		},
		{
			name:     "empty project",
			files:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for _, f := range tt.files {
				_ = os.WriteFile(filepath.Join(tmpDir, f), []byte(""), 0o644)
			}

			types := DetectProjectType(tmpDir)

			if len(types) != len(tt.expected) {
				t.Errorf("DetectProjectType() = %v, want %v", types, tt.expected)

				return
			}

			for i, typ := range types {
				if typ != tt.expected[i] {
					t.Errorf("DetectProjectType()[%d] = %s, want %s", i, typ, tt.expected[i])
				}
			}
		})
	}
}

func TestSeverityConstants(t *testing.T) {
	tests := []struct {
		severity Severity
		want     string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeverityInfo, "info"},
	}

	for _, tt := range tests {
		if string(tt.severity) != tt.want {
			t.Errorf("Severity %v = %s, want %s", tt.severity, string(tt.severity), tt.want)
		}
	}
}

func TestReportDuration(t *testing.T) {
	g := NewGoVet()
	tmpDir := t.TempDir()

	// Create a go.mod so it actually runs
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := g.Lint(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if report.Duration == 0 {
		t.Error("Duration should be non-zero")
	}
}

func TestGolangCILintParseOutput(t *testing.T) {
	g := NewGolangCILint()
	baseDir := "/project"

	output := []byte(`{
		"Issues": [
			{
				"FromLinter": "govet",
				"Text": "some issue",
				"Pos": {"Filename": "/project/main.go", "Line": 10, "Column": 5},
				"Severity": "error"
			},
			{
				"FromLinter": "gofmt",
				"Text": "formatting issue",
				"Pos": {"Filename": "/project/pkg/util.go", "Line": 20, "Column": 1},
				"Severity": "warning"
			},
			{
				"FromLinter": "staticcheck",
				"Text": "info issue",
				"Pos": {"Filename": "/project/cmd/main.go", "Line": 5, "Column": 2},
				"Severity": "info"
			}
		]
	}`)

	issues := g.parseOutput(output, baseDir)
	if len(issues) != 3 {
		t.Fatalf("parseOutput() = %d issues, want 3", len(issues))
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("issues[0].Severity = %q, want error", issues[0].Severity)
	}
	if issues[0].Rule != "govet" {
		t.Errorf("issues[0].Rule = %q, want govet", issues[0].Rule)
	}
	if issues[0].Line != 10 {
		t.Errorf("issues[0].Line = %d, want 10", issues[0].Line)
	}
	if issues[1].Severity != SeverityWarning {
		t.Errorf("issues[1].Severity = %q, want warning", issues[1].Severity)
	}
	if issues[2].Severity != SeverityInfo {
		t.Errorf("issues[2].Severity = %q, want info", issues[2].Severity)
	}
}

func TestGolangCILintParseOutput_InvalidJSON(t *testing.T) {
	g := NewGolangCILint()
	issues := g.parseOutput([]byte("not json"), "/base")
	if issues != nil {
		t.Errorf("parseOutput() invalid JSON = %v, want nil", issues)
	}
}

func TestESLintParseOutput(t *testing.T) {
	e := NewESLint()
	baseDir := "/project"

	output := []byte(`[
		{
			"filePath": "/project/src/app.ts",
			"messages": [
				{"ruleId": "no-unused-vars", "severity": 1, "message": "unused var", "line": 5, "column": 3},
				{"ruleId": "no-console", "severity": 2, "message": "no console", "line": 10, "column": 1}
			]
		}
	]`)

	issues := e.parseOutput(output, baseDir)
	if len(issues) != 2 {
		t.Fatalf("parseOutput() = %d issues, want 2", len(issues))
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("issues[0].Severity = %q, want warning (severity=1)", issues[0].Severity)
	}
	if issues[1].Severity != SeverityError {
		t.Errorf("issues[1].Severity = %q, want error (severity=2)", issues[1].Severity)
	}
	if issues[0].Rule != "no-unused-vars" {
		t.Errorf("issues[0].Rule = %q, want no-unused-vars", issues[0].Rule)
	}
}

func TestESLintParseOutput_InvalidJSON(t *testing.T) {
	e := NewESLint()
	issues := e.parseOutput([]byte("not json"), "/base")
	if issues != nil {
		t.Errorf("parseOutput() invalid JSON = %v, want nil", issues)
	}
}

func TestGoVetParseOutput(t *testing.T) {
	g := NewGoVet()
	baseDir := "/project"

	output := "/project/main.go:15:3: some vet issue\n/project/pkg/util.go:20:1: another issue\n"
	issues := g.parseOutput(output, baseDir)
	if len(issues) != 2 {
		t.Fatalf("parseOutput() = %d issues, want 2", len(issues))
	}
	if issues[0].Line != 15 {
		t.Errorf("issues[0].Line = %d, want 15", issues[0].Line)
	}
	if issues[0].Column != 3 {
		t.Errorf("issues[0].Column = %d, want 3", issues[0].Column)
	}
	if issues[0].Message != "some vet issue" {
		t.Errorf("issues[0].Message = %q, want 'some vet issue'", issues[0].Message)
	}
	if issues[0].Rule != "vet" {
		t.Errorf("issues[0].Rule = %q, want vet", issues[0].Rule)
	}
}

func TestGoVetParseOutput_SkipsShortLines(t *testing.T) {
	g := NewGoVet()
	// Lines without enough ":" parts should be skipped
	output := "some line without colons\nanother short line\n"
	issues := g.parseOutput(output, "/base")
	if len(issues) != 0 {
		t.Errorf("parseOutput() = %d issues for short lines, want 0", len(issues))
	}
}

func TestDetectProjectType_Pyproject(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(""), 0o644)

	types := DetectProjectType(tmpDir)
	if len(types) != 1 || types[0] != "python" {
		t.Errorf("DetectProjectType() = %v, want [python]", types)
	}
}

func TestGolangCILintLint_NotGoProject(t *testing.T) {
	g := NewGolangCILint()
	ctx := context.Background()
	// Directory without go.mod should return empty issues (not a Go project)
	tmpDir := t.TempDir()

	report, err := g.Lint(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report == nil {
		t.Fatal("Lint() returned nil report")
	}
}

func TestESLintLint_NotJSProject(t *testing.T) {
	e := NewESLint()
	ctx := context.Background()
	// Directory without package.json should return empty issues
	tmpDir := t.TempDir()

	report, err := e.Lint(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report == nil {
		t.Fatal("Lint() returned nil report")
	}
}
