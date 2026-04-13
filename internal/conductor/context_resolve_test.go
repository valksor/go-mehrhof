package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/varpool"
)

// writeTestFile creates a file inside dir for testing.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestContainedPath_Valid(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	// Create a test file
	writeTestFile(t, dir, "src/main.go", "package main")

	path, err := r.containedPath("src/main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks on expected path to match containedPath behavior
	// (macOS /var → /private/var).
	wantDir, _ := filepath.EvalSymlinks(dir)
	wantPath := filepath.Join(wantDir, "src/main.go")
	if path != wantPath {
		t.Errorf("expected %q, got %q", wantPath, path)
	}
}

func TestContainedPath_AbsoluteRejected(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: "/tmp/worktree"}

	_, err := r.containedPath("/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
	if got := err.Error(); got != `absolute paths not allowed: "/etc/passwd"` {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestContainedPath_TraversalRejected(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	cases := []string{
		"../../../etc/passwd",
		"src/../../..",
		"../.ssh/id_rsa",
	}
	for _, ref := range cases {
		_, err := r.containedPath(ref)
		if err == nil {
			t.Errorf("expected error for traversal path %q", ref)
		}
	}
}

func TestResolveFile_FullFile(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	writeTestFile(t, dir, "main.go", content)

	resolved, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeFile,
		Ref:  "main.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", resolved.Content, content)
	}
	if resolved.Label != "main.go" {
		t.Errorf("label = %q, want %q", resolved.Label, "main.go")
	}
}

func TestResolveFile_LineRange(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	lines := "line1\nline2\nline3\nline4\nline5\n"
	writeTestFile(t, dir, "data.txt", lines)

	resolved, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeFile,
		Ref:  "data.txt:2-4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content != "line2\nline3\nline4" {
		t.Errorf("content = %q, want %q", resolved.Content, "line2\nline3\nline4")
	}
	if resolved.Label != "data.txt (lines 2-4)" {
		t.Errorf("label = %q, want %q", resolved.Label, "data.txt (lines 2-4)")
	}
}

func TestResolveFile_SingleLine(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	lines := "line1\nline2\nline3\n"
	writeTestFile(t, dir, "data.txt", lines)

	resolved, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeFile,
		Ref:  "data.txt:2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content != "line2" {
		t.Errorf("content = %q, want %q", resolved.Content, "line2")
	}
}

func TestResolveFile_Missing(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	_, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeFile,
		Ref:  "nonexistent.go",
	})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	_, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeFile,
		Ref:  "../../../etc/passwd",
	})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestResolveFile_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	_, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeFile,
		Ref:  "/home/user/.ssh/id_rsa",
	})
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestResolveTerminal(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: t.TempDir()}

	resolved, err := r.Resolve(context.Background(), ContextItem{
		Type:  ContextTypeTerminal,
		Ref:   "Error: connection refused\nat line 42",
		Label: "Build Output",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Label != "Build Output" {
		t.Errorf("label = %q, want %q", resolved.Label, "Build Output")
	}
	if resolved.Content != "Error: connection refused\nat line 42" {
		t.Error("content mismatch")
	}
}

func TestResolveTerminal_Empty(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: t.TempDir()}

	_, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeTerminal,
		Ref:  "",
	})
	if err == nil {
		t.Fatal("expected error for empty terminal output")
	}
}

func TestResolveURL_PassThrough(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: t.TempDir()}

	resolved, err := r.Resolve(context.Background(), ContextItem{
		Type:  ContextTypeURL,
		Ref:   "https://react.dev/reference",
		Label: "React Docs",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content != "https://react.dev/reference" {
		t.Error("URL should pass through as content")
	}
	if resolved.Label != "React Docs" {
		t.Errorf("label = %q", resolved.Label)
	}
}

func TestResolveSymbol_GrepFallback(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	// Create a file with a symbol
	writeTestFile(t, dir, "main.go", "func HandleRequest() {\n\treturn nil\n}\n")

	resolved, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeSymbol,
		Ref:  "HandleRequest",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content == "" {
		t.Fatal("expected grep to find the symbol")
	}
}

func TestResolveSymbol_NotFound(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	// Create a file without the target symbol
	writeTestFile(t, dir, "main.go", "func Other() {}\n")

	_, err := r.Resolve(context.Background(), ContextItem{
		Type: ContextTypeSymbol,
		Ref:  "NonExistentSymbol12345",
	})
	if err == nil {
		t.Fatal("expected error for missing symbol")
	}
}

func TestResolveAll_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	r := &ContextResolver{WorktreeRoot: dir}

	writeTestFile(t, dir, "exists.go", "package main")

	items := []ContextItem{
		{Type: ContextTypeFile, Ref: "exists.go"},
		{Type: ContextTypeFile, Ref: "missing.go"},
		{Type: ContextTypeTerminal, Ref: "some output"},
	}

	results, errs := r.ResolveAll(context.Background(), items)
	if len(results) != 2 {
		t.Errorf("expected 2 resolved items, got %d", len(results))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestResolveAll_Empty(t *testing.T) {
	r := &ContextResolver{WorktreeRoot: t.TempDir()}

	results, errs := r.ResolveAll(context.Background(), nil)
	if len(results) != 0 || len(errs) != 0 {
		t.Error("expected empty results for nil items")
	}
}

func TestParseFileRef(t *testing.T) { //nolint:paralleltest // table-driven subtests not needed
	tests := []struct {
		ref      string
		wantPath string
		wantLine string
	}{
		{"main.go", "main.go", ""},
		{"main.go:42", "main.go", "42"},
		{"main.go:40-60", "main.go", "40-60"},
		{"src/pkg/file.go:10", "src/pkg/file.go", "10"},
		{"no_line_spec:abc", "no_line_spec:abc", ""}, // not a valid line spec
	}

	for _, tt := range tests {
		path, line := ParseFileRef(tt.ref)
		if path != tt.wantPath || line != tt.wantLine {
			t.Errorf("parseFileRef(%q) = (%q, %q), want (%q, %q)",
				tt.ref, path, line, tt.wantPath, tt.wantLine)
		}
	}
}

func TestParseLineSpec(t *testing.T) {
	tests := []struct {
		spec  string
		total int
		start int
		end   int
		err   bool
	}{
		{"42", 100, 42, 42, false},
		{"10-20", 100, 10, 20, false},
		{"1-5", 3, 1, 3, false},    // end clamped to totalLines
		{"50-10", 100, 0, 0, true}, // start > end
		{"abc", 100, 0, 0, true},
	}

	for _, tt := range tests {
		start, end, err := parseLineSpec(tt.spec, tt.total)
		if (err != nil) != tt.err {
			t.Errorf("parseLineSpec(%q, %d) error = %v, want error = %v", tt.spec, tt.total, err, tt.err)

			continue
		}
		if err == nil && (start != tt.start || end != tt.end) {
			t.Errorf("parseLineSpec(%q, %d) = (%d, %d), want (%d, %d)",
				tt.spec, tt.total, start, end, tt.start, tt.end)
		}
	}
}

func TestBuildPhaseContext_WithContextItems(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "handler.go", "func Handle() { return nil }")

	wu := &WorkUnit{
		Title:       "Fix handler",
		Description: "The handler returns nil incorrectly",
		ContextItems: []ContextItem{
			{Type: ContextTypeFile, Ref: "handler.go", Label: "Handler file"},
			{Type: ContextTypeTerminal, Ref: "panic: nil pointer", Label: "Error output"},
		},
	}
	pool := varpool.New()

	profile := PhaseContextProfile{
		IncludeTask:         true,
		IncludeContextItems: true,
		MaxTokenBudget:      0, // unlimited
	}

	resolver := &ContextResolver{WorktreeRoot: dir}
	deps := contextDeps{Resolver: resolver}

	result, metrics := BuildPhaseContext(context.Background(), profile, wu, pool, deps)

	if result == "" {
		t.Fatal("expected non-empty context")
	}

	// Should include task + 2 context items = 3 sections
	if len(metrics.SectionsIncluded) != 3 {
		t.Errorf("expected 3 sections, got %d: %v", len(metrics.SectionsIncluded), metrics.SectionsIncluded)
	}
}
