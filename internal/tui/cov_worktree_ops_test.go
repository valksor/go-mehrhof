package tui

import (
	"context"
	"strings"
	"testing"
)

func TestWtApprove(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		if out := wtCall(t, wtApprove, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("approve", func(t *testing.T) {
		out := wtCall(t, wtApprove, "submit", map[string]any{"approve": map[string]any{}})
		if out != "Approved: submit" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtChecklist(t *testing.T) {
	t.Run("check usage", func(t *testing.T) {
		if out := wtCall(t, wtChecklistCheck, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("check", func(t *testing.T) {
		out := wtCall(t, wtChecklistCheck, "tests-pass", map[string]any{"review.checklist.check": map[string]any{}})
		if out != "Checked: tests-pass" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("uncheck usage", func(t *testing.T) {
		if out := wtCall(t, wtChecklistUncheck, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("uncheck", func(t *testing.T) {
		out := wtCall(t, wtChecklistUncheck, "tests-pass", map[string]any{"review.checklist.uncheck": map[string]any{}})
		if out != "Unchecked: tests-pass" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list empty", func(t *testing.T) {
		out := wtCall(t, wtChecklist, "", map[string]any{"review.checklist.get": map[string]any{"required": []any{}}})
		if out != "No checklist items." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list with marks", func(t *testing.T) {
		out := wtCall(t, wtChecklist, "", map[string]any{
			"review.checklist.get": map[string]any{
				"required": []string{"a", "b"},
				"checked":  []string{"a"},
			},
		})
		if !strings.Contains(out, "✓ 1. a") {
			t.Errorf("checked item missing: %q", out)
		}
		if !strings.Contains(out, "☐ 2. b") {
			t.Errorf("unchecked item missing: %q", out)
		}
	})
}

func TestWtCI(t *testing.T) {
	out := wtCall(t, wtCI, "", map[string]any{
		"ci.status": map[string]any{
			"status": "running",
			"checks": []map[string]any{
				{"name": "build", "status": "passed"},
				{"name": "lint", "status": "failed"},
			},
		},
	})
	if !strings.Contains(out, "CI: running") {
		t.Errorf("out = %q", out)
	}
	if !strings.Contains(out, "✓ build") {
		t.Errorf("passed check missing: %q", out)
	}
	if !strings.Contains(out, "✗ lint") {
		t.Errorf("failed check missing: %q", out)
	}
}

func TestWtPolicy(t *testing.T) {
	t.Run("compliant", func(t *testing.T) {
		out := wtCall(t, wtPolicy, "", map[string]any{"policy.check": map[string]any{"compliant": true}})
		if out != "Policy: compliant." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("violations", func(t *testing.T) {
		out := wtCall(t, wtPolicy, "", map[string]any{
			"policy.check": map[string]any{
				"compliant":  false,
				"violations": []map[string]any{{"rule": "no-force-push", "message": "force push detected"}},
			},
		})
		if !strings.Contains(out, "Policy violations:") || !strings.Contains(out, "no-force-push") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtQuality(t *testing.T) {
	out := wtCall(t, wtQuality, "", map[string]any{"quality.respond": map[string]any{"status": "passed"}})
	if out != "Quality: passed" {
		t.Errorf("out = %q", out)
	}
}

func TestWtRetry(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		wantPhase string
		wantText  string
	}{
		{"loaded -> plan", stateLoaded, phasePlan, "Retrying: reset to loaded, re-running " + phasePlan + "."},
		{"planned -> implement", statePlanned, phaseImplement, "Retrying: reset to planned, re-running " + phaseImplement + "."},
		{"implemented -> review", stateImplemented, phaseReview, "Retrying: reset to implemented, re-running " + phaseReview + "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStubServer(t)
			s.on("reset", map[string]any{"state": tt.state})
			s.on(tt.wantPhase, map[string]any{})
			out, err := wtRetry(context.Background(), s.client(t), "", false)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if out != tt.wantText {
				t.Errorf("out = %q, want %q", out, tt.wantText)
			}
			calls := s.calls()
			if len(calls) != 2 || calls[1].method != tt.wantPhase {
				t.Errorf("calls = %+v, want second call %q", calls, tt.wantPhase)
			}
		})
	}
	t.Run("non-mappable state", func(t *testing.T) {
		out := wtCall(t, wtRetry, "", map[string]any{"reset": map[string]any{"state": "simplifying"}})
		if out != "Task reset to simplifying — use a phase command to continue." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtAudit(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := wtCall(t, wtAudit, "", map[string]any{"task.export": map[string]any{"entries": []any{}}})
		if out != "No audit entries." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("entries", func(t *testing.T) {
		out := wtCall(t, wtAudit, "", map[string]any{
			"task.export": map[string]any{"entries": []map[string]any{
				{"action": "plan", "timestamp": "10:00", "details": "wrote spec"},
				{"action": "implement", "timestamp": "10:30"},
			}},
		})
		if !strings.Contains(out, "[10:00] plan — wrote spec") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "[10:30] implement") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtFiles(t *testing.T) {
	t.Run("search usage", func(t *testing.T) {
		if out := wtCall(t, wtFilesSearch, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search none", func(t *testing.T) {
		out := wtCall(t, wtFilesSearch, "*.go", map[string]any{"files.search": map[string]any{"files": []any{}}})
		if out != "No matching files." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search results", func(t *testing.T) {
		out := wtCall(t, wtFilesSearch, "*.go", map[string]any{"files.search": map[string]any{"files": []string{"a.go", "b.go"}}})
		if out != "a.go\nb.go" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list none", func(t *testing.T) {
		out := wtCall(t, wtFiles, "", map[string]any{"files.list": map[string]any{"files": []any{}}})
		if out != "No files." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list results", func(t *testing.T) {
		out := wtCall(t, wtFiles, "src", map[string]any{"files.list": map[string]any{"files": []string{"x.go"}}})
		if out != "x.go" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtGitStatus(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		out := wtCall(t, wtGitStatus, "", map[string]any{"git.status": map[string]any{"branch": "main"}})
		if out != "Branch: main (clean)" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("has changes", func(t *testing.T) {
		out := wtCall(t, wtGitStatus, "", map[string]any{"git.status": map[string]any{"branch": "feat", "has_changes": true}})
		if out != "Branch: feat (has changes)" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("summary", func(t *testing.T) {
		out := wtCall(t, wtGitStatus, "", map[string]any{"git.status": map[string]any{"branch": "feat", "summary": "2 files changed"}})
		if out != "Branch: feat\n2 files changed" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtGitLog(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := wtCall(t, wtGitLog, "", map[string]any{"git.log": map[string]any{"entries": []any{}}})
		if out != "No commits." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("entries", func(t *testing.T) {
		out := wtCall(t, wtGitLog, "", map[string]any{
			"git.log": map[string]any{"entries": []map[string]any{{"sha": "abcdef1234", "message": "first"}}},
		})
		if out != "abcdef1 first" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtCodegraph(t *testing.T) {
	t.Run("callers usage", func(t *testing.T) {
		if out := wtCall(t, wtCodegraphCallers, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("callers none", func(t *testing.T) {
		out := wtCall(t, wtCodegraphCallers, "Foo", map[string]any{"codegraph.callers": map[string]any{"callers": []any{}}})
		if out != "No callers found." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("callers", func(t *testing.T) {
		out := wtCall(t, wtCodegraphCallers, "Foo", map[string]any{
			"codegraph.callers": map[string]any{"callers": []map[string]any{{"name": "Bar", "file": "x.go", "line": 12}}},
		})
		if out != "Bar — x.go:12" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("deps usage", func(t *testing.T) {
		if out := wtCall(t, wtCodegraphDeps, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("deps none", func(t *testing.T) {
		out := wtCall(t, wtCodegraphDeps, "Foo", map[string]any{"codegraph.deps": map[string]any{"deps": []any{}}})
		if out != "No dependencies found." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("deps", func(t *testing.T) {
		out := wtCall(t, wtCodegraphDeps, "Foo", map[string]any{
			"codegraph.deps": map[string]any{"deps": []map[string]any{{"name": "Baz", "kind": "func", "file": "y.go"}}},
		})
		if out != "func Baz — y.go" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("index", func(t *testing.T) {
		out := wtCall(t, wtCodegraphIndex, "", map[string]any{"codegraph.index": map[string]any{"symbols": 42}})
		if out != "Indexed 42 symbols." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("stats", func(t *testing.T) {
		out := wtCall(t, wtCodegraphStats, "", map[string]any{"codegraph.stats": map[string]any{"symbols": 10}})
		if !strings.Contains(out, "symbols") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search usage", func(t *testing.T) {
		if out := wtCall(t, wtCodegraphSearch, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search none", func(t *testing.T) {
		out := wtCall(t, wtCodegraphSearch, "Foo", map[string]any{"codegraph.search": map[string]any{"symbols": []any{}}})
		if out != "No symbols found." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search", func(t *testing.T) {
		out := wtCall(t, wtCodegraphSearch, "Foo", map[string]any{
			"codegraph.search": map[string]any{"symbols": []map[string]any{{"name": "Foo", "kind": "func", "file": "z.go", "line": 7}}},
		})
		if out != "func Foo — z.go:7" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtParityHandlers(t *testing.T) {
	t.Run("screenshots empty", func(t *testing.T) {
		out := wtCall(t, wtScreenshots, "", map[string]any{"screenshots.list": map[string]any{"screenshots": []any{}}})
		if out != "No screenshots." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("screenshots listed", func(t *testing.T) {
		out := wtCall(t, wtScreenshots, "", map[string]any{
			"screenshots.list": map[string]any{"screenshots": []map[string]any{{"id": "shot1234abcd", "path": "/img/a.png"}}},
		})
		if out != "shot1234 /img/a.png" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("hooks empty", func(t *testing.T) {
		out := wtCall(t, wtHooks, "", map[string]any{"hooks.list": map[string]any{"hooks": []any{}}})
		if out != "No hooks configured." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("hooks listed", func(t *testing.T) {
		out := wtCall(t, wtHooks, "", map[string]any{
			"hooks.list": map[string]any{"hooks": []map[string]any{
				{"event": "post-plan", "command": "echo done"},
				{"name": "named", "command": "run.sh"},
			}},
		})
		if !strings.Contains(out, "post-plan: echo done") || !strings.Contains(out, "named: run.sh") {
			t.Errorf("out = %q", out)
		}
	})
}
