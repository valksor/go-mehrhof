package commands

import (
	"strings"
	"testing"
)

// --- config check populated + JSON ---

func TestRunConfigCheck_Drifts(t *testing.T) {
	setBoolPtr(t, &configCheckJSON, false)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("config.check", map[string]any{
		"drifts": []any{
			map[string]any{"path": "workers.max", "expected": 5, "actual": 8},
		},
		"count": 1,
	})

	out := captureStdout(t, func() {
		if err := runConfigCheck(nil, nil); err != nil {
			t.Errorf("runConfigCheck: %v", err)
		}
	})
	if !strings.Contains(out, "Config drift (1") || !strings.Contains(out, "workers.max") {
		t.Errorf("config check drifts output:\n%s", out)
	}
}

func TestRunConfigCheck_JSON(t *testing.T) {
	setBoolPtr(t, &configCheckJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("config.check", map[string]any{"drifts": []any{}, "count": 0})

	out := captureStdout(t, func() {
		if err := runConfigCheck(nil, nil); err != nil {
			t.Errorf("runConfigCheck json: %v", err)
		}
	})
	if !strings.Contains(out, "drifts") {
		t.Errorf("config check json output:\n%s", out)
	}
}

// --- provider test connected / failed ---

func TestRunProviderTest_Connected(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("providers.test", map[string]any{"ok": true, "detail": "authenticated as octocat"})

	out := captureStdout(t, func() {
		if err := runProviderTest(ProviderTestCmd, []string{"github"}); err != nil {
			t.Errorf("runProviderTest connected: %v", err)
		}
	})
	if !strings.Contains(out, "github: connected") || !strings.Contains(out, "octocat") {
		t.Errorf("provider test connected output:\n%s", out)
	}
}

func TestRunProviderTest_Failed(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("providers.test", map[string]any{"ok": false, "error": "401 unauthorized"})

	out := captureStdout(t, func() {
		if err := runProviderTest(ProviderTestCmd, []string{"gitlab"}); err != nil {
			t.Errorf("runProviderTest failed: %v", err)
		}
	})
	if !strings.Contains(out, "gitlab: failed") || !strings.Contains(out, "401") {
		t.Errorf("provider test failed output:\n%s", out)
	}
}

// --- review adversarial + submission + fix ---

func TestRunReview_Adversarial(t *testing.T) {
	if err := ReviewCmd.Flags().Set("adversarial", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ReviewCmd.Flags().Set("adversarial", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("adversarial.run", map[string]any{"started": true, "job_id": "adv-1"})

	out := captureStdout(t, func() {
		if err := runReview(ReviewCmd, nil); err != nil {
			t.Errorf("runReview adversarial: %v", err)
		}
	})
	if !strings.Contains(out, "started") {
		t.Errorf("review adversarial output:\n%s", out)
	}
}

func TestRunReview_SubmitFix(t *testing.T) {
	if err := ReviewCmd.Flags().Set("fix", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ReviewCmd.Flags().Set("fix", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("review", map[string]any{"job_id": "rev-1", "status": "reviewing"})

	out := captureStdout(t, func() {
		if err := runReview(ReviewCmd, nil); err != nil {
			t.Errorf("runReview submit fix: %v", err)
		}
	})
	if !strings.Contains(out, "Fix mode enabled") {
		t.Errorf("review fix output:\n%s", out)
	}
}

// --- submit json + malformed response fallback ---

func TestRunSubmit_JSON(t *testing.T) {
	if err := SubmitCmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SubmitCmd.Flags().Set("json", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("submit", map[string]any{"url": "https://x/pr/1"})

	out := captureStdout(t, func() {
		if err := runSubmit(SubmitCmd, nil); err != nil {
			t.Errorf("runSubmit json: %v", err)
		}
	})
	if !strings.Contains(out, "https://x/pr/1") {
		t.Errorf("submit json output:\n%s", out)
	}
}

func TestRunSubmit_Sections(t *testing.T) {
	if err := SubmitCmd.Flags().Set("section", "Testing=manual"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SubmitCmd.Flags().Set("section", "") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("submit", map[string]any{"url": "https://x/pr/2", "title": "PR2"})

	out := captureStdout(t, func() {
		if err := runSubmit(SubmitCmd, nil); err != nil {
			t.Errorf("runSubmit sections: %v", err)
		}
	})
	if !strings.Contains(out, "PR created") {
		t.Errorf("submit sections output:\n%s", out)
	}
}

// --- batch json + filters ---

func TestRunBatch_JSON(t *testing.T) {
	setBoolPtr(t, &batchJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("tasks.batch", map[string]any{"action": "status", "total": 0, "results": []any{}})

	out := captureStdout(t, func() {
		if err := runBatch(BatchCmd, []string{"status"}); err != nil {
			t.Errorf("runBatch json: %v", err)
		}
	})
	if !strings.Contains(out, "results") {
		t.Errorf("batch json output:\n%s", out)
	}
}

// --- stats history json ---

func TestRunStatsHistory_JSON(t *testing.T) {
	origH, origJSON := statsHistory, statsJSON
	t.Cleanup(func() { statsHistory, statsJSON = origH, origJSON })
	statsHistory, statsJSON = true, true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("metrics.history", map[string]any{
		"enabled": true,
		"entries": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "rpc_requests": 1},
		},
	})

	out := captureStdout(t, func() {
		if err := runStats(StatsCmd, nil); err != nil {
			t.Errorf("runStats history json: %v", err)
		}
	})
	if !strings.Contains(out, "rpc_requests") {
		t.Errorf("stats history json output:\n%s", out)
	}
}

// --- codegraph index/search/callers/deps JSON + populated ---

func TestRunCodegraphCallers_JSON(t *testing.T) {
	setBoolPtr(t, &codegraphJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.callers", map[string]any{"callers": []any{}})

	out := captureStdout(t, func() {
		if err := runCodegraphCallers(nil, []string{"Foo"}); err != nil {
			t.Errorf("runCodegraphCallers json: %v", err)
		}
	})
	if !strings.Contains(out, "callers") {
		t.Errorf("codegraph callers json output:\n%s", out)
	}
}

func TestRunCodegraphDeps_JSON(t *testing.T) {
	setBoolPtr(t, &codegraphJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.deps", map[string]any{"deps": []any{}})

	out := captureStdout(t, func() {
		if err := runCodegraphDeps(nil, []string{"pkg"}); err != nil {
			t.Errorf("runCodegraphDeps json: %v", err)
		}
	})
	if !strings.Contains(out, "deps") {
		t.Errorf("codegraph deps json output:\n%s", out)
	}
}

func TestRunCodegraphIndex_Populated(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("codegraph.index", map[string]any{"indexed": 50, "files": 12, "symbols": 200, "edges": 300})

	// runCodegraphIndex reports progress via a spinner (stderr); assert the
	// happy path returns nil rather than checking stdout.
	if err := runCodegraphIndex(CodegraphCmd, nil); err != nil {
		t.Errorf("runCodegraphIndex: %v", err)
	}
}

// --- memory stats JSON ---

func TestRunMemoryStats_JSON(t *testing.T) {
	setBoolPtr(t, &memoryStatsJSON, true)

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.stats", map[string]any{"count": 5, "size_bytes": 1024})

	out := captureStdout(t, func() {
		if err := runMemoryStats(memoryStatsCmd, nil); err != nil {
			t.Errorf("runMemoryStats json: %v", err)
		}
	})
	if !strings.Contains(out, "count") {
		t.Errorf("memory stats json output:\n%s", out)
	}
}

// --- diff JSON ---

func TestRunDiff_JSON(t *testing.T) {
	setBoolPtr(t, &diffJSON, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{"checkpoints": []any{}})
	stub.SetResponse("git.diff", map[string]any{"diff": "+x"})

	out := captureStdout(t, func() {
		if err := runDiff(DiffCmd, nil); err != nil {
			t.Errorf("runDiff json: %v", err)
		}
	})
	if !strings.Contains(out, "diff") {
		t.Errorf("diff json output:\n%s", out)
	}
}

// --- show spec via full command execution (so persistent --json merges) ---

func TestShowSpec_ExecuteJSON(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("show.spec", map[string]any{"specifications": []any{
		map[string]any{"path": "s.md", "content": "x"},
	}})

	out := captureStdout(t, func() {
		ShowCmd.SetArgs([]string{"spec", "--json"})
		if err := ShowCmd.Execute(); err != nil {
			t.Errorf("show spec --json execute: %v", err)
		}
	})
	t.Cleanup(func() { ShowCmd.SetArgs(nil); _ = ShowCmd.PersistentFlags().Set("json", "false") })
	if !strings.Contains(out, "specifications") {
		t.Errorf("show spec json output:\n%s", out)
	}
}
