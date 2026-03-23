package codegraph

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testGoFile = `package example

import "fmt"

const MaxRetries = 3

var DefaultTimeout = 30

type Config struct {
	Name    string
	Timeout int
}

type Runner interface {
	Run() error
}

func NewConfig(name string) *Config {
	return &Config{Name: name, Timeout: DefaultTimeout}
}

func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name required")
	}
	return nil
}

func Start(c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	fmt.Println("starting", c.Name)
	return nil
}
`

func setupTestGraph(t *testing.T) (*Graph, string) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	g, err := New(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() { _ = g.Close() })

	return g, dir
}

func writeTestFile(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "example.go")
	if err := os.WriteFile(path, []byte(testGoFile), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	return path
}

func TestIndexFileAndQuerySymbol(t *testing.T) {
	g, dir := setupTestGraph(t)
	path := writeTestFile(t, dir)
	ctx := context.Background()

	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}

	// Query a function.
	syms, err := g.QuerySymbol(ctx, "NewConfig")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for NewConfig, got %d", len(syms))
	}
	if syms[0].Kind != "function" {
		t.Errorf("expected kind=function, got %s", syms[0].Kind)
	}
	if syms[0].Package != "example" {
		t.Errorf("expected package=example, got %s", syms[0].Package)
	}

	// Query a method.
	syms, err = g.QuerySymbol(ctx, "Config.Validate")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for Config.Validate, got %d", len(syms))
	}
	if syms[0].Kind != "method" {
		t.Errorf("expected kind=method, got %s", syms[0].Kind)
	}

	// Query an interface.
	syms, err = g.QuerySymbol(ctx, "Runner")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for Runner, got %d", len(syms))
	}
	if syms[0].Kind != "interface" {
		t.Errorf("expected kind=interface, got %s", syms[0].Kind)
	}

	// Query a type.
	syms, err = g.QuerySymbol(ctx, "Config")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for Config, got %d", len(syms))
	}
	if syms[0].Kind != "type" {
		t.Errorf("expected kind=type, got %s", syms[0].Kind)
	}

	// Query a const.
	syms, err = g.QuerySymbol(ctx, "MaxRetries")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for MaxRetries, got %d", len(syms))
	}
	if syms[0].Kind != "const" {
		t.Errorf("expected kind=const, got %s", syms[0].Kind)
	}

	// Query a var.
	syms, err = g.QuerySymbol(ctx, "DefaultTimeout")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for DefaultTimeout, got %d", len(syms))
	}
	if syms[0].Kind != "var" {
		t.Errorf("expected kind=var, got %s", syms[0].Kind)
	}
}

func TestQueryCallers(t *testing.T) {
	g, dir := setupTestGraph(t)
	path := writeTestFile(t, dir)
	ctx := context.Background()

	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}

	// Start calls c.Validate — the caller name is "Start", callee is "c.Validate".
	// Since we store the method as "Config.Validate", the call edge uses "c.Validate"
	// which won't resolve to "Config.Validate" (we can't resolve receiver types statically).
	// But Start also calls fmt.Println, which should create an edge.

	// Query callers of NewConfig — no one calls it in the test file.
	callers, err := g.QueryCallersOf(ctx, "NewConfig")
	if err != nil {
		t.Fatalf("QueryCallersOf: %v", err)
	}
	if len(callers) != 0 {
		t.Errorf("expected 0 callers of NewConfig, got %d", len(callers))
	}
}

func TestStats(t *testing.T) {
	g, dir := setupTestGraph(t)
	path := writeTestFile(t, dir)
	ctx := context.Background()

	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}

	stats := g.Stats(ctx)
	if stats["symbols"] == 0 {
		t.Error("expected symbols > 0")
	}
	if stats["files"] != 1 {
		t.Errorf("expected files=1, got %d", stats["files"])
	}
}

func TestIndexDirectory(t *testing.T) {
	g, dir := setupTestGraph(t)
	ctx := context.Background()

	// Write multiple Go files.
	writeTestFile(t, dir)

	secondFile := `package example

func Helper() string {
	return "help"
}
`
	if err := os.WriteFile(filepath.Join(dir, "helper.go"), []byte(secondFile), 0o644); err != nil {
		t.Fatalf("write helper.go: %v", err)
	}

	if err := g.IndexDirectory(ctx, dir); err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}

	stats := g.Stats(ctx)
	if stats["files"] != 2 {
		t.Errorf("expected files=2, got %d", stats["files"])
	}

	syms, err := g.QuerySymbol(ctx, "Helper")
	if err != nil {
		t.Fatalf("QuerySymbol: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol for Helper, got %d", len(syms))
	}
}

func TestReindex(t *testing.T) {
	g, dir := setupTestGraph(t)
	path := writeTestFile(t, dir)
	ctx := context.Background()

	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile first: %v", err)
	}

	count1 := g.Stats(ctx)["symbols"]

	// Re-index same file — should not duplicate symbols.
	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile second: %v", err)
	}

	count2 := g.Stats(ctx)["symbols"]
	if count1 != count2 {
		t.Errorf("symbol count changed after re-index: %d -> %d", count1, count2)
	}
}

func TestQuerySymbolPattern(t *testing.T) {
	g, dir := setupTestGraph(t)
	path := writeTestFile(t, dir)
	ctx := context.Background()

	if err := g.IndexFile(ctx, path); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}

	// Pattern match: "Config%" should match Config and Config.Validate
	syms, err := g.QuerySymbolPattern(ctx, "Config%")
	if err != nil {
		t.Fatalf("QuerySymbolPattern: %v", err)
	}
	if len(syms) < 2 {
		t.Errorf("expected >= 2 symbols matching Config%%, got %d", len(syms))
	}
}
