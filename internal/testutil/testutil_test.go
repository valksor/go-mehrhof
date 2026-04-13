package testutil_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/testutil"
)

func TestTempDir_IsCreated(t *testing.T) {
	dir := testutil.TempDir(t)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("TempDir() returned path %q that does not exist: %v", dir, err)
	}
	if !info.IsDir() {
		t.Errorf("TempDir() path %q is not a directory", dir)
	}
}

func TestTempDir_UnderTmp(t *testing.T) {
	dir := testutil.TempDir(t)
	// TempDir intentionally uses /tmp (not os.TempDir) to keep paths short for Unix sockets.
	// On macOS, /tmp is a symlink to /private/tmp, so accept both.
	if !strings.HasPrefix(dir, "/tmp") && !strings.HasPrefix(dir, "/private/tmp") {
		t.Errorf("TempDir() = %q, want path under /tmp or /private/tmp", dir)
	}
}

func TestTempDir_PathLength(t *testing.T) {
	dir := testutil.TempDir(t)
	// Unix socket paths are limited to ~108 characters.
	if len(dir) >= 90 {
		t.Errorf("TempDir() path length %d >= 90, may be too long for Unix sockets", len(dir))
	}
}

func TestTempSocketPath_EndsSock(t *testing.T) {
	path := testutil.TempSocketPath(t)
	if !strings.HasSuffix(path, "test.sock") {
		t.Errorf("TempSocketPath() = %q, want suffix test.sock", path)
	}
}

func TestInitGitRepo_GitDirExists(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	info, err := os.Stat(dir + "/.git")
	if err != nil {
		t.Fatalf(".git not found after InitGitRepo(): %v", err)
	}
	if !info.IsDir() {
		t.Error(".git should be a directory")
	}
}

func TestInitGitRepo_HasInitialCommit(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	out, err := exec.Command("git", "-C", dir, "log", "--oneline").Output() //nolint:noctx // test verification: no context available
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		t.Error("expected at least one commit after InitGitRepo()")
	}
}

func TestInitGitRepo_READMEExists(t *testing.T) {
	dir := testutil.TempDir(t)
	testutil.InitGitRepo(t, dir)

	data, err := os.ReadFile(dir + "/README.md")
	if err != nil {
		t.Fatalf("README.md not found after InitGitRepo(): %v", err)
	}
	if len(data) == 0 {
		t.Error("README.md should not be empty")
	}
}
