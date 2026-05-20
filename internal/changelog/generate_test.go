package changelog

import (
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/git"
)

func TestGatherCommits(t *testing.T) {
	raw := []git.LogEntry{
		{SHA: "abc123def456", Message: "Add feature", Author: "Alice", Date: "2026-05-01"},
		{SHA: "fed987cba654", Message: "Fix bug", Author: "Bob", Date: "2026-05-02"},
	}

	got := GatherCommits(raw)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Message != "Add feature" {
		t.Errorf("got[0].Message = %q", got[0].Message)
	}
	if got[0].Body != "" {
		t.Errorf("GatherCommits should not populate Body, got %q", got[0].Body)
	}
}

func TestGatherCommits_Empty(t *testing.T) {
	if got := GatherCommits(nil); len(got) != 0 {
		t.Errorf("nil input should give empty slice, got %d", len(got))
	}
}

func TestGatherCommitsFull(t *testing.T) {
	raw := []git.LogEntryFull{
		{LogEntry: git.LogEntry{SHA: "abc123", Message: "Add feature", Author: "Alice", Date: "2026-05-01"}, Body: "Detailed description.\nMultiple lines."},
	}

	got := GatherCommitsFull(raw)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Body == "" {
		t.Error("GatherCommitsFull must preserve Body")
	}
}

func TestGeneratePrompt_BasicStructure(t *testing.T) {
	commits := []CommitInfo{
		{SHA: "abcdef1234", Message: "Add login", Author: "Alice", Date: "2026-05-01"},
	}
	out := GeneratePrompt(commits, "diff content", "diff stat", "")

	expected := []string{
		"Keep a Changelog",
		"### Added",
		"### Changed",
		"### Fixed",
		"### Removed",
		"## Commits",
		"## Diff",
		"abcdef12 Add login (Alice, 2026-05-01)",
		"```diff",
		"diff content",
	}
	for _, want := range expected {
		if !strings.Contains(out, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestGeneratePrompt_WithNote(t *testing.T) {
	out := GeneratePrompt(nil, "", "", "Release v1.2.3")
	if !strings.Contains(out, "Additional context: Release v1.2.3") {
		t.Error("note not embedded")
	}
}

func TestGeneratePrompt_WithBody(t *testing.T) {
	commits := []CommitInfo{
		{SHA: "abc12345", Message: "Refactor", Author: "Alice", Date: "2026-05-01", Body: "Line 1\nLine 2"},
	}
	out := GeneratePrompt(commits, "", "", "")
	if !strings.Contains(out, "Line 1") || !strings.Contains(out, "Line 2") {
		t.Error("commit body lines should appear in prompt")
	}
}

func TestGeneratePrompt_ShortSHA(t *testing.T) {
	commits := []CommitInfo{
		{SHA: "abc", Message: "short SHA", Author: "Alice", Date: "2026-05-01"},
	}
	out := GeneratePrompt(commits, "", "", "")
	if !strings.Contains(out, "abc short SHA") {
		t.Error("short SHAs (<8 chars) should appear in full")
	}
}

func TestGeneratePrompt_TruncatesManyCommits(t *testing.T) {
	commits := make([]CommitInfo, maxCommits+50)
	for i := range commits {
		commits[i] = CommitInfo{SHA: "deadbeef", Message: "Commit", Author: "a", Date: "d"}
	}
	out := GeneratePrompt(commits, "", "", "")
	if !strings.Contains(out, "(showing 500 of 550 commits)") {
		t.Errorf("expected truncation notice, output:\n%s", out[:200])
	}
}

func TestGeneratePrompt_LargeDiffSummary(t *testing.T) {
	large := strings.Repeat("a", maxDiffBytes+1)
	out := GeneratePrompt(nil, large, "stat summary here", "")
	if !strings.Contains(out, "(diff too large, showing stat summary)") {
		t.Error("expected stat-summary message for large diffs")
	}
	if !strings.Contains(out, "stat summary here") {
		t.Error("expected diff stat to appear")
	}
	if strings.Contains(out, "```diff\n") {
		t.Error("large diff should not embed full diff with ```diff fence")
	}
}
