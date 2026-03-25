package conductor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ForkComparison holds the comparison results between forks.
type ForkComparison struct {
	Forks []ForkCompareEntry `json:"forks"`
}

// ForkCompareEntry holds comparison data for one fork.
type ForkCompareEntry struct {
	ForkInfo
	DiffStats     DiffStats `json:"diff_stats"`
	FilesAdded    int       `json:"files_added"`
	FilesModified int       `json:"files_modified"`
	LinesAdded    int       `json:"lines_added"`
	LinesRemoved  int       `json:"lines_removed"`
}

// DiffStats holds file change statistics.
type DiffStats struct {
	Files   int `json:"files"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// CompareForks generates a comparison of all active forks.
func (c *Conductor) CompareForks(ctx context.Context) (*ForkComparison, error) {
	c.mu.RLock()
	wu := c.workUnit
	repo := c.git
	c.mu.RUnlock()

	if wu == nil {
		return nil, fmt.Errorf("compare forks: no active task")
	}

	if repo == nil {
		return nil, fmt.Errorf("compare forks: git not available")
	}

	if len(wu.Forks) == 0 {
		return &ForkComparison{Forks: []ForkCompareEntry{}}, nil
	}

	entries := make([]ForkCompareEntry, 0, len(wu.Forks))

	for _, fork := range wu.Forks {
		entry := ForkCompareEntry{ForkInfo: fork}

		// Get diff stats from the fork's base checkpoint to the fork's current branch
		stat, err := repo.DiffAgainst(ctx, fork.CheckpointSHA+"..."+fork.Branch, true)
		if err != nil {
			// If diff fails, include fork with zero stats
			entries = append(entries, entry)

			continue
		}

		entry.DiffStats, entry.FilesAdded, entry.FilesModified, entry.LinesAdded, entry.LinesRemoved = parseDiffStat(stat)
		entries = append(entries, entry)
	}

	return &ForkComparison{Forks: entries}, nil
}

// parseDiffStat parses git diff --stat output into structured stats.
// Example output:
//
//	file1.go | 10 ++++------
//	file2.go | 5 +++++
//	2 files changed, 9 insertions(+), 6 deletions(-)
func parseDiffStat(stat string) (ds DiffStats, added, modified, linesAdded, linesRemoved int) {
	lines := strings.Split(strings.TrimSpace(stat), "\n")
	if len(lines) == 0 {
		return
	}

	// Parse the summary line (last line)
	summary := lines[len(lines)-1]

	// Parse "X file(s) changed"
	if idx := strings.Index(summary, " file"); idx > 0 {
		parts := strings.Fields(summary[:idx])
		if len(parts) > 0 {
			if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				ds.Files = n
			}
		}
	}

	// Parse "X insertion(s)(+)"
	if idx := strings.Index(summary, " insertion"); idx > 0 {
		before := summary[:idx]
		parts := strings.Fields(before)
		if len(parts) > 0 {
			if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				ds.Added = n
				linesAdded = n
			}
		}
	}

	// Parse "X deletion(s)(-)"
	if idx := strings.Index(summary, " deletion"); idx > 0 {
		before := summary[:idx]
		parts := strings.Fields(before)
		if len(parts) > 0 {
			if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				ds.Removed = n
				linesRemoved = n
			}
		}
	}

	// Count file entries from individual lines (all non-summary lines).
	// Note: git diff --stat cannot reliably distinguish new files from modified
	// files, so all entries are classified as modified.
	for _, line := range lines[:len(lines)-1] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "|") {
			modified++
		}
	}

	return ds, added, modified, linesAdded, linesRemoved
}
