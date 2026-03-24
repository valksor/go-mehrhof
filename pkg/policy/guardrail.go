package policy

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/valksor/kvelmo/pkg/findings"
)

// Guardrail validates preconditions or postconditions for a phase.
type Guardrail interface {
	Name() string
	Check(ctx context.Context, phase string, workDir string, specs []string) []findings.Finding
}

// guardrailRegistry maps guardrail names to factory functions.
var guardrailRegistry = map[string]func() Guardrail{
	"require-spec":  func() Guardrail { return &RequireSpec{} },
	"max-diff-size": func() Guardrail { return &MaxDiffSize{MaxLines: 2000} },
}

// GetGuardrail returns a guardrail instance by name.
func GetGuardrail(name string) (Guardrail, bool) {
	factory, ok := guardrailRegistry[name]
	if !ok {
		return nil, false
	}

	return factory(), true
}

// ListGuardrails returns all registered guardrail names in sorted order.
func ListGuardrails() []string {
	names := make([]string, 0, len(guardrailRegistry))
	for name := range guardrailRegistry {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// RequireSpec blocks phases that need specifications if none exist.
// Applies to: implement, simplify, optimize, review.
type RequireSpec struct{}

func (r *RequireSpec) Name() string { return "require-spec" }

func (r *RequireSpec) Check(_ context.Context, phase string, _ string, specs []string) []findings.Finding {
	requiresSpec := phase == "implement" || phase == "simplify" || phase == "optimize" || phase == "review"
	if !requiresSpec {
		return nil
	}

	if len(specs) > 0 {
		return nil
	}

	return []findings.Finding{{
		ID:          "guardrail-require-spec",
		Severity:    findings.SeverityHigh,
		Category:    findings.CategoryPolicy,
		Rule:        "require-spec",
		Message:     fmt.Sprintf("phase %q requires at least one specification file, but none exist", phase),
		Remediation: "Run the plan phase first to generate a specification, or create one manually.",
		Source:      "guardrail",
	}}
}

// MaxDiffSize warns when the diff exceeds a configurable line count.
// This is a post-check guardrail — it reports findings but does not block.
type MaxDiffSize struct {
	MaxLines int
}

func (m *MaxDiffSize) Name() string { return "max-diff-size" }

func (m *MaxDiffSize) Check(ctx context.Context, _ string, workDir string, _ []string) []findings.Finding {
	maxLines := m.MaxLines
	if maxLines <= 0 {
		maxLines = 2000
	}

	diffLines, err := countDiffLines(ctx, workDir)
	if err != nil {
		// Cannot determine diff size — skip silently rather than block.
		return nil
	}

	if diffLines <= maxLines {
		return nil
	}

	return []findings.Finding{{
		ID:          "guardrail-max-diff-size",
		Severity:    findings.SeverityMedium,
		Category:    findings.CategoryPolicy,
		Rule:        "max-diff-size",
		Message:     fmt.Sprintf("diff is %d lines, exceeding the %d-line threshold", diffLines, maxLines),
		Evidence:    strconv.Itoa(diffLines) + " lines changed",
		Remediation: "Consider breaking the change into smaller increments.",
		Source:      "guardrail",
	}}
}

// countDiffLines counts the number of lines in the working tree diff.
func countDiffLines(ctx context.Context, workDir string) (int, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmd.Dir = workDir

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git diff --stat: %w", err)
	}

	// The last line of git diff --stat contains the summary, e.g.:
	// " 5 files changed, 120 insertions(+), 30 deletions(-)"
	// Count insertions + deletions.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return 0, nil
	}

	summary := lines[len(lines)-1]

	var total int

	// Parse insertions.
	if idx := strings.Index(summary, "insertion"); idx > 0 {
		// Walk backwards to find the number.
		numStr := extractLastNumber(summary[:idx])
		if n, err := strconv.Atoi(numStr); err == nil {
			total += n
		}
	}

	// Parse deletions.
	if idx := strings.Index(summary, "deletion"); idx > 0 {
		numStr := extractLastNumber(summary[:idx])
		if n, err := strconv.Atoi(numStr); err == nil {
			total += n
		}
	}

	return total, nil
}

// extractLastNumber extracts the last whitespace-separated number from s.
func extractLastNumber(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}

	return fields[len(fields)-1]
}
