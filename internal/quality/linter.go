// Package quality provides code quality checking capabilities for kvelmo.
package quality

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Severity represents the severity level of a lint issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue represents a lint finding.
type Issue struct {
	Severity    Severity `json:"severity"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	Message     string   `json:"message"`
	Rule        string   `json:"rule"`
	Remediation string   `json:"remediation,omitempty"`
}

// CoverageStatus indicates how thoroughly a linter was able to run.
type CoverageStatus string

const (
	CoverageFull        CoverageStatus = "full"        // Linter ran to completion
	CoverageSkipped     CoverageStatus = "skipped"     // Wrong project type
	CoverageUnavailable CoverageStatus = "unavailable" // Tool not installed
	CoverageError       CoverageStatus = "error"       // Tool crashed or timed out
)

// Report contains the results of a lint run.
type Report struct {
	Linter         string         `json:"linter"`
	Issues         []Issue        `json:"issues"`
	Duration       time.Duration  `json:"duration"`
	Coverage       CoverageStatus `json:"coverage"`
	CoverageReason string         `json:"coverage_reason,omitempty"`
}

// HasErrors returns true if the report contains any error-severity issues.
func (r *Report) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}

	return false
}

// ErrorCount returns the number of error-severity issues.
func (r *Report) ErrorCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			count++
		}
	}

	return count
}

// Score computes a weighted quality score (0-100) across linter reports.
// Errors deduct 10 points, warnings 3, info 1. Each linter scored independently,
// then averaged. Inspired by mcp-certify's gate/score duality.
func Score(reports []*Report) float64 {
	if len(reports) == 0 {
		return 100
	}
	total := 0.0
	for _, r := range reports {
		linterScore := 100.0
		for _, issue := range r.Issues {
			switch issue.Severity {
			case SeverityError:
				linterScore -= 10
			case SeverityWarning:
				linterScore -= 3
			case SeverityInfo:
				linterScore -= 1
			}
		}
		if linterScore < 0 {
			linterScore = 0
		}
		total += linterScore
	}

	return total / float64(len(reports))
}

// HasBlockers returns true if any report contains error-severity issues.
// This is the "gate" half of the gate/score duality — hard pass/fail.
func HasBlockers(reports []*Report) bool {
	for _, r := range reports {
		if r.HasErrors() {
			return true
		}
	}

	return false
}

// Linter defines the interface for code quality checkers.
type Linter interface {
	Lint(ctx context.Context, dir string) (*Report, error)
	Name() string
	Available() bool
}

// Runner orchestrates multiple linters based on project type.
type Runner struct {
	linters []Linter
}

// NewRunner creates a runner with default linters.
func NewRunner() *Runner {
	return &Runner{
		linters: []Linter{
			NewGolangCILint(),
			NewESLint(),
		},
	}
}

// AddLinter adds a linter to the runner.
func (r *Runner) AddLinter(l Linter) {
	r.linters = append(r.linters, l)
}

// Run executes all available linters and returns combined results.
func (r *Runner) Run(ctx context.Context, dir string) ([]*Report, error) {
	var reports []*Report

	for _, linter := range r.linters {
		select {
		case <-ctx.Done():
			return reports, ctx.Err()
		default:
		}

		report, err := linter.Lint(ctx, dir)
		if err != nil {
			return reports, fmt.Errorf("%s: %w", linter.Name(), err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// DetectProjectType determines the project type based on files present.
func DetectProjectType(dir string) []string {
	var types []string

	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		types = append(types, "go")
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		types = append(types, "javascript")
	}
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		types = append(types, "typescript")
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		types = append(types, "python")
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		types = append(types, "python")
	}

	return types
}
