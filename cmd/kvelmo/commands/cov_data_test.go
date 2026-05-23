package commands

import (
	"strings"
	"testing"
)

// --- List / projects (global socket) ---

func TestRunList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{
			map[string]any{"id": "t1", "state": "implementing", "task_title": "Fix login", "path": "/p1"},
			map[string]any{"id": "t2", "state": "reviewing", "path": "/p2"},
		},
	})

	out := captureStdout(t, func() {
		if err := runList(ListCmd, nil); err != nil {
			t.Errorf("runList: %v", err)
		}
	})
	if !strings.Contains(out, "Fix login") || !strings.Contains(out, "t2") {
		t.Errorf("list output missing rows:\n%s", out)
	}
}

func TestRunList_Empty(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.list", map[string]any{"tasks": []any{}})

	out := captureStdout(t, func() {
		if err := runList(ListCmd, nil); err != nil {
			t.Errorf("runList: %v", err)
		}
	})
	if !strings.Contains(out, "No registered projects") {
		t.Errorf("list empty output = %q", out)
	}
}

func TestRunList_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	if err := runList(ListCmd, nil); err == nil {
		t.Fatal("expected error with no global socket")
	}
}

func TestRunTaskHistory_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.history", map[string]any{
		"tasks": []any{
			map[string]any{
				"title":        "Old task",
				"final_state":  "finished",
				"source":       "github",
				"completed_at": "2026-05-01T10:00:00Z",
			},
		},
		"count": 1,
	})

	out := captureStdout(t, func() {
		if err := runTaskHistory(false); err != nil {
			t.Errorf("runTaskHistory: %v", err)
		}
	})
	if !strings.Contains(out, "Old task") || !strings.Contains(out, "1 task(s) found") {
		t.Errorf("history output:\n%s", out)
	}
}

func TestRunListSearchCmd_WithSocket(t *testing.T) {
	orig := listSearchJSON
	t.Cleanup(func() { listSearchJSON = orig })
	listSearchJSON = false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.search", map[string]any{
		"tasks": []any{
			map[string]any{"title": "auth refactor", "final_state": "finished", "source": "file", "completed_at": "2026-05-02T10:00:00Z"},
		},
		"count": 1,
	})

	out := captureStdout(t, func() {
		if err := runListSearchCmd(listSearchCmd, []string{"auth"}); err != nil {
			t.Errorf("runListSearchCmd: %v", err)
		}
	})
	if !strings.Contains(out, "auth refactor") {
		t.Errorf("search output:\n%s", out)
	}
}

func TestRunProjects_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("projects.list", map[string]any{
		"projects": []any{
			map[string]any{"path": "/p1", "state": "implementing", "last_seen": "2026-05-01T10:00:00Z", "socket_path": "/p1/.kvelmo/worktree.sock"},
		},
	})

	out := captureStdout(t, func() {
		if err := runProjects(ProjectsCmd, nil); err != nil {
			t.Errorf("runProjects: %v", err)
		}
	})
	if !strings.Contains(out, "/p1") {
		t.Errorf("projects output:\n%s", out)
	}
}

func TestRunProjects_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	if err := runProjects(ProjectsCmd, nil); err == nil {
		t.Fatal("expected error with no global socket")
	}
}

func TestRunWorkers_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("workers.list", map[string]any{"workers": []any{
		map[string]any{"id": "w1", "agent": "claude", "status": "idle"},
	}})

	_ = captureStdout(t, func() {
		if err := runWorkers(WorkersCmd, nil); err != nil {
			t.Errorf("runWorkers: %v", err)
		}
	})
}

func TestRunAgentStatus_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("agent.status", map[string]any{"agent_available": true, "checks": []any{}})

	_ = captureStdout(t, func() {
		if err := runAgentStatus(AgentCmd, nil); err != nil {
			t.Errorf("runAgentStatus: %v", err)
		}
	})
}

func TestRunStrategyList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("strategy.list", []string{"react", "plan-execute"})

	out := captureStdout(t, func() {
		if err := runStrategyList(StrategyCmd, nil); err != nil {
			t.Errorf("runStrategyList: %v", err)
		}
	})
	if !strings.Contains(out, "react") {
		t.Errorf("strategy output:\n%s", out)
	}
}

// --- Worktree socket inspect commands ---

func TestRunCheckpoints_WithSocket(t *testing.T) {
	orig := checkpointsJSON
	t.Cleanup(func() { checkpointsJSON = orig })
	checkpointsJSON = false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{
		"checkpoints": []any{
			map[string]any{"sha": "aaaaaaaabbbb", "message": "plan"},
			map[string]any{"sha": "ccccccccdddd", "message": "implement"},
		},
		"redo_stack": []any{map[string]any{"sha": "eeeeeeeeffff"}},
	})

	out := captureStdout(t, func() {
		if err := runCheckpoints(CheckpointsCmd, nil); err != nil {
			t.Errorf("runCheckpoints: %v", err)
		}
	})
	if !strings.Contains(out, "implement") || !strings.Contains(out, "Redo stack") {
		t.Errorf("checkpoints output:\n%s", out)
	}
}

func TestRunCheckpointsGoto_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoint.goto", map[string]any{"sha": "deadbeefcafebabe"})

	out := captureStdout(t, func() {
		if err := runCheckpointsGoto(checkpointsGotoCmd, []string{"deadbeef"}); err != nil {
			t.Errorf("runCheckpointsGoto: %v", err)
		}
	})
	if !strings.Contains(out, "Moved to checkpoint deadbeef") {
		t.Errorf("goto output = %q", out)
	}
}

func TestRunShowSpec_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("show.spec", map[string]any{
		"specifications": []any{
			map[string]any{"path": "spec.md", "content": "Hello spec"},
		},
	})

	out := captureStdout(t, func() {
		if err := runShowSpec(showSpecCmd, nil); err != nil {
			t.Errorf("runShowSpec: %v", err)
		}
	})
	if !strings.Contains(out, "spec.md") || !strings.Contains(out, "Hello spec") {
		t.Errorf("show spec output:\n%s", out)
	}
}

func TestRunShowPlan_Empty(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("show.plan", map[string]any{"specifications": []any{}})

	out := captureStdout(t, func() {
		if err := runShowPlan(showPlanCmd, nil); err != nil {
			t.Errorf("runShowPlan: %v", err)
		}
	})
	if !strings.Contains(out, "No specifications found") {
		t.Errorf("show plan empty output = %q", out)
	}
}

func TestRunTagAddRemoveList_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.tag", map[string]any{"tags": []string{"bug", "ui"}})

	addOut := captureStdout(t, func() {
		if err := runTagAdd(tagAddCmd, []string{"bug", "ui"}); err != nil {
			t.Errorf("runTagAdd: %v", err)
		}
	})
	if !strings.Contains(addOut, "Added tags: bug, ui") {
		t.Errorf("tag add output = %q", addOut)
	}

	rmOut := captureStdout(t, func() {
		if err := runTagRemove(tagRemoveCmd, []string{"bug"}); err != nil {
			t.Errorf("runTagRemove: %v", err)
		}
	})
	if !strings.Contains(rmOut, "Removed tag: bug") {
		t.Errorf("tag remove output = %q", rmOut)
	}

	listOut := captureStdout(t, func() {
		if err := runTagList(tagListCmd, nil); err != nil {
			t.Errorf("runTagList: %v", err)
		}
	})
	if !strings.Contains(listOut, "bug") || !strings.Contains(listOut, "ui") {
		t.Errorf("tag list output = %q", listOut)
	}
}

func TestRunApprove_Event(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("approve", map[string]any{"approved": true})

	out := captureStdout(t, func() {
		if err := runApprove(ApproveCmd, []string{"submit"}); err != nil {
			t.Errorf("runApprove: %v", err)
		}
	})
	if !strings.Contains(out, "Approved: submit") {
		t.Errorf("approve output = %q", out)
	}
}

func TestRunApprove_NodeReject(t *testing.T) {
	if err := ApproveCmd.Flags().Set("node", "review_sql"); err != nil {
		t.Fatal(err)
	}
	if err := ApproveCmd.Flags().Set("reject", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ApproveCmd.Flags().Set("node", "")
		_ = ApproveCmd.Flags().Set("reject", "false")
	})

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("approve.node", map[string]any{"approved": false})

	out := captureStdout(t, func() {
		if err := runApprove(ApproveCmd, nil); err != nil {
			t.Errorf("runApprove node reject: %v", err)
		}
	})
	if !strings.Contains(out, "Rejected node: review_sql") {
		t.Errorf("approve node output = %q", out)
	}
}

func TestRunApprove_RejectWithoutNode(t *testing.T) {
	if err := ApproveCmd.Flags().Set("reject", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ApproveCmd.Flags().Set("reject", "false") })

	if err := runApprove(ApproveCmd, nil); err == nil {
		t.Fatal("expected error for --reject without --node")
	}
}

func TestRunApprove_NoArgs(t *testing.T) {
	if err := runApprove(ApproveCmd, nil); err == nil {
		t.Fatal("expected error when no event and no node")
	}
}

func TestRunChecklist_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.checklist.get", map[string]any{
		"required": []string{"tests pass", "docs updated"},
		"checked":  []string{"tests pass"},
	})

	out := captureStdout(t, func() {
		if err := runChecklist(ChecklistCmd, nil); err != nil {
			t.Errorf("runChecklist: %v", err)
		}
	})
	if !strings.Contains(out, "[x] tests pass") || !strings.Contains(out, "[ ] docs updated") {
		t.Errorf("checklist output:\n%s", out)
	}
}
