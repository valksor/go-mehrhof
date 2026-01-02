package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// PHPCSFixer implements the Linter interface for PHP-CS-Fixer.
// PHP-CS-Fixer is a tool to automatically fix PHP coding standards issues.
type PHPCSFixer struct{}

// NewPHPCSFixer creates a new PHP-CS-Fixer linter.
func NewPHPCSFixer() *PHPCSFixer {
	return &PHPCSFixer{}
}

// Name returns the linter identifier.
func (p *PHPCSFixer) Name() string {
	return "php-cs-fixer"
}

// Available checks if php-cs-fixer is installed.
func (p *PHPCSFixer) Available() bool {
	_, err := exec.LookPath("php-cs-fixer")

	return err == nil
}

// Run executes php-cs-fixer with JSON output for parsing.
func (p *PHPCSFixer) Run(ctx context.Context, workDir string, files []string) (*Result, error) {
	// Build command arguments
	// php-cs-fixer fix --dry-run --format=json
	args := []string{"fix", "--dry-run", "--format=json"}

	if len(files) > 0 {
		// Filter to PHP files
		var phpFiles []string
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext == ".php" {
				phpFiles = append(phpFiles, f)
			}
		}
		if len(phpFiles) == 0 {
			return &Result{
				Linter:  p.Name(),
				Passed:  true,
				Summary: "No PHP files to lint",
			}, nil
		}
		// php-cs-fixer requires --path-mode=intersection when specifying files
		args = append(args, "--path-mode=intersection", "--")
		args = append(args, phpFiles...)
	} else {
		// Lint current directory
		args = append(args, ".")
	}

	cmd := exec.CommandContext(ctx, "php-cs-fixer", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()

	// Parse JSON output
	result, parseErr := p.ParseOutput(output)
	if parseErr != nil {
		// If parsing fails but we got output, include it
		if err != nil {
			//nolint:nilerr // Error embedded in Result struct
			return &Result{
				Linter:  p.Name(),
				Passed:  false,
				Summary: "Linter failed: " + string(output),
				Error:   err,
			}, nil
		}

		return nil, fmt.Errorf("parse php-cs-fixer output: %w", parseErr)
	}

	// php-cs-fixer returns exit code 8 when issues are found (with --dry-run)
	return result, nil
}

// phpcsFixerOutput represents php-cs-fixer's JSON output structure.
type phpcsFixerOutput struct {
	Files []phpcsFixerFile `json:"files"`
}

// phpcsFixerFile represents a single file in php-cs-fixer's JSON output.
type phpcsFixerFile struct {
	Name          string   `json:"name"`
	AppliedFixers []string `json:"appliedFixers"`
	Diff          string   `json:"diff,omitempty"`
}

// ParseOutput parses php-cs-fixer JSON output into a Result.
func (p *PHPCSFixer) ParseOutput(output []byte) (*Result, error) {
	// php-cs-fixer outputs empty JSON object or nothing for no issues
	if len(output) == 0 {
		return &Result{
			Linter:  p.Name(),
			Passed:  true,
			Summary: "No issues found",
		}, nil
	}

	// Try to parse as JSON
	var parsed phpcsFixerOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		// Output might not be JSON (e.g., error message)
		// Check if it looks like a success message
		if strings.Contains(string(output), "Fixed 0 of") ||
			strings.Contains(string(output), "\"files\":[]") {
			return &Result{
				Linter:  p.Name(),
				Passed:  true,
				Summary: "No issues found",
			}, nil
		}

		return nil, err
	}

	// No files means no issues
	if len(parsed.Files) == 0 {
		return &Result{
			Linter:  p.Name(),
			Passed:  true,
			Summary: "No issues found",
		}, nil
	}

	// Convert to Issue format
	// php-cs-fixer reports at file level with applied fixers, not line-level
	issues := make([]Issue, 0)
	for _, f := range parsed.Files {
		for _, fixer := range f.AppliedFixers {
			issues = append(issues, Issue{
				Path:     f.Name,
				Line:     0, // php-cs-fixer doesn't report line numbers
				Column:   0,
				Message:  "Would apply fixer: " + fixer,
				Severity: SeverityWarning, // Style issues are warnings
				Rule:     fixer,
			})
		}
	}

	// php-cs-fixer issues are style-based, so we pass (warnings only)
	passed := true
	summary := "No issues found"
	if len(issues) > 0 {
		summary = fmt.Sprintf("%d style issues found in %d files", len(issues), len(parsed.Files))
	}

	return &Result{
		Linter:  p.Name(),
		Issues:  issues,
		Passed:  passed,
		Summary: summary,
	}, nil
}
