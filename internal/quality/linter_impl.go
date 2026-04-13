package quality

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GolangCILint wraps golangci-lint for Go projects.
type GolangCILint struct {
	configPath string
}

// NewGolangCILint creates a new golangci-lint wrapper.
func NewGolangCILint() *GolangCILint {
	return &GolangCILint{}
}

// WithConfig sets a custom config path.
func (g *GolangCILint) WithConfig(path string) *GolangCILint {
	g.configPath = path

	return g
}

// Name returns the linter name.
func (g *GolangCILint) Name() string {
	return "golangci-lint"
}

// Available checks if golangci-lint is installed.
func (g *GolangCILint) Available() bool {
	_, err := exec.LookPath("golangci-lint")

	return err == nil
}

// Lint runs golangci-lint on the directory.
func (g *GolangCILint) Lint(ctx context.Context, dir string) (*Report, error) {
	start := time.Now()
	report := &Report{
		Linter: g.Name(),
		Issues: []Issue{},
	}

	if !g.Available() {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityInfo,
			Message:  "golangci-lint not installed",
			Rule:     "tool-missing",
		})
		report.Duration = time.Since(start)
		report.Coverage = CoverageUnavailable
		report.CoverageReason = "golangci-lint not found in PATH"

		return report, nil
	}

	// Check if this is a Go project
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		report.Duration = time.Since(start)
		report.Coverage = CoverageSkipped
		report.CoverageReason = "no go.mod found"

		return report, nil // Not a Go project
	}

	args := []string{"run", "--out-format=json"}
	if g.configPath != "" {
		args = append(args, "--config", g.configPath)
	}
	args = append(args, "./...")

	cmd := exec.CommandContext(ctx, "golangci-lint", args...)
	cmd.Dir = dir
	output, _ := cmd.Output() // Ignore error, golangci-lint returns non-zero on issues

	issues := g.parseOutput(output, dir)
	report.Issues = append(report.Issues, issues...)
	report.Duration = time.Since(start)
	report.Coverage = CoverageFull

	return report, nil
}

func (g *GolangCILint) parseOutput(output []byte, baseDir string) []Issue {
	var result struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
				Column   int    `json:"Column"`
			} `json:"Pos"`
			Severity string `json:"Severity"`
		} `json:"Issues"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil
	}

	var issues []Issue
	for _, i := range result.Issues {
		relPath, _ := filepath.Rel(baseDir, i.Pos.Filename)
		if relPath == "" {
			relPath = i.Pos.Filename
		}

		severity := SeverityWarning
		switch strings.ToLower(i.Severity) {
		case "error":
			severity = SeverityError
		case "info":
			severity = SeverityInfo
		}

		issues = append(issues, Issue{
			Severity: severity,
			File:     relPath,
			Line:     i.Pos.Line,
			Column:   i.Pos.Column,
			Message:  i.Text,
			Rule:     i.FromLinter,
		})
	}

	return issues
}

// ESLint wraps eslint for JavaScript/TypeScript projects.
type ESLint struct {
	configPath string
}

// NewESLint creates a new eslint wrapper.
func NewESLint() *ESLint {
	return &ESLint{}
}

// WithConfig sets a custom config path.
func (e *ESLint) WithConfig(path string) *ESLint {
	e.configPath = path

	return e
}

// Name returns the linter name.
func (e *ESLint) Name() string {
	return "eslint"
}

// Available checks if eslint is installed (via npx).
func (e *ESLint) Available() bool {
	_, err := exec.LookPath("npx")

	return err == nil
}

// Lint runs eslint on the directory.
func (e *ESLint) Lint(ctx context.Context, dir string) (*Report, error) {
	start := time.Now()
	report := &Report{
		Linter: e.Name(),
		Issues: []Issue{},
	}

	if !e.Available() {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityInfo,
			Message:  "npx not installed (required for eslint)",
			Rule:     "tool-missing",
		})
		report.Duration = time.Since(start)
		report.Coverage = CoverageUnavailable
		report.CoverageReason = "npx not found in PATH"

		return report, nil
	}

	// Check if this is a JS/TS project
	hasPackageJSON := false
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		hasPackageJSON = true
	}
	if !hasPackageJSON {
		report.Duration = time.Since(start)
		report.Coverage = CoverageSkipped
		report.CoverageReason = "no package.json found"

		return report, nil // Not a JS/TS project
	}

	args := []string{"eslint", "--format=json"}
	if e.configPath != "" {
		args = append(args, "--config", e.configPath)
	}
	args = append(args, ".")

	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = dir
	output, _ := cmd.Output() // Ignore error, eslint returns non-zero on issues

	issues := e.parseOutput(output, dir)
	report.Issues = append(report.Issues, issues...)
	report.Duration = time.Since(start)
	report.Coverage = CoverageFull

	return report, nil
}

func (e *ESLint) parseOutput(output []byte, baseDir string) []Issue {
	var results []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			RuleID   string `json:"ruleId"`
			Severity int    `json:"severity"` // 1 = warning, 2 = error
			Message  string `json:"message"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(output, &results); err != nil {
		return nil
	}

	var issues []Issue
	for _, r := range results {
		relPath, _ := filepath.Rel(baseDir, r.FilePath)
		if relPath == "" {
			relPath = r.FilePath
		}

		for _, m := range r.Messages {
			severity := SeverityWarning
			if m.Severity == 2 {
				severity = SeverityError
			}

			issues = append(issues, Issue{
				Severity: severity,
				File:     relPath,
				Line:     m.Line,
				Column:   m.Column,
				Message:  m.Message,
				Rule:     m.RuleID,
			})
		}
	}

	return issues
}

// GoVet runs go vet as a simple built-in linter.
type GoVet struct{}

// NewGoVet creates a new go vet wrapper.
func NewGoVet() *GoVet {
	return &GoVet{}
}

// Name returns the linter name.
func (g *GoVet) Name() string {
	return "go-vet"
}

// Available checks if go is installed.
func (g *GoVet) Available() bool {
	_, err := exec.LookPath("go")

	return err == nil
}

// Lint runs go vet on the directory.
func (g *GoVet) Lint(ctx context.Context, dir string) (*Report, error) {
	start := time.Now()
	report := &Report{
		Linter: g.Name(),
		Issues: []Issue{},
	}

	if !g.Available() {
		report.Duration = time.Since(start)
		report.Coverage = CoverageUnavailable
		report.CoverageReason = "go not found in PATH"

		return report, nil
	}

	// Check if this is a Go project
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		report.Duration = time.Since(start)
		report.Coverage = CoverageSkipped
		report.CoverageReason = "no go.mod found"

		return report, nil
	}

	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()

	issues := g.parseOutput(string(output), dir)
	report.Issues = append(report.Issues, issues...)
	report.Duration = time.Since(start)
	report.Coverage = CoverageFull

	return report, nil
}

func (g *GoVet) parseOutput(output string, baseDir string) []Issue {
	var issues []Issue
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		// Format: file.go:line:col: message
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}

		relPath, _ := filepath.Rel(baseDir, parts[0])
		if relPath == "" {
			relPath = parts[0]
		}

		var lineNum, col int
		_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)
		_, _ = fmt.Sscanf(parts[2], "%d", &col)

		issues = append(issues, Issue{
			Severity: SeverityWarning,
			File:     relPath,
			Line:     lineNum,
			Column:   col,
			Message:  strings.TrimSpace(parts[3]),
			Rule:     "vet",
		})
	}

	return issues
}
