// conductor_hold_the_line.go — diff-aware quality gate filtering.
package conductor

import (
	"context"
	"log/slog"

	"github.com/valksor/kvelmo/pkg/findings"
)

// classifyFindings annotates findings with origin (introduced vs pre-existing)
// using the git diff between the base branch and HEAD. When hold-the-line is
// enabled in settings, only introduced findings are returned so pre-existing
// tech debt does not block agent work.
func (c *Conductor) classifyFindings(ctx context.Context, input []findings.Finding) []findings.Finding {
	if !c.holdTheLineEnabled() {
		return input
	}

	if c.git == nil {
		slog.Debug("hold-the-line: git not available, skipping classification")

		return input
	}

	base, err := c.getBaseBranch(ctx)
	if err != nil {
		slog.Warn("hold-the-line: cannot determine base branch, skipping classification", "error", err)

		return input
	}

	rawHunks, err := c.git.DiffHunks(ctx, base)
	if err != nil {
		slog.Warn("hold-the-line: diff hunks failed, skipping classification", "error", err)

		return input
	}

	diffHunks := convertHunks(rawHunks)
	classified := findings.ClassifyByDiff(input, diffHunks)
	introduced := findings.FilterIntroduced(classified)

	slog.Info("hold-the-line: classified findings",
		"total", len(classified),
		"introduced", len(introduced),
		"pre_existing", len(classified)-len(introduced),
	)

	return introduced
}

// holdTheLineEnabled checks settings. Defaults to true when not explicitly set.
func (c *Conductor) holdTheLineEnabled() bool {
	s := c.getEffectiveSettings()
	if s == nil || s.Workflow.HoldTheLine == nil {
		return true
	}

	return *s.Workflow.HoldTheLine
}

// convertHunks converts git's [2]int representation to findings.LineRange.
func convertHunks(raw map[string][][2]int) map[string][]findings.LineRange {
	result := make(map[string][]findings.LineRange, len(raw))
	for file, hunks := range raw {
		ranges := make([]findings.LineRange, len(hunks))
		for i, h := range hunks {
			ranges[i] = findings.LineRange{Start: h[0], End: h[1]}
		}
		result[file] = ranges
	}

	return result
}
