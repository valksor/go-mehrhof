package provision

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProvision_CopiesFiles(t *testing.T) {
	srcDir := t.TempDir()
	wtDir := t.TempDir()

	// Create source files.
	writeFile(t, filepath.Join(srcDir, ".env"), "SECRET=abc")
	writeFile(t, filepath.Join(srcDir, ".env.local"), "LOCAL=xyz")
	writeFile(t, filepath.Join(srcDir, "unrelated.txt"), "nope")

	opts := Options{
		CopyPatterns: []string{".env", ".env.*"},
	}

	result, err := Provision(context.Background(), srcDir, wtDir, opts)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if len(result.FilesCopied) != 2 {
		t.Fatalf("expected 2 files copied, got %d: %v", len(result.FilesCopied), result.FilesCopied)
	}

	// Verify content was copied.
	got, err := os.ReadFile(filepath.Join(wtDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "SECRET=abc" {
		t.Errorf("copied .env content = %q, want %q", string(got), "SECRET=abc")
	}

	// Verify unrelated file was not copied.
	if _, err := os.Stat(filepath.Join(wtDir, "unrelated.txt")); err == nil {
		t.Error("unrelated.txt should not be copied")
	}
}

func TestProvision_CreatesSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	wtDir := t.TempDir()

	// Create source directory to symlink.
	nmDir := filepath.Join(srcDir, "node_modules")
	if err := os.Mkdir(nmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nmDir, "marker.txt"), "exists")

	opts := Options{
		SymlinkPatterns: []string{"node_modules", "nonexistent_dir"},
	}

	result, err := Provision(context.Background(), srcDir, wtDir, opts)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if len(result.SymlinksCreated) != 1 {
		t.Fatalf("expected 1 symlink, got %d: %v", len(result.SymlinksCreated), result.SymlinksCreated)
	}

	if result.SymlinksCreated[0] != "node_modules" {
		t.Errorf("symlink name = %q, want %q", result.SymlinksCreated[0], "node_modules")
	}

	// Verify symlink target is readable.
	link := filepath.Join(wtDir, "node_modules")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file/dir")
	}

	// Verify content through symlink.
	data, err := os.ReadFile(filepath.Join(link, "marker.txt"))
	if err != nil {
		t.Fatalf("ReadFile through symlink: %v", err)
	}
	if string(data) != "exists" {
		t.Errorf("content through symlink = %q, want %q", string(data), "exists")
	}
}

func TestProvision_RunsSetupCommands(t *testing.T) {
	srcDir := t.TempDir()
	wtDir := t.TempDir()

	opts := Options{
		SetupCommands: []string{"echo hello > setup_marker.txt"},
	}

	result, err := Provision(context.Background(), srcDir, wtDir, opts)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if len(result.CommandsRun) != 1 {
		t.Fatalf("expected 1 command run, got %d", len(result.CommandsRun))
	}

	data, err := os.ReadFile(filepath.Join(wtDir, "setup_marker.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(data); got != "hello\n" {
		t.Errorf("setup command output = %q, want %q", got, "hello\n")
	}
}

func TestProvision_FailingCommandReturnsError(t *testing.T) {
	srcDir := t.TempDir()
	wtDir := t.TempDir()

	opts := Options{
		SetupCommands: []string{"exit 1"},
	}

	_, err := Provision(context.Background(), srcDir, wtDir, opts)
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestProvision_EmptyDirsReturnsError(t *testing.T) {
	_, err := Provision(context.Background(), "", "/tmp/wt", Options{})
	if err == nil {
		t.Error("expected error for empty srcDir")
	}

	_, err = Provision(context.Background(), "/tmp/src", "", Options{})
	if err == nil {
		t.Error("expected error for empty worktreeDir")
	}
}

func TestProvision_SkipsExistingSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	wtDir := t.TempDir()

	// Create source directory.
	if err := os.Mkdir(filepath.Join(srcDir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-create symlink in worktree.
	existingTarget := t.TempDir()
	if err := os.Symlink(existingTarget, filepath.Join(wtDir, "node_modules")); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		SymlinkPatterns: []string{"node_modules"},
	}

	result, err := Provision(context.Background(), srcDir, wtDir, opts)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	// Should not have created a new symlink since one already exists.
	if len(result.SymlinksCreated) != 0 {
		t.Errorf("expected 0 symlinks, got %d", len(result.SymlinksCreated))
	}
}

func TestProvision_ResultEmpty(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   bool
	}{
		{"empty", Result{}, true},
		{"has files", Result{FilesCopied: []string{".env"}}, false},
		{"has symlinks", Result{SymlinksCreated: []string{"nm"}}, false},
		{"has commands", Result{CommandsRun: []string{"echo hi"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Empty(); got != tt.want {
				t.Errorf("Empty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPreview(t *testing.T) {
	srcDir := t.TempDir()

	writeFile(t, filepath.Join(srcDir, ".env"), "X=1")
	if err := os.Mkdir(filepath.Join(srcDir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		CopyPatterns:    []string{".env"},
		SymlinkPatterns: []string{"node_modules", "missing"},
		SetupCommands:   []string{"echo test"},
	}

	result, err := Preview(srcDir, opts)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	if len(result.FilesCopied) != 1 || result.FilesCopied[0] != ".env" {
		t.Errorf("FilesCopied = %v, want [.env]", result.FilesCopied)
	}

	if len(result.SymlinksCreated) != 1 || result.SymlinksCreated[0] != "node_modules" {
		t.Errorf("SymlinksCreated = %v, want [node_modules]", result.SymlinksCreated)
	}

	if len(result.CommandsRun) != 1 {
		t.Errorf("CommandsRun = %v, want [echo test]", result.CommandsRun)
	}
}

func TestDefaultOptions_GoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	opts := DefaultOptions(dir)

	assertContains(t, opts.CopyPatterns, "go.sum")
	assertContains(t, opts.CopyPatterns, ".env")
	assertContains(t, opts.CopyPatterns, ".tool-versions")

	// Should not have node or python patterns.
	assertNotContains(t, opts.SymlinkPatterns, "node_modules")
	assertNotContains(t, opts.SymlinkPatterns, ".venv")
}

func TestDefaultOptions_NodeProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")

	opts := DefaultOptions(dir)

	assertContains(t, opts.CopyPatterns, ".nvmrc")
	assertContains(t, opts.CopyPatterns, ".node-version")
	assertContains(t, opts.SymlinkPatterns, "node_modules")
}

func TestDefaultOptions_PythonProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]")

	opts := DefaultOptions(dir)

	assertContains(t, opts.CopyPatterns, ".python-version")
	assertContains(t, opts.SymlinkPatterns, ".venv")
	assertContains(t, opts.SymlinkPatterns, "venv")
}

func TestDefaultOptions_MultiEcosystem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "package.json"), "{}")

	opts := DefaultOptions(dir)

	assertContains(t, opts.CopyPatterns, "go.sum")
	assertContains(t, opts.SymlinkPatterns, "node_modules")
}

func TestMergeOptions(t *testing.T) {
	base := Options{
		CopyPatterns:    []string{".env", ".tool-versions"},
		SymlinkPatterns: []string{"node_modules"},
	}
	override := Options{
		CopyPatterns:    []string{".env", "custom.conf"},
		SymlinkPatterns: []string{"vendor"},
		SetupCommands:   []string{"make setup"},
	}

	merged := MergeOptions(base, override)

	// Should have deduplicated .env.
	assertContains(t, merged.CopyPatterns, ".env")
	assertContains(t, merged.CopyPatterns, ".tool-versions")
	assertContains(t, merged.CopyPatterns, "custom.conf")

	// Count .env occurrences.
	count := 0
	for _, p := range merged.CopyPatterns {
		if p == ".env" {
			count++
		}
	}
	if count != 1 {
		t.Errorf(".env appears %d times, want 1", count)
	}

	assertContains(t, merged.SymlinkPatterns, "node_modules")
	assertContains(t, merged.SymlinkPatterns, "vendor")

	if len(merged.SetupCommands) != 1 || merged.SetupCommands[0] != "make setup" {
		t.Errorf("SetupCommands = %v, want [make setup]", merged.SetupCommands)
	}
}

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, items []string, want string) {
	t.Helper()

	for _, item := range items {
		if item == want {
			return
		}
	}

	t.Errorf("slice %v does not contain %q", items, want)
}

func assertNotContains(t *testing.T, items []string, notWant string) {
	t.Helper()

	for _, item := range items {
		if item == notWant {
			t.Errorf("slice %v should not contain %q", items, notWant)

			return
		}
	}
}
