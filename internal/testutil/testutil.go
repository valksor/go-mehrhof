// Package testutil provides shared test helpers used across kvelmo test packages.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory under /tmp and registers cleanup.
// Use this instead of t.TempDir() when the path may be used as a Unix socket
// path (which has a ~108-character limit).
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "kvelmo-test-") //nolint:usetesting // intentional: /tmp keeps paths short for Unix socket limits (~108 chars)
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	return dir
}

// TempSocketPath returns a short Unix socket path suitable for tests.
func TempSocketPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(TempDir(t), "test.sock")
}

// InitGitRepo initializes a git repository in dir with an initial commit.
func InitGitRepo(t *testing.T, dir string) {
	t.Helper()
	setup := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range setup {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil { //nolint:noctx // test helper: no context available
			t.Fatalf("git setup %v: %v\n%s", args, err, out)
		}
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	commit := [][]string{
		{"git", "-C", dir, "add", "README.md"},
		{"git", "-C", dir, "commit", "-m", "initial"},
	}
	for _, args := range commit {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil { //nolint:noctx // test helper: no context available
			t.Fatalf("git commit %v: %v\n%s", args, err, out)
		}
	}
}
