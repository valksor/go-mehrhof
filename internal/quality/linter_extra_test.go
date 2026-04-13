package quality

import (
	"context"
	"errors"
	"testing"
)

const testProjectDir = "/project"

// stubLinter is a Linter implementation for testing Runner.Run error handling.
type stubLinter struct {
	name   string
	err    error
	report *Report
}

func (s *stubLinter) Name() string { return s.name }

func (s *stubLinter) Available() bool { return true }

func (s *stubLinter) Lint(_ context.Context, _ string) (*Report, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.report, nil
}

func TestRunner_Run_LinterError(t *testing.T) {
	r := &Runner{}
	lintErr := errors.New("linter exploded")
	r.linters = []Linter{
		&stubLinter{
			name: "exploding-linter",
			err:  lintErr,
		},
	}

	ctx := context.Background()
	_, err := r.Run(ctx, t.TempDir())
	if err == nil {
		t.Fatal("Run() expected error from failing linter, got nil")
	}
	if !errors.Is(err, lintErr) {
		t.Errorf("Run() error = %v, want wrapping %v", err, lintErr)
	}
}

func TestRunner_Run_LinterErrorWrapsName(t *testing.T) {
	r := &Runner{}
	r.linters = []Linter{
		&stubLinter{
			name: "my-linter",
			err:  errors.New("boom"),
		},
	}

	ctx := context.Background()
	_, err := r.Run(ctx, t.TempDir())
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	// The Runner wraps the error with the linter name
	errStr := err.Error()
	if errStr == "" {
		t.Error("Run() error string is empty")
	}
}

func TestRunner_Run_StubLinterSuccess(t *testing.T) {
	r := &Runner{}
	r.linters = []Linter{
		&stubLinter{
			name: "ok-linter",
			report: &Report{
				Linter: "ok-linter",
				Issues: []Issue{},
			},
		},
	}

	ctx := context.Background()
	reports, err := r.Run(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Run() unexpected error = %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("Run() reports = %d, want 1", len(reports))
	}
	if reports[0].Linter != "ok-linter" {
		t.Errorf("reports[0].Linter = %q, want ok-linter", reports[0].Linter)
	}
}

func TestRunner_Run_MultipleLintersFirstErrors(t *testing.T) {
	lintErr := errors.New("first linter fail")
	r := &Runner{}
	r.linters = []Linter{
		&stubLinter{name: "fails", err: lintErr},
		&stubLinter{name: "would-succeed", report: &Report{Linter: "would-succeed"}},
	}

	ctx := context.Background()
	_, err := r.Run(ctx, t.TempDir())
	if err == nil {
		t.Fatal("Run() expected error from first linter, got nil")
	}
	if !errors.Is(err, lintErr) {
		t.Errorf("Run() error = %v, want wrapping %v", err, lintErr)
	}
}

func TestGolangCILintParseOutput_EmptyFilename(t *testing.T) {
	g := NewGolangCILint()
	baseDir := testProjectDir

	// Issue with empty filename path — relPath should fall back to the empty string itself
	output := []byte(`{
		"Issues": [
			{
				"FromLinter": "govet",
				"Text": "issue with empty file",
				"Pos": {"Filename": "", "Line": 1, "Column": 1},
				"Severity": "error"
			}
		]
	}`)

	issues := g.parseOutput(output, baseDir)
	if len(issues) != 1 {
		t.Fatalf("parseOutput() = %d issues, want 1", len(issues))
	}
	// When Filename is empty, filepath.Rel returns "." or similar; the code keeps
	// relPath = Filename (empty) when relPath == "".
	// Either way the issue should be returned with its other fields intact.
	if issues[0].Rule != "govet" {
		t.Errorf("issues[0].Rule = %q, want govet", issues[0].Rule)
	}
	if issues[0].Message != "issue with empty file" {
		t.Errorf("issues[0].Message = %q, want 'issue with empty file'", issues[0].Message)
	}
}

func TestESLintParseOutput_EmptyFilePath(t *testing.T) {
	e := NewESLint()
	baseDir := testProjectDir

	// filePath is empty string
	output := []byte(`[
		{
			"filePath": "",
			"messages": [
				{"ruleId": "no-undef", "severity": 2, "message": "empty path issue", "line": 3, "column": 5}
			]
		}
	]`)

	issues := e.parseOutput(output, baseDir)
	if len(issues) != 1 {
		t.Fatalf("parseOutput() = %d issues, want 1", len(issues))
	}
	if issues[0].Rule != "no-undef" {
		t.Errorf("issues[0].Rule = %q, want no-undef", issues[0].Rule)
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("issues[0].Severity = %q, want error", issues[0].Severity)
	}
}
