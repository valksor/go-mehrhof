package socket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/browser"
	"github.com/valksor/kvelmo/pkg/conductor"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/screenshot"
	"github.com/valksor/kvelmo/pkg/worker"
)

// helper: create a GlobalSocket suitable for direct handler testing.
// Uses a temp dir so it doesn't load any real projects.json from the workspace.
func newTestGlobalSocket(t *testing.T) *GlobalSocket {
	t.Helper()

	return NewGlobalSocket(filepath.Join(t.TempDir(), "global.sock"))
}

// ============================================================
// WorktreeSocket handler tests
// ============================================================

func TestWorktreeHandlePing(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handlePing(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handlePing() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handlePing() returned error: %s", resp.Error.Message)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
}

func TestWorktreeHandleAbort_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleAbort(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAbort() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleAbort() with nil conductor should return error response")
	}
}

func TestWorktreeHandleReset_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleReset(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReset() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleReset() with nil conductor should return error response")
	}
}

func TestWorktreeHandleUndo_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleUndo(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleUndo() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleUndo() with nil conductor should return error response")
	}
}

func TestWorktreeHandleRedo_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleRedo(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRedo() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleRedo() with nil conductor should return error response")
	}
}

func TestWorktreeHandleCheckpoints_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleCheckpoints(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpoints() error = %v", err)
	}
	if resp.Error != nil {
		t.Errorf("handleCheckpoints() with nil conductor should return empty list, got error: %s", resp.Error.Message)
	}
}

func TestWorktreeHandleCheckpoints_NoWorkUnit(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// Conductor has no work unit set

	resp, err := w.handleCheckpoints(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpoints() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleCheckpoints() returned error: %s", resp.Error.Message)
	}
	// Should return empty checkpoint arrays
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["checkpoints"]; !ok {
		t.Error("result should have 'checkpoints' key")
	}
}

func TestWorktreeHandleCheckpointGoto_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	params, err := json.Marshal(map[string]string{"sha": "abc123"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleCheckpointGoto() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleCheckpointGoto() with nil conductor should return error response")
	}
}

func TestWorktreeHandleCheckpointGoto_MissingSHA(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	// Empty SHA should return invalid params error
	params, err := json.Marshal(map[string]string{"sha": ""})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleCheckpointGoto() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleCheckpointGoto() with empty SHA should return error response")
	}
}

func TestWorktreeHandleReviewList_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleReviewList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReviewList() error = %v", err)
	}
	// With nil conductor, should return empty list (not error) for basic sockets
	if resp.Error != nil {
		t.Errorf("handleReviewList() with nil conductor should return empty list, got error: %v", resp.Error)
	}
	if resp.Result == nil {
		t.Error("handleReviewList() with nil conductor should return result with empty reviews")
	}
}

func TestWorktreeHandleReviewList_EmptyResult(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// Conductor exists but no task loaded — ListReviews should return empty list

	resp, err := w.handleReviewList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReviewList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleReviewList() returned error: %s", resp.Error.Message)
	}
}

func TestWorktreeHandleReviewView_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleReviewView(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReviewView() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleReviewView() with nil conductor should return error response")
	}
}

func TestWorktreeHandleReviewView_NotFound(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, err := json.Marshal(ReviewViewParams{Number: 999})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleReviewView(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleReviewView() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleReviewView() with non-existent review number should return error")
	}
}

func TestWorktreeHandleGitStatus_NilRepo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t) // no repo configured

	resp, err := w.handleGitStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleGitStatus() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGitStatus() with nil repo should return error response")
	}
}

func TestWorktreeHandleGitDiff_NilRepo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleGitDiff(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleGitDiff() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGitDiff() with nil repo should return error response")
	}
}

func TestWorktreeHandleGitLog_NilRepo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleGitLog(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleGitLog() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGitLog() with nil repo should return error response")
	}
}

// ============================================================
// GlobalSocket handler tests
// ============================================================

func TestGlobalHandlePing(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handlePing(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handlePing() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handlePing() returned error: %s", resp.Error.Message)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
}

func TestGlobalHandleDocsURL(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleDocsURL(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDocsURL() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDocsURL() returned error: %s", resp.Error.Message)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["url"] == "" {
		t.Error("url should not be empty")
	}
	if result["version"] == "" {
		t.Error("version should not be empty")
	}
}

func TestGlobalHandleListWorkers_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t) // nil pool

	resp, err := g.handleListWorkers(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleListWorkers() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleListWorkers() returned error: %s", resp.Error.Message)
	}
	var result WorkersListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Workers) != 0 {
		t.Errorf("workers = %d, want 0", len(result.Workers))
	}
}

func TestGlobalHandleWorkerStats_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleWorkerStats(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleWorkerStats() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleWorkerStats() returned error: %s", resp.Error.Message)
	}
	var result WorkersStats
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.TotalWorkers != 0 {
		t.Errorf("TotalWorkers = %d, want 0", result.TotalWorkers)
	}
}

func TestGlobalHandleAddWorker_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(map[string]string{"agent": "claude", "model": "sonnet"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleAddWorker(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleAddWorker() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleAddWorker() with nil pool should return error response")
	}
}

func TestGlobalHandleRemoveWorker_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(map[string]string{"id": "worker-1"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleRemoveWorker(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRemoveWorker() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleRemoveWorker() with nil pool should return error response")
	}
}

func TestGlobalHandleListJobs_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleListJobs(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleListJobs() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleListJobs() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["jobs"]; !ok {
		t.Error("result should have 'jobs' key")
	}
}

func TestGlobalHandleGetJob_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(map[string]string{"id": "job-123"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleGetJob() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGetJob() with nil pool should return error response")
	}
}

func TestGlobalHandleUnregisterProject(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// Register a project first
	regParams, err := json.Marshal(RegisterParams{Path: "/test/project", SocketPath: "/test.sock"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	regResp, err := g.handleRegisterProject(ctx, &Request{ID: "1", Params: regParams})
	if err != nil {
		t.Fatalf("handleRegisterProject() error = %v", err)
	}
	if regResp.Error != nil {
		t.Fatalf("handleRegisterProject() error response: %s", regResp.Error.Message)
	}
	var regResult map[string]string
	if err := json.Unmarshal(regResp.Result, &regResult); err != nil {
		t.Fatalf("unmarshal register result: %v", err)
	}
	projectID := regResult["id"]
	if projectID == "" {
		t.Fatal("expected non-empty project id")
	}

	// Unregister the project
	unregParams, err := json.Marshal(UnregisterParams{ID: projectID})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleUnregisterProject(ctx, &Request{ID: "2", Params: unregParams})
	if err != nil {
		t.Fatalf("handleUnregisterProject() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleUnregisterProject() returned error: %s", resp.Error.Message)
	}
}

func TestGlobalHandleSettingsGet(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleSettingsGet(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSettingsGet() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleSettingsGet() returned error: %s", resp.Error.Message)
	}
	// Should return a settings response with at least a schema
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["effective"]; !ok {
		t.Error("result should have 'effective' key")
	}
}

func TestGlobalHandleBrowse_ValidPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	tmpDir := t.TempDir()

	// Register tmpDir as a worktree to allow browsing
	g.mu.Lock()
	g.worktrees["test"] = &WorktreeInfo{ID: "test", Path: tmpDir}
	g.mu.Unlock()

	params, err := json.Marshal(BrowseParams{Path: tmpDir})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowse(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowse() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBrowse() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["entries"]; !ok {
		t.Error("result should have 'entries' key")
	}
}

func TestGlobalHandleBrowse_InvalidPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(BrowseParams{Path: "/nonexistent-path-xyz-12345"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowse(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowse() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowse() with invalid path should return error response")
	}
}

func TestGlobalHandleSettingsSet_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// Invalid JSON params
	resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleSettingsSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleSettingsSet() with invalid params should return error response")
	}
}

// Additional nil-conductor tests for remaining WorktreeSocket handlers

func TestWorktreeHandleStart_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	params, err := json.Marshal(StartParams{Source: "github:owner/repo#1"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleStart(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleStart() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleStart() with nil conductor should return error response")
	}
}

func TestWorktreeHandleStart_EmptySource(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params, err := json.Marshal(StartParams{Source: ""})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleStart(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleStart() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleStart() with empty source should return error response")
	}
}

func TestWorktreeHandleStart_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleStart(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleStart() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleStart() with invalid JSON should return error response")
	}
}

func TestWorktreeHandlePlan_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handlePlan(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handlePlan() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handlePlan() with nil conductor should return error response")
	}
}

func TestWorktreeHandleImplement_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleImplement(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleImplement() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleImplement() with nil conductor should return error response")
	}
}

func TestWorktreeHandleOptimize_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleOptimize(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleOptimize() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleOptimize() with nil conductor should return error response")
	}
}

func TestWorktreeHandleSimplify_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleSimplify(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSimplify() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleSimplify() with nil conductor should return error response")
	}
}

func TestWorktreeHandleReview_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleReview(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReview() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleReview() with nil conductor should return error response")
	}
}

func TestWorktreeHandleSubmit_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleSubmit(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleSubmit() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleSubmit() with nil conductor should return error response")
	}
}

func TestWorktreeHandleShutdown(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleShutdown(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleShutdown() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleShutdown() returned error: %s", resp.Error.Message)
	}
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["status"] != "shutting_down" {
		t.Errorf("status = %q, want shutting_down", result["status"])
	}
}

// Tests for conductor-based handlers that fail due to wrong state

func TestWorktreeHandlePlan_WrongState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// No work unit loaded - Plan should fail (wrong state)

	resp, err := w.handlePlan(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handlePlan() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handlePlan() without loaded task should return error")
	}
}

func TestWorktreeHandleImplement_WrongState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleImplement(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleImplement() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleImplement() without planned task should return error")
	}
}

func TestWorktreeHandleUndo_NothingToUndo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleUndo(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleUndo() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleUndo() with nothing to undo should return error")
	}
}

func TestWorktreeHandleRedo_NothingToRedo(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleRedo(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRedo() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleRedo() with nothing to redo should return error")
	}
}

// Additional conductor test: handleAbort from loaded state should succeed.
func TestWorktreeHandleAbort_LoadedState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleAbort(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAbort() error = %v", err)
	}
	// Abort from loaded state should succeed (goes to loaded state since no job running)
	if resp == nil {
		t.Fatal("handleAbort() returned nil response")
	}
	if resp.Error != nil {
		t.Fatalf("handleAbort() from loaded state returned error: %s", resp.Error.Message)
	}
}

// ============================================================
// handleAbandon / handleDelete / handleUpdate
// ============================================================

func TestWorktreeHandleAbandon_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleAbandon(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAbandon() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleAbandon() with nil conductor should return error response")
	}
}

func TestWorktreeHandleAbandon_Success(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// No work unit — Abandon() with no task is a no-op (success)

	resp, err := w.handleAbandon(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleAbandon() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleAbandon() returned error: %s", resp.Error.Message)
	}
}

func TestWorktreeHandleDelete_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleDelete(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleDelete() with nil conductor should return error response")
	}
}

func TestWorktreeHandleDelete_StateNone(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// StateNone is a valid terminal-ish state for Delete

	resp, err := w.handleDelete(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDelete() from StateNone returned error: %s", resp.Error.Message)
	}
}

func TestWorktreeHandleDelete_WrongState(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleDelete(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleDelete() from non-terminal state should return error response")
	}
}

func TestWorktreeHandleUpdate_NilConductor(t *testing.T) {
	ctx := context.Background()
	w := &WorktreeSocket{server: NewServer(""), streams: make(map[string]chan []byte)}

	resp, err := w.handleUpdate(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleUpdate() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleUpdate() with nil conductor should return error response")
	}
}

func TestWorktreeHandleUpdate_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// No work unit → UpdateTask returns error

	resp, err := w.handleUpdate(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleUpdate() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleUpdate() with no task should return error response")
	}
}

// ============================================================
// Screenshot handler tests
// ============================================================

func newTestWorktreeSocketWithScreenshots(t *testing.T) *WorktreeSocket {
	t.Helper()
	w := newTestWorktreeSocket(t)
	w.screenshots = screenshot.NewStore(t.TempDir())

	return w
}

func TestWorktreeHandleScreenshotsList_EmptyTaskID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	// No conductor work unit and no params → taskID is empty → returns empty list

	resp, err := w.handleScreenshotsList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleScreenshotsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleScreenshotsList() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["screenshots"]; !ok {
		t.Error("result should have 'screenshots' key")
	}
}

func TestWorktreeHandleScreenshotsList_WithTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	resp, err := w.handleScreenshotsList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleScreenshotsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleScreenshotsList() returned error: %s", resp.Error.Message)
	}
}

func TestWorktreeHandleScreenshotsGet_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)

	resp, err := w.handleScreenshotsGet(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleScreenshotsGet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsGet() with invalid params should return error")
	}
}

func TestWorktreeHandleScreenshotsGet_MissingTaskID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	// No work unit set, empty task_id

	params, err := json.Marshal(ScreenshotGetParams{ScreenshotID: "some-id"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsGet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsGet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsGet() with missing task_id should return error")
	}
}

func TestWorktreeHandleScreenshotsGet_MissingScreenshotID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, err := json.Marshal(ScreenshotGetParams{TaskID: "test-task-id", ScreenshotID: ""})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsGet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsGet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsGet() with missing screenshot_id should return error")
	}
}

func TestWorktreeHandleScreenshotsGet_NotFound(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, err := json.Marshal(ScreenshotGetParams{
		TaskID:       "test-task-id",
		ScreenshotID: "nonexistent-screenshot",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsGet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsGet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsGet() with unknown screenshot should return error")
	}
}

func TestWorktreeHandleScreenshotsCapture_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)

	resp, err := w.handleScreenshotsCapture(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleScreenshotsCapture() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsCapture() with invalid params should return error")
	}
}

func TestWorktreeHandleScreenshotsCapture_MissingTaskID(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)

	params, err := json.Marshal(ScreenshotCaptureParams{Data: "abc"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsCapture(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsCapture() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsCapture() with no task_id should return error")
	}
}

func TestWorktreeHandleScreenshotsCapture_MissingData(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, err := json.Marshal(ScreenshotCaptureParams{TaskID: "test-task-id", Data: ""})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsCapture(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsCapture() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsCapture() with missing data should return error")
	}
}

func TestWorktreeHandleScreenshotsCapture_InvalidBase64(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, err := json.Marshal(ScreenshotCaptureParams{
		TaskID: "test-task-id",
		Data:   "not valid base64!!!",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsCapture(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsCapture() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsCapture() with invalid base64 should return error")
	}
}

func TestWorktreeHandleScreenshotsCapture_Success(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	// Minimal 1x1 PNG (valid PNG bytes)
	minimalPNG, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
	)
	encoded := base64.StdEncoding.EncodeToString(minimalPNG)

	params, err := json.Marshal(ScreenshotCaptureParams{
		TaskID: "test-task-id",
		Source: "user",
		Format: "png",
		Data:   encoded,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsCapture(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsCapture() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleScreenshotsCapture() returned error: %s", resp.Error.Message)
	}
}

func TestWorktreeHandleScreenshotsDelete_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)

	resp, err := w.handleScreenshotsDelete(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleScreenshotsDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsDelete() with invalid params should return error")
	}
}

func TestWorktreeHandleScreenshotsDelete_MissingIDs(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)

	params, err := json.Marshal(ScreenshotDeleteParams{TaskID: "", ScreenshotID: ""})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsDelete(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsDelete() with missing IDs should return error")
	}
}

func TestWorktreeHandleScreenshotsDelete_NotFound(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocketWithScreenshots(t)

	params, err := json.Marshal(ScreenshotDeleteParams{
		TaskID:       "test-task-id",
		ScreenshotID: "nonexistent",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleScreenshotsDelete(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleScreenshotsDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleScreenshotsDelete() with unknown screenshot should return error")
	}
}

// ============================================================
// Global socket: chat handlers
// ============================================================

func TestGlobalHandleChatStop_NilPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t) // nil pool

	resp, err := g.handleChatStop(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleChatStop() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleChatStop() with nil pool should return error response")
	}
}

func TestGlobalHandleChatHistory_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleChatHistory(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleChatHistory() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleChatHistory() with invalid params should return error")
	}
}

func TestGlobalHandleChatHistory_NoActiveTask(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// worktree_id not registered → worktreeState is "" → "no active task"
	params, err := json.Marshal(ChatHistoryRequest{WorktreeID: "nonexistent-worktree"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleChatHistory(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleChatHistory() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleChatHistory() with no active task should return error")
	}
}

func TestGlobalHandleChatClear_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleChatClear(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleChatClear() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleChatClear() with invalid params should return error")
	}
}

func TestGlobalHandleChatClear_NoActiveTask(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(ChatClearRequest{WorktreeID: "nonexistent-worktree"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleChatClear(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleChatClear() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleChatClear() with no active task should return error")
	}
}

// ============================================================
// Global socket: files handlers
// ============================================================

func TestGlobalHandleFilesList_NoPathNoProjects(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// No params, no registered projects → error
	resp, err := g.handleFilesList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleFilesList() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleFilesList() with no path and no projects should return error")
	}
}

func TestGlobalHandleFilesList_WithPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	tmpDir := t.TempDir()

	// Register tmpDir as a worktree to allow file listing
	g.mu.Lock()
	g.worktrees["test"] = &WorktreeInfo{ID: "test", Path: tmpDir}
	g.mu.Unlock()

	params, err := json.Marshal(FilesListParams{Path: tmpDir})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleFilesList(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleFilesList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleFilesList() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["entries"]; !ok {
		t.Error("result should have 'entries' key")
	}
}

func TestGlobalHandleFilesSearch_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleFilesSearch() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleFilesSearch() with invalid params should return error")
	}
}

func TestGlobalHandleFilesSearch_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(FilesSearchParams{Query: "", Path: "/tmp"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleFilesSearch() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleFilesSearch() with empty query should return error")
	}
}

func TestGlobalHandleFilesSearch_NoPathNoProjects(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, err := json.Marshal(FilesSearchParams{Query: "main"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleFilesSearch() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleFilesSearch() with no path and no projects should return error")
	}
}

func TestGlobalHandleFilesSearch_WithPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	tmpDir := t.TempDir()

	// Register tmpDir as a worktree to allow file search
	g.mu.Lock()
	g.worktrees["test"] = &WorktreeInfo{ID: "test", Path: tmpDir}
	g.mu.Unlock()

	params, err := json.Marshal(FilesSearchParams{Query: "test", Path: tmpDir})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleFilesSearch() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleFilesSearch() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["entries"]; !ok {
		t.Error("result should have 'entries' key")
	}
}

// ============================================================
// GlobalSocket accessor methods
// ============================================================

func TestGlobalSocket_Server(t *testing.T) {
	g := newTestGlobalSocket(t)
	if g.Server() == nil {
		t.Error("Server() returned nil")
	}
}

func TestGlobalSocket_Pool_NilByDefault(t *testing.T) {
	g := newTestGlobalSocket(t)
	if g.Pool() != nil {
		t.Error("Pool() should be nil for new socket without pool")
	}
}

func TestGlobalSocket_SetPool(t *testing.T) {
	g := newTestGlobalSocket(t)
	pool := worker.NewPool(worker.DefaultPoolConfig())
	g.SetPool(pool)
	if g.Pool() != pool {
		t.Error("Pool() after SetPool should return the same pool")
	}
}

func TestGlobalSocket_GetWorktree_Found(t *testing.T) {
	g := newTestGlobalSocket(t)
	g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/test/path"}
	wt := g.GetWorktree("wt-1")
	if wt == nil {
		t.Fatal("GetWorktree() found = nil, want info")
	}
	if wt.ID != "wt-1" {
		t.Errorf("GetWorktree().ID = %q, want wt-1", wt.ID)
	}
}

func TestGlobalSocket_GetWorktree_NotFound(t *testing.T) {
	g := newTestGlobalSocket(t)
	if wt := g.GetWorktree("nonexistent"); wt != nil {
		t.Errorf("GetWorktree() not found = %v, want nil", wt)
	}
}

func TestGlobalSocket_ListWorktrees_Empty(t *testing.T) {
	g := newTestGlobalSocket(t)
	if wts := g.ListWorktrees(); len(wts) != 0 {
		t.Errorf("ListWorktrees() empty = %d, want 0", len(wts))
	}
}

func TestGlobalSocket_ListWorktrees_WithItems(t *testing.T) {
	g := newTestGlobalSocket(t)
	g.worktrees["id1"] = &WorktreeInfo{ID: "id1"}
	g.worktrees["id2"] = &WorktreeInfo{ID: "id2"}
	if wts := g.ListWorktrees(); len(wts) != 2 {
		t.Errorf("ListWorktrees() with items = %d, want 2", len(wts))
	}
}

// ============================================================
// handleTasksList
// ============================================================

func TestGlobalHandleTasksList_Empty(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleTasksList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTasksList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleTasksList() returned error: %s", resp.Error.Message)
	}
	var result TasksListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Tasks) != 0 {
		t.Errorf("Tasks = %d, want 0", len(result.Tasks))
	}
}

func TestGlobalHandleTasksList_WithWorktree(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	g.worktrees["wt-x"] = &WorktreeInfo{ID: "wt-x", Path: "/does/not/exist", State: "loaded"}

	resp, err := g.handleTasksList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTasksList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleTasksList() returned error: %s", resp.Error.Message)
	}
	var result TasksListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Errorf("Tasks = %d, want 1", len(result.Tasks))
	}
}

// ============================================================
// handleWorkerStats with pool
// ============================================================

func TestGlobalHandleWorkerStats_WithPool(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	pool := worker.NewPool(worker.DefaultPoolConfig())
	g.SetPool(pool)

	resp, err := g.handleWorkerStats(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleWorkerStats() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleWorkerStats() returned error: %s", resp.Error.Message)
	}
}

// ============================================================
// handleGetJob
// ============================================================

func TestGlobalHandleGetJob_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	pool := worker.NewPool(worker.DefaultPoolConfig())
	g.SetPool(pool)

	resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleGetJob() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGetJob() invalid params should return error response")
	}
}

func TestGlobalHandleGetJob_NotFound(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	pool := worker.NewPool(worker.DefaultPoolConfig())
	g.SetPool(pool)

	params, err := json.Marshal(map[string]string{"id": "nonexistent-job"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleGetJob() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleGetJob() not found should return error response")
	}
}

// ============================================================
// getBrowserOpts
// ============================================================

func TestGetBrowserOpts_NoWorktreeID(t *testing.T) {
	g := newTestGlobalSocket(t)
	opts := g.getBrowserOpts(BrowserParams{SessionName: "test-session"})
	if opts == nil {
		t.Fatal("getBrowserOpts() returned nil")
	}
	if opts.SessionName != "test-session" {
		t.Errorf("SessionName = %q, want test-session", opts.SessionName)
	}
	if opts.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty (no worktree)", opts.WorktreePath)
	}
}

func TestGetBrowserOpts_UnknownWorktreeID(t *testing.T) {
	g := newTestGlobalSocket(t)
	opts := g.getBrowserOpts(BrowserParams{WorktreeID: "unknown"})
	if opts == nil {
		t.Fatal("getBrowserOpts() returned nil")
	}
	if opts.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty (unknown ID)", opts.WorktreePath)
	}
}

func TestGetBrowserOpts_KnownWorktreeID(t *testing.T) {
	g := newTestGlobalSocket(t)
	g.worktrees["wt-known"] = &WorktreeInfo{ID: "wt-known", Path: "/known/path"}
	opts := g.getBrowserOpts(BrowserParams{WorktreeID: "wt-known"})
	if opts.WorktreePath != "/known/path" {
		t.Errorf("WorktreePath = %q, want /known/path", opts.WorktreePath)
	}
}

// ============================================================
// handleBrowserStatus / handleBrowserConfigGet
// ============================================================

func TestGlobalHandleBrowserStatus(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserStatus(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserStatus() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBrowserStatus() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["installed"]; !ok {
		t.Error("result should have 'installed' key")
	}
}

func TestGlobalHandleBrowserConfigGet(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserConfigGet(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserConfigGet() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBrowserConfigGet() returned error: %s", resp.Error.Message)
	}
}

// ============================================================
// handleBrowserConfigSet – validation errors only
// ============================================================

func TestGlobalHandleBrowserConfigSet_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserConfigSet(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleBrowserConfigSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserConfigSet() invalid params should return error response")
	}
}

func TestGlobalHandleBrowserConfigSet_InvalidBrowser(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserConfigSetParams{Key: "browser", Value: "opera"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserConfigSet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserConfigSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserConfigSet() invalid browser should return error response")
	}
}

func TestGlobalHandleBrowserConfigSet_InvalidTimeout(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserConfigSetParams{Key: "timeout", Value: "not-a-number"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserConfigSet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserConfigSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserConfigSet() invalid timeout should return error response")
	}
}

func TestGlobalHandleBrowserConfigSet_UnknownKey(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserConfigSetParams{Key: "unknown_key", Value: "value"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserConfigSet(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserConfigSet() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserConfigSet() unknown key should return error response")
	}
}

// ============================================================
// Browser action handlers – validation error paths
// ============================================================

func TestGlobalHandleBrowserEval_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserEval(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleBrowserEval() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserEval() invalid params should return error response")
	}
}

func TestGlobalHandleBrowserEval_EmptyJS(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserEvalParams{JS: ""})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserEval(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserEval() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserEval() empty js should return error response")
	}
}

func TestGlobalHandleBrowserNavigate_InvalidParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserNavigate(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleBrowserNavigate() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserNavigate() invalid params should return error response")
	}
}

func TestGlobalHandleBrowserNavigate_EmptyURL(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserNavigateParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserNavigate(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserNavigate() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserNavigate() empty url should return error response")
	}
}

func TestGlobalHandleBrowserClick_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserClickParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserClick(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserClick() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserClick() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserType_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserTypeParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserType(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserType() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserType() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserWait_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserWaitParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserWait(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserWait() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserWait() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserFill_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserFillParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserFill(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserFill() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserFill() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserSelect_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserSelectParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserSelect(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserSelect() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserSelect() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserSelect_EmptyValues(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserSelectParams{Selector: "#my-select", Values: []string{}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserSelect(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserSelect() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserSelect() empty values should return error response")
	}
}

func TestGlobalHandleBrowserHover_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserHoverParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserHover(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserHover() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserHover() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserFocus_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserFocusParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserFocus(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserFocus() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserFocus() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserScroll_EmptyDirection(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserScrollParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserScroll(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserScroll() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserScroll() empty direction should return error response")
	}
}

func TestGlobalHandleBrowserPress_EmptyKey(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserPressParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserPress(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserPress() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserPress() empty key should return error response")
	}
}

func TestGlobalHandleBrowserDialog_InvalidAction(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserDialogParams{Action: "unknown"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserDialog(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserDialog() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserDialog() invalid action should return error response")
	}
}

func TestGlobalHandleBrowserUpload_EmptySelector(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserUploadParams{})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserUpload(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserUpload() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserUpload() empty selector should return error response")
	}
}

func TestGlobalHandleBrowserUpload_EmptyFiles(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	params, err := json.Marshal(BrowserUploadParams{Selector: "#input", Files: []string{}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := g.handleBrowserUpload(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowserUpload() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowserUpload() empty files should return error response")
	}
}

// ============================================================
// Path functions
// ============================================================

func TestGlobalLockPath(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	path := GlobalLockPath()
	if path == "" {
		t.Error("GlobalLockPath() returned empty string")
	}
	if filepath.Base(path) != "global.lock" {
		t.Errorf("GlobalLockPath() base = %q, want global.lock", filepath.Base(path))
	}
}

func TestWorktreeSocketPath(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	path := WorktreeSocketPath("/some/project/path")
	if path == "" {
		t.Error("WorktreeSocketPath() returned empty string")
	}
	if filepath.Ext(path) != ".sock" {
		t.Errorf("WorktreeSocketPath() ext = %q, want .sock", filepath.Ext(path))
	}
}

func TestWorktreeSocketPath_Deterministic(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	p1 := WorktreeSocketPath("/some/project")
	p2 := WorktreeSocketPath("/some/project")
	p3 := WorktreeSocketPath("/other/project")
	if p1 != p2 {
		t.Error("WorktreeSocketPath() is not deterministic")
	}
	if p1 == p3 {
		t.Error("WorktreeSocketPath() same for different paths")
	}
}

func TestSocketExists_NotExist(t *testing.T) {
	if SocketExists("/nonexistent/path/to/socket.sock") {
		t.Error("SocketExists() nonexistent = true, want false")
	}
}

func TestEnsureDir(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := EnsureDir(); err != nil {
		t.Errorf("EnsureDir() error = %v", err)
	}
}

// ============================================================
// Browser pass-through handlers (skip when browser installed to avoid launching)
// ============================================================

func TestGlobalHandleBrowserSnapshot(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserSnapshot(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserSnapshot() Go error = %v", err)
	}
	// Handler should return an error response when browser not installed
	if resp == nil {
		t.Error("handleBrowserSnapshot() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserSnapshot() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserConsole(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserConsole(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserConsole() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserConsole() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserConsole() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserNetwork(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserNetwork(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserNetwork() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserNetwork() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserNetwork() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserScreenshot(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserScreenshot(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserScreenshot() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserScreenshot() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserScreenshot() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserBack(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserBack(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserBack() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserBack() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserBack() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserForward(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserForward(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserForward() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserForward() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserForward() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserReload(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserReload(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserReload() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserReload() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserReload() should return error response when browser not installed")
	}
}

func TestGlobalHandleBrowserPDF(t *testing.T) {
	if browser.IsInstalled() {
		t.Skip("skipping browser handler test - browser is installed and would launch Chrome")
	}
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	resp, err := g.handleBrowserPDF(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBrowserPDF() Go error = %v", err)
	}
	if resp == nil {
		t.Error("handleBrowserPDF() returned nil response")
	} else if resp.Error == nil {
		t.Error("handleBrowserPDF() should return error response when browser not installed")
	}
}

// ============================================================
// WorktreeSocket accessor methods
// ============================================================

func TestWorktreeSocket_Stop(t *testing.T) {
	w := newTestWorktreeSocket(t)
	// Stop before Start — just verify no panic and server.Stop() is called
	_ = w.Stop()
}

func TestWorktreeSocket_Path(t *testing.T) {
	w := newTestWorktreeSocket(t)
	// newTestWorktreeSocket uses NewServer("") so path is ""
	_ = w.Path() // just cover the one-liner
}

func TestWorktreeSocket_Server(t *testing.T) {
	w := newTestWorktreeSocket(t)
	if w.Server() == nil {
		t.Error("Server() returned nil")
	}
}

func TestWorktreeSocket_Conductor(t *testing.T) {
	w := newTestWorktreeSocket(t)
	if w.Conductor() == nil {
		t.Error("Conductor() returned nil")
	}
}

// ============================================================
// handleStreamSubscribe
// ============================================================

func TestWorktreeHandleStreamSubscribe(t *testing.T) {
	ctx := t.Context()
	w := newTestWorktreeSocket(t)

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	resp, err := w.handleStreamSubscribe(ctx, &Request{ID: "1"}, c1)
	if err != nil {
		t.Fatalf("handleStreamSubscribe() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleStreamSubscribe() returned error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := result["subscription_id"]; !ok {
		t.Error("result should have 'subscription_id' key")
	}
}

// ============================================================
// handleBrowse (worktree)
// ============================================================

func TestWorktreeHandleBrowse_PathNotFound(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	params, err := json.Marshal(WorktreeBrowseParams{Path: "/nonexistent/path/xyz"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleBrowse(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowse() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleBrowse() nonexistent path should return error response")
	}
}

func TestWorktreeHandleBrowse_WithDir(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)
	tmpDir := t.TempDir()
	w.path = tmpDir // Set worktree path so browse validates correctly

	params, err := json.Marshal(WorktreeBrowseParams{Path: tmpDir})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	resp, err := w.handleBrowse(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBrowse() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBrowse() returned error: %s", resp.Error.Message)
	}
}

// ============================================================
// handleReview (worktree) – error path when no task loaded
// ============================================================

func TestWorktreeHandleReview_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleReview(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReview() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleReview() with no task should return error response")
	}
}

// ============================================================
// handleReset (worktree) – error path when no task loaded
// ============================================================

func TestWorktreeHandleReset_NoTask(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(t)

	resp, err := w.handleReset(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleReset() error = %v", err)
	}
	if resp.Error == nil {
		t.Error("handleReset() with no task should return error response")
	}
}
