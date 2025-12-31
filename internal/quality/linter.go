// Package quality provides linter integration for automated code quality checks.
// Linters are auto-detected based on project files (go.mod, package.json, etc.)
// and their output is fed into the review phase.
package quality

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Severity indicates the importance of a lint issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue represents a single lint issue found by a linter.
type Issue struct {
	Path     string   // File path relative to workspace
	Line     int      // Line number (1-based)
	Column   int      // Column number (1-based), 0 if not available
	Message  string   // Issue description
	Severity Severity // error, warning, or info
	Rule     string   // Linter rule name (e.g., "errcheck", "unused")
}

// Result holds the output from running a linter.
type Result struct {
	Linter  string  // Linter name
	Issues  []Issue // All issues found
	Passed  bool    // True if no errors (warnings allowed)
	Summary string  // Human-readable summary
	Error   error   // Non-nil if linter execution failed
}

// Linter defines the interface for all linter implementations.
type Linter interface {
	// Name returns the linter identifier (e.g., "golangci-lint", "eslint").
	Name() string

	// Available returns true if the linter binary is installed and accessible.
	Available() bool

	// Run executes the linter on the specified files.
	// If files is empty, it lints the entire workspace.
	Run(ctx context.Context, workDir string, files []string) (*Result, error)
}

// Registry manages available linters and auto-detection.
type Registry struct {
	linters []Linter
}

// NewRegistry creates a new linter registry with standard linters registered.
func NewRegistry() *Registry {
	r := &Registry{}
	// Register standard linters
	r.Register(NewGolangCI())
	r.Register(NewESLint())
	r.Register(NewRuff())
	return r
}

// Register adds a linter to the registry.
func (r *Registry) Register(l Linter) {
	r.linters = append(r.linters, l)
}

// Available returns all linters whose binaries are installed.
func (r *Registry) Available() []Linter {
	var available []Linter
	for _, l := range r.linters {
		if l.Available() {
			available = append(available, l)
		}
	}
	return available
}

// DetectForProject returns linters appropriate for the given project.
// Detection is based on project files like go.mod, package.json, pyproject.toml.
func (r *Registry) DetectForProject(workDir string) []Linter {
	var detected []Linter

	for _, l := range r.linters {
		if !l.Available() {
			continue
		}

		switch l.Name() {
		case "golangci-lint":
			if fileExists(filepath.Join(workDir, "go.mod")) {
				detected = append(detected, l)
			}
		case "eslint":
			if fileExists(filepath.Join(workDir, "package.json")) {
				detected = append(detected, l)
			}
		case "ruff":
			if fileExists(filepath.Join(workDir, "pyproject.toml")) ||
				fileExists(filepath.Join(workDir, "setup.py")) ||
				fileExists(filepath.Join(workDir, "requirements.txt")) {
				detected = append(detected, l)
			}
		}
	}

	return detected
}

// RunAll executes all detected linters and aggregates results.
func (r *Registry) RunAll(ctx context.Context, workDir string, files []string) []*Result {
	linters := r.DetectForProject(workDir)
	results := make([]*Result, 0, len(linters))

	for _, l := range linters {
		result, err := l.Run(ctx, workDir, files)
		if err != nil {
			results = append(results, &Result{
				Linter: l.Name(),
				Error:  err,
			})
			continue
		}
		results = append(results, result)
	}

	return results
}

// FormatResults creates a human-readable summary of lint results.
// This is suitable for inclusion in an AI agent prompt.
func FormatResults(results []*Result) string {
	if len(results) == 0 {
		return ""
	}

	var sb fmt.Stringer = &stringBuilder{}
	b := sb.(*stringBuilder)

	b.WriteString("## Automated Lint Results\n\n")

	totalIssues := 0
	for _, r := range results {
		if r.Error != nil {
			b.WriteString(fmt.Sprintf("### %s (error)\n", r.Linter))
			b.WriteString(fmt.Sprintf("Failed to run: %v\n\n", r.Error))
			continue
		}

		totalIssues += len(r.Issues)

		if len(r.Issues) == 0 {
			b.WriteString(fmt.Sprintf("### %s ✓\n", r.Linter))
			b.WriteString("No issues found.\n\n")
			continue
		}

		b.WriteString(fmt.Sprintf("### %s (%d issues)\n", r.Linter, len(r.Issues)))
		for _, issue := range r.Issues {
			location := issue.Path
			if issue.Line > 0 {
				location = fmt.Sprintf("%s:%d", issue.Path, issue.Line)
				if issue.Column > 0 {
					location = fmt.Sprintf("%s:%d", location, issue.Column)
				}
			}

			severity := string(issue.Severity)
			if severity == "" {
				severity = "warning"
			}

			rule := ""
			if issue.Rule != "" {
				rule = fmt.Sprintf(" [%s]", issue.Rule)
			}

			b.WriteString(fmt.Sprintf("- **%s** %s: %s%s\n", severity, location, issue.Message, rule))
		}
		b.WriteString("\n")
	}

	if totalIssues > 0 {
		b.WriteString(fmt.Sprintf("**Total: %d issues found. Please address these in your review.**\n", totalIssues))
	}

	return b.String()
}

// Helper to check if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Simple string builder wrapper.
type stringBuilder struct {
	data []byte
}

func (b *stringBuilder) WriteString(s string) {
	b.data = append(b.data, s...)
}

func (b *stringBuilder) String() string {
	return string(b.data)
}
