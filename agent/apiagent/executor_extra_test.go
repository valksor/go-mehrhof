package apiagent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/apiagent"
)

func TestExecutorNoWorkDir(t *testing.T) {
	exec := &apiagent.ToolExecutor{
		WorkDir:           "",
		PermissionHandler: agent.KvelmoPermissionHandler,
	}

	_, err := exec.Execute(context.Background(), "read_file", map[string]any{"path": "x"})
	if err == nil {
		t.Fatal("expected error when WorkDir is empty")
	}
	if !strings.Contains(err.Error(), "WorkDir") {
		t.Errorf("expected WorkDir error, got %v", err)
	}
}

func TestExecutorDangerousOperationDenied(t *testing.T) {
	exec := newTestExecutor(t)

	// rm -rf / is a dangerous bash operation; it must be denied before execution.
	_, err := exec.Execute(context.Background(), "bash", map[string]any{
		"command": "rm -rf /",
	})
	if err == nil {
		t.Fatal("expected dangerous operation to be denied")
	}
	if !strings.Contains(err.Error(), "dangerous") {
		t.Errorf("expected 'dangerous' in error, got %v", err)
	}
}

func TestExecutorPermissionDenied(t *testing.T) {
	// A handler that always denies, combined with a tool that is not auto-approved
	// (Bash is not in the global safe list under EvaluatePermission), should be denied.
	exec := &apiagent.ToolExecutor{
		WorkDir:           t.TempDir(),
		PermissionHandler: func(_ agent.PermissionRequest) bool { return false },
	}

	_, err := exec.Execute(context.Background(), "bash", map[string]any{
		"command": "echo hi",
	})
	if err == nil {
		t.Fatal("expected permission denial error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected 'permission denied' in error, got %v", err)
	}
}

func TestExecutorBashRequiresCommand(t *testing.T) {
	exec := newTestExecutor(t)

	_, err := exec.Execute(context.Background(), "bash", map[string]any{})
	if err == nil {
		t.Fatal("expected error when bash command missing")
	}
}

func TestExecutorBashCustomTimeout(t *testing.T) {
	exec := newTestExecutor(t)

	// A 1-second timeout against a 5-second sleep should fail with a timeout.
	start := time.Now()
	_, err := exec.Execute(context.Background(), "bash", map[string]any{
		"command": "sleep 5",
		"timeout": float64(1),
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error for sleeping command")
	}
	if elapsed > 3*time.Second {
		t.Errorf("timeout was not honored, command took %v", elapsed)
	}
}

func TestExecutorBashNonZeroExit(t *testing.T) {
	exec := newTestExecutor(t)

	out, err := exec.Execute(context.Background(), "bash", map[string]any{
		"command": "echo to-stderr >&2; exit 3",
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	// Output should include the captured stderr.
	if !strings.Contains(err.Error(), "to-stderr") {
		t.Errorf("expected stderr capture in error, got %q / %q", out, err.Error())
	}
}

func TestExecutorReadFileMissing(t *testing.T) {
	exec := newTestExecutor(t)

	_, err := exec.Execute(context.Background(), "read_file", map[string]any{
		"path": "does-not-exist.txt",
	})
	if err == nil {
		t.Fatal("expected error reading missing file")
	}
}

func TestExecutorReadFilePathRequired(t *testing.T) {
	exec := newTestExecutor(t)

	_, err := exec.Execute(context.Background(), "read_file", map[string]any{})
	if err == nil {
		t.Fatal("expected error when path missing")
	}
}

func TestExecutorReadFileOffsetExceedsLength(t *testing.T) {
	exec := newTestExecutor(t)
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "short.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := exec.Execute(context.Background(), "read_file", map[string]any{
		"path":   "short.txt",
		"offset": float64(100),
	})
	if err == nil {
		t.Fatal("expected error when offset exceeds file length")
	}
}

func TestExecutorWriteFilePathRequired(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "write_file", map[string]any{"content": "x"})
	if err == nil {
		t.Fatal("expected error when path missing")
	}
}

func TestExecutorEditFilePathRequired(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "edit_file", map[string]any{
		"old_string": "a", "new_string": "b",
	})
	if err == nil {
		t.Fatal("expected error when path missing")
	}
}

func TestExecutorEditFileOldStringRequired(t *testing.T) {
	exec := newTestExecutor(t)
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "f.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := exec.Execute(context.Background(), "edit_file", map[string]any{
		"path": "f.txt", "new_string": "b",
	})
	if err == nil {
		t.Fatal("expected error when old_string missing")
	}
}

func TestExecutorEditFileMissingFile(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "edit_file", map[string]any{
		"path": "absent.txt", "old_string": "a", "new_string": "b",
	})
	if err == nil {
		t.Fatal("expected error editing missing file")
	}
}

func TestExecutorGlobPatternRequired(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "glob", map[string]any{})
	if err == nil {
		t.Fatal("expected error when glob pattern missing")
	}
}

func TestExecutorGlobNoMatches(t *testing.T) {
	exec := newTestExecutor(t)
	out, err := exec.Execute(context.Background(), "glob", map[string]any{
		"pattern": "*.nonexistent",
	})
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if out != "No matches found" {
		t.Errorf("expected 'No matches found', got %q", out)
	}
}

func TestExecutorGlobRecursive(t *testing.T) {
	exec := newTestExecutor(t)

	// Build a tree: top.go, sub/inner.go, sub/skip.txt, .hidden/secret.go
	mustWrite(t, filepath.Join(exec.WorkDir, "top.go"), "x")
	mustWrite(t, filepath.Join(exec.WorkDir, "sub", "inner.go"), "x")
	mustWrite(t, filepath.Join(exec.WorkDir, "sub", "skip.txt"), "x")
	mustWrite(t, filepath.Join(exec.WorkDir, ".hidden", "secret.go"), "x")

	out, err := exec.Execute(context.Background(), "glob", map[string]any{
		"pattern": "**/*.go",
	})
	if err != nil {
		t.Fatalf("recursive glob failed: %v", err)
	}

	if !contains(out, "top.go") {
		t.Errorf("expected top.go in output: %s", out)
	}
	if !contains(out, filepath.Join("sub", "inner.go")) {
		t.Errorf("expected sub/inner.go in output: %s", out)
	}
	if contains(out, "skip.txt") {
		t.Errorf("did not expect skip.txt: %s", out)
	}
	// Hidden directories must be skipped.
	if contains(out, "secret.go") {
		t.Errorf("hidden dir should be skipped: %s", out)
	}
}

func TestExecutorGlobRecursiveBareDoubleStar(t *testing.T) {
	exec := newTestExecutor(t)
	mustWrite(t, filepath.Join(exec.WorkDir, "a.txt"), "x")
	mustWrite(t, filepath.Join(exec.WorkDir, "dir", "b.txt"), "x")

	// Bare ** (empty suffix) should match every file.
	out, err := exec.Execute(context.Background(), "glob", map[string]any{
		"pattern": "**",
	})
	if err != nil {
		t.Fatalf("glob ** failed: %v", err)
	}
	if !contains(out, "a.txt") || !contains(out, filepath.Join("dir", "b.txt")) {
		t.Errorf("expected all files matched by **: %s", out)
	}
}

func TestExecutorGlobWithExplicitPath(t *testing.T) {
	exec := newTestExecutor(t)
	mustWrite(t, filepath.Join(exec.WorkDir, "sub", "x.go"), "x")

	out, err := exec.Execute(context.Background(), "glob", map[string]any{
		"pattern": "*.go",
		"path":    "sub",
	})
	if err != nil {
		t.Fatalf("glob with path failed: %v", err)
	}
	if !contains(out, "x.go") {
		t.Errorf("expected x.go in output: %s", out)
	}
}

func TestExecutorGlobPathEscape(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "glob", map[string]any{
		"pattern": "*.go",
		"path":    "../../etc",
	})
	if err == nil {
		t.Fatal("expected path-escape error")
	}
}

func TestExecutorGrepPatternRequired(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "grep", map[string]any{})
	if err == nil {
		t.Fatal("expected error when grep pattern missing")
	}
}

func TestExecutorGrepFindsMatch(t *testing.T) {
	exec := newTestExecutor(t)
	mustWrite(t, filepath.Join(exec.WorkDir, "code.go"), "package main\nfunc Target() {}\n")
	mustWrite(t, filepath.Join(exec.WorkDir, "other.txt"), "nothing here\n")

	out, err := exec.Execute(context.Background(), "grep", map[string]any{
		"pattern": "Target",
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}
	if !contains(out, "Target") {
		t.Errorf("expected match for Target, got %q", out)
	}
}

func TestExecutorGrepNoMatches(t *testing.T) {
	exec := newTestExecutor(t)
	mustWrite(t, filepath.Join(exec.WorkDir, "code.go"), "package main\n")

	out, err := exec.Execute(context.Background(), "grep", map[string]any{
		"pattern": "ZZZ_no_such_token",
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}
	if out != "No matches found" {
		t.Errorf("expected 'No matches found', got %q", out)
	}
}

func TestExecutorGrepWithGlobFilter(t *testing.T) {
	exec := newTestExecutor(t)
	mustWrite(t, filepath.Join(exec.WorkDir, "code.go"), "needle in go file\n")
	mustWrite(t, filepath.Join(exec.WorkDir, "code.txt"), "needle in txt file\n")

	out, err := exec.Execute(context.Background(), "grep", map[string]any{
		"pattern": "needle",
		"glob":    "*.go",
	})
	if err != nil {
		t.Fatalf("grep with glob failed: %v", err)
	}
	if !contains(out, "code.go") {
		t.Errorf("expected code.go match: %s", out)
	}
	if contains(out, "code.txt") {
		t.Errorf("glob filter should exclude code.txt: %s", out)
	}
}

func TestExecutorGrepPathEscape(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "grep", map[string]any{
		"pattern": "x",
		"path":    "../../etc",
	})
	if err == nil {
		t.Fatal("expected path-escape error")
	}
}

func TestExecutorListDirMissing(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "list_dir", map[string]any{
		"path": "no-such-dir",
	})
	if err == nil {
		t.Fatal("expected error listing nonexistent dir")
	}
}

func TestExecutorListDirPathEscape(t *testing.T) {
	exec := newTestExecutor(t)
	_, err := exec.Execute(context.Background(), "list_dir", map[string]any{
		"path": "../../etc",
	})
	if err == nil {
		t.Fatal("expected path-escape error")
	}
}

func TestExecutorResolvePathEscape(t *testing.T) {
	exec := newTestExecutor(t)

	// Read with a traversal path should be rejected by resolvePath.
	_, err := exec.Execute(context.Background(), "read_file", map[string]any{
		"path": "../outside.txt",
	})
	if err == nil {
		t.Fatal("expected path-escape rejection")
	}
	if !contains(err.Error(), "escapes") {
		t.Errorf("expected escape error, got %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
