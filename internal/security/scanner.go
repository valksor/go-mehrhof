// Package security provides security scanning capabilities for kvelmo.
package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Severity represents the severity level of a security finding.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// Finding represents a security issue found during scanning.
type Finding struct {
	Severity   Severity `json:"severity"`
	Type       string   `json:"type"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
}

// Report contains the results of a security scan.
type Report struct {
	Scanner  string        `json:"scanner"`
	Findings []Finding     `json:"findings"`
	Duration time.Duration `json:"duration"`
}

// Scanner defines the interface for security scanners.
type Scanner interface {
	Scan(ctx context.Context, dir string) (*Report, error)
	Name() string
}

// SecretScanner detects hardcoded secrets in source code.
type SecretScanner struct {
	patterns []secretPattern
}

type secretPattern struct {
	name     string
	pattern  *regexp.Regexp
	severity Severity
}

// NewSecretScanner creates a new secret scanner with default patterns.
func NewSecretScanner() *SecretScanner {
	return &SecretScanner{
		patterns: []secretPattern{
			{
				name:     "AWS Access Key",
				pattern:  regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
				severity: SeverityCritical,
			},
			{
				name:     "AWS Secret Key",
				pattern:  regexp.MustCompile(`(?i)aws[_\-]?secret[_\-]?(?:access)?[_\-]?key['\"]?\s*[:=]\s*['\"]?([A-Za-z0-9/+=]{40})`),
				severity: SeverityCritical,
			},
			{
				name:     "GitHub Token",
				pattern:  regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,255}`),
				severity: SeverityCritical,
			},
			{
				name:     "Generic API Key",
				pattern:  regexp.MustCompile(`(?i)(?:api[_\-]?key|apikey)['\"]?\s*[:=]\s*['\"]?([A-Za-z0-9_\-]{20,})['\"]?`),
				severity: SeverityHigh,
			},
			{
				name:     "Generic Secret",
				pattern:  regexp.MustCompile(`(?i)(?:secret|password|passwd|pwd)['\"]?\s*[:=]\s*['\"]?([^\s'"]{8,})['\"]?`),
				severity: SeverityHigh,
			},
			{
				name:     "Private Key",
				pattern:  regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
				severity: SeverityCritical,
			},
			{
				name:     "JWT Token",
				pattern:  regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`),
				severity: SeverityMedium,
			},
		},
	}
}

// Name returns the scanner name.
func (s *SecretScanner) Name() string {
	return "secret-scanner"
}

// Scan scans the directory for hardcoded secrets.
func (s *SecretScanner) Scan(ctx context.Context, dir string) (*Report, error) {
	start := time.Now()
	report := &Report{
		Scanner:  s.Name(),
		Findings: []Finding{},
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue walking on individual file errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories and non-interesting files
		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if !isSourceFile(path) {
			return nil
		}

		findings, err := s.scanFile(path, dir)
		if err != nil {
			return nil //nolint:nilerr // Skip files we can't read, continue walking
		}
		report.Findings = append(report.Findings, findings...)

		return nil
	})

	report.Duration = time.Since(start)

	return report, err
}

func (s *SecretScanner) scanFile(path, baseDir string) ([]Finding, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var findings []Finding
	scanner := bufio.NewScanner(file)
	lineNum := 0

	relPath, _ := filepath.Rel(baseDir, path)
	if relPath == "" {
		relPath = path
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, p := range s.patterns {
			if p.pattern.MatchString(line) {
				findings = append(findings, Finding{
					Severity:   p.severity,
					Type:       "secret",
					File:       relPath,
					Line:       lineNum,
					Message:    fmt.Sprintf("Potential %s detected", p.name),
					Suggestion: "Remove the secret and use environment variables or a secrets manager",
				})
			}
		}
	}

	return findings, scanner.Err()
}

// DependencyScanner checks for vulnerable dependencies.
type DependencyScanner struct{}

// NewDependencyScanner creates a new dependency scanner.
func NewDependencyScanner() *DependencyScanner {
	return &DependencyScanner{}
}

// Name returns the scanner name.
func (d *DependencyScanner) Name() string {
	return "dependency-scanner"
}

// Scan checks go.mod for vulnerable dependencies using govulncheck if available.
func (d *DependencyScanner) Scan(ctx context.Context, dir string) (*Report, error) {
	start := time.Now()
	report := &Report{
		Scanner:  d.Name(),
		Findings: []Finding{},
	}

	// Check if govulncheck is available
	if _, err := exec.LookPath("govulncheck"); err != nil {
		report.Findings = append(report.Findings, Finding{
			Severity:   SeverityInfo,
			Type:       "tool-missing",
			File:       "",
			Line:       0,
			Message:    "govulncheck not installed - skipping vulnerability scan",
			Suggestion: "Install with: go install golang.org/x/vuln/cmd/govulncheck@latest",
		})
		report.Duration = time.Since(start)

		return report, nil //nolint:nilerr // Tool missing is an info finding, not an error
	}

	// Run govulncheck
	cmd := exec.CommandContext(ctx, "govulncheck", "-json", "./...")
	cmd.Dir = dir
	output, _ := cmd.Output() // Ignore error, govulncheck returns non-zero on findings

	findings := d.parseGovulncheckOutput(output)
	report.Findings = append(report.Findings, findings...)
	report.Duration = time.Since(start)

	return report, nil
}

func (d *DependencyScanner) parseGovulncheckOutput(output []byte) []Finding {
	var findings []Finding

	// Parse NDJSON output
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		var entry struct {
			Finding *struct {
				OSV   string `json:"osv"`
				Trace []struct {
					Module  string `json:"module"`
					Package string `json:"package"`
				} `json:"trace"`
			} `json:"finding"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if entry.Finding != nil && len(entry.Finding.Trace) > 0 {
			trace := entry.Finding.Trace[0]
			findings = append(findings, Finding{
				Severity:   SeverityHigh,
				Type:       "vulnerability",
				File:       "go.mod",
				Line:       0,
				Message:    fmt.Sprintf("Vulnerable dependency: %s (%s)", trace.Module, entry.Finding.OSV),
				Suggestion: "Update the dependency to a patched version",
			})
		}
	}

	return findings
}

// Runner runs multiple scanners and aggregates results.
type Runner struct {
	scanners []Scanner
}

// NewRunner creates a runner with default scanners.
func NewRunner() *Runner {
	return &Runner{
		scanners: []Scanner{
			NewSecretScanner(),
			NewDependencyScanner(),
		},
	}
}

// AddScanner adds a scanner to the runner.
func (r *Runner) AddScanner(s Scanner) {
	r.scanners = append(r.scanners, s)
}

// Run executes all scanners and returns combined results.
func (r *Runner) Run(ctx context.Context, dir string) ([]*Report, error) {
	var reports []*Report

	for _, scanner := range r.scanners {
		select {
		case <-ctx.Done():
			return reports, ctx.Err()
		default:
		}

		report, err := scanner.Scan(ctx, dir)
		if err != nil {
			return reports, fmt.Errorf("%s: %w", scanner.Name(), err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// Helper functions

func shouldSkipDir(name string) bool {
	skip := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		".venv":        true,
		"__pycache__":  true,
		".idea":        true,
		".vscode":      true,
		"dist":         true,
		"build":        true,
	}

	return skip[name]
}

func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	sourceExts := map[string]bool{
		".go":   true,
		".js":   true,
		".ts":   true,
		".tsx":  true,
		".jsx":  true,
		".py":   true,
		".rb":   true,
		".java": true,
		".yml":  true,
		".yaml": true,
		".json": true,
		".env":  true,
		".sh":   true,
		".bash": true,
	}

	return sourceExts[ext]
}
