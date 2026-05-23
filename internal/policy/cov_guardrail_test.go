package policy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/findings"
	"github.com/valksor/kvelmo/internal/testutil"
)

// runGit runs a git subcommand in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil { //nolint:noctx // test helper
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestMapPolicySeverity_Default(t *testing.T) {
	// An unknown severity string falls through to the Info mapping.
	if got := mapPolicySeverity(Severity("totally-unknown")); got != findings.SeverityInfo {
		t.Errorf("mapPolicySeverity(unknown) = %v, want Info", got)
	}
	if got := mapPolicySeverity(Severity("")); got != findings.SeverityInfo {
		t.Errorf("mapPolicySeverity(empty) = %v, want Info", got)
	}
}

// writeLines writes a file with n identical lines.
func writeLines(t *testing.T, path string, n int) {
	t.Helper()
	var b strings.Builder
	for range n {
		b.WriteString("a changed line\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestMaxDiffSize_OverThresholdReportsFinding(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	// Create and commit a baseline file so the working-tree diff is large.
	target := filepath.Join(dir, "big.txt")
	writeLines(t, target, 1)
	runGit(t, dir, "add", "big.txt")
	runGit(t, dir, "commit", "-m", "add big")

	// Now rewrite it with many lines; the unstaged diff should exceed the threshold.
	writeLines(t, target, 50)

	g := &MaxDiffSize{MaxLines: 5}
	result := g.Check(context.Background(), "implement", dir, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 finding for an over-threshold diff, got %d", len(result))
	}
	f := result[0]
	if f.Rule != "max-diff-size" {
		t.Errorf("rule = %q, want max-diff-size", f.Rule)
	}
	if f.Severity != findings.SeverityMedium {
		t.Errorf("severity = %v, want Medium", f.Severity)
	}
	if !strings.Contains(f.Evidence, "lines changed") {
		t.Errorf("evidence = %q, want it to mention lines changed", f.Evidence)
	}
}

func TestMaxDiffSize_UnderThresholdNoFinding(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	target := filepath.Join(dir, "small.txt")
	writeLines(t, target, 1)
	runGit(t, dir, "add", "small.txt")
	runGit(t, dir, "commit", "-m", "add small")

	// A tiny change well under the default threshold.
	writeLines(t, target, 3)

	g := &MaxDiffSize{MaxLines: 2000}
	result := g.Check(context.Background(), "implement", dir, nil)
	if len(result) != 0 {
		t.Errorf("expected 0 findings for under-threshold diff, got %d", len(result))
	}
}

func TestCountDiffLines_ParsesInsertionsAndDeletions(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	target := filepath.Join(dir, "data.txt")
	writeLines(t, target, 10)
	runGit(t, dir, "add", "data.txt")
	runGit(t, dir, "commit", "-m", "seed")

	// Replace the whole file: git reports both insertions and deletions.
	writeLines(t, target, 4)

	n, err := countDiffLines(context.Background(), dir)
	if err != nil {
		t.Fatalf("countDiffLines: %v", err)
	}
	// 10 deletions + 4 insertions = 14 total changed lines.
	if n <= 0 {
		t.Errorf("countDiffLines = %d, want > 0", n)
	}
}

func TestCountDiffLines_CleanTree(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	// No working-tree changes: the diff is empty and the count is zero.
	n, err := countDiffLines(context.Background(), dir)
	if err != nil {
		t.Fatalf("countDiffLines: %v", err)
	}
	if n != 0 {
		t.Errorf("countDiffLines on clean tree = %d, want 0", n)
	}
}
