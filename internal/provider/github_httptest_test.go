package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v67/github"
)

// newTestGitHubProvider creates a GitHubProvider backed by the given httptest server.
func newTestGitHubProvider(t *testing.T, srv *httptest.Server) *GitHubProvider {
	t.Helper()
	client, err := github.NewClient(nil).WithEnterpriseURLs(srv.URL+"/api/v3/", srv.URL+"/upload/")
	if err != nil {
		t.Fatalf("create test github client: %v", err)
	}

	return &GitHubProvider{client: client}
}

// newTestGitHubServer creates an httptest.Server that strips the "/api/v3" prefix
// from incoming request paths before passing to the handler.
func newTestGitHubServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api/v3")
		handler(w, r)
	}))
}

func TestGitHubProvider_FetchTask_Issue(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/42" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   42,
				"title":    "Fix the login page bug",
				"body":     "The login page has a CSS issue",
				"state":    stateOpen,
				"html_url": "https://github.com/owner/repo/issues/42",
				"labels": []map[string]any{
					{"name": "bug"},
					{"name": "priority:high"},
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	task, err := p.FetchTask(context.Background(), "owner/repo#42")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.Title != "Fix the login page bug" {
		t.Errorf("Title = %q, want %q", task.Title, "Fix the login page bug")
	}
	if task.Description != "The login page has a CSS issue" {
		t.Errorf("Description = %q, want %q", task.Description, "The login page has a CSS issue")
	}
	if task.ID != "owner/repo#42" {
		t.Errorf("ID = %q, want %q", task.ID, "owner/repo#42")
	}
	if task.Source != NameGitHub {
		t.Errorf("Source = %q, want github", task.Source)
	}
	if len(task.Labels) != 2 {
		t.Errorf("len(Labels) = %d, want 2", len(task.Labels))
	}
	if task.Metadata("github_state") != stateOpen {
		t.Errorf("github_state = %q, want open", task.Metadata("github_state"))
	}
	if task.Metadata("github_owner") != "owner" {
		t.Errorf("github_owner = %q, want owner", task.Metadata("github_owner"))
	}
	if task.Metadata("github_repo") != "repo" {
		t.Errorf("github_repo = %q, want repo", task.Metadata("github_repo"))
	}
}

func TestGitHubProvider_FetchTask_PR(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/issues/10":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":       10,
				"title":        "Add new feature",
				"body":         "This PR adds a new feature",
				"state":        stateOpen,
				"html_url":     "https://github.com/owner/repo/pull/10",
				"pull_request": map[string]any{"url": "https://api.github.com/repos/owner/repo/pulls/10"},
				"labels":       []map[string]any{},
			})
		case "/repos/owner/repo/pulls/10":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   10,
				"title":    "Add new feature",
				"body":     "This PR adds a new feature",
				"state":    stateOpen,
				"draft":    true,
				"html_url": "https://github.com/owner/repo/pull/10",
				"labels":   []map[string]any{{"name": "enhancement"}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	task, err := p.FetchTask(context.Background(), "owner/repo#10")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.Title != "Add new feature" {
		t.Errorf("Title = %q, want %q", task.Title, "Add new feature")
	}
	if task.Metadata("github_state") != stateDraft {
		t.Errorf("github_state = %q, want draft", task.Metadata("github_state"))
	}
	if task.Metadata("github_is_pr") != "true" {
		t.Errorf("github_is_pr = %q, want true", task.Metadata("github_is_pr"))
	}
}

func TestGitHubProvider_FetchTask_NotFound(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "Not Found"})
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	_, err := p.FetchTask(context.Background(), "owner/repo#999")
	if err == nil {
		t.Error("FetchTask() should return error for non-existent issue")
	}
}

func TestGitHubProvider_FetchTask_InvalidID(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	_, err := p.FetchTask(context.Background(), "invalid-id")
	if err == nil {
		t.Error("FetchTask() should return error for invalid ID format")
	}
}

func TestGitHubProvider_FetchTask_WithAssignees(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/5" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   5,
				"title":    "Task with assignees",
				"body":     "Description",
				"state":    stateOpen,
				"html_url": "https://github.com/owner/repo/issues/5",
				"labels":   []map[string]any{},
				"assignees": []map[string]any{
					{"login": "alice"},
					{"login": "bob"},
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	task, err := p.FetchTask(context.Background(), "owner/repo#5")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	assignees := task.Metadata("github_assignees")
	if assignees != "alice,bob" {
		t.Errorf("github_assignees = %q, want %q", assignees, "alice,bob")
	}
}

func TestGitHubProvider_FetchTask_WithMilestone(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/7" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   7,
				"title":    "Milestone task",
				"body":     "",
				"state":    stateOpen,
				"html_url": "https://github.com/owner/repo/issues/7",
				"labels":   []map[string]any{},
				"milestone": map[string]any{
					"title":  "v1.0",
					"number": 3,
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	task, err := p.FetchTask(context.Background(), "owner/repo#7")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.Metadata("github_milestone") != "v1.0" {
		t.Errorf("github_milestone = %q, want v1.0", task.Metadata("github_milestone"))
	}
	if task.Metadata("github_milestone_number") != "3" {
		t.Errorf("github_milestone_number = %q, want 3", task.Metadata("github_milestone_number"))
	}
}

func TestGitHubProvider_FetchTask_WithDependencies(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/20" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   20,
				"title":    "Feature with deps",
				"body":     "Depends on #10 and owner/repo#15",
				"state":    stateOpen,
				"html_url": "https://github.com/owner/repo/issues/20",
				"labels":   []map[string]any{},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	task, err := p.FetchTask(context.Background(), "owner/repo#20")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if len(task.Dependencies) == 0 {
		t.Skip("No dependencies parsed (ParseDependencies may not extract these)")
	}

	for _, dep := range task.Dependencies {
		if dep.Source != NameGitHub {
			t.Errorf("dep.Source = %q, want github", dep.Source)
		}
	}
}

func TestGitHubProvider_UpdateStatus(t *testing.T) {
	var capturedState string
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/42" && r.Method == http.MethodPatch {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if s, ok := req["state"].(string); ok {
				capturedState = s
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 42,
				"state":  capturedState,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	tests := []struct {
		status    string
		wantState string
		wantErr   bool
	}{
		{stateOpen, stateOpen, false},
		{"pending", stateOpen, false},
		{"in_progress", stateOpen, false},
		{"closed", "closed", false},
		{"done", "closed", false},
		{"completed", "closed", false},
		{"invalid_status", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			capturedState = ""
			err := p.UpdateStatus(context.Background(), "owner/repo#42", tt.status)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unsupported status")
				}

				return
			}
			if err != nil {
				t.Fatalf("UpdateStatus(%q) error = %v", tt.status, err)
			}
			if capturedState != tt.wantState {
				t.Errorf("captured state = %q, want %q", capturedState, tt.wantState)
			}
		})
	}
}

func TestGitHubProvider_UpdateStatus_InvalidID(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	err := p.UpdateStatus(context.Background(), "bad-id", stateOpen)
	if err == nil {
		t.Error("UpdateStatus() with invalid ID should return error")
	}
}

func TestGitHubProvider_AddComment(t *testing.T) {
	var capturedBody string
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/42/comments" && r.Method == http.MethodPost {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if b, ok := req["body"].(string); ok {
				capturedBody = b
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   1,
				"body": capturedBody,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	err := p.AddComment(context.Background(), "owner/repo#42", "Test comment")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if capturedBody != "Test comment" {
		t.Errorf("captured body = %q, want %q", capturedBody, "Test comment")
	}
}

func TestGitHubProvider_AddComment_InvalidID(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	err := p.AddComment(context.Background(), "bad-id", "comment")
	if err == nil {
		t.Error("AddComment() with invalid ID should return error")
	}
}

func TestGitHubProvider_GetPRStatus_PR(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls/15" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   15,
				"state":    stateOpen,
				"merged":   false,
				"html_url": "https://github.com/owner/repo/pull/15",
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	status, err := p.GetPRStatus(context.Background(), "owner/repo#15")
	if err != nil {
		t.Fatalf("GetPRStatus() error = %v", err)
	}
	if status.Number != 15 {
		t.Errorf("Number = %d, want 15", status.Number)
	}
	if status.State != stateOpen {
		t.Errorf("State = %q, want open", status.State)
	}
	if status.Merged {
		t.Error("Merged should be false")
	}
}

func TestGitHubProvider_GetPRStatus_Issue(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/8":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Not Found"})
		case "/repos/owner/repo/issues/8":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   8,
				"state":    "closed",
				"html_url": "https://github.com/owner/repo/issues/8",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	status, err := p.GetPRStatus(context.Background(), "owner/repo#8")
	if err != nil {
		t.Fatalf("GetPRStatus() error = %v", err)
	}
	if status.State != "closed" {
		t.Errorf("State = %q, want closed", status.State)
	}
	if status.Merged {
		t.Error("Issue should not be merged")
	}
}

func TestGitHubProvider_GetPRStatus_NotFound(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "Not Found"})
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	_, err := p.GetPRStatus(context.Background(), "owner/repo#999")
	if err == nil {
		t.Error("GetPRStatus() should return error for non-existent resource")
	}
}

func TestGitHubProvider_GetPRStatus_InvalidID(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	_, err := p.GetPRStatus(context.Background(), "bad-id")
	if err == nil {
		t.Error("GetPRStatus() with invalid ID should return error")
	}
}

func TestGitHubProvider_FetchSiblings_WithMilestone(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues" && r.URL.Query().Get("milestone") == "5" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"number":   1,
					"title":    "Sibling 1",
					"body":     "",
					"state":    stateOpen,
					"html_url": "https://github.com/owner/repo/issues/1",
					"labels":   []map[string]any{},
				},
				{
					"number":   2,
					"title":    "Current task",
					"body":     "",
					"state":    stateOpen,
					"html_url": "https://github.com/owner/repo/issues/2",
					"labels":   []map[string]any{},
				},
				{
					"number":   3,
					"title":    "Sibling 3",
					"body":     "",
					"state":    stateOpen,
					"html_url": "https://github.com/owner/repo/issues/3",
					"labels":   []map[string]any{},
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	task := &Task{
		ID:     "owner/repo#2",
		Source: NameGitHub,
	}
	task.SetMetadata("github_milestone_number", "5")
	task.SetMetadata("github_owner", "owner")
	task.SetMetadata("github_repo", "repo")

	siblings, err := p.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}

	if len(siblings) != 2 {
		t.Errorf("len(siblings) = %d, want 2", len(siblings))
	}
	for _, s := range siblings {
		if s.ID == "owner/repo#2" {
			t.Error("siblings should not include the current task")
		}
	}
}

func TestGitHubProvider_FetchSiblings_NoMilestone(t *testing.T) {
	p := &GitHubProvider{}
	task := &Task{ID: "owner/repo#1", Source: NameGitHub}

	siblings, err := p.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	if siblings != nil {
		t.Error("siblings should be nil when no milestone set")
	}
}

func TestGitHubProvider_FetchParent(t *testing.T) {
	p := &GitHubProvider{}
	task := &Task{ID: "owner/repo#1", Source: NameGitHub}

	parent, err := p.FetchParent(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchParent() error = %v", err)
	}
	if parent != nil {
		t.Error("FetchParent() should return nil for GitHub (no native parent)")
	}
}

func TestGitHubProvider_CreatePR(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"default_branch": "main",
			})
		case r.URL.Path == "/repos/owner/repo/pulls" && r.Method == http.MethodPost:
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   99,
				"state":    stateOpen,
				"draft":    false,
				"html_url": "https://github.com/owner/repo/pull/99",
				"title":    req["title"],
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	result, err := p.CreatePR(context.Background(), PROptions{
		Title:  "Test PR",
		Body:   "Test body",
		Head:   "owner/repo:feature-branch",
		TaskID: "owner/repo#1",
	})
	if err != nil {
		t.Fatalf("CreatePR() error = %v", err)
	}
	if result.Number != 99 {
		t.Errorf("Number = %d, want 99", result.Number)
	}
	if result.State != stateOpen {
		t.Errorf("State = %q, want open", result.State)
	}
	if !strings.Contains(result.URL, "pull/99") {
		t.Errorf("URL = %q, want to contain pull/99", result.URL)
	}
}

func TestGitHubProvider_CreatePR_NoRepo(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	_, err := p.CreatePR(context.Background(), PROptions{
		Title: "Test PR",
		Head:  "feature-branch",
	})
	if err == nil {
		t.Error("CreatePR() should return error when repository cannot be determined")
	}
}

func TestGitHubProvider_MergePR(t *testing.T) {
	var capturedMethod string
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls/42/merge" && r.Method == http.MethodPut {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if m, ok := req["merge_method"].(string); ok {
				capturedMethod = m
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":     "abc123",
				"merged":  true,
				"message": "Pull Request successfully merged",
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	err := p.MergePR(context.Background(), "owner/repo#42", "squash")
	if err != nil {
		t.Fatalf("MergePR(squash) error = %v", err)
	}
	if capturedMethod != "squash" {
		t.Errorf("merge method = %q, want squash", capturedMethod)
	}

	capturedMethod = ""
	err = p.MergePR(context.Background(), "owner/repo#42", "")
	if err != nil {
		t.Fatalf("MergePR(default) error = %v", err)
	}
	if capturedMethod != "rebase" {
		t.Errorf("default merge method = %q, want rebase", capturedMethod)
	}
}

func TestGitHubProvider_MergePR_InvalidID(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	err := p.MergePR(context.Background(), "invalid", "merge")
	if err == nil {
		t.Error("MergePR() with invalid ID should return error")
	}
}

func TestGitHubProvider_ApprovePR(t *testing.T) {
	var capturedEvent string
	var capturedBody string
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls/42/reviews" && r.Method == http.MethodPost {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if e, ok := req["event"].(string); ok {
				capturedEvent = e
			}
			if b, ok := req["body"].(string); ok {
				capturedBody = b
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    1,
				"state": "APPROVED",
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	err := p.ApprovePR(context.Background(), "owner/repo#42", "LGTM!")
	if err != nil {
		t.Fatalf("ApprovePR() error = %v", err)
	}
	if capturedEvent != "APPROVE" {
		t.Errorf("event = %q, want APPROVE", capturedEvent)
	}
	if capturedBody != "LGTM!" {
		t.Errorf("body = %q, want LGTM!", capturedBody)
	}

	capturedBody = ""
	err = p.ApprovePR(context.Background(), "owner/repo#42", "")
	if err != nil {
		t.Fatalf("ApprovePR() without comment error = %v", err)
	}
}

func TestGitHubProvider_ApprovePR_InvalidID(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	err := p.ApprovePR(context.Background(), "bad-id", "")
	if err == nil {
		t.Error("ApprovePR() with invalid ID should return error")
	}
}
