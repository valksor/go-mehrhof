package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// hunkHeaderRe matches unified diff hunk headers like @@ -a,b +c,d @@.
// We capture the new-side start line and optional count.
var hunkHeaderRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// DiffHunks returns the changed line ranges per file between the given base
// ref and HEAD. Each entry maps a file path (relative to repo root) to a
// list of [start, end] pairs representing inclusive line ranges that were
// added or modified.
//
// The base argument is typically a branch name like "main" or "origin/main".
// It uses the three-dot diff (base...HEAD) to show only changes introduced
// on the current branch since it diverged from base.
func (r *Repository) DiffHunks(ctx context.Context, base string) (map[string][][2]int, error) {
	out, err := r.run(ctx, "diff", "--unified=0", base+"...HEAD")
	if err != nil {
		return nil, fmt.Errorf("diff hunks: %w", err)
	}

	return parseUnifiedDiffHunks(out), nil
}

// parseUnifiedDiffHunks parses `git diff --unified=0` output and extracts
// new-side line ranges per file.
func parseUnifiedDiffHunks(diffOutput string) map[string][][2]int {
	result := make(map[string][][2]int)

	var currentFile string

	for line := range strings.SplitSeq(diffOutput, "\n") {
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			currentFile = after

			continue
		}

		if currentFile == "" {
			continue
		}

		matches := hunkHeaderRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		start, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		count := 1
		if matches[2] != "" {
			count, err = strconv.Atoi(matches[2])
			if err != nil {
				continue
			}
		}

		// A count of 0 means pure deletion — no new lines to flag.
		if count == 0 {
			continue
		}

		end := start + count - 1
		result[currentFile] = append(result[currentFile], [2]int{start, end})
	}

	return result
}
