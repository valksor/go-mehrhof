package commands

import (
	"strings"
	"testing"
)

// --- review list populated ---

func TestRunReviewList_Populated2(t *testing.T) {
	setBoolPtr(t, &reviewListJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.list", map[string]any{
		"reviews": []any{
			map[string]any{"number": 1, "title": "First review", "status": "approved", "created_at": "2026-05-01T10:00:00Z"},
			map[string]any{"number": 2, "title": "Second", "status": "rejected"},
		},
	})

	out := captureStdout(t, func() {
		if err := runReviewList(ReviewCmd, nil); err != nil {
			t.Errorf("runReviewList: %v", err)
		}
	})
	if !strings.Contains(out, "First review") || !strings.Contains(out, "Approved") {
		t.Errorf("review list populated output:\n%s", out)
	}
}

// --- review view content ---

func TestRunReviewView_FullContent(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.view", map[string]any{
		"number": 1, "title": "My Review", "status": "approved", "content": "Looks good to me",
		"reviewer": "alice", "created_at": "2026-05-01T10:00:00Z",
	})

	out := captureStdout(t, func() {
		if err := runReviewView(reviewViewCmd, []string{"1"}); err != nil {
			t.Errorf("runReviewView: %v", err)
		}
	})
	if !strings.Contains(out, "Looks good to me") {
		t.Errorf("review view content output:\n%s", out)
	}
}

// --- queue add with title ---

func TestRunQueueAdd_WithTitle(t *testing.T) {
	orig := queueAddTitle
	t.Cleanup(func() { queueAddTitle = orig })
	queueAddTitle = "My Task"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("queue.add", map[string]any{"queued": true, "id": "q1", "position": 1})

	out := captureStdout(t, func() {
		if err := runQueueAdd(queueAddCmd, []string{"github:org/repo#1"}); err != nil {
			t.Errorf("runQueueAdd: %v", err)
		}
	})
	if out == "" {
		t.Error("queue add produced no output")
	}
}

// --- projects verbose ---

func TestRunProjects_Verbose(t *testing.T) {
	origV := projectsVerbose
	t.Cleanup(func() { projectsVerbose = origV })
	projectsVerbose = true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("projects.list", map[string]any{
		"projects": []any{
			map[string]any{"path": "/p1", "state": "implementing", "last_seen": "2026-05-01T10:00:00Z", "socket_path": "/p1/.kvelmo/worktree.sock"},
		},
	})

	out := captureStdout(t, func() {
		if err := runProjects(ProjectsCmd, nil); err != nil {
			t.Errorf("runProjects verbose: %v", err)
		}
	})
	if !strings.Contains(out, "Socket:") {
		t.Errorf("projects verbose output:\n%s", out)
	}
}

// --- memory stats populated display ---

func TestRunMemoryStats_Display(t *testing.T) {
	setBoolPtr(t, &memoryStatsJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.stats", map[string]any{
		"total_documents": 42, "total_size_bytes": 8192,
		"by_type": map[string]any{"decision": 10, "outcome": 32},
	})

	out := captureStdout(t, func() {
		if err := runMemoryStats(memoryStatsCmd, nil); err != nil {
			t.Errorf("runMemoryStats: %v", err)
		}
	})
	if out == "" {
		t.Error("memory stats display produced no output")
	}
}

// --- update populated ---

func TestRunUpdate_Updated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("update", map[string]any{"updated": true, "changes": []any{"title changed"}})

	out := captureStdout(t, func() {
		if err := runUpdate(UpdateCmd, nil); err != nil {
			t.Errorf("runUpdate: %v", err)
		}
	})
	if out == "" {
		t.Error("update produced no output")
	}
}

// --- status skip-phases + pending prompt ---

func TestRunStatus_SkipPhasesPrompt(t *testing.T) {
	origF, origB, origA := statusFailed, statusBlocked, statusAll
	t.Cleanup(func() { statusFailed, statusBlocked, statusAll = origF, origB, origA })
	statusFailed, statusBlocked, statusAll = false, false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{
		"state": "implementing", "path": "/p",
		"skip_phases":       []string{"simplify", "optimize"},
		"pending_prompt_id": "p1",
		"active_job_id":     "job-1",
	})

	out := captureStdout(t, func() {
		if err := runStatus(StatusCmd, nil); err != nil {
			t.Errorf("runStatus skip-phases: %v", err)
		}
	})
	if !strings.Contains(out, "Skip:") || !strings.Contains(out, "Quality gate") {
		t.Errorf("status skip-phases output:\n%s", out)
	}
}

// --- stats history multi-entry display (>5 entries triggers tail) ---

func TestRunStatsHistory_ManyEntries(t *testing.T) {
	origH, origJSON := statsHistory, statsJSON
	t.Cleanup(func() { statsHistory, statsJSON = origH, origJSON })
	statsHistory, statsJSON = true, false

	entries := make([]any, 0, 7)
	for i := range 7 {
		entries = append(entries, map[string]any{
			"timestamp": "2026-05-0" + string(rune('1'+i)) + "T10:00:00Z", "rpc_requests": i, "jobs_completed": i, "rpc_errors": 0,
		})
	}

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("metrics.history", map[string]any{"enabled": true, "entries": entries})

	out := captureStdout(t, func() {
		if err := runStats(StatsCmd, nil); err != nil {
			t.Errorf("runStats history many: %v", err)
		}
	})
	if !strings.Contains(out, "7 entries") {
		t.Errorf("stats history many output:\n%s", out)
	}
}
