package quality

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/valksor/kvelmo/pkg/findings"
)

// SlopChecker detects low-quality AI-generated code patterns such as
// boilerplate phrases, excessive comments, redundant error wrapping,
// commented-out code blocks, and ignored errors without justification.
type SlopChecker struct{}

// NewSlopChecker creates a new SlopChecker.
func NewSlopChecker() *SlopChecker {
	return &SlopChecker{}
}

// Name returns the checker name.
func (s *SlopChecker) Name() string { return "slop-detector" }

// Check scans changed files in workDir for AI slop patterns and returns findings.
func (s *SlopChecker) Check(ctx context.Context, workDir string) ([]findings.Finding, error) {
	files, err := changedFiles(ctx, workDir)
	if err != nil {
		return nil, fmt.Errorf("listing changed files: %w", err)
	}

	var all []findings.Finding

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		absPath := filepath.Join(workDir, file)

		info, statErr := os.Stat(absPath)
		if statErr != nil || info.IsDir() {
			continue
		}

		ff, scanErr := s.scanFile(file, absPath)
		if scanErr != nil {
			continue // skip unreadable files
		}

		all = append(all, ff...)
	}

	return all, nil
}

// changedFiles returns file paths relative to workDir that have been modified
// according to git diff. Falls back to an empty list if git is unavailable.
func changedFiles(ctx context.Context, workDir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		// Try unstaged changes as fallback.
		cmd2 := exec.CommandContext(ctx, "git", "diff", "--name-only")
		cmd2.Dir = workDir

		output, err = cmd2.Output()
		if err != nil {
			return nil, fmt.Errorf("git diff: %w", err)
		}
	}

	var files []string

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// AI boilerplate phrases that appear in comments written by LLMs.
var aiBoilerplatePhrases = []string{
	"here's",
	"let me",
	"i'll",
	"as an ai",
	"sure,",
	"certainly",
	"i've implemented",
}

// commentedCodePattern matches lines that look like commented-out Go/JS code.
var commentedCodePattern = regexp.MustCompile(
	`^\s*//\s*(fmt\.\w+|log\.\w+|return\s|if\s|for\s|switch\s|func\s|var\s|const\s|type\s|import\s|package\s)\s*\(`,
)

// commentedCodeSimple matches simpler commented-out code patterns like
// statements ending with parentheses or assignments.
var commentedCodeSimple = regexp.MustCompile(
	`^\s*//\s*\w+(\.\w+)+\(.*\)`,
)

// redundantErrWrap matches error wrapping where the message itself says "error" or "failed to fail".
var redundantErrWrapPattern = regexp.MustCompile(
	`fmt\.Errorf\(\s*"(?:error|err|failed to fail)[:\s].*%w"`,
)

// ignoredErrPattern matches `_ = err` or `_ = someFunc()` patterns.
var ignoredErrPattern = regexp.MustCompile(
	`_\s*=\s*err\b`,
)

func (s *SlopChecker) scanFile(relPath, absPath string) ([]findings.Finding, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var (
		ff           []findings.Finding
		totalLines   int
		commentLines int
	)

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		totalLines++

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		isComment := strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")

		if isComment {
			commentLines++
		}

		// Check 1: AI boilerplate phrases in comments.
		if isComment {
			lower := strings.ToLower(trimmed)
			for _, phrase := range aiBoilerplatePhrases {
				if strings.Contains(lower, phrase) {
					ff = append(ff, findings.Finding{
						ID:          fmt.Sprintf("slop-ai-%s-%d", relPath, lineNum),
						Severity:    findings.SeverityMedium,
						Category:    findings.CategoryQuality,
						Rule:        "slop-ai-boilerplate",
						File:        relPath,
						Line:        lineNum,
						Message:     fmt.Sprintf("AI boilerplate phrase %q in comment", phrase),
						Remediation: "Remove conversational AI language from code comments",
						Evidence:    trimmed,
						Source:      "slop-detector",
					})

					break // one finding per line
				}
			}
		}

		// Check 3: Redundant error wrapping.
		if redundantErrWrapPattern.MatchString(line) {
			ff = append(ff, findings.Finding{
				ID:          fmt.Sprintf("slop-errwrap-%s-%d", relPath, lineNum),
				Severity:    findings.SeverityLow,
				Category:    findings.CategoryQuality,
				Rule:        "slop-redundant-error-wrap",
				File:        relPath,
				Line:        lineNum,
				Message:     "Redundant error wrapping with generic prefix",
				Remediation: "Use a descriptive context prefix instead of 'error' or 'failed to fail'",
				Evidence:    trimmed,
				Source:      "slop-detector",
			})
		}

		// Check 4: Commented-out code blocks.
		if isComment && (commentedCodePattern.MatchString(line) || commentedCodeSimple.MatchString(line)) {
			ff = append(ff, findings.Finding{
				ID:          fmt.Sprintf("slop-deadcode-%s-%d", relPath, lineNum),
				Severity:    findings.SeverityLow,
				Category:    findings.CategoryQuality,
				Rule:        "slop-commented-out-code",
				File:        relPath,
				Line:        lineNum,
				Message:     "Commented-out code should be removed",
				Remediation: "Delete commented-out code; use version control to retrieve old code",
				Evidence:    trimmed,
				Source:      "slop-detector",
			})
		}

		// Check 5: Ignored errors without justification.
		if ignoredErrPattern.MatchString(line) {
			hasJustification := false
			// Check if the same line or immediately preceding line has a nolint/justification comment.
			if strings.Contains(line, "//") {
				commentPart := line[strings.Index(line, "//")+2:]
				commentPart = strings.TrimSpace(commentPart)
				if len(commentPart) > 0 {
					hasJustification = true
				}
			}

			if !hasJustification {
				ff = append(ff, findings.Finding{
					ID:          fmt.Sprintf("slop-errigno-%s-%d", relPath, lineNum),
					Severity:    findings.SeverityMedium,
					Category:    findings.CategoryQuality,
					Rule:        "slop-ignored-error",
					File:        relPath,
					Line:        lineNum,
					Message:     "Error assigned to blank identifier without justification comment",
					Remediation: "Handle the error or add a justification comment explaining why it is safe to ignore",
					Evidence:    trimmed,
					Source:      "slop-detector",
				})
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return ff, scanErr
	}

	// Check 2: Excessive comment-to-code ratio.
	if totalLines > 10 {
		ratio := float64(commentLines) / float64(totalLines)
		if ratio > 0.4 {
			ff = append(ff, findings.Finding{
				ID:       "slop-comments-" + relPath,
				Severity: findings.SeverityLow,
				Category: findings.CategoryQuality,
				Rule:     "slop-excessive-comments",
				File:     relPath,
				Message: fmt.Sprintf("Excessive comment-to-code ratio: %.0f%% (%d/%d lines are comments)",
					ratio*100, commentLines, totalLines),
				Remediation: "Reduce unnecessary comments; code should be self-documenting where possible",
				Source:      "slop-detector",
			})
		}
	}

	return ff, nil
}
