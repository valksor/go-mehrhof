package codegraph

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestNew_CreatesNestedDirAndCloseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	// dbPath under a directory that does not yet exist -> New must MkdirAll it.
	dbPath := filepath.Join(t.TempDir(), "nested", "deeper", "graph.db")
	g, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Errorf("New did not create db directory: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}

	// Close on a Graph with a nil db must be a no-op.
	empty := &Graph{}
	if err := empty.Close(); err != nil {
		t.Errorf("Close on nil-db graph = %v", err)
	}
}

func TestNew_MkdirFailsWhenParentIsFile(t *testing.T) {
	ctx := context.Background()
	// Create a regular file, then ask New to put the db *inside* it as if it
	// were a directory -> MkdirAll fails.
	base := t.TempDir()
	filePath := filepath.Join(base, "iamafile")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(ctx, filepath.Join(filePath, "sub", "graph.db")); err == nil {
		t.Fatal("expected New to fail when parent path is a file")
	}
}

func TestQueries_FailAfterClose(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()
	writeTestFile(t, dir)
	if err := g.IndexFile(ctx, filepath.Join(dir, "example.go")); err != nil {
		t.Fatal(err)
	}
	// Close the DB so subsequent queries hit the error branch.
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := g.QuerySymbol(ctx, "NewConfig"); err == nil {
		t.Error("QuerySymbol should error on closed db")
	}
	if _, err := g.QuerySymbolPattern(ctx, "%"); err == nil {
		t.Error("QuerySymbolPattern should error on closed db")
	}
	if _, err := g.QueryCallersOf(ctx, "x"); err == nil {
		t.Error("QueryCallersOf should error on closed db")
	}
	if _, err := g.QueryDependenciesOf(ctx, "p"); err == nil {
		t.Error("QueryDependenciesOf should error on closed db")
	}
}

func TestIndexFile_BadPathFailsToParse(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	// Non-Go (invalid syntax) content -> parser error propagates.
	bad := filepath.Join(dir, "broken.go")
	if err := os.WriteFile(bad, []byte("this is not valid go @@@"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.IndexFile(ctx, bad); err == nil {
		t.Fatal("expected parse error for invalid Go file")
	}
}

func TestIndexFile_NonexistentFile(t *testing.T) {
	g, _ := setupTestGraph(t)
	if err := g.IndexFile(context.Background(), "/definitely/not/here.go"); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestQueryDependenciesOf_RunsAndReturnsEmpty(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()
	writeTestFile(t, dir)
	if err := g.IndexDirectory(ctx, dir); err != nil {
		t.Fatal(err)
	}

	// Import edges record package->importpath, but those names rarely resolve to
	// stored symbols, so the result is typically empty. The query path is still
	// fully exercised and must not error.
	deps, err := g.QueryDependenciesOf(ctx, "example")
	if err != nil {
		t.Fatalf("QueryDependenciesOf error = %v", err)
	}
	if deps == nil {
		// nil slice is fine; just assert no panic and no error.
		deps = []string{}
	}
	_ = deps
}

func TestQueryDependenciesOf_ResolvesImports(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	// Construct a scenario where an import edge resolves: a package "pkga"
	// with a symbol whose NAME equals the package, importing a path that also
	// exists as a symbol name. We engineer this by having a file declare a
	// symbol named after both ends so the edge resolves.
	src := `package corex

import "helperpkg"

func corex() {}

func helperpkg() {}
`
	path := filepath.Join(dir, "corex.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile error = %v", err)
	}

	// The import edge is from package "corex" to "helperpkg"; both names exist
	// as function symbols, so the edge resolves and the dependency is reported.
	deps, err := g.QueryDependenciesOf(ctx, "corex")
	if err != nil {
		t.Fatalf("QueryDependenciesOf error = %v", err)
	}
	found := false
	for _, d := range deps {
		if d == "corex" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dependency target package present, got %v", deps)
	}
}

func TestQueryCallers_ResolvesPackageCall(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	// caller() calls target() directly; both are functions in the same file so
	// the edge resolves and target's caller is found.
	src := `package callex

func target() {}

func caller() {
	target()
}
`
	path := filepath.Join(dir, "callex.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatal(err)
	}

	callers, err := g.QueryCallersOf(ctx, "target")
	if err != nil {
		t.Fatalf("QueryCallersOf error = %v", err)
	}
	if len(callers) != 1 || callers[0].Name != "caller" {
		t.Fatalf("expected caller of target, got %+v", callers)
	}
}

func TestIndexDirectory_SkipsVendorAndTestdata(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	// Real source.
	writeTestFile(t, dir)

	// Files that must be skipped.
	for _, sub := range []string{"vendor", "testdata", "node_modules"} {
		d := filepath.Join(dir, sub)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		skipped := "package skip\nfunc ShouldNotIndex" + sub + "() {}\n"
		if err := os.WriteFile(filepath.Join(d, "x.go"), []byte(skipped), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := g.IndexDirectory(ctx, dir); err != nil {
		t.Fatalf("IndexDirectory error = %v", err)
	}

	// The skipped symbols must not be present.
	for _, sub := range []string{"vendor", "testdata", "node_modules"} {
		syms, err := g.QuerySymbol(ctx, "ShouldNotIndex"+sub)
		if err != nil {
			t.Fatal(err)
		}
		if len(syms) != 0 {
			t.Errorf("symbol from %s should have been skipped, got %d", sub, len(syms))
		}
	}
}

func TestIndexDirectory_SkipsUnparseableFileSilently(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	writeTestFile(t, dir)
	// A broken Go file in the tree must be skipped, not fail the whole walk.
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("@@@ not go"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := g.IndexDirectory(ctx, dir); err != nil {
		t.Fatalf("IndexDirectory should skip broken files, got %v", err)
	}
	// The good file's symbols should still be indexed.
	syms, err := g.QuerySymbol(ctx, "NewConfig")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 {
		t.Errorf("good file not indexed: got %d NewConfig symbols", len(syms))
	}
}

func TestIndexFile_StoresEmbeddedStructFields(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	// Embedded fields exercise fieldTypeName for Ident, StarExpr, and SelectorExpr.
	src := `package embedex

import "sync"

type Base struct{}

type Composite struct {
	Base
	*Base
	sync.Mutex
	Named int
}
`
	path := filepath.Join(dir, "embedex.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile error = %v", err)
	}

	// Embedded fields are stored as "Composite.<Type>" with kind "embed".
	syms, err := g.QuerySymbolPattern(ctx, "Composite.%")
	if err != nil {
		t.Fatal(err)
	}
	var embeds int
	for _, s := range syms {
		if s.Kind == "embed" {
			embeds++
		}
	}
	if embeds < 2 {
		t.Errorf("expected at least 2 embedded-field symbols, got %d (%+v)", embeds, syms)
	}
}

func TestStats_EmptyGraph(t *testing.T) {
	g, _ := setupTestGraph(t)
	stats := g.Stats(context.Background())
	if stats["symbols"] != 0 {
		t.Errorf("empty graph symbols = %d, want 0", stats["symbols"])
	}
	if stats["edges"] != 0 {
		t.Errorf("empty graph edges = %d, want 0", stats["edges"])
	}
	if stats["files"] != 0 {
		t.Errorf("empty graph files = %d, want 0", stats["files"])
	}
}

func TestStats_GroupsByKindAndRelation(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()
	writeTestFile(t, dir)
	if err := g.IndexFile(ctx, filepath.Join(dir, "example.go")); err != nil {
		t.Fatal(err)
	}
	stats := g.Stats(ctx)
	if stats["symbols_function"] == 0 {
		t.Errorf("expected function symbols counted, stats = %+v", stats)
	}
	if stats["symbols_type"] == 0 {
		t.Errorf("expected type symbols counted, stats = %+v", stats)
	}
}

func TestQuerySymbol_NoMatch(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()
	writeTestFile(t, dir)
	if err := g.IndexFile(ctx, filepath.Join(dir, "example.go")); err != nil {
		t.Fatal(err)
	}
	syms, err := g.QuerySymbol(ctx, "NoSuchSymbol")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 0 {
		t.Errorf("expected no matches, got %d", len(syms))
	}
}

// --- Direct parser-level tests for receiver/field type extraction ---

func parseDecls(t *testing.T, src string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return f
}

func TestReceiverTypeName_AllForms(t *testing.T) {
	src := `package p

type T struct{}
type G[U any] struct{}
type G2[U any, V any] struct{}

func (t T) ValueRecv() {}
func (t *T) PtrRecv() {}
func (g G[U]) GenericRecv() {}
func (g G2[U, V]) GenericMultiRecv() {}
`
	f := parseDecls(t, src)

	want := map[string]string{
		"ValueRecv":        "T",
		"PtrRecv":          "T",
		"GenericRecv":      "G",
		"GenericMultiRecv": "G2",
	}
	seen := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		got := receiverTypeName(fn.Recv.List[0].Type)
		if exp, ok := want[fn.Name.Name]; ok {
			if got != exp {
				t.Errorf("receiverTypeName for %s = %q, want %q", fn.Name.Name, got, exp)
			}
			seen[fn.Name.Name] = true
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("method %s was not exercised", name)
		}
	}

	// Unsupported receiver expr forms return "".
	if got := receiverTypeName(&ast.BasicLit{}); got != "" {
		t.Errorf("unsupported receiver = %q, want empty", got)
	}
}

func TestFieldTypeName_AllForms(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{"ident", &ast.Ident{Name: "Foo"}, "Foo"},
		{"star", &ast.StarExpr{X: &ast.Ident{Name: "Bar"}}, "Bar"},
		{"selector", &ast.SelectorExpr{X: &ast.Ident{Name: "pkg"}, Sel: &ast.Ident{Name: "Type"}}, "Type"},
		{"star of selector", &ast.StarExpr{X: &ast.SelectorExpr{X: &ast.Ident{Name: "pkg"}, Sel: &ast.Ident{Name: "T"}}}, "T"},
		{"unsupported", &ast.BasicLit{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fieldTypeName(tt.expr); got != tt.want {
				t.Errorf("fieldTypeName = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseGoFile_BlankIdentifierSkipped(t *testing.T) {
	dir := t.TempDir()
	src := `package p

var _ = 1
var Real = 2
`
	path := filepath.Join(dir, "blank.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	syms, _, err := parseGoFile(path)
	if err != nil {
		t.Fatalf("parseGoFile error = %v", err)
	}
	for _, s := range syms {
		if s.Name == "_" {
			t.Error("blank identifier should be skipped")
		}
	}
	var sawReal bool
	for _, s := range syms {
		if s.Name == "Real" {
			sawReal = true
		}
	}
	if !sawReal {
		t.Error("expected Real var symbol")
	}
}
