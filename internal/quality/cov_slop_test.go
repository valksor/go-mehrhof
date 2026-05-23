package quality

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGitWithFile creates a git repo at dir with one committed file, then makes
// an uncommitted modification so `git diff` reports it as changed.
func initGitWithFile(t *testing.T, dir, name, initial, modified string) {
	t.Helper()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:noctx // test setup
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil { //nolint:noctx // test setup
		t.Fatalf("git init: %v\n%s", err, out)
	}
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, name), []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}
	runGit("add", ".")
	runGit("-c", "commit.gpgsign=false", "commit", "-m", "initial")

	if err := os.WriteFile(filepath.Join(dir, name), []byte(modified), 0o644); err != nil {
		t.Fatalf("write modified: %v", err)
	}
}

func TestSlopChecker_Check_DetectsSlop(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	// The modified version introduces AI boilerplate and an ignored error.
	modified := "package main\n" +
		"\n" +
		"// Here's the function that does the work\n" +
		"func doWork() {\n" +
		"\t_ = err\n" +
		"\t// return fmt.Errorf(\"boom\")\n" +
		"}\n"
	initGitWithFile(t, dir, "code.go", "package main\n", modified)

	checker := NewSlopChecker()
	if checker.Name() != slopCheckerName {
		t.Errorf("Name() = %q, want %q", checker.Name(), slopCheckerName)
	}

	ff, err := checker.Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(ff) == 0 {
		t.Fatal("expected slop findings on a file with AI boilerplate and ignored error")
	}

	rules := map[string]bool{}
	for _, f := range ff {
		rules[f.Rule] = true
		if f.File != "code.go" {
			t.Errorf("finding File = %q, want code.go", f.File)
		}
	}
	if !rules["slop-ai-boilerplate"] {
		t.Errorf("expected slop-ai-boilerplate finding, got rules %v", rules)
	}
	if !rules["slop-ignored-error"] {
		t.Errorf("expected slop-ignored-error finding, got rules %v", rules)
	}
}

func TestSlopChecker_Check_NoGitRepo(t *testing.T) {
	// changedFiles falls back to nil for a non-git directory, so Check returns
	// no findings and no error.
	ff, err := NewSlopChecker().Check(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(ff) != 0 {
		t.Errorf("Check() on non-git dir = %v, want no findings", ff)
	}
}

func TestChangedFiles_NonGit(t *testing.T) {
	files, err := changedFiles(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("changedFiles() error = %v", err)
	}
	if files != nil {
		t.Errorf("changedFiles() on non-git dir = %v, want nil", files)
	}
}

func TestChangedFiles_GitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	initGitWithFile(t, dir, "tracked.txt", "one\n", "two\n")

	files, err := changedFiles(context.Background(), dir)
	if err != nil {
		t.Fatalf("changedFiles() error = %v", err)
	}
	found := false
	for _, f := range files {
		if f == "tracked.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("changedFiles() = %v, want it to include tracked.txt", files)
	}
}
