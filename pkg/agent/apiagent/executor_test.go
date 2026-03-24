package apiagent_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/apiagent"
)

func newTestExecutor(t *testing.T) *apiagent.ToolExecutor {
	t.Helper()

	dir := t.TempDir()

	return &apiagent.ToolExecutor{
		WorkDir:           dir,
		PermissionHandler: agent.KvelmoPermissionHandler,
	}
}

func TestExecutorReadFile(t *testing.T) {
	exec := newTestExecutor(t)

	// Create a test file
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Execute(context.Background(), "read_file", map[string]any{
		"path": "test.txt",
	})
	if err != nil {
		t.Fatalf("read_file failed: %v", err)
	}

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Should contain line numbers
	if !contains(output, "1") || !contains(output, "line one") {
		t.Errorf("output missing expected content: %s", output)
	}
}

func TestExecutorWriteFile(t *testing.T) {
	exec := newTestExecutor(t)

	output, err := exec.Execute(context.Background(), "write_file", map[string]any{
		"path":    "new.txt",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("write_file failed: %v", err)
	}

	if output == "" {
		t.Fatal("expected non-empty output")
	}

	data, err := os.ReadFile(filepath.Join(exec.WorkDir, "new.txt"))
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}

	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestExecutorWriteFileCreatesDirectories(t *testing.T) {
	exec := newTestExecutor(t)

	_, err := exec.Execute(context.Background(), "write_file", map[string]any{
		"path":    "subdir/nested/file.txt",
		"content": "deep",
	})
	if err != nil {
		t.Fatalf("write_file with nested dirs failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(exec.WorkDir, "subdir/nested/file.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "deep" {
		t.Errorf("expected 'deep', got %q", string(data))
	}
}

func TestExecutorEditFile(t *testing.T) {
	exec := newTestExecutor(t)

	// Create a file to edit
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "edit.txt"), []byte("foo bar baz"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := exec.Execute(context.Background(), "edit_file", map[string]any{
		"path":       "edit.txt",
		"old_string": "bar",
		"new_string": "qux",
	})
	if err != nil {
		t.Fatalf("edit_file failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(exec.WorkDir, "edit.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "foo qux baz" {
		t.Errorf("expected 'foo qux baz', got %q", string(data))
	}
}

func TestExecutorEditFileNotFound(t *testing.T) {
	exec := newTestExecutor(t)

	if err := os.WriteFile(filepath.Join(exec.WorkDir, "edit.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := exec.Execute(context.Background(), "edit_file", map[string]any{
		"path":       "edit.txt",
		"old_string": "nonexistent",
		"new_string": "replacement",
	})
	if err == nil {
		t.Fatal("expected error for missing old_string")
	}
}

func TestExecutorEditFileAmbiguous(t *testing.T) {
	exec := newTestExecutor(t)

	if err := os.WriteFile(filepath.Join(exec.WorkDir, "dup.txt"), []byte("aaa bbb aaa"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := exec.Execute(context.Background(), "edit_file", map[string]any{
		"path":       "dup.txt",
		"old_string": "aaa",
		"new_string": "bbb",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous old_string")
	}
}

func TestExecutorGlob(t *testing.T) {
	exec := newTestExecutor(t)

	// Create files
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(exec.WorkDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	output, err := exec.Execute(context.Background(), "glob", map[string]any{
		"pattern": "*.go",
	})
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}

	if !contains(output, "a.go") || !contains(output, "b.go") {
		t.Errorf("expected a.go and b.go in output: %s", output)
	}

	if contains(output, "c.txt") {
		t.Errorf("did not expect c.txt in output: %s", output)
	}
}

func TestExecutorListDir(t *testing.T) {
	exec := newTestExecutor(t)

	if err := os.WriteFile(filepath.Join(exec.WorkDir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(exec.WorkDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Execute(context.Background(), "list_dir", map[string]any{})
	if err != nil {
		t.Fatalf("list_dir failed: %v", err)
	}

	if !contains(output, "file.txt") {
		t.Errorf("expected file.txt in output: %s", output)
	}

	if !contains(output, "subdir/") {
		t.Errorf("expected subdir/ in output: %s", output)
	}
}

func TestExecutorBash(t *testing.T) {
	exec := newTestExecutor(t)

	output, err := exec.Execute(context.Background(), "bash", map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}

	if !contains(output, "hello") {
		t.Errorf("expected 'hello' in output: %s", output)
	}
}

func TestExecutorBashWorkDir(t *testing.T) {
	exec := newTestExecutor(t)

	output, err := exec.Execute(context.Background(), "bash", map[string]any{
		"command": "pwd",
	})
	if err != nil {
		t.Fatalf("bash pwd failed: %v", err)
	}

	if !contains(output, exec.WorkDir) {
		t.Errorf("expected workdir %s in output: %s", exec.WorkDir, output)
	}
}

func TestExecutorUnknownTool(t *testing.T) {
	exec := newTestExecutor(t)

	_, err := exec.Execute(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestExecutorReadFileWithOffsetAndLimit(t *testing.T) {
	exec := newTestExecutor(t)

	content := "a\nb\nc\nd\ne\n"
	if err := os.WriteFile(filepath.Join(exec.WorkDir, "lines.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Execute(context.Background(), "read_file", map[string]any{
		"path":   "lines.txt",
		"offset": float64(2),
		"limit":  float64(2),
	})
	if err != nil {
		t.Fatalf("read_file with offset/limit failed: %v", err)
	}

	if !contains(output, "b") || !contains(output, "c") {
		t.Errorf("expected lines b and c, got: %s", output)
	}

	if contains(output, "\ta\n") || contains(output, "\td\n") {
		t.Errorf("did not expect lines a or d: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
