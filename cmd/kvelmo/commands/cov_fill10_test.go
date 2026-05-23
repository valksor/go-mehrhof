package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/recorder"
)

// --- update: changed + new specification branch ---

func TestRunUpdate_ChangedWithSpec(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("update", map[string]any{
		"status":            "ok",
		"changed":           true,
		"new_specification": "delta-spec.md",
	})

	out := captureStdout(t, func() {
		if err := runUpdate(UpdateCmd, nil); err != nil {
			t.Errorf("runUpdate changed: %v", err)
		}
	})
	if !strings.Contains(out, "Task updated") || !strings.Contains(out, "Delta specification: delta-spec.md") {
		t.Errorf("update changed output:\n%s", out)
	}
}

// --- approve: approve a node (not reject) ---

func TestRunApprove_NodeApprove(t *testing.T) {
	if err := ApproveCmd.Flags().Set("node", "n1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ApproveCmd.Flags().Set("node", "") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("approve.node", map[string]any{"approved": true})

	out := captureStdout(t, func() {
		if err := runApprove(ApproveCmd, nil); err != nil {
			t.Errorf("runApprove node: %v", err)
		}
	})
	if !strings.Contains(out, "Approved node: n1") {
		t.Errorf("approve node output = %q", out)
	}
}

// --- memory search: types filter + empty results ---

func TestRunMemorySearch_TypesFilterEmpty(t *testing.T) {
	if err := memorySearchCmd.Flags().Set("types", "specification,decision"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memorySearchCmd.Flags().Set("types", "") })

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{"results": []any{}})

	out := captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"auth"}); err != nil {
			t.Errorf("runMemorySearch types: %v", err)
		}
	})
	if out == "" {
		t.Error("memory search with types produced no output")
	}
}

// --- memory clear: failure branch (ok=false) ---

func TestRunMemoryClear_NotCleared(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.clear", map[string]any{"ok": false})

	out := captureStdout(t, func() {
		if err := runMemoryClear(memoryClearCmd, nil); err != nil {
			t.Errorf("runMemoryClear not-cleared: %v", err)
		}
	})
	if out == "" {
		t.Error("memory clear (not cleared) produced no output")
	}
}

// --- recap: phase metrics present but none match standard phases -> "none" ---

func TestPrintRecap_PhasesNone(t *testing.T) {
	out := captureStdout(t, func() {
		printRecapWithUnknownPhase(t)
	})
	if !strings.Contains(out, "none") {
		t.Errorf("recap phases-none output:\n%s", out)
	}
}

func printRecapWithUnknownPhase(t *testing.T) {
	t.Helper()
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("recap", map[string]any{
		"state":            "implementing",
		"path":             "/p",
		"checkpoint_count": 1,
		"next_action":      "keep going",
		// A phase key not in {plan,implement,simplify,optimize,review}.
		"phase_metrics": map[string]any{"mystery_phase": map[string]any{}},
	})
	if err := runRecap(RecapCmd, nil); err != nil {
		t.Errorf("runRecap phases-none: %v", err)
	}
}

// --- show spec: multiple specifications -> separator branch ---

func TestRunShowSpec_MultipleSpecs(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("show.spec", map[string]any{
		"specifications": []any{
			map[string]any{"path": "spec-a.md", "content": "alpha"},
			map[string]any{"path": "spec-b.md", "content": "beta"},
		},
	})

	out := captureStdout(t, func() {
		if err := runShowSpec(showSpecCmd, nil); err != nil {
			t.Errorf("runShowSpec multi: %v", err)
		}
	})
	if !strings.Contains(out, "spec-a.md") || !strings.Contains(out, "spec-b.md") || !strings.Contains(out, "---") {
		t.Errorf("show spec multi output:\n%s", out)
	}
}

// --- agent status: agent not available ---

func TestRunAgentStatus_NotAvailable(t *testing.T) {
	setBoolPtr(t, &agentStatusJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("agent.status", map[string]any{"agent_available": false, "checks": []any{}})

	out := captureStdout(t, func() {
		if err := runAgentStatus(AgentCmd, nil); err != nil {
			t.Errorf("runAgentStatus not-available: %v", err)
		}
	})
	if !strings.Contains(out, "not available") {
		t.Errorf("agent not-available output:\n%s", out)
	}
}

// --- recordings replay: inbound direction, long-line truncation, invalid event JSON ---

func TestRunRecordingsReplay_EdgeCases(t *testing.T) {
	origFilter := recordingsTypeFilter
	t.Cleanup(func() { recordingsTypeFilter = origFilter })
	recordingsTypeFilter = ""

	dir := t.TempDir()
	path := filepath.Join(dir, "edge.jsonl")

	now := time.Now().UTC()
	header := recorder.Header{JobID: "job-1", Agent: "claude", Model: "test", StartedAt: now}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	longContent, err := json.Marshal(map[string]any{"content": strings.Repeat("Z", 200)})
	if err != nil {
		t.Fatalf("marshal long content: %v", err)
	}

	records := []recorder.Record{
		{Timestamp: now, JobID: "job-1", Direction: recorder.Outbound, Type: "header", Event: headerJSON},
		// Inbound record -> "IN " direction branch.
		{Timestamp: now, JobID: "job-1", Direction: recorder.Inbound, Type: "prompt", Event: json.RawMessage(`{"q":"hi"}`)},
		// Long event content -> truncation branch.
		{Timestamp: now, JobID: "job-1", Direction: recorder.Outbound, Type: "stream", Event: longContent},
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create recording: %v", err)
	}
	enc := json.NewEncoder(f)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	_ = f.Close()

	out := captureStdout(t, func() {
		if err := runRecordingsReplay(recordingsReplayCmd, []string{path}); err != nil {
			t.Errorf("runRecordingsReplay edge: %v", err)
		}
	})
	if !strings.Contains(out, "[IN ]") || !strings.Contains(out, "...") {
		t.Errorf("recordings replay edge output:\n%s", out)
	}
}
