package commands

import (
	"strings"
	"testing"
)

// --- strategy list: json / empty / populated ---

func TestRunStrategyList_JSON(t *testing.T) {
	setBoolPtr(t, &strategyJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("strategy.list", []any{"react", "plan-execute"})

	out := captureStdout(t, func() {
		if err := runStrategyList(StrategyCmd, nil); err != nil {
			t.Errorf("runStrategyList json: %v", err)
		}
	})
	if !strings.Contains(out, "react") {
		t.Errorf("strategy list json output:\n%s", out)
	}
}

func TestRunStrategyList_Populated(t *testing.T) {
	setBoolPtr(t, &strategyJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("strategy.list", []any{"react", "reflexion"})

	out := captureStdout(t, func() {
		if err := runStrategyList(StrategyCmd, nil); err != nil {
			t.Errorf("runStrategyList populated: %v", err)
		}
	})
	if !strings.Contains(out, "Available strategies") || !strings.Contains(out, "reflexion") {
		t.Errorf("strategy list populated output:\n%s", out)
	}
}

func TestRunStrategyList_Empty(t *testing.T) {
	setBoolPtr(t, &strategyJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("strategy.list", []any{})

	out := captureStdout(t, func() {
		if err := runStrategyList(StrategyCmd, nil); err != nil {
			t.Errorf("runStrategyList empty: %v", err)
		}
	})
	if !strings.Contains(out, "No strategies registered") {
		t.Errorf("strategy list empty output = %q", out)
	}
}

// --- workers: json + populated display (working + current job) ---

func TestRunWorkers_JSON(t *testing.T) {
	setBoolPtr(t, &workersJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("workers.list", map[string]any{"workers": []any{}})

	out := captureStdout(t, func() {
		if err := runWorkers(WorkersCmd, nil); err != nil {
			t.Errorf("runWorkers json: %v", err)
		}
	})
	if !strings.Contains(out, "workers") {
		t.Errorf("workers json output:\n%s", out)
	}
}

func TestRunWorkers_PopulatedWorking(t *testing.T) {
	setBoolPtr(t, &workersJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("workers.list", map[string]any{
		"workers": []any{
			map[string]any{"id": "w1", "status": "working", "current_job": "job-9"},
			map[string]any{"id": "w2", "status": "idle"},
		},
		"stats": map[string]any{"working_workers": 1, "total_workers": 2, "queued_jobs": 0},
	})

	out := captureStdout(t, func() {
		if err := runWorkers(WorkersCmd, nil); err != nil {
			t.Errorf("runWorkers populated: %v", err)
		}
	})
	if !strings.Contains(out, "job-9") || !strings.Contains(out, "w1") {
		t.Errorf("workers populated output:\n%s", out)
	}
}

// --- quality: json branch ---

func TestRunQuality_JSON(t *testing.T) {
	setBoolPtr(t, &qualityJSON, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("autofix.status", map[string]any{"attempts": 1})

	out := captureStdout(t, func() {
		if err := runQuality(QualityCmd, nil); err != nil {
			t.Errorf("runQuality json: %v", err)
		}
	})
	if !strings.Contains(out, "attempts") {
		t.Errorf("quality json output:\n%s", out)
	}
}

// --- discover: json branch ---

func TestRunDiscover_JSON(t *testing.T) {
	setBoolPtr(t, &discoveryJSON, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("discovery.scan", map[string]any{"commands": []any{}})

	out := captureStdout(t, func() {
		if err := runDiscover(DiscoverCmd, nil); err != nil {
			t.Errorf("runDiscover json: %v", err)
		}
	})
	if !strings.Contains(out, "commands") {
		t.Errorf("discover json output:\n%s", out)
	}
}

// --- codegraph stats / index: json branch ---

func TestRunCodegraphStats_JSON(t *testing.T) {
	setBoolPtr(t, &codegraphJSON, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.stats", map[string]any{"symbols": 3})

	out := captureStdout(t, func() {
		if err := runCodegraphStats(nil, nil); err != nil {
			t.Errorf("runCodegraphStats json: %v", err)
		}
	})
	if !strings.Contains(out, "symbols") {
		t.Errorf("codegraph stats json output:\n%s", out)
	}
}

func TestRunCodegraphIndex_JSON(t *testing.T) {
	setBoolPtr(t, &codegraphJSON, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.index", map[string]any{"files": 2, "symbols": 5})

	out := captureStdout(t, func() {
		if err := runCodegraphIndex(CodegraphCmd, []string{"pkg/path"}); err != nil {
			t.Errorf("runCodegraphIndex json: %v", err)
		}
	})
	if !strings.Contains(out, "files") {
		t.Errorf("codegraph index json output:\n%s", out)
	}
}

// --- codegraph deps: populated ---

func TestRunCodegraphDeps_Populated(t *testing.T) {
	setBoolPtr(t, &codegraphJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.deps", map[string]any{"dependencies": []any{"fmt", "os"}})

	out := captureStdout(t, func() {
		if err := runCodegraphDeps(nil, []string{"mypkg"}); err != nil {
			t.Errorf("runCodegraphDeps populated: %v", err)
		}
	})
	if !strings.Contains(out, "Dependencies of") || !strings.Contains(out, "fmt") {
		t.Errorf("codegraph deps populated output:\n%s", out)
	}
}

// --- implement: --json branch (no agent spawn; result returned directly) ---

func TestRunImplement_JSON(t *testing.T) {
	setBoolPtr(t, &implementJSON, true)
	t.Cleanup(func() { implementJSON = false })
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("implement", map[string]any{"state": "implemented"})

	out := captureStdout(t, func() {
		if err := runImplement(ImplementCmd, nil); err != nil {
			t.Errorf("runImplement json: %v", err)
		}
	})
	if !strings.Contains(out, "state") {
		t.Errorf("implement json output:\n%s", out)
	}
}

// --- review list: --json branch ---

func TestRunReviewList_JSON(t *testing.T) {
	setBoolPtr(t, &reviewListJSON, true)
	t.Cleanup(func() { reviewListJSON = false })
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review.list", map[string]any{"reviews": []any{}})

	out := captureStdout(t, func() {
		if err := runReviewList(ReviewCmd, nil); err != nil {
			t.Errorf("runReviewList json: %v", err)
		}
	})
	if !strings.Contains(out, "reviews") {
		t.Errorf("review list json output:\n%s", out)
	}
}

// --- remote approve / merge: --json branch ---

func TestRunRemoteApprove_JSON(t *testing.T) {
	if err := RemoteApproveCmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RemoteApproveCmd.Flags().Set("json", "false") })
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("remote.approve", map[string]any{"approved": true})

	out := captureStdout(t, func() {
		if err := runRemoteApprove(RemoteApproveCmd, nil); err != nil {
			t.Errorf("runRemoteApprove json: %v", err)
		}
	})
	if !strings.Contains(out, "approved") {
		t.Errorf("remote approve json output:\n%s", out)
	}
}

func TestRunRemoteMerge_JSON(t *testing.T) {
	if err := RemoteMergeCmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = RemoteMergeCmd.Flags().Set("json", "false") })
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("remote.merge", map[string]any{"merged": true})

	out := captureStdout(t, func() {
		if err := runRemoteMerge(RemoteMergeCmd, nil); err != nil {
			t.Errorf("runRemoteMerge json: %v", err)
		}
	})
	if !strings.Contains(out, "merged") {
		t.Errorf("remote merge json output:\n%s", out)
	}
}

// --- plan --dry-run / simplify --dry-run param branches ---

func TestRunPlan_DryRun(t *testing.T) {
	setBoolPtr(t, &planDryRun, true)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("plan", map[string]any{"state": "planned"})

	out := captureStdout(t, func() {
		if err := runPlan(PlanCmd, nil); err != nil {
			t.Errorf("runPlan dry-run: %v", err)
		}
	})
	if out == "" {
		t.Error("plan dry-run produced no output")
	}
}

func TestRunSimplify_DryRun(t *testing.T) {
	setBoolPtr(t, &simplifyDryRun, true)
	setBoolPtr(t, &simplifyWait, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("simplify", map[string]any{"state": "simplified"})

	out := captureStdout(t, func() {
		if err := runSimplify(SimplifyCmd, nil); err != nil {
			t.Errorf("runSimplify dry-run: %v", err)
		}
	})
	if out == "" {
		t.Error("simplify dry-run produced no output")
	}
}

// --- batch with state/tag/match filters ---

func TestRunBatch_Filters(t *testing.T) {
	if err := BatchCmd.Flags().Set("state", "implementing"); err != nil {
		t.Fatal(err)
	}
	if err := BatchCmd.Flags().Set("tag", "backend"); err != nil {
		t.Fatal(err)
	}
	if err := BatchCmd.Flags().Set("match", "proj"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = BatchCmd.Flags().Set("state", "")
		_ = BatchCmd.Flags().Set("tag", "")
		_ = BatchCmd.Flags().Set("match", "")
	})

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.batch", map[string]any{
		"action": "status",
		"total":  1,
		"results": []any{
			map[string]any{"path": "/p1", "state": "implementing", "success": true},
		},
	})

	out := captureStdout(t, func() {
		if err := runBatch(BatchCmd, []string{"status"}); err != nil {
			t.Errorf("runBatch filters: %v", err)
		}
	})
	if !strings.Contains(out, "/p1") {
		t.Errorf("batch with filters output:\n%s", out)
	}
}

// --- eventlog with type/phase/since filters ---

func TestRunEventlog_Filters(t *testing.T) {
	setBoolPtr(t, &eventlogJSON, false)
	if err := EventlogCmd.Flags().Set("type", "phase_started"); err != nil {
		t.Fatal(err)
	}
	if err := EventlogCmd.Flags().Set("phase", "plan"); err != nil {
		t.Fatal(err)
	}
	if err := EventlogCmd.Flags().Set("since", "1h"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = EventlogCmd.Flags().Set("type", "")
		_ = EventlogCmd.Flags().Set("phase", "")
		_ = EventlogCmd.Flags().Set("since", "")
	})

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("eventlog.query", map[string]any{
		"entries": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "type": "phase_started", "phase": "plan", "message": ""},
		},
		"total": 1,
	})

	out := captureStdout(t, func() {
		if err := runEventlog(EventlogCmd, nil); err != nil {
			t.Errorf("runEventlog filters: %v", err)
		}
	})
	if out == "" {
		t.Error("eventlog with filters produced no output")
	}
}
