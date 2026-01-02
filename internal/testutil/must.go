// Package testutil provides testing utilities, including must-style helpers.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/go-mehrhof/internal/storage"
)

// T panics if err is non-nil, returning v. Useful for test setup.
// Usage: f := must.T(os.ReadFile("file.txt")).
func T[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Eq panics if got != want. Useful for assertions in test setup.
func Eq[T comparable](got, want T) {
	if got != want {
		panic(fmt.Sprintf("got %v, want %v", got, want))
	}
}

// EqFatal calls t.Fatal if got != want.
func EqFatal[T comparable](t *testing.T, got, want T, msgAndArgs ...interface{}) {
	t.Helper()
	if got != want {
		t.Fatal(append([]interface{}{fmt.Sprintf("got %v, want %v", got, want)}, msgAndArgs...)...)
	}
}

// NoError calls t.Fatal if err is non-nil.
func NoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err != nil {
		t.Fatal(append([]interface{}{err}, msgAndArgs...)...)
	}
}

// PanicHandler catches panics and reports them as test failures.
// Usage: defer testutil.PanicHandler(t).
func PanicHandler(t *testing.T) {
	t.Helper()
	if r := recover(); r != nil {
		t.Fatalf("panic recovered: %v", r)
	}
}

// TempDir creates a temporary directory for testing.
// Returns the path and a cleanup function.
func TempDir(t *testing.T) (string, func()) {
	t.Helper()

	path, err := os.MkdirTemp("", "mehr-test-*")
	if err != nil {
		t.Fatal(err)
	}

	return path, func() {
		_ = os.RemoveAll(path)
	}
}

// TempFile creates a temporary file with the given content.
// Returns the file path and a cleanup function.
func TempFile(t *testing.T, content string) (string, func()) {
	t.Helper()

	tmpDir, cleanup := TempDir(t)
	path := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return path, cleanup
}

// CreateTempWorkspace creates a temporary workspace with .mehrhof directory.
// Returns the workspace path, workspace instance, and cleanup function.
func CreateTempWorkspace(t *testing.T) (string, *storage.Workspace, func()) {
	t.Helper()

	path, cleanup := TempDir(t)

	// Create .mehrhof directory
	mehrhofDir := filepath.Join(path, ".mehrhof")
	if err := os.MkdirAll(mehrhofDir, 0o755); err != nil {
		cleanup()
		t.Fatal(err)
	}

	ws := T(storage.OpenWorkspace(path, nil))

	return path, ws, cleanup
}

// Chdir changes to a directory and returns a function to restore the original directory.
func Chdir(t *testing.T, dir string) func() {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	return func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore original directory: %v", err)
		}
	}
}

// SetEnv sets an environment variable and returns a function to restore the original value.
func SetEnv(t *testing.T, key, value string) func() {
	t.Helper()

	origValue, exists := os.LookupEnv(key)

	if err := os.Setenv(key, value); err != nil {
		t.Fatal(err)
	}

	return func() {
		if !exists {
			_ = os.Unsetenv(key)
		} else {
			if err := os.Setenv(key, origValue); err != nil {
				t.Fatalf("failed to restore env var %s: %v", key, err)
			}
		}
	}
}
