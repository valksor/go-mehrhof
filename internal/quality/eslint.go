package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ESLint implements the Linter interface for ESLint.
// ESLint is the standard linter for JavaScript and TypeScript.
type ESLint struct{}

// NewESLint creates a new ESLint linter.
func NewESLint() *ESLint {
	return &ESLint{}
}

// Name returns the linter identifier.
func (e *ESLint) Name() string {
	return "eslint"
}

// Available checks if eslint is installed.
// Checks for both global and local (npx) availability.
func (e *ESLint) Available() bool {
	// Check for global eslint
	if _, err := exec.LookPath("eslint"); err == nil {
		return true
	}
	// Check for npx (Node.js package runner)
	_, err := exec.LookPath("npx")
	return err == nil
}

// Run executes eslint with JSON output for parsing.
func (e *ESLint) Run(ctx context.Context, workDir string, files []string) (*Result, error) {
	// Determine how to run eslint
	var cmd *exec.Cmd

	// Build arguments - use JSON formatter for parsing
	args := []string{"--format=json"}

	if len(files) > 0 {
		// Filter to JS/TS files
		var jsFiles []string
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx" ||
				ext == ".mjs" || ext == ".cjs" || ext == ".mts" || ext == ".cts" {
				jsFiles = append(jsFiles, f)
			}
		}
		if len(jsFiles) == 0 {
			return &Result{
				Linter:  e.Name(),
				Passed:  true,
				Summary: "No JS/TS files to lint",
			}, nil
		}
		args = append(args, jsFiles...)
	} else {
		// Lint common source directories
		args = append(args, ".")
	}

	// Prefer local eslint via npx, fallback to global
	if _, err := exec.LookPath("eslint"); err == nil {
		cmd = exec.CommandContext(ctx, "eslint", args...)
	} else {
		// Use npx to run local eslint
		npxArgs := append([]string{"eslint"}, args...)
		cmd = exec.CommandContext(ctx, "npx", npxArgs...)
	}
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()

	// Parse JSON output
	result, parseErr := e.parseOutput(output)
	if parseErr != nil {
		// If parsing fails but we got output, include it
		if err != nil {
			//nolint:nilerr // Error embedded in Result struct
			return &Result{
				Linter:  e.Name(),
				Passed:  false,
				Summary: fmt.Sprintf("Linter failed: %s", string(output)),
				Error:   err,
			}, nil
		}
		// Return raw output as error context
		return nil, fmt.Errorf("parse eslint output: %w (output: %s)", parseErr, string(output))
	}

	// eslint returns exit code 1 when issues are found
	return result, nil
}

// eslintResult represents the JSON output from eslint.
type eslintResult struct {
	FilePath        string      `json:"filePath"`
	Messages        []eslintMsg `json:"messages"`
	ErrorCount      int         `json:"errorCount"`
	WarningCount    int         `json:"warningCount"`
	FixableErrCount int         `json:"fixableErrorCount"`
	FixableWarnCnt  int         `json:"fixableWarningCount"`
	Source          string      `json:"source,omitempty"`
}

type eslintMsg struct {
	RuleID    string `json:"ruleId"`
	Severity  int    `json:"severity"` // 1 = warning, 2 = error
	Message   string `json:"message"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	NodeType  string `json:"nodeType,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
	EndColumn int    `json:"endColumn,omitempty"`
}

// parseOutput parses eslint JSON output into a Result.
func (e *ESLint) parseOutput(output []byte) (*Result, error) {
	// eslint outputs empty array for no issues
	if len(output) == 0 || string(output) == "[]" || string(output) == "[]\n" {
		return &Result{
			Linter:  e.Name(),
			Passed:  true,
			Summary: "No issues found",
		}, nil
	}

	var parsed []eslintResult
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, err
	}

	var issues []Issue
	for _, fileResult := range parsed {
		// Make path relative if possible
		relPath := fileResult.FilePath

		for _, msg := range fileResult.Messages {
			severity := SeverityWarning
			if msg.Severity == 2 {
				severity = SeverityError
			}

			rule := msg.RuleID
			if rule == "" {
				rule = "parse-error"
			}

			issues = append(issues, Issue{
				Path:     relPath,
				Line:     msg.Line,
				Column:   msg.Column,
				Message:  msg.Message,
				Severity: severity,
				Rule:     rule,
			})
		}
	}

	// Calculate pass status (no errors)
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
		Linter:  e.Name(),
		Issues:  issues,
		Passed:  passed,
		Summary: summary,
	}, nil
}
