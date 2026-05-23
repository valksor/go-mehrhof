package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/catalog"
	"github.com/valksor/kvelmo/internal/taskgroup"
	"github.com/valksor/kvelmo/internal/testutil"
)

// --- global_batch.go ---

func TestGlobalHandleBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleBatch(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleBatch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing action", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleBatch(ctx, &Request{ID: "1", Params: json.RawMessage(`{}`)})
		if err != nil {
			t.Fatalf("handleBatch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing action")
		}
	})

	t.Run("invalid action", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(BatchParams{Action: "frobnicate"})
		resp, err := g.handleBatch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBatch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid action")
		}
	})

	t.Run("valid action over unreachable worktree", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/proj", SocketPath: "/nonexistent/wt.sock"}
		g.mu.Unlock()

		params, _ := json.Marshal(BatchParams{Action: "plan"})
		resp, err := g.handleBatch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBatch() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result struct {
			Results []BatchResultItem `json:"results"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Results) != 1 {
			t.Fatalf("results = %d, want 1", len(result.Results))
		}
		if result.Results[0].Error == "" {
			t.Error("expected connect-failed error for unreachable worktree")
		}
	})

	t.Run("match filter skips non-matching paths", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/alpha", SocketPath: "/nonexistent/a.sock"}
		g.mu.Unlock()

		params, _ := json.Marshal(BatchParams{Action: "plan", Filter: map[string]string{"match": "beta"}})
		resp, err := g.handleBatch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBatch() error = %v", err)
		}
		var result struct {
			Results []BatchResultItem `json:"results"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Results) != 0 {
			t.Errorf("match filter should have skipped all worktrees, got %d results", len(result.Results))
		}
	})
}

// --- global_export.go ---

func TestGlobalHandleExport(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleExport(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleExport() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("export with worktrees includes metrics and tasks", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: "/p", State: "loaded", SocketPath: "/nonexistent/wt.sock"}
		g.mu.Unlock()

		params, _ := json.Marshal(map[string]string{"format": "json", "since": "3d"})
		resp, err := g.handleExport(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleExport() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["metrics"]; !ok {
			t.Error("expected metrics in export")
		}
		if _, ok := result["tasks"]; !ok {
			t.Error("expected tasks in export")
		}
	})
}

// --- global_files.go ---

func TestGlobalHandleFilesList(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleFilesList(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleFilesList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("no projects registered", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleFilesList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleFilesList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response when no projects registered")
		}
	})

	t.Run("path outside registered projects", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: dir}
		g.mu.Unlock()
		params, _ := json.Marshal(FilesListParams{Path: "/etc"})
		resp, err := g.handleFilesList(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleFilesList() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for path outside projects")
		}
	})

	t.Run("lists files in registered project", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package x"), 0o644); err != nil {
			t.Fatal(err)
		}
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: dir}
		g.mu.Unlock()

		resp, err := g.handleFilesList(ctx, &Request{ID: "1"})
		if err != nil {
			t.Fatalf("handleFilesList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result struct {
			Entries []FileEntry `json:"entries"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Entries) == 0 {
			t.Error("expected at least one file entry")
		}
	})
}

func TestGlobalHandleFilesSearch(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleFilesSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("missing query", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(FilesSearchParams{Query: ""})
		resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleFilesSearch() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for missing query")
		}
	})

	t.Run("search within registered project", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		if err := os.WriteFile(filepath.Join(dir, "server.go"), []byte("package x"), 0o644); err != nil {
			t.Fatal(err)
		}
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: dir}
		g.mu.Unlock()

		params, _ := json.Marshal(FilesSearchParams{Query: "server", MaxResults: 10})
		resp, err := g.handleFilesSearch(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleFilesSearch() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
	})
}

func TestGlobalHandleBrowse_AllowedRoots(t *testing.T) {
	ctx := context.Background()

	t.Run("path outside allowed roots denied", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(BrowseParams{Path: "/etc/ssh"})
		resp, err := g.handleBrowse(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBrowse() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for path outside allowed roots")
		}
	})

	t.Run("registered project root is browsable", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		dir := testutil.TempDir(t)
		sub := filepath.Join(dir, "src")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		g.mu.Lock()
		g.worktrees["wt-1"] = &WorktreeInfo{ID: "wt-1", Path: dir}
		g.mu.Unlock()
		params, _ := json.Marshal(BrowseParams{Path: dir})
		resp, err := g.handleBrowse(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleBrowse() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
	})
}

func TestFileEntry_Searchable(t *testing.T) {
	f := FileEntry{Name: "main.go", RelPath: "cmd/main.go"}
	if f.SearchTitle() != "main.go" {
		t.Errorf("SearchTitle() = %q", f.SearchTitle())
	}
	if f.SearchDescription() != "cmd/main.go" {
		t.Errorf("SearchDescription() = %q", f.SearchDescription())
	}
	if f.SearchTags() != nil {
		t.Errorf("SearchTags() should be nil")
	}
}

// --- global_catalog.go ---

func TestGlobalHandleCatalog(t *testing.T) {
	ctx := context.Background()

	// Wire a real catalog rooted at a temp dir. Built-ins are always present.
	prev := catalogInstance
	SetCatalog(catalog.New(testutil.TempDir(t)))
	t.Cleanup(func() { catalogInstance = prev })

	t.Run("list returns templates with pagination", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]int{"page": 1, "per_page": 5})
		resp, err := g.handleCatalogList(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCatalogList() error = %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %s", resp.Error.Message)
		}
		var result map[string]any
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := result["templates"]; !ok {
			t.Error("expected templates key")
		}
	})

	t.Run("get invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleCatalogGet(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleCatalogGet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("get nonexistent template", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"name": "does-not-exist-xyz"})
		resp, err := g.handleCatalogGet(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCatalogGet() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent template")
		}
	})

	t.Run("import invalid params", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		resp, err := g.handleCatalogImport(ctx, &Request{ID: "1", Params: json.RawMessage(`bad`)})
		if err != nil {
			t.Fatalf("handleCatalogImport() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})

	t.Run("import nonexistent path", func(t *testing.T) {
		g := newTestGlobalSocket(t)
		params, _ := json.Marshal(map[string]string{"path": "/nonexistent/template.yaml"})
		resp, err := g.handleCatalogImport(ctx, &Request{ID: "1", Params: params})
		if err != nil {
			t.Fatalf("handleCatalogImport() error = %v", err)
		}
		if resp.Error == nil {
			t.Fatal("expected error response for nonexistent import path")
		}
	})
}

func TestGlobalHandleCatalog_NotConfigured(t *testing.T) {
	ctx := context.Background()
	prev := catalogInstance
	catalogInstance = nil
	t.Cleanup(func() { catalogInstance = prev })

	g := newTestGlobalSocket(t)

	// List returns an empty set rather than an error when unconfigured.
	resp, err := g.handleCatalogList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleCatalogList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	// Get/Import error out when unconfigured.
	getParams, _ := json.Marshal(map[string]string{"name": "x"}) //nolint:errchkjson // test data
	resp, _ = g.handleCatalogGet(ctx, &Request{ID: "1", Params: getParams})
	if resp.Error == nil {
		t.Error("handleCatalogGet should error when catalog not configured")
	}
}

// --- global_taskgroup.go ---

func TestGlobalHandleTaskGroup(t *testing.T) {
	ctx := context.Background()

	prev := taskGroupCoordinator
	SetTaskGroupCoordinator(taskgroup.NewCoordinator(taskgroup.NewStore(testutil.TempDir(t))))
	t.Cleanup(func() { taskGroupCoordinator = prev })

	g := newTestGlobalSocket(t)

	// Create.
	createParams, _ := json.Marshal(map[string]string{"label": "release-batch"}) //nolint:errchkjson // test data
	resp, err := g.handleTaskGroupCreate(ctx, &Request{ID: "1", Params: createParams})
	if err != nil {
		t.Fatalf("handleTaskGroupCreate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("create returned error: %s", resp.Error.Message)
	}
	var group struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Result, &group); err != nil {
		t.Fatalf("unmarshal group: %v", err)
	}
	if group.ID == "" {
		t.Fatal("expected non-empty group id")
	}

	t.Run("create missing label", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"label": ""})
		r, _ := g.handleTaskGroupCreate(ctx, &Request{ID: "2", Params: params})
		if r.Error == nil {
			t.Fatal("expected error response for missing label")
		}
	})

	t.Run("list includes created group", func(t *testing.T) {
		r, err := g.handleTaskGroupList(ctx, &Request{ID: "3"})
		if err != nil {
			t.Fatalf("handleTaskGroupList() error = %v", err)
		}
		var result struct {
			Groups []any `json:"groups"`
		}
		_ = json.Unmarshal(r.Result, &result)
		if len(result.Groups) == 0 {
			t.Error("expected at least one group")
		}
	})

	t.Run("status missing id", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"id": ""})
		r, _ := g.handleTaskGroupStatus(ctx, &Request{ID: "4", Params: params})
		if r.Error == nil {
			t.Fatal("expected error response for missing id")
		}
	})

	t.Run("status of created group", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"id": group.ID})
		r, err := g.handleTaskGroupStatus(ctx, &Request{ID: "5", Params: params})
		if err != nil {
			t.Fatalf("handleTaskGroupStatus() error = %v", err)
		}
		if r.Error != nil {
			t.Fatalf("status returned error: %s", r.Error.Message)
		}
	})

	t.Run("add missing id/task_id", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"id": "", "task_id": ""})
		r, _ := g.handleTaskGroupAdd(ctx, &Request{ID: "6", Params: params})
		if r.Error == nil {
			t.Fatal("expected error response for missing id/task_id")
		}
	})

	t.Run("add task to group", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"id": group.ID, "task_id": "T-1", "project_dir": "/proj", "state": "loaded"})
		r, err := g.handleTaskGroupAdd(ctx, &Request{ID: "7", Params: params})
		if err != nil {
			t.Fatalf("handleTaskGroupAdd() error = %v", err)
		}
		if r.Error != nil {
			t.Fatalf("add returned error: %s", r.Error.Message)
		}
	})

	t.Run("remove invalid params", func(t *testing.T) {
		r, _ := g.handleTaskGroupRemove(ctx, &Request{ID: "8", Params: json.RawMessage(`bad`)})
		if r.Error == nil {
			t.Fatal("expected error response for invalid params")
		}
	})
}

func TestGlobalHandleTaskGroup_NotConfigured(t *testing.T) {
	ctx := context.Background()
	prev := taskGroupCoordinator
	taskGroupCoordinator = nil
	t.Cleanup(func() { taskGroupCoordinator = prev })

	g := newTestGlobalSocket(t)

	// List degrades to empty set.
	resp, err := g.handleTaskGroupList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleTaskGroupList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	// Create errors out.
	createParams, _ := json.Marshal(map[string]string{"label": "x"}) //nolint:errchkjson // test data
	resp, _ = g.handleTaskGroupCreate(ctx, &Request{ID: "1", Params: createParams})
	if resp.Error == nil {
		t.Error("handleTaskGroupCreate should error when coordinator not configured")
	}
}
