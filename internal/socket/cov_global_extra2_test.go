package socket

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
)

// --- chat.go: handleChatSendEnhanced validation paths ---

func TestGlobalHandleChatSendEnhanced(t *testing.T) {
	ctx := context.Background()
	// A nil conn is fine: validation paths return before any connection use.
	var conn net.Conn

	t.Run("no pool", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleChatSendEnhanced(ctx, &Request{ID: "1", Params: json.RawMessage(`{}`)}, conn)
		if err != nil {
			t.Fatalf("handleChatSendEnhanced() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no worker pool")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocketWithPool2(t)
		resp, err := g.handleChatSendEnhanced(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)}, conn)
		if err != nil {
			t.Fatalf("handleChatSendEnhanced() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing message", func(t *testing.T) {
		g := newTestGlobalSocketWithPool2(t)
		params, _ := json.Marshal(ChatSendRequest{Message: ""})
		resp, err := g.handleChatSendEnhanced(ctx, &Request{ID: "1", Params: params}, conn)
		if err != nil {
			t.Fatalf("handleChatSendEnhanced() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing message")
		}
	})

	t.Run("no active task", func(t *testing.T) {
		g := newTestGlobalSocketWithPool2(t)
		params, _ := json.Marshal(ChatSendRequest{Message: "hello", WorktreeID: "unknown"})
		resp, err := g.handleChatSendEnhanced(ctx, &Request{ID: "1", Params: params}, conn)
		if err != nil {
			t.Fatalf("handleChatSendEnhanced() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no active task")
		}
	})
}

// --- global_workers.go: handleGetJob ---

func TestGlobalHandleGetJob(t *testing.T) {
	ctx := context.Background()

	t.Run("no pool", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: json.RawMessage(`{}`)})
		if err != nil {
			t.Fatalf("handleGetJob() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response with no pool")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocketWithPool2(t)
		resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleGetJob() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("job not found", func(t *testing.T) {
		g := newTestGlobalSocketWithPool2(t)
		params, _ := json.Marshal(map[string]string{"id": "no-such-job"})
		resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleGetJob() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for unknown job id")
		}
	})

	t.Run("returns submitted job", func(t *testing.T) {
		g := newTestGlobalSocketWithPool2(t)
		job, err := g.pool.SubmitWithOptions(worker.JobTypeChat, "wt-1", "do something", &worker.JobOptions{})
		if err != nil {
			t.Fatalf("SubmitWithOptions: %v", err)
		}
		params, _ := json.Marshal(map[string]string{"id": job.ID})
		resp, err := g.handleGetJob(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleGetJob() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result["id"] != job.ID {
			t.Errorf("id = %v, want %q", result["id"], job.ID)
		}
	})
}

// --- global_backup.go ---

func TestGlobalHandleBackupRestore(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleBackupRestore(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleBackupRestore() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing archive path", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(backupRestoreParams{ArchivePath: ""})
		resp, err := g.handleBackupRestore(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBackupRestore() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing archive path")
		}
	})

	t.Run("nonexistent archive", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(backupRestoreParams{ArchivePath: "/nonexistent/backup.tar.gz"})
		resp, err := g.handleBackupRestore(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBackupRestore() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent archive")
		}
	})
}

// --- global_report.go ---

func TestGlobalHandleReportGenerate(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleReportGenerate(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleReportGenerate() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("generates a report", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/p", State: "implemented"}
		g.mu.Unlock()
		params, _ := json.Marshal(map[string]string{"format": "md", "since": "7d"})
		resp, err := g.handleReportGenerate(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleReportGenerate() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["markdown"]; !ok {
			t.Error("expected markdown in report result")
		}
	})
}

// --- global_configcheck.go ---

func TestGlobalHandleConfigCheck(t *testing.T) {
	g := newTestGlobalSocket(t)
	resp, err := g.handleConfigCheck(context.Background(), &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleConfigCheck() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["drifts"]; !ok {
		t.Error("expected drifts key in config check result")
	}
}

// --- global_settings.go: handleSettingsSet validation paths only ---

func TestGlobalHandleSettingsSet_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleSettingsSet(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleSettingsSet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nil params")
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleSettingsSet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]any{"scope": "bogus", "values": map[string]any{}})
		resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleSettingsSet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid scope")
		}
	})

	t.Run("project scope without path or projects", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]any{"scope": settings.ScopeProject, "values": map[string]any{}})
		resp, err := g.handleSettingsSet(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleSettingsSet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for project scope without path")
		}
	})
}
