package changelog

import (
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/internal/git"
)

const (
	// maxDiffBytes is the threshold above which the full diff is replaced
	// with a stat summary to stay within reasonable token budgets.
	maxDiffBytes = 100_000

	// maxCommits caps the number of commits included in the prompt.
	maxCommits = 500
)

// CommitInfo holds the commit metadata needed for changelog generation.
type CommitInfo struct {
	SHA     string
	Message string
	Author  string
	Date    string
	Body    string // optional, included when --full is set
}

// GatherCommits converts git log entries to CommitInfo.
func GatherCommits(raw []git.LogEntry) []CommitInfo {
	commits := make([]CommitInfo, len(raw))
	for i, c := range raw {
		commits[i] = CommitInfo{
			SHA: c.SHA, Message: c.Message, Author: c.Author, Date: c.Date,
		}
	}

	return commits
}

// GatherCommitsFull converts full git log entries (with body) to CommitInfo.
func GatherCommitsFull(raw []git.LogEntryFull) []CommitInfo {
	commits := make([]CommitInfo, len(raw))
	for i, c := range raw {
		commits[i] = CommitInfo{
			SHA: c.SHA, Message: c.Message, Author: c.Author, Date: c.Date, Body: c.Body,
		}
	}

	return commits
}

// GeneratePrompt builds the AI prompt for changelog generation from git context.
func GeneratePrompt(commits []CommitInfo, diff, diffStat, note string) string {
	var b strings.Builder

	b.WriteString("Generate a changelog in Keep a Changelog format (https://keepachangelog.com/) ")
	b.WriteString("from the following git commits and diff.\n\n")
	b.WriteString("Group entries under: ### Added, ### Changed, ### Fixed, ### Removed\n")
	b.WriteString("Only include categories that have entries.\n")
	b.WriteString("Each entry should be a concise, human-readable description of the change — ")
	b.WriteString("do NOT just echo commit messages. Analyze the diff to understand what actually changed.\n")
	b.WriteString("Output ONLY the changelog markdown, no preamble or explanation.\n\n")

	if note != "" {
		fmt.Fprintf(&b, "Additional context: %s\n\n", note)
	}

	b.WriteString("## Commits\n\n")
	if len(commits) > maxCommits {
		fmt.Fprintf(&b, "(showing %d of %d commits)\n\n", maxCommits, len(commits))
		commits = commits[:maxCommits]
	}
	for _, c := range commits {
		fmt.Fprintf(&b, "- %s %s (%s, %s)\n", c.SHA[:min(8, len(c.SHA))], c.Message, c.Author, c.Date)
		if c.Body != "" {
			for line := range strings.SplitSeq(c.Body, "\n") {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}

	b.WriteString("\n## Diff\n\n")
	if len(diff) > maxDiffBytes {
		b.WriteString("(diff too large, showing stat summary)\n\n")
		fmt.Fprintf(&b, "```\n%s\n```\n", diffStat)
	} else {
		fmt.Fprintf(&b, "```diff\n%s\n```\n", diff)
	}

	return b.String()
}
