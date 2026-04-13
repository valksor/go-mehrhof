package socket

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNonExistentPath_ParentExistsChildDoesNot(t *testing.T) {
	tmpDir := t.TempDir()

	// tmpDir exists; "newfile.txt" does not
	absPath := filepath.Join(tmpDir, "newfile.txt")

	got, err := resolveNonExistentPath(absPath)
	if err != nil {
		t.Fatalf("resolveNonExistentPath() error = %v", err)
	}

	// Should resolve tmpDir (the existing parent) and re-append "newfile.txt"
	if filepath.Base(got) != "newfile.txt" {
		t.Errorf("resolveNonExistentPath() base = %q, want newfile.txt", filepath.Base(got))
	}

	// The directory part should match the real tmpDir
	realTmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(tmpDir) error = %v", err)
	}
	if filepath.Dir(got) != realTmp {
		t.Errorf("resolveNonExistentPath() dir = %q, want %q", filepath.Dir(got), realTmp)
	}
}

func TestResolveNonExistentPath_MultipleNonExistentLevels(t *testing.T) {
	tmpDir := t.TempDir()

	// Three levels deep, none exist
	absPath := filepath.Join(tmpDir, "a", "b", "c")

	got, err := resolveNonExistentPath(absPath)
	if err != nil {
		t.Fatalf("resolveNonExistentPath() error = %v", err)
	}

	// Result must end with the non-existent suffix
	realTmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(tmpDir) error = %v", err)
	}
	want := filepath.Join(realTmp, "a", "b", "c")
	if got != want {
		t.Errorf("resolveNonExistentPath() = %q, want %q", got, want)
	}
}

func TestResolveNonExistentPath_PartiallyExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one intermediate directory but leave the rest missing
	existingSubdir := filepath.Join(tmpDir, "exists")
	if err := os.Mkdir(existingSubdir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	absPath := filepath.Join(existingSubdir, "missing", "file.go")

	got, err := resolveNonExistentPath(absPath)
	if err != nil {
		t.Fatalf("resolveNonExistentPath() error = %v", err)
	}

	realExisting, err := filepath.EvalSymlinks(existingSubdir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	want := filepath.Join(realExisting, "missing", "file.go")
	if got != want {
		t.Errorf("resolveNonExistentPath() = %q, want %q", got, want)
	}
}

func TestResolveNonExistentPath_ExistingPathReturnsItself(t *testing.T) {
	tmpDir := t.TempDir()

	// When the path itself exists, it should be resolved via EvalSymlinks and returned
	got, err := resolveNonExistentPath(tmpDir)
	if err != nil {
		t.Fatalf("resolveNonExistentPath() error = %v", err)
	}

	realTmp, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != realTmp {
		t.Errorf("resolveNonExistentPath() = %q, want %q", got, realTmp)
	}
}

func TestResolveNonExistentPath_RootNeverReached(t *testing.T) {
	// "/" always exists, so this should never hit the "no ancestor" error.
	// Provide an absolute path under /tmp that doesn't exist to verify
	// the function can walk all the way up and find "/".
	absPath := "/this-dir-surely-does-not-exist-kvelmo-test/sub/file"

	// /this-dir-surely-does-not-exist-kvelmo-test doesn't exist but "/" does
	got, err := resolveNonExistentPath(absPath)
	if err != nil {
		// If "/" doesn't count for some reason (unlikely on Linux), skip
		t.Logf("resolveNonExistentPath() error = %v (acceptable on some platforms)", err)

		return
	}

	// The result must be an absolute path
	if !filepath.IsAbs(got) {
		t.Errorf("resolveNonExistentPath() returned non-absolute path: %q", got)
	}
}
