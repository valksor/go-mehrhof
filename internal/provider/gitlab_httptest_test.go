package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newGitLabTestProvider creates a GitLabProvider backed by a test HTTP server.
// The gitlab SDK client uses the baseURL set at construction time, so we pass
// the test server URL as the host. The SDK appends "/api/v4" automatically via
// gitlab.WithBaseURL.
func newGitLabTestProvider(t *testing.T, handler http.HandlerFunc) (*GitLabProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	p, err := NewGitLabProviderWithHost("test-token", srv.URL)
	if err != nil {
		srv.Close()
		t.Fatalf("NewGitLabProviderWithHost() error = %v", err)
	}

	return p, srv
}

// ============================================================
// FetchTask — issue
// ============================================================

func TestGitLabProvider_FetchTask_Issue_HTTPTest(t *testing.T) {
	issue := map[string]any{
		"id":          100,
		"iid":         42,
		"title":       "Fix login bug",
		"description": "Details about the bug",
		"web_url":     "https://gitlab.example.com/group/repo/-/issues/42",
		"state":       "opened",
		"labels":      []string{"bug"},
		"assignees":   []map[string]any{},
		"milestone":   nil,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issue) //nolint:errchkjson // test helper
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	task, err := p.FetchTask(context.Background(), "group/repo#42")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.ID != "group/repo#42" {
		t.Errorf("task.ID = %q, want group/repo#42", task.ID)
	}
	if task.Title != "Fix login bug" {
		t.Errorf("task.Title = %q, want 'Fix login bug'", task.Title)
	}
	if task.Description != "Details about the bug" {
		t.Errorf("task.Description = %q, want 'Details about the bug'", task.Description)
	}
	if task.Source != NameGitLab {
		t.Errorf("task.Source = %q, want gitlab", task.Source)
	}
	if task.Metadata("gitlab_state") != "opened" {
		t.Errorf("gitlab_state = %q, want opened", task.Metadata("gitlab_state"))
	}
	if task.Metadata("gitlab_project") != "group/repo" {
		t.Errorf("gitlab_project = %q, want group/repo", task.Metadata("gitlab_project"))
	}
}

// ============================================================
// FetchTask — merge request
// ============================================================

func TestGitLabProvider_FetchTask_MR_HTTPTest(t *testing.T) {
	mr := map[string]any{
		"id":          200,
		"iid":         7,
		"title":       "Add dark mode",
		"description": "Implements dark mode",
		"web_url":     "https://gitlab.example.com/group/repo/-/merge_requests/7",
		"state":       "opened",
		"draft":       false,
		"labels":      []string{"enhancement"},
		"assignees":   []map[string]any{},
		"milestone":   nil,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mr) //nolint:errchkjson // test helper
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	task, err := p.FetchTask(context.Background(), "group/repo!7")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.ID != "group/repo!7" {
		t.Errorf("task.ID = %q, want group/repo!7", task.ID)
	}
	if task.Title != "Add dark mode" {
		t.Errorf("task.Title = %q, want 'Add dark mode'", task.Title)
	}
	if task.Metadata("gitlab_is_mr") != "true" {
		t.Errorf("gitlab_is_mr = %q, want true", task.Metadata("gitlab_is_mr"))
	}
	if task.Metadata("gitlab_state") != "opened" {
		t.Errorf("gitlab_state = %q, want opened", task.Metadata("gitlab_state"))
	}
}

func TestGitLabProvider_FetchTask_InvalidID(t *testing.T) {
	p, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	_, err = p.FetchTask(context.Background(), "not-a-valid-id")
	if err == nil {
		t.Error("FetchTask() should return error for invalid ID")
	}
}

// ============================================================
// UpdateStatus — issue
// ============================================================

func TestGitLabProvider_UpdateStatus_Issue_HTTPTest(t *testing.T) {
	var capturedBody map[string]any

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		}
		w.Header().Set("Content-Type", "application/json")
		// Return a minimal updated issue
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
			"id":    1,
			"iid":   5,
			"title": "Issue",
			"state": "closed",
		})
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	err := p.UpdateStatus(context.Background(), "group/repo#5", "done")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if stateEvent, ok := capturedBody["state_event"].(string); !ok || stateEvent != "close" {
		t.Errorf("state_event = %v, want close", capturedBody["state_event"])
	}
}

func TestGitLabProvider_UpdateStatus_Reopen_HTTPTest(t *testing.T) {
	var capturedBody map[string]any

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
			"id":    1,
			"iid":   5,
			"title": "Issue",
			"state": "opened",
		})
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	err := p.UpdateStatus(context.Background(), "group/repo#5", "open")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if stateEvent, ok := capturedBody["state_event"].(string); !ok || stateEvent != "reopen" {
		t.Errorf("state_event = %v, want reopen", capturedBody["state_event"])
	}
}

// ============================================================
// AddComment — issue
// ============================================================

func TestGitLabProvider_AddComment_Issue_HTTPTest(t *testing.T) {
	var capturedBody map[string]any

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
			"id":   99,
			"body": "Great progress!",
		})
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	err := p.AddComment(context.Background(), "group/repo#10", "Great progress!")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	if body, ok := capturedBody["body"].(string); !ok || body != "Great progress!" {
		t.Errorf("body = %v, want 'Great progress!'", capturedBody["body"])
	}
}

func TestGitLabProvider_AddComment_MR_HTTPTest(t *testing.T) {
	var capturedBody map[string]any

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
			"id":   100,
			"body": "LGTM",
		})
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	err := p.AddComment(context.Background(), "group/repo!3", "LGTM")
	if err != nil {
		t.Fatalf("AddComment() for MR error = %v", err)
	}

	if body, ok := capturedBody["body"].(string); !ok || body != "LGTM" {
		t.Errorf("body = %v, want LGTM", capturedBody["body"])
	}
}

func TestGitLabProvider_AddComment_InvalidID(t *testing.T) {
	p, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	err = p.AddComment(context.Background(), "not-a-valid-id", "comment")
	if err == nil {
		t.Error("AddComment() should return error for invalid ID")
	}
}

// ============================================================
// FetchSiblings — always returns nil for GitLab
// ============================================================

func TestGitLabProvider_FetchSiblings_HTTPTest(t *testing.T) {
	// GitLab doesn't implement native sibling fetching — always nil
	p, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	task := &Task{ID: "group/repo#5"}
	task.SetMetadata("gitlab_project", "group/repo")
	task.SetMetadata("gitlab_milestone_id", "10")

	siblings, err := p.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	if siblings != nil {
		t.Errorf("FetchSiblings() = %v, want nil", siblings)
	}
}

// ============================================================
// FetchParent — always returns nil for GitLab
// ============================================================

func TestGitLabProvider_FetchParent_HTTPTest(t *testing.T) {
	p, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	task := &Task{ID: "group/repo#5"}
	task.SetMetadata("gitlab_project", "group/repo")

	parent, err := p.FetchParent(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchParent() error = %v", err)
	}
	if parent != nil {
		t.Errorf("FetchParent() = %v, want nil (GitLab has no native hierarchy)", parent)
	}
}

// ============================================================
// Error handling paths
// ============================================================

func TestGitLabProvider_FetchTask_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized) // 401 triggers immediate error (no retry)
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	_, err := p.FetchTask(context.Background(), "group/repo#1")
	if err == nil {
		t.Error("FetchTask() should return error for 401 response")
	}
}

func TestGitLabProvider_UpdateStatus_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	err := p.UpdateStatus(context.Background(), "group/repo#1", "done")
	if err == nil {
		t.Error("UpdateStatus() should return error for 401 response")
	}
}

func TestGitLabProvider_AddComment_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	p, srv := newGitLabTestProvider(t, handler)
	defer srv.Close()

	err := p.AddComment(context.Background(), "group/repo#1", "comment")
	if err == nil {
		t.Error("AddComment() should return error for 401 response")
	}
}
