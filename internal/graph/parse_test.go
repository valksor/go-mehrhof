package graph

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/worker"
)

func TestParseGraphDef_Simple(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: scaffold
    job_type: implement
    prompt: "Create file structure"
  - id: core
    job_type: implement
    prompt: "Implement core logic"
    depends_on: [scaffold]
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if g.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes, got %d", g.NodeCount())
	}

	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "scaffold" {
		t.Fatalf("expected root [scaffold], got %v", roots)
	}

	core := g.Nodes["core"]
	if core.JobType != worker.JobTypeImplement {
		t.Fatalf("expected implement job type, got %v", core.JobType)
	}
	if len(core.DependsOn) != 1 || core.DependsOn[0] != "scaffold" {
		t.Fatalf("expected depends_on [scaffold], got %v", core.DependsOn)
	}
}

func TestParseGraphDef_FailBranch(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: tests
    job_type: implement
    prompt: "Write tests"
    fail_branch: fix
  - id: fix
    job_type: implement
    prompt: "Fix failing tests"
    depends_on: [tests]
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tests := g.Nodes["tests"]
	if tests.FailBranch == nil || *tests.FailBranch != "fix" {
		t.Fatal("expected fail_branch pointing to fix")
	}
}

func TestParseGraphDef_Iteration(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: a
    job_type: implement
    prompt: "do it"
    max_iterations: 5
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	node := g.Nodes["a"]
	if node.MaxIterations != 5 {
		t.Fatalf("expected max_iterations=5, got %d", node.MaxIterations)
	}
	if node.IterationCheck == nil {
		t.Fatal("YAML-defined iteration should set IterationCheck")
	}
	if !node.IterationCheck("any output") {
		t.Fatal("YAML iteration check should always return true")
	}
}

func TestParseGraphDef_ErrorStrategy(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: lint
    job_type: implement
    prompt: "run lint"
    error_strategy: default_value
    default_output: "no lint results"
  - id: test
    job_type: implement
    prompt: "run tests"
    error_strategy: retry_then_fail
    max_retries: 3
    retry_delay: 1s
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	lint := g.Nodes["lint"]
	if lint.ErrorStrategy != ErrorDefaultValue {
		t.Fatalf("expected ErrorDefaultValue, got %d", lint.ErrorStrategy)
	}
	if lint.DefaultOutput != "no lint results" {
		t.Fatalf("expected default output, got %q", lint.DefaultOutput)
	}

	test := g.Nodes["test"]
	if test.ErrorStrategy != ErrorRetryThenFail {
		t.Fatalf("expected ErrorRetryThenFail, got %d", test.ErrorStrategy)
	}
	if test.MaxRetries != 3 {
		t.Fatalf("expected max_retries=3, got %d", test.MaxRetries)
	}
	if test.RetryDelay != time.Second {
		t.Fatalf("expected retry_delay=1s, got %v", test.RetryDelay)
	}
}

func TestParseGraphDef_EmptyNodes(t *testing.T) {
	yaml := `
phase: implement
nodes: []
`
	_, err := ParseGraphDef([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty nodes")
	}
}

func TestParseGraphDef_MissingID(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - job_type: implement
    prompt: "no id"
`
	_, err := ParseGraphDef([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing id")
	}
	if !contains(err.Error(), "no id") {
		t.Fatalf("error should mention no id: %v", err)
	}
}

func TestParseGraphDef_InvalidYAML(t *testing.T) {
	_, err := ParseGraphDef([]byte(`{{{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseGraphDefFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.yaml")

	yaml := `
phase: plan
nodes:
  - id: research
    job_type: plan
    prompt: "Research the problem"
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := ParseGraphDefFile(path)
	if err != nil {
		t.Fatalf("parse file error: %v", err)
	}
	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node, got %d", g.NodeCount())
	}
}

func TestParseGraphDefFile_NotFound(t *testing.T) {
	_, err := ParseGraphDefFile("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExpandPromptVars(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:      "a",
		Prompt:  "Fix: {{error}} in {{file}}",
		JobType: worker.JobTypeImplement,
	})
	_ = g.AddNode(&Node{
		ID:        "b",
		Prompt:    "Review changes to {{file}}",
		JobType:   worker.JobTypeReview,
		DependsOn: []NodeID{"a"},
	})
	_ = g.Validate()

	vars := map[string]string{
		"error": "null pointer",
		"file":  "main.go",
	}

	ExpandPromptVars(g, vars)

	if g.Nodes["a"].Prompt != "Fix: null pointer in main.go" {
		t.Fatalf("unexpected prompt: %s", g.Nodes["a"].Prompt)
	}
	if g.Nodes["b"].Prompt != "Review changes to main.go" {
		t.Fatalf("unexpected prompt: %s", g.Nodes["b"].Prompt)
	}
}

func TestExpandPromptVars_UnknownVarsLeftAsIs(t *testing.T) {
	g := New()
	_ = g.AddNode(&Node{
		ID:      "a",
		Prompt:  "Hello {{name}} and {{unknown}}",
		JobType: worker.JobTypeImplement,
	})
	_ = g.Validate()

	ExpandPromptVars(g, map[string]string{"name": "world"})

	if g.Nodes["a"].Prompt != "Hello world and {{unknown}}" {
		t.Fatalf("unexpected prompt: %s", g.Nodes["a"].Prompt)
	}
}

func TestResolveJobType(t *testing.T) {
	tests := []struct {
		input string
		want  worker.JobType
	}{
		{"plan", worker.JobTypePlan},
		{"Plan", worker.JobTypePlan},
		{"implement", worker.JobTypeImplement},
		{"review", worker.JobTypeReview},
		{"simplify", worker.JobTypeSimplify},
		{"optimize", worker.JobTypeOptimize},
		{"dry_run", worker.JobTypeDryRun},
		{"dryrun", worker.JobTypeDryRun},
		{"custom_type", worker.JobType("custom_type")},
	}

	for _, tt := range tests {
		got := resolveJobType(tt.input)
		if got != tt.want {
			t.Errorf("resolveJobType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseGraphDef_DefaultLabel(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: my_node
    job_type: implement
    prompt: "do it"
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	node := g.Nodes["my_node"]
	if node.Label != "my_node" {
		t.Fatalf("expected label to default to id, got %q", node.Label)
	}
}

func TestParseGraphDef_ExplicitLabel(t *testing.T) {
	yaml := `
phase: implement
nodes:
  - id: a
    label: "My Custom Label"
    job_type: implement
    prompt: "do it"
`
	g, err := ParseGraphDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if g.Nodes["a"].Label != "My Custom Label" {
		t.Fatalf("expected custom label, got %q", g.Nodes["a"].Label)
	}
}
