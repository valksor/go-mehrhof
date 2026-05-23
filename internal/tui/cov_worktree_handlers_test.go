package tui

import (
	"context"
	"strings"
	"testing"
)

// wtCall invokes a worktree handler against a stub server with the given canned
// results and returns the output string. Fails the test on handler error.
// dryRun is always false here; dry-run param plumbing is exercised separately
// via the workflow-handler table and the Model command tests.
func wtCall(t *testing.T, h worktreeHandler, args string, results map[string]any) string {
	t.Helper()
	s := newStubServer(t)
	for method, res := range results {
		s.on(method, res)
	}
	out, err := h(context.Background(), s.client(t), args, false)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	return out
}

func TestWtWorkflowHandlers(t *testing.T) {
	tests := []struct {
		name    string
		handler worktreeHandler
		args    string
		dryRun  bool
		method  string
		want    string
	}{
		{"quick", wtQuick, "fix the thing", false, "start", "Quick fix started."},
		{"plan", wtPlan, "", false, phasePlan, "Planning started."},
		{"plan dry-run", wtPlan, "", true, phasePlan, "Planning started."},
		{"implement", wtImplement, "", false, phaseImplement, "Implementation started."},
		{"implement dry-run", wtImplement, "", true, phaseImplement, "Implementation started."},
		{"simplify", wtSimplify, "", false, phaseSimplify, "Simplification started."},
		{"optimize", wtOptimize, "", true, phaseOptimize, "Optimization started."},
		{"review", wtReview, "", false, phaseReview, "Review started."},
		{"review fix", wtReviewFix, "", false, phaseReview, "Review with fixes started."},
		{"submit", wtSubmit, "", true, phaseSubmit, "Submit started."},
		{"finish", wtFinish, "", false, "task.finish", "Finish started."},
		{"abandon", wtAbandon, "", false, "abandon", "Task abandoned."},
		{"delete", wtDelete, "", false, "delete", "Task deleted."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStubServer(t)
			out, err := tt.handler(context.Background(), s.client(t), tt.args, tt.dryRun)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if out != tt.want {
				t.Errorf("out = %q, want %q", out, tt.want)
			}
			calls := s.calls()
			if len(calls) == 0 || calls[0].method != tt.method {
				t.Errorf("first call = %+v, want method %q", calls, tt.method)
			}
		})
	}
}

func TestWtQuickEmptyArgs(t *testing.T) {
	s := newStubServer(t)
	out, err := wtQuick(context.Background(), s.client(t), "", false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "Usage: /quick") {
		t.Errorf("out = %q, want usage", out)
	}
	if len(s.calls()) != 0 {
		t.Errorf("expected no RPC call for empty args, got %+v", s.calls())
	}
}

func TestWtControlHandlers(t *testing.T) {
	tests := []struct {
		name    string
		handler worktreeHandler
		method  string
		want    string
	}{
		{"undo", wtUndo, "undo", "Undone to previous checkpoint."},
		{"redo", wtRedo, "redo", "Redone to next checkpoint."},
		{"stop", wtStop, "stop", "Operation stopped."},
		{"abort", wtAbort, "abort", "Operation aborted."},
		{"reset", wtReset, "reset", "Task reset."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStubServer(t)
			out, err := tt.handler(context.Background(), s.client(t), "", false)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if out != tt.want {
				t.Errorf("out = %q, want %q", out, tt.want)
			}
			if c := s.calls(); len(c) == 0 || c[0].method != tt.method {
				t.Errorf("calls = %+v, want method %q", c, tt.method)
			}
		})
	}
}

func TestWtUpdate(t *testing.T) {
	t.Run("changed", func(t *testing.T) {
		out := wtCall(t, wtUpdate, "", map[string]any{"update": map[string]any{"changed": true}})
		if out != "Task updated from source." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("unchanged", func(t *testing.T) {
		out := wtCall(t, wtUpdate, "", map[string]any{"update": map[string]any{"changed": false}})
		if out != "Task is already up to date." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtStatus(t *testing.T) {
	t.Run("with title", func(t *testing.T) {
		out := wtCall(t, wtStatus, "", map[string]any{"status": map[string]any{"state": "planning", "title": "Add feature"}})
		if out != "State: planning — Add feature" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("no title", func(t *testing.T) {
		out := wtCall(t, wtStatus, "", map[string]any{"status": map[string]any{"state": "loaded"}})
		if out != "State: loaded" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtCheckpoints(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := wtCall(t, wtCheckpoints, "", map[string]any{"checkpoints": map[string]any{"checkpoints": []any{}}})
		if out != "No checkpoints." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("listed", func(t *testing.T) {
		out := wtCall(t, wtCheckpoints, "", map[string]any{
			"checkpoints": map[string]any{"checkpoints": []map[string]any{
				{"sha": "abc1234567", "message": "planned"},
				{"sha": "def8901234", "message": "implemented"},
			}},
		})
		if !strings.Contains(out, "1. abc1234 — planned") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "2. def8901 — implemented") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtCheckpointsGoto(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		out := wtCall(t, wtCheckpointsGoto, "", nil)
		if !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("jumps", func(t *testing.T) {
		s := newStubServer(t)
		out, err := wtCheckpointsGoto(context.Background(), s.client(t), "abcdef1234", false)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if out != "Jumped to checkpoint abcdef1." {
			t.Errorf("out = %q", out)
		}
		if c := s.calls(); len(c) == 0 || c[0].method != "checkpoint.goto" {
			t.Errorf("calls = %+v", c)
		}
	})
}

func TestWtRecap(t *testing.T) {
	t.Run("with recap", func(t *testing.T) {
		out := wtCall(t, wtRecap, "", map[string]any{"recap": map[string]any{"recap": "Did the thing"}})
		if out != "Did the thing" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("empty", func(t *testing.T) {
		out := wtCall(t, wtRecap, "", map[string]any{"recap": map[string]any{}})
		if out != "No recap available." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtDiff(t *testing.T) {
	t.Run("with diff", func(t *testing.T) {
		out := wtCall(t, wtDiff, "", map[string]any{"git.diff": map[string]any{"diff": "+added line"}})
		if out != "+added line" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("no changes", func(t *testing.T) {
		out := wtCall(t, wtDiff, "", map[string]any{"git.diff": map[string]any{}})
		if out != "No changes." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtShowSpecPlan(t *testing.T) {
	t.Run("spec with content", func(t *testing.T) {
		out := wtCall(t, wtShowSpec, "", map[string]any{
			"show.spec": map[string]any{"specifications": []map[string]any{{"content": "Spec A"}, {"content": "Spec B"}}},
		})
		if !strings.Contains(out, "Spec A") || !strings.Contains(out, "Spec B") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("plan empty", func(t *testing.T) {
		out := wtCall(t, wtShowPlan, "", map[string]any{"show.plan": map[string]any{"specifications": []any{}}})
		if out != "No specification available." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtListAndSearch(t *testing.T) {
	t.Run("list history", func(t *testing.T) {
		out := wtCall(t, wtList, "", map[string]any{
			"task.history": map[string]any{"tasks": []map[string]any{{"id": "aaaa1111bbbb", "title": "T1", "state": "loaded"}}},
		})
		if !strings.Contains(out, "T1") || !strings.Contains(out, "[loaded]") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search usage", func(t *testing.T) {
		out := wtCall(t, wtListSearch, "", nil)
		if !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search results", func(t *testing.T) {
		out := wtCall(t, wtListSearch, "bug", map[string]any{
			"task.search": map[string]any{"tasks": []map[string]any{{"id": "cccc2222dddd", "title": "Bug fix", "state": "planned"}}},
		})
		if !strings.Contains(out, "Bug fix") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtEventlog(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := wtCall(t, wtEventlog, "", map[string]any{"eventlog.query": map[string]any{"events": []any{}}})
		if out != "No events." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("events", func(t *testing.T) {
		out := wtCall(t, wtEventlog, "", map[string]any{
			"eventlog.query": map[string]any{"events": []map[string]any{
				{"type": "state_changed", "timestamp": "10:00", "message": "now planning"},
				{"type": "checkpoint", "timestamp": "10:01"},
			}},
		})
		if !strings.Contains(out, "[10:00] state_changed: now planning") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "[10:01] checkpoint") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtTags(t *testing.T) {
	t.Run("add usage", func(t *testing.T) {
		if out := wtCall(t, wtTagAdd, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("add", func(t *testing.T) {
		out := wtCall(t, wtTagAdd, "urgent", map[string]any{"task.tag": map[string]any{}})
		if out != `Tag "urgent" added.` {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove usage", func(t *testing.T) {
		if out := wtCall(t, wtTagRemove, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove", func(t *testing.T) {
		out := wtCall(t, wtTagRemove, "urgent", map[string]any{"task.tag": map[string]any{}})
		if out != `Tag "urgent" removed.` {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list empty", func(t *testing.T) {
		out := wtCall(t, wtTags, "", map[string]any{"task.tag": map[string]any{"tags": []any{}}})
		if out != "No tags." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list", func(t *testing.T) {
		out := wtCall(t, wtTags, "", map[string]any{"task.tag": map[string]any{"tags": []string{"a", "b"}}})
		if out != "Tags: a, b" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtQueue(t *testing.T) {
	t.Run("add usage", func(t *testing.T) {
		if out := wtCall(t, wtQueueAdd, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("add", func(t *testing.T) {
		out := wtCall(t, wtQueueAdd, "issue#42", map[string]any{"queue.add": map[string]any{}})
		if out != "Queued: issue#42" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove usage", func(t *testing.T) {
		if out := wtCall(t, wtQueueRemove, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove", func(t *testing.T) {
		out := wtCall(t, wtQueueRemove, "id1", map[string]any{"queue.remove": map[string]any{}})
		if out != "Removed from queue." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("reorder usage", func(t *testing.T) {
		if out := wtCall(t, wtQueueReorder, "onlyone", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("reorder bad position", func(t *testing.T) {
		out := wtCall(t, wtQueueReorder, "id1 notanumber", nil)
		if out != "Position must be a number." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("reorder ok", func(t *testing.T) {
		out := wtCall(t, wtQueueReorder, "abcdefghij 3", map[string]any{"queue.reorder": map[string]any{}})
		if out != "Moved abcdefgh to position 3." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list empty", func(t *testing.T) {
		out := wtCall(t, wtQueueList, "", map[string]any{"queue.list": map[string]any{"queue": []any{}}})
		if out != "Queue is empty." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list with items", func(t *testing.T) {
		out := wtCall(t, wtQueueList, "", map[string]any{
			"queue.list": map[string]any{"queue": []map[string]any{
				{"id": "aaaa1111bbbb", "title": "Titled"},
				{"id": "cccc2222dddd", "source": "src-only"},
			}},
		})
		if !strings.Contains(out, "Titled") || !strings.Contains(out, "src-only") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtForks(t *testing.T) {
	t.Run("create usage", func(t *testing.T) {
		if out := wtCall(t, wtForkCreate, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("create", func(t *testing.T) {
		out := wtCall(t, wtForkCreate, "alt", map[string]any{"fork.create": map[string]any{}})
		if out != `Fork "alt" created.` {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list empty", func(t *testing.T) {
		out := wtCall(t, wtForkList, "", map[string]any{"fork.list": map[string]any{"forks": []any{}}})
		if out != "No active forks." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list", func(t *testing.T) {
		out := wtCall(t, wtForkList, "", map[string]any{
			"fork.list": map[string]any{"forks": []map[string]any{{"id": "fork1234abcd", "label": "alt", "state": "active"}}},
		})
		if !strings.Contains(out, "fork1234 — alt [active]") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("compare", func(t *testing.T) {
		out := wtCall(t, wtForkCompare, "", map[string]any{"fork.compare": map[string]any{"diff": "stuff"}})
		if !strings.Contains(out, "diff") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("select usage", func(t *testing.T) {
		if out := wtCall(t, wtForkSelect, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("select", func(t *testing.T) {
		out := wtCall(t, wtForkSelect, "fork1234abcd", map[string]any{"fork.select": map[string]any{}})
		if out != "Switched to fork fork1234." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtCache(t *testing.T) {
	t.Run("stats", func(t *testing.T) {
		out := wtCall(t, wtCacheStats, "", map[string]any{"cache.stats": map[string]any{"hits": 3}})
		if !strings.Contains(out, "hits") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("clear", func(t *testing.T) {
		out := wtCall(t, wtCacheClear, "", map[string]any{"cache.clear": map[string]any{}})
		if out != "Cache cleared." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtExport(t *testing.T) {
	t.Run("path", func(t *testing.T) {
		out := wtCall(t, wtExport, "", map[string]any{"task.export": map[string]any{"path": "/tmp/out.json"}})
		if out != "Exported to /tmp/out.json" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("data", func(t *testing.T) {
		out := wtCall(t, wtExport, "csv", map[string]any{"task.export": map[string]any{"data": "a,b,c"}})
		if out != "a,b,c" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("complete", func(t *testing.T) {
		out := wtCall(t, wtExport, "json", map[string]any{"task.export": map[string]any{}})
		if out != "Export complete." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtChangelog(t *testing.T) {
	t.Run("usage empty", func(t *testing.T) {
		if out := wtCall(t, wtChangelog, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("usage bad range", func(t *testing.T) {
		if out := wtCall(t, wtChangelog, "v1 v2", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("generated", func(t *testing.T) {
		out := wtCall(t, wtChangelog, "v1..v2 some note", map[string]any{
			"changelog.generate": map[string]any{"markdown": "## Changes"},
		})
		if out != "## Changes" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("full empty result", func(t *testing.T) {
		out := wtCall(t, wtChangelogFull, "v1..v2", map[string]any{"changelog.generate": map[string]any{}})
		if out != "No commits between v1 and v2" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtRemote(t *testing.T) {
	t.Run("approve", func(t *testing.T) {
		out := wtCall(t, wtRemoteApprove, "", map[string]any{"remote.approve": map[string]any{}})
		if out != "PR approved." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("merge", func(t *testing.T) {
		out := wtCall(t, wtRemoteMerge, "", map[string]any{"remote.merge": map[string]any{}})
		if out != "PR merged." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtDiscover(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		out := wtCall(t, wtDiscover, "", map[string]any{"discovery.scan": map[string]any{"count": 0}})
		if out != "No project commands discovered." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("listed", func(t *testing.T) {
		out := wtCall(t, wtDiscover, "", map[string]any{
			"discovery.scan": map[string]any{"count": 2, "commands": []string{"make build", "make test"}},
		})
		if !strings.Contains(out, "Discovered commands (2)") || !strings.Contains(out, "make build") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestWtExplain(t *testing.T) {
	s := newStubServer(t)
	out, err := wtExplain(context.Background(), s.client(t), "", false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out != "Asking agent to explain..." {
		t.Errorf("out = %q", out)
	}
	if c := s.calls(); len(c) == 0 || c[0].method != "chat.send" {
		t.Errorf("calls = %+v", c)
	}
}
