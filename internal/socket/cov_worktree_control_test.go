package socket

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/testutil"
)

// --- worktree_phase_control.go ---

func TestWorktreeHandleFinish(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleFinish(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleFinish() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleFinish(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleFinish() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("wrong state errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateLoaded)
		params, _ := json.Marshal(FinishParams{})
		resp, err := w.handleFinish(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleFinish() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response finishing from loaded state")
		}
	})
}

func TestWorktreeHandleRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleRefresh(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRefresh() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("no task errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleRefresh(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRefresh() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response refreshing with no task")
		}
	})
}

func TestWorktreeHandleRemoteApprove(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleRemoteApprove(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRemoteApprove() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleRemoteApprove(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleRemoteApprove() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no PR errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateSubmitted)
		params, _ := json.Marshal(RemoteApproveParams{Comment: "ok"})
		resp, err := w.handleRemoteApprove(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleRemoteApprove() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response approving with no remote PR")
		}
	})
}

func TestWorktreeHandleRemoteMerge(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleRemoteMerge(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleRemoteMerge() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleRemoteMerge(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleRemoteMerge() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no PR errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateSubmitted)
		params, _ := json.Marshal(RemoteMergeParams{Method: "squash"})
		resp, err := w.handleRemoteMerge(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleRemoteMerge() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response merging with no remote PR")
		}
	})
}

func TestWorktreeHandleAbandon_KeepBranch(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	setWorkUnitInState(t, w, conductor.StateLoaded)

	params, _ := json.Marshal(AbandonParams{KeepBranch: true}) //nolint:errchkjson // test data
	resp, err := w.handleAbandon(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleAbandon() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if w.conductor.State() != conductor.StateNone {
		t.Errorf("state after abandon = %s, want none", w.conductor.State())
	}
}

func TestWorktreeHandleDelete_InvalidParams(t *testing.T) {
	ctx := context.Background()
	w := newTestWorktreeSocket(ctx, t)
	resp, err := w.handleDelete(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
	if err != nil {
		t.Fatalf("handleDelete() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid params")
	}
}

// --- worktree_approve.go ---

func TestWorktreeHandleApproveNode(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleApproveNode(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleApproveNode() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleApproveNode(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleApproveNode() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("approve nonexistent node", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplementing)
		params, _ := json.Marshal(map[string]any{"node_id": "no-such-node"})
		resp, err := w.handleApproveNode(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleApproveNode() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent node approval")
		}
	})

	t.Run("reject nonexistent node", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplementing)
		params, _ := json.Marshal(map[string]any{"node_id": "no-such-node", "reject": true})
		resp, err := w.handleApproveNode(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleApproveNode() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent node rejection")
		}
	})
}

func TestWorktreeHandleReviewChecklistCheckUncheck(t *testing.T) {
	ctx := context.Background()

	t.Run("check nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, _ := w.handleReviewChecklistCheck(ctx, &Request{ID: "1"})
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("check invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, _ := w.handleReviewChecklistCheck(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("check no work unit errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(map[string]string{"item": "item-a"})
		resp, _ := w.handleReviewChecklistCheck(ctx, &Request{ID: "1", Params: params})
		if resp.Error == nil {
			t.Fatal("expected error response when no task loaded")
		}
	})

	t.Run("check then uncheck succeeds with work unit", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateReviewing)

		params, _ := json.Marshal(map[string]string{"item": "tests-pass"})
		resp, _ := w.handleReviewChecklistCheck(ctx, &Request{ID: "1", Params: params})
		if resp.Error != nil {
			t.Fatalf("check returned error: %s", resp.Error.Message)
		}
		var checkResult map[string]any
		_ = json.Unmarshal(resp.Result, &checkResult)
		if checkResult["checked"] != "tests-pass" {
			t.Errorf("checked = %v, want tests-pass", checkResult["checked"])
		}

		resp, _ = w.handleReviewChecklistUncheck(ctx, &Request{ID: "2", Params: params})
		if resp.Error != nil {
			t.Fatalf("uncheck returned error: %s", resp.Error.Message)
		}
		var uncheckResult map[string]any
		_ = json.Unmarshal(resp.Result, &uncheckResult)
		if uncheckResult["unchecked"] != "tests-pass" {
			t.Errorf("unchecked = %v, want tests-pass", uncheckResult["unchecked"])
		}
	})
}

// --- worktree_checkpoint.go ---

func TestWorktreeHandleCheckpointGoto(t *testing.T) {
	ctx := context.Background()

	t.Run("nil conductor", func(t *testing.T) {
		w := nilConductorWorktree()
		resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCheckpointGoto() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with nil conductor")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleCheckpointGoto() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing sha", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		params, _ := json.Marshal(CheckpointGotoParams{SHA: ""})
		resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCheckpointGoto() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing sha")
		}
	})

	t.Run("nonexistent sha errors", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		setWorkUnitInState(t, w, conductor.StateImplemented)
		params, _ := json.Marshal(CheckpointGotoParams{SHA: "deadbeef"})
		resp, err := w.handleCheckpointGoto(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCheckpointGoto() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent checkpoint sha")
		}
	})
}

func TestWorktreeHandleCheckpointPreview(t *testing.T) {
	ctx := context.Background()

	t.Run("no repo", func(t *testing.T) {
		w := newTestWorktreeSocket(ctx, t)
		resp, err := w.handleCheckpointPreview(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleCheckpointPreview() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no repo")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		resp, err := w.handleCheckpointPreview(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleCheckpointPreview() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing sha", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		params, _ := json.Marshal(CheckpointGotoParams{SHA: ""})
		resp, err := w.handleCheckpointPreview(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCheckpointPreview() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing sha")
		}
	})

	t.Run("preview against HEAD", func(t *testing.T) {
		w := gitWorktree(ctx, t)
		params, _ := json.Marshal(CheckpointGotoParams{SHA: "HEAD"})
		resp, err := w.handleCheckpointPreview(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCheckpointPreview() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		_ = json.Unmarshal(resp.Result, &result)
		if result["sha"] != "HEAD" {
			t.Errorf("sha = %v, want HEAD", result["sha"])
		}
	})
}

func TestWorktreeHandleCheckpoints_WithRepo(t *testing.T) {
	ctx := context.Background()
	w := gitWorktree(ctx, t)
	setWorkUnitInState(t, w, conductor.StateImplemented)
	// Inject a checkpoint SHA so the enrich path runs against the real repo.
	w.conductor.WorkUnit().Checkpoints = []string{"HEAD"}

	resp, err := w.handleCheckpoints(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCheckpoints() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result struct {
		Checkpoints []CheckpointInfo `json:"checkpoints"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Checkpoints) != 1 {
		t.Fatalf("checkpoints = %d, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Message == "" {
		t.Error("expected enriched commit message for HEAD checkpoint")
	}
}

// --- chat.go (global) ---

func TestGlobalHandleChatStop(t *testing.T) {
	ctx := context.Background()

	t.Run("no pool", func(t *testing.T) {
		g := newTestGlobalSocket(t) // no pool
		resp, err := g.handleChatStop(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleChatStop() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no worker pool")
		}
	})
}

func TestGlobalHandleChatHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleChatHistory(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleChatHistory() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no active task", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(ChatHistoryRequest{WorktreeID: "unknown"})
		resp, err := g.handleChatHistory(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleChatHistory() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no active task")
		}
	})

	t.Run("loads history for active worktree", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "task-1", Path: dir, State: "implementing"}
		g.mu.Unlock()

		params, _ := json.Marshal(ChatHistoryRequest{WorktreeID: "wt-1"})
		resp, err := g.handleChatHistory(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleChatHistory() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result ChatHistoryResponse
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.TaskID != "task-1" {
			t.Errorf("TaskID = %q, want task-1", result.TaskID)
		}
	})
}

func TestGlobalHandleChatClear(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleChatClear(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleChatClear() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no active task", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(ChatClearRequest{WorktreeID: "unknown"})
		resp, err := g.handleChatClear(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleChatClear() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no active task")
		}
	})

	t.Run("clears history for active worktree", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "task-1", Path: dir, State: "implementing"}
		g.mu.Unlock()

		params, _ := json.Marshal(ChatClearRequest{WorktreeID: "wt-1"})
		resp, err := g.handleChatClear(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleChatClear() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]string
		_ = json.Unmarshal(resp.Result, &result)
		if result["status"] != "cleared" {
			t.Errorf("status = %q, want cleared", result["status"])
		}
	})
}
