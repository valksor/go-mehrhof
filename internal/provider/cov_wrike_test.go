package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wrikeTestServer redirects the package httpClient to an httptest server and
// returns a WrikeProvider plus a cleanup func.
func wrikeTestServer(t *testing.T, handler http.HandlerFunc) (*WrikeProvider, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{base: http.DefaultTransport, targetURL: srv.URL}

	cleanup := func() {
		httpClient.Transport = origTransport
		srv.Close()
	}

	return NewWrikeProvider("test-token"), cleanup
}

func TestWrikeProvider_FetchParent_Success(t *testing.T) {
	p, cleanup := wrikeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tasks/parent-task-1") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "parent-task-1", "title": "Parent task", "status": "Active"},
				},
			})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer cleanup()

	task := &Task{ID: "child-1"}
	task.SetMetadata("wrike_super_task_id", "parent-task-1")

	parent, err := p.FetchParent(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchParent() error = %v", err)
	}
	if parent == nil || parent.ID != "parent-task-1" {
		t.Fatalf("FetchParent() = %+v, want parent-task-1", parent)
	}
	if parent.Title != "Parent task" {
		t.Errorf("parent.Title = %q, want 'Parent task'", parent.Title)
	}
}

func TestWrikeProvider_FetchSiblings_Success(t *testing.T) {
	p, cleanup := wrikeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/folders/folder-1/tasks") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "self-task", "title": "Self", "status": "Active"},
					{"id": "sib-1", "title": "Sibling 1", "status": "Active"},
					{"id": "sib-2", "title": "Sibling 2", "status": "Completed"},
				},
			})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer cleanup()

	task := &Task{ID: "self-task"}
	task.SetMetadata("wrike_parent_folder_id", "folder-1")

	siblings, err := p.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	if len(siblings) != 2 {
		t.Fatalf("FetchSiblings() returned %d, want 2 (self excluded)", len(siblings))
	}
	for _, s := range siblings {
		if s.ID == "self-task" {
			t.Error("FetchSiblings() must not include the task itself")
		}
	}
}

func TestWrikeProvider_FetchParent_FetchError(t *testing.T) {
	p, cleanup := wrikeTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer cleanup()

	task := &Task{ID: "child-1"}
	task.SetMetadata("wrike_super_task_id", "parent-task-1")

	if _, err := p.FetchParent(context.Background(), task); err == nil {
		t.Error("FetchParent() should error when the parent fetch fails")
	}
}

func TestWrikeProvider_FetchSiblings_FolderError(t *testing.T) {
	p, cleanup := wrikeTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	task := &Task{ID: "self-task"}
	task.SetMetadata("wrike_parent_folder_id", "folder-1")

	if _, err := p.FetchSiblings(context.Background(), task); err == nil {
		t.Error("FetchSiblings() should error when the folder fetch fails")
	}
}
