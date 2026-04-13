package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrikeProvider_taskDataToTask(t *testing.T) {
	p := NewWrikeProvider("test-token")

	data := &wrikeTaskData{
		ID:             "IEAABC123",
		Title:          "Implement login flow",
		Description:    "Full description of the login flow",
		Permalink:      "https://www.wrike.com/open.htm/task-IEAABC123",
		Status:         "Active",
		ParentIDs:      []string{"folder-1", "folder-2"},
		SuperParentIDs: []string{"space-1"},
		SuperTaskIDs:   []string{"parent-task-1"},
		SubTaskIDs:     []string{"child-1", "child-2"},
	}

	task := p.taskDataToTask(data)

	if task.ID != "IEAABC123" {
		t.Errorf("ID = %q, want IEAABC123", task.ID)
	}
	if task.Title != "Implement login flow" {
		t.Errorf("Title = %q, want %q", task.Title, "Implement login flow")
	}
	if task.Description != "Full description of the login flow" {
		t.Errorf("Description = %q, want full description", task.Description)
	}
	if task.URL != "https://www.wrike.com/open.htm/task-IEAABC123" {
		t.Errorf("URL = %q", task.URL)
	}
	if task.Source != "wrike" {
		t.Errorf("Source = %q, want wrike", task.Source)
	}
	if len(task.Labels) != 1 || task.Labels[0] != "Active" {
		t.Errorf("Labels = %v, want [Active]", task.Labels)
	}
	if task.Metadata("wrike_parent_folder_id") != "folder-1" {
		t.Errorf("wrike_parent_folder_id = %q, want folder-1", task.Metadata("wrike_parent_folder_id"))
	}
	if task.Metadata("wrike_super_task_id") != "parent-task-1" {
		t.Errorf("wrike_super_task_id = %q, want parent-task-1", task.Metadata("wrike_super_task_id"))
	}
}

func TestWrikeProvider_taskDataToTask_NoParents(t *testing.T) {
	p := NewWrikeProvider("test-token")

	data := &wrikeTaskData{
		ID:    "IEAADEF456",
		Title: "Orphan task",
	}

	task := p.taskDataToTask(data)

	if task.Metadata("wrike_parent_folder_id") != "" {
		t.Errorf("wrike_parent_folder_id should be empty, got %q", task.Metadata("wrike_parent_folder_id"))
	}
	if task.Metadata("wrike_super_task_id") != "" {
		t.Errorf("wrike_super_task_id should be empty, got %q", task.Metadata("wrike_super_task_id"))
	}
}

func TestWrikeProvider_taskDataToTask_SuperParentFallback(t *testing.T) {
	p := NewWrikeProvider("test-token")

	// When ParentIDs is empty but SuperParentIDs is present, should fallback
	data := &wrikeTaskData{
		ID:             "IEAAGHI789",
		Title:          "Task with super parent only",
		SuperParentIDs: []string{"space-42"},
	}

	task := p.taskDataToTask(data)

	if task.Metadata("wrike_parent_folder_id") != "space-42" {
		t.Errorf("wrike_parent_folder_id = %q, want space-42 (fallback to SuperParentIDs)", task.Metadata("wrike_parent_folder_id"))
	}
}

func TestWrikeProvider_FetchTask_NoToken(t *testing.T) {
	p := NewWrikeProvider("")

	_, err := p.FetchTask(context.Background(), "IEAABC123")
	if err == nil {
		t.Error("FetchTask() should return error when token is empty")
	}
}

func TestWrikeProvider_FetchParent_NoToken(t *testing.T) {
	p := NewWrikeProvider("")

	task := &Task{ID: "task-1"}
	task.SetMetadata("wrike_super_task_id", "parent-1")

	_, err := p.FetchParent(context.Background(), task)
	if err == nil {
		t.Error("FetchParent() should return error when token is empty")
	}
}

func TestWrikeProvider_FetchParent_NoParent(t *testing.T) {
	p := NewWrikeProvider("test-token")

	task := &Task{ID: "task-1"}
	// No wrike_super_task_id metadata

	parent, err := p.FetchParent(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchParent() error = %v", err)
	}
	if parent != nil {
		t.Error("FetchParent() should return nil when task has no parent")
	}
}

func TestWrikeProvider_FetchSiblings_NoToken(t *testing.T) {
	p := NewWrikeProvider("")

	task := &Task{ID: "task-1"}
	task.SetMetadata("wrike_parent_folder_id", "folder-1")

	_, err := p.FetchSiblings(context.Background(), task)
	if err == nil {
		t.Error("FetchSiblings() should return error when token is empty")
	}
}

func TestWrikeProvider_FetchSiblings_NoFolder(t *testing.T) {
	p := NewWrikeProvider("test-token")

	task := &Task{ID: "task-1"}
	// No wrike_parent_folder_id metadata

	siblings, err := p.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	if siblings != nil {
		t.Error("FetchSiblings() should return nil when task has no parent folder")
	}
}

func TestWrikeProvider_UpdateStatus_NoToken(t *testing.T) {
	p := NewWrikeProvider("")

	err := p.UpdateStatus(context.Background(), "IEAABC123", "Active")
	if err == nil {
		t.Error("UpdateStatus() should return error when token is empty")
	}
}

func TestWrikeProvider_AddComment_NoToken(t *testing.T) {
	p := NewWrikeProvider("")

	err := p.AddComment(context.Background(), "IEAABC123", "test comment")
	if err == nil {
		t.Error("AddComment() should return error when token is empty")
	}
}

func TestWrikeProvider_FetchTask_ServerResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":             "IEAABC123",
					"title":          "Test Wrike Task",
					"description":    "A test description",
					"permalink":      "https://www.wrike.com/open.htm/task-IEAABC123",
					"status":         "Active",
					"parentIds":      []string{"folder-1"},
					"superTaskIds":   []string{"parent-task-1"},
					"superParentIds": []string{},
					"subTaskIds":     []string{},
				},
			},
		})
	}))
	defer srv.Close()

	// Override the package-level httpClient to redirect to our test server
	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewWrikeProvider("test-token")
	task, err := p.FetchTask(context.Background(), "IEAABC123")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.ID != "IEAABC123" {
		t.Errorf("ID = %q, want IEAABC123", task.ID)
	}
	if task.Title != "Test Wrike Task" {
		t.Errorf("Title = %q, want Test Wrike Task", task.Title)
	}
	if task.Metadata("wrike_parent_folder_id") != "folder-1" {
		t.Errorf("wrike_parent_folder_id = %q, want folder-1", task.Metadata("wrike_parent_folder_id"))
	}
}

func TestWrikeProvider_FetchTask_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
		})
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewWrikeProvider("test-token")
	_, err := p.FetchTask(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("FetchTask() should return error for empty data response")
	}
}

func TestWrikeProvider_UpdateStatus_ServerResponse(t *testing.T) {
	var capturedStatus string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			capturedStatus = req["status"]
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewWrikeProvider("test-token")
	err := p.UpdateStatus(context.Background(), "IEAABC123", "Completed")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if capturedStatus != "Completed" {
		t.Errorf("captured status = %q, want Completed", capturedStatus)
	}
}

func TestWrikeProvider_UpdateStatus_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewWrikeProvider("test-token")
	err := p.UpdateStatus(context.Background(), "IEAABC123", "Active")
	if err == nil {
		t.Error("UpdateStatus() should return error for 500 response")
	}
}

func TestWrikeProvider_AddComment_ServerResponse(t *testing.T) {
	var capturedText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			capturedText = req["text"]
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewWrikeProvider("test-token")
	err := p.AddComment(context.Background(), "IEAABC123", "Great progress!")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if capturedText != "Great progress!" {
		t.Errorf("captured text = %q, want %q", capturedText, "Great progress!")
	}
}

func TestWrikeProvider_AddComment_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}
	defer func() { httpClient.Transport = origTransport }()

	p := NewWrikeProvider("test-token")
	err := p.AddComment(context.Background(), "IEAABC123", "comment")
	if err == nil {
		t.Error("AddComment() should return error for 500 response")
	}
}

// rewriteTransport rewrites all request URLs to point to the test server.
type rewriteTransport struct {
	base      http.RoundTripper
	targetURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to our test server while preserving the path
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	// Parse the target URL to get host
	req.URL.Host = t.targetURL[len("http://"):]

	return t.base.RoundTrip(req)
}
