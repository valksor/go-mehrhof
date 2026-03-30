package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/worker"
)

const testAgentClaude = "claude"

// newTestGlobalSocketWithPool2 creates a GlobalSocket with a real worker pool for testing.
func newTestGlobalSocketWithPool2(t *testing.T) *GlobalSocket {
	t.Helper()
	pool := worker.NewPool(worker.PoolConfig{MaxWorkers: 2})

	return NewGlobalSocketWithPool(filepath.Join(t.TempDir(), "global.sock"), pool)
}

// mustMarshal marshals v and fatals on error.
func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	return data
}

func TestGlobalHandleListWorkers_WithPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	resp, err := g.handleListWorkers(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}

	var result WorkersListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Workers == nil {
		t.Error("expected non-nil workers list")
	}
}

func TestGlobalHandleListJobs_WithPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	resp, err := g.handleListJobs(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["jobs"]; !ok {
		t.Error("expected 'jobs' key in result")
	}
}

func TestGlobalHandleAddWorker_WithPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	params := mustMarshal(t, AddWorkerParams{Agent: testAgentClaude})
	resp, err := g.handleAddWorker(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}

	var result WorkerInfo
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ID == "" {
		t.Error("expected non-empty worker ID")
	}
	if result.AgentName != testAgentClaude {
		t.Errorf("agent = %q, want %q", result.AgentName, testAgentClaude)
	}
}

func TestGlobalHandleAddWorker_DefaultAgent(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	// Empty agent should default to "claude"
	params := mustMarshal(t, AddWorkerParams{Agent: ""})
	resp, err := g.handleAddWorker(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("error: %s", resp.Error.Message)
	}

	var result WorkerInfo
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.AgentName != testAgentClaude {
		t.Errorf("agent = %q, want %q (default)", result.AgentName, testAgentClaude)
	}
}

func TestGlobalHandleAddWorker_ExceedMaxWorkers(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t) // max 2 workers

	// Add 2 workers to fill the pool
	for i := range 2 {
		params := mustMarshal(t, AddWorkerParams{Agent: testAgentClaude})
		resp, err := g.handleAddWorker(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Error != nil {
			t.Fatalf("add worker %d: %s", i, resp.Error.Message)
		}
	}

	// Adding a 3rd should fail
	params := mustMarshal(t, AddWorkerParams{Agent: testAgentClaude})
	resp, err := g.handleAddWorker(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error when exceeding max workers")
	}
}

func TestGlobalHandleRemoveWorker_NonexistentWorker(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	params := mustMarshal(t, RemoveWorkerParams{ID: "nonexistent"})
	resp, err := g.handleRemoveWorker(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent worker")
	}
}

func TestGlobalHandleRemoveWorker_AddThenRemove(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocketWithPool2(t)

	// Add a worker
	addParams := mustMarshal(t, AddWorkerParams{Agent: testAgentClaude})
	addResp, err := g.handleAddWorker(ctx, &Request{ID: "1", Params: addParams})
	if err != nil {
		t.Fatal(err)
	}
	if addResp.Error != nil {
		t.Fatalf("add error: %s", addResp.Error.Message)
	}

	var workerInfo WorkerInfo
	if err := json.Unmarshal(addResp.Result, &workerInfo); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Remove the worker
	removeParams := mustMarshal(t, RemoveWorkerParams{ID: workerInfo.ID})
	removeResp, err := g.handleRemoveWorker(ctx, &Request{ID: "2", Params: removeParams})
	if err != nil {
		t.Fatal(err)
	}
	if removeResp.Error != nil {
		t.Fatalf("remove error: %s", removeResp.Error.Message)
	}
}
