package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Ruff implements the Linter interface for Ruff.
// Ruff is an extremely fast Python linter, written in Rust.
type Ruff struct{}

// NewRuff creates a new Ruff linter.
func NewRuff() *Ruff {
	return &Ruff{}
}

// Name returns the linter identifier.
func (r *Ruff) Name() string {
	return "ruff"
}

// Available checks if ruff is installed.
func (r *Ruff) Available() bool {
	_, err := exec.LookPath("ruff")
	return err == nil
}

// Run executes ruff with JSON output for parsing.
func (r *Ruff) Run(ctx context.Context, workDir string, files []string) (*Result, error) {
	// Build command arguments
	// ruff check --output-format=json
	args := []string{"check", "--output-format=json"}

	if len(files) > 0 {
		// Filter to Python files
		var pyFiles []string
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext == ".py" || ext == ".pyi" {
				pyFiles = append(pyFiles, f)
			}
		}
		if len(pyFiles) == 0 {
			return &Result{
				Linter:  r.Name(),
				Passed:  true,
				Summary: "No Python files to lint",
			}, nil
		}
		args = append(args, pyFiles...)
	} else {
		// Lint current directory
		args = append(args, ".")
	}

	cmd := exec.CommandContext(ctx, "ruff", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()

	// Parse JSON output
	result, parseErr := r.parseOutput(output)
	if parseErr != nil {
		// If parsing fails but we got output, include it
		if err != nil {
			return &Result{
				Linter:  r.Name(),
				Passed:  false,
				Summary: fmt.Sprintf("Linter failed: %s", string(output)),
				Error:   err,
			}, nil
		}
		return nil, fmt.Errorf("parse ruff output: %w", parseErr)
	}

	// ruff returns exit code 1 when issues are found
	return result, nil
}

// ruffIssue represents a single issue in Ruff's JSON output.
type ruffIssue struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Fix      *any   `json:"fix,omitempty"`
	Location struct {
		File   string `json:"file"`
		Row    int    `json:"row"`
		Column int    `json:"column"`
	} `json:"location"`
	EndLocation struct {
		Row    int `json:"row"`
		Column int `json:"column"`
	} `json:"end_location"`
	Filename string `json:"filename"`
	NoQA     *any   `json:"noqa_row,omitempty"`
	URL      string `json:"url,omitempty"`
}

// parseOutput parses ruff JSON output into a Result.
func (r *Ruff) parseOutput(output []byte) (*Result, error) {
	// ruff outputs empty array for no issues
	if len(output) == 0 || string(output) == "[]" || string(output) == "[]\n" {
		return &Result{
			Linter:  r.Name(),
			Passed:  true,
			Summary: "No issues found",
		}, nil
	}

	var parsed []ruffIssue
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(parsed))
	for _, i := range parsed {
		// Get file path - try location.file first, then filename
		filePath := i.Location.File
		if filePath == "" {
			filePath = i.Filename
		}

		// Determine severity based on rule code
		// E = error, W = warning, F = fatal/error
		severity := SeverityWarning
		if len(i.Code) > 0 {
			prefix := string(i.Code[0])
			if prefix == "E" || prefix == "F" {
				severity = SeverityError
			}
		}

		issues = append(issues, Issue{
			Path:     filePath,
			Line:     i.Location.Row,
			Column:   i.Location.Column,
			Message:  i.Message,
			Severity: severity,
			Rule:     i.Code,
		})
	}

	// Calculate pass status
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
		Linter:  r.Name(),
		Issues:  issues,
		Passed:  passed,
		Summary: summary,
	}, nil
}
