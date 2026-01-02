package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GolangCI implements the Linter interface for golangci-lint.
// golangci-lint is a fast Go linters aggregator.
type GolangCI struct{}

// NewGolangCI creates a new golangci-lint linter.
func NewGolangCI() *GolangCI {
	return &GolangCI{}
}

// Name returns the linter identifier.
func (g *GolangCI) Name() string {
	return "golangci-lint"
}

// Available checks if golangci-lint is installed.
func (g *GolangCI) Available() bool {
	_, err := exec.LookPath("golangci-lint")
	return err == nil
}

// Run executes golangci-lint with JSON output for parsing.
func (g *GolangCI) Run(ctx context.Context, workDir string, files []string) (*Result, error) {
	// Build command arguments
	args := []string{"run", "--out-format=json"}

	// If specific files are provided, lint only those
	if len(files) > 0 {
		// Filter to only Go files
		var goFiles []string
		for _, f := range files {
			if strings.HasSuffix(f, ".go") {
				goFiles = append(goFiles, f)
			}
		}
		if len(goFiles) == 0 {
			// No Go files to lint
			return &Result{
				Linter:  g.Name(),
				Passed:  true,
				Summary: "No Go files to lint",
			}, nil
		}
		args = append(args, goFiles...)
	} else {
		// Lint entire project
		args = append(args, "./...")
	}

	cmd := exec.CommandContext(ctx, "golangci-lint", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()

	// Parse JSON output
	result, parseErr := g.parseOutput(output)
	if parseErr != nil {
		// If parsing fails but we got an error, include the raw output
		if err != nil {
			//nolint:nilerr // Error embedded in Result struct
			return &Result{
				Linter:  g.Name(),
				Passed:  false,
				Summary: fmt.Sprintf("Linter failed: %s", string(output)),
				Error:   err,
			}, nil
		}
		return nil, fmt.Errorf("parse golangci-lint output: %w", parseErr)
	}

	// golangci-lint returns exit code 1 when issues are found, but that's not an error
	return result, nil
}

// golangciOutput represents the JSON output from golangci-lint.
type golangciOutput struct {
	Issues []golangciIssue `json:"Issues"`
}

type golangciIssue struct {
	FromLinter string        `json:"FromLinter"`
	Text       string        `json:"Text"`
	Pos        golangciPos   `json:"Pos"`
	SourceLine []interface{} `json:"SourceLines,omitempty"`
	Severity   string        `json:"Severity,omitempty"`
}

type golangciPos struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

// parseOutput parses golangci-lint JSON output into a Result.
func (g *GolangCI) parseOutput(output []byte) (*Result, error) {
	// golangci-lint may output nothing if no issues found
	if len(output) == 0 {
		return &Result{
			Linter:  g.Name(),
			Passed:  true,
			Summary: "No issues found",
		}, nil
	}

	var parsed golangciOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		// Try to handle non-JSON output (e.g., error messages)
		outputStr := string(output)
		if strings.Contains(outputStr, "no go files") || strings.Contains(outputStr, "no Go files") {
			return &Result{
				Linter:  g.Name(),
				Passed:  true,
				Summary: "No Go files to analyze",
			}, nil
		}
		return nil, err
	}

	issues := make([]Issue, 0, len(parsed.Issues))
	for _, i := range parsed.Issues {
		severity := SeverityWarning
		if i.Severity == "error" {
			severity = SeverityError
		}

		issues = append(issues, Issue{
			Path:     i.Pos.Filename,
			Line:     i.Pos.Line,
			Column:   i.Pos.Column,
			Message:  i.Text,
			Severity: severity,
			Rule:     i.FromLinter,
		})
	}

	// Passed if no errors (warnings are acceptable)
	passed := true
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			passed = false
			break
		}
	}

	summary := "No issues found"
	if len(issues) > 0 {
		summary = fmt.Sprintf("%d issues found", len(issues))
	}

	return &Result{
		Linter:  g.Name(),
		Issues:  issues,
		Passed:  passed,
		Summary: summary,
	}, nil
}
