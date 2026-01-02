package directory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Name = %q, want %q", info.Name, ProviderName)
	}
	if info.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(info.Schemes) == 0 {
		t.Error("Schemes should not be empty")
	}
	if info.Schemes[0] != "dir" {
		t.Errorf("Schemes[0] = %q, want %q", info.Schemes[0], "dir")
	}
	if !info.Capabilities[provider.CapRead] {
		t.Error("should have read capability")
	}
	if !info.Capabilities[provider.CapList] {
		t.Error("should have list capability")
	}
}

func TestNew(t *testing.T) {
	ctx := context.Background()
	cfg := provider.Config{}

	p, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	prov, ok := p.(*Provider)
	if !ok {
		t.Fatal("New should return *Provider")
	}

	if prov.basePath != "." {
		t.Errorf("basePath = %q, want %q", prov.basePath, ".")
	}
}

func TestNewWithBasePath(t *testing.T) {
	ctx := context.Background()
	cfg := provider.NewConfig().Set("base_path", "/custom/path")

	p, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	prov, ok := p.(*Provider)
	if !ok {
		t.Fatal("New did not return *Provider")
	}
	if prov.basePath != "/custom/path" {
		t.Errorf("basePath = %q, want %q", prov.basePath, "/custom/path")
	}
}

func TestMatchWithPrefix(t *testing.T) {
	p := &Provider{basePath: "."}

	if !p.Match("dir:tasks") {
		t.Error("should match dir: prefix")
	}
	if !p.Match("dir:/absolute/path") {
		t.Error("should match dir: with absolute path")
	}
}

func TestMatchExplicitOnly(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	// Only explicit prefix should match
	if !p.Match("dir:subdir") {
		t.Error("should match dir: prefix")
	}

	// Bare paths should NOT match (explicit scheme required)
	if p.Match("subdir") {
		t.Error("should NOT match bare directory path (explicit scheme required)")
	}
	if p.Match("tasks/") {
		t.Error("should NOT match trailing slash without prefix (explicit scheme required)")
	}
	if p.Match("nonexistent") {
		t.Error("should NOT match bare non-existent directory")
	}
}

func TestMatchFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.md")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	// Files should not match (they match file provider)
	if p.Match("file.md") {
		t.Error("should not match file")
	}
}

func TestParse(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "tasks")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	path, err := p.Parse("tasks")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if path != subDir {
		t.Errorf("Parse = %q, want %q", path, subDir)
	}
}

func TestParseWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "tasks")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	path, err := p.Parse("dir:tasks")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if path != subDir {
		t.Errorf("Parse = %q, want %q", path, subDir)
	}
}

func TestParseWithTrailingSlash(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "tasks")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	path, err := p.Parse("tasks/")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if path != subDir {
		t.Errorf("Parse = %q, want %q", path, subDir)
	}
}

func TestParseNotFound(t *testing.T) {
	p := &Provider{basePath: t.TempDir()}

	_, err := p.Parse("nonexistent")
	if err == nil {
		t.Error("Parse should fail for non-existent directory")
	}
}

func TestParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.md")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	_, err := p.Parse("file.md")
	if err == nil {
		t.Error("Parse should fail for file")
	}
}

func TestFetch(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "my-task")
	if err := os.Mkdir(taskDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	// Create README
	readme := filepath.Join(taskDir, "README.md")
	if err := os.WriteFile(readme, []byte("# Task Title\n\nTask description."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create subtask file
	subtask := filepath.Join(taskDir, "subtask.md")
	if err := os.WriteFile(subtask, []byte("# Subtask"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskDir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.ID != "my-task" {
		t.Errorf("ID = %q, want %q", wu.ID, "my-task")
	}
	if wu.Title != "Task Title" {
		t.Errorf("Title = %q, want %q", wu.Title, "Task Title")
	}
	if wu.Provider != ProviderName {
		t.Errorf("Provider = %q, want %q", wu.Provider, ProviderName)
	}
	if len(wu.Subtasks) != 1 {
		t.Errorf("Subtasks = %d, want 1", len(wu.Subtasks))
	}
}

func TestFetchNoReadme(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "no-readme")
	if err := os.Mkdir(taskDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskDir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Should use directory name as title when no README
	if wu.Title != "no-readme" {
		t.Errorf("Title = %q, want %q", wu.Title, "no-readme")
	}
}

func TestFetchWithTaskMd(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task-dir")
	if err := os.Mkdir(taskDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	// Create task.md instead of README.md
	taskFile := filepath.Join(taskDir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Task From task.md\n\nContent."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskDir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.Title != "Task From task.md" {
		t.Errorf("Title = %q, want %q", wu.Title, "Task From task.md")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task files
	task1 := filepath.Join(tmpDir, "task1.md")
	task2 := filepath.Join(tmpDir, "task2.md")
	if err := os.WriteFile(task1, []byte("# Task 1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(task2, []byte("# Task 2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create README (should be skipped)
	readme := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readme, []byte("# README"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a subdirectory (should be skipped)
	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	units, err := p.List(ctx, provider.ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(units) != 2 {
		t.Errorf("List returned %d units, want 2", len(units))
	}
}

func TestListSkipsReadmeFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create README-like files (should be skipped)
	files := []string{"README.md", "readme.md", "task.md", "index.md"}
	for _, name := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("# "+name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	// Create a regular task file
	task := filepath.Join(tmpDir, "regular-task.md")
	if err := os.WriteFile(task, []byte("# Regular Task"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	units, err := p.List(ctx, provider.ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Only regular-task.md should be listed
	if len(units) != 1 {
		t.Errorf("List returned %d units, want 1", len(units))
	}
}

func TestListEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	units, err := p.List(ctx, provider.ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(units) != 0 {
		t.Errorf("List returned %d units, want 0", len(units))
	}
}

func TestSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "snapshot-dir")
	if err := os.Mkdir(taskDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	// Create various files
	if err := os.WriteFile(filepath.Join(taskDir, "readme.md"), []byte("# README"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "config.yaml"), []byte("key: value"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "notes.txt"), []byte("some notes"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Binary file should be skipped
	if err := os.WriteFile(filepath.Join(taskDir, "binary.exe"), []byte{0x00, 0x01}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	snap, err := p.Snapshot(ctx, taskDir)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.Type != "directory" {
		t.Errorf("Type = %q, want %q", snap.Type, "directory")
	}
	if snap.Ref != taskDir {
		t.Errorf("Ref = %q, want %q", snap.Ref, taskDir)
	}
	// Should capture .md, .yaml, .txt but not .exe
	if len(snap.Files) != 3 {
		t.Errorf("Files = %d, want 3", len(snap.Files))
	}
}

func TestSnapshotWithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "nested-dir")
	subDir := filepath.Join(taskDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(filepath.Join(taskDir, "root.md"), []byte("# Root"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.md"), []byte("# Nested"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	snap, err := p.Snapshot(ctx, taskDir)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Should capture both root and nested files
	if len(snap.Files) != 2 {
		t.Errorf("Files = %d, want 2", len(snap.Files))
	}

	// Check relative paths
	hasRoot, hasNested := false, false
	for _, f := range snap.Files {
		if f.Path == "root.md" {
			hasRoot = true
		}
		if f.Path == "sub/nested.md" {
			hasNested = true
		}
	}
	if !hasRoot {
		t.Error("missing root.md in snapshot")
	}
	if !hasNested {
		t.Error("missing sub/nested.md in snapshot")
	}
}

func TestSnapshotNotFound(t *testing.T) {
	p := &Provider{basePath: t.TempDir()}
	ctx := context.Background()

	_, err := p.Snapshot(ctx, "/nonexistent/dir")
	if err == nil {
		t.Error("Snapshot should fail for non-existent directory")
	}
}

func TestResolvePath(t *testing.T) {
	p := &Provider{basePath: "/base"}

	// Absolute path should be unchanged
	if got := p.resolvePath("/absolute/path"); got != "/absolute/path" {
		t.Errorf("resolvePath for absolute = %q, want %q", got, "/absolute/path")
	}

	// Relative path should be joined with basePath
	if got := p.resolvePath("relative"); got != "/base/relative" {
		t.Errorf("resolvePath for relative = %q, want %q", got, "/base/relative")
	}
}

func TestGenerateID(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		path string
		want string
	}{
		{"/path/to/tasks", "tasks"},
		{"simple", "simple"},
		{"/complex/path/my-project", "my-project"},
	}

	for _, tt := range tests {
		got := p.generateID(tt.path)
		if got != tt.want {
			t.Errorf("generateID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestFindReadme(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "test-readme")
	if err := os.Mkdir(taskDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	// Test when no README exists
	path, title, desc, fm := p.findReadme(taskDir)
	if path != "" || title != "" || desc != "" {
		t.Error("findReadme should return empty when no README exists")
	}
	if fm != nil {
		t.Error("findReadme should return nil frontmatter when no README exists")
	}

	// Create README.md
	readme := filepath.Join(taskDir, "README.md")
	if err := os.WriteFile(readme, []byte("# Found Title\n\nDescription here."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	path, title, desc, fm = p.findReadme(taskDir)
	if path != readme {
		t.Errorf("path = %q, want %q", path, readme)
	}
	if title != "Found Title" {
		t.Errorf("title = %q, want %q", title, "Found Title")
	}
	if desc != "Description here." {
		t.Errorf("desc = %q, want %q", desc, "Description here.")
	}
	if fm != nil {
		t.Error("findReadme should return nil frontmatter for README without YAML frontmatter")
	}
}

func TestFindSubtasks(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "test-subtasks")
	if err := os.Mkdir(taskDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	// Create files
	if err := os.WriteFile(filepath.Join(taskDir, "README.md"), []byte("# README"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte("# Task"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "subtask1.md"), []byte("# Subtask 1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "subtask2.md"), []byte("# Subtask 2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a subdirectory (should be skipped)
	if err := os.Mkdir(filepath.Join(taskDir, "subdir"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	subtasks := p.findSubtasks(taskDir)

	// Should only include subtask1.md and subtask2.md (not README.md, task.md, or subdir)
	if len(subtasks) != 2 {
		t.Errorf("findSubtasks returned %d subtasks, want 2", len(subtasks))
	}
}

func TestProviderNameConstant(t *testing.T) {
	if ProviderName != "directory" {
		t.Errorf("ProviderName = %q, want %q", ProviderName, "directory")
	}
}
