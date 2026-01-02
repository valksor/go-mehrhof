// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CreateTempGitRepo creates an initialized git repository in a temporary directory.
// It configures user.email and user.name, and creates an initial commit.
// Returns the path to the repository root.
func CreateTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo
	if err := runGit(t, dir, "init"); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git user (required for commits)
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test User")

	// Create initial commit (many operations require at least one commit)
	WriteFile(t, filepath.Join(dir, "README.md"), "# Test Repository\n")
	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	return dir
}

// CreateTempGitRepoWithBranch creates a git repo and switches to the specified branch.
func CreateTempGitRepoWithBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := CreateTempGitRepo(t)
	mustRunGit(t, dir, "checkout", "-b", branch)

	return dir
}

// WriteFile creates a file with the given content, creating parent directories as needed.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// WriteFileAndCommit writes a file and commits it.
func WriteFileAndCommit(t *testing.T, dir, relativePath, content, message string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, relativePath), content)
	mustRunGit(t, dir, "add", relativePath)
	mustRunGit(t, dir, "commit", "-m", message)
}

// CreateTempGitRepoInDir initializes a git repository in an existing directory.
func CreateTempGitRepoInDir(t *testing.T, dir string) {
	t.Helper()
	initGitRepo(t, dir)
}

// CreateTaskFile creates a task markdown file with the specified content.
func CreateTaskFile(t *testing.T, dir, filename, title, description string) string {
	t.Helper()
	content := "---\ntitle: " + title + "\n---\n\n" + description + "\n"
	path := filepath.Join(dir, filename)
	WriteFile(t, path, content)

	return path
}

// CreateTaskDir creates a task directory with a README and optional subtask files.
func CreateTaskDir(t *testing.T, baseDir, taskName, readmeContent string, subtasks []string) string {
	t.Helper()
	taskDir := filepath.Join(baseDir, taskName)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", taskDir, err)
	}

	// Create README
	WriteFile(t, filepath.Join(taskDir, "README.md"), readmeContent)

	// Create subtask files
	for i, subtask := range subtasks {
		filename := filepath.Join(taskDir, subtask)
		content := "---\ntitle: Subtask " + string(rune('1'+i)) + "\n---\n\n" + subtask + "\n"
		WriteFile(t, filename, content)
	}

	return taskDir
}

// runGit runs a git command and returns any error.
func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v failed: %s", args, output)
	}

	return err
}

// mustRunGit runs a git command and fails the test if it errors.
func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := runGit(t, dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

// RunGit runs a git command and returns its output.
func RunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\nOutput: %s", args, err, output)
	}

	return string(output)
}

// CreateMakefile creates a Makefile with optional quality target.
func CreateMakefile(t *testing.T, dir string, hasQualityTarget bool) {
	t.Helper()
	content := ".PHONY: build test\n\nbuild:\n\t@echo building\n\ntest:\n\t@echo testing\n"
	if hasQualityTarget {
		content += "\n.PHONY: quality\nquality:\n\t@echo running quality checks\n"
	}
	WriteFile(t, filepath.Join(dir, "Makefile"), content)
}

// AssertFileExists fails the test if the file does not exist.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// AssertFileNotExists fails the test if the file exists.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to not exist: %s", path)
	}
}

// AssertFileContent fails the test if the file content doesn't match.
func AssertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Errorf("file %s:\n  got:  %q\n  want: %q", path, string(data), expected)
	}
}

// AssertFileContains fails the test if the file doesn't contain the substring.
func AssertFileContains(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if !contains(string(data), substr) {
		t.Errorf("file %s does not contain %q", path, substr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
