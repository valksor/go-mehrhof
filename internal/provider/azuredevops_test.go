package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripHTMLTags(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<div>Multi<br/>Line</div>", "MultiLine"},
		{"No tags", "No tags"},
		{"", ""},
		{"   <em>trim me</em>   ", "trim me"},
		{"<a href=\"http://x\">link</a>", "link"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := stripHTMLTags(tc.in); got != tc.want {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeAzureDevOpsID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"azuredevops:12345", "12345"},
		{"12345", "12345"},
		{"  azuredevops:42  ", "azuredevops:42"}, // TrimPrefix only matches at start; surrounding whitespace stays unless leading prefix matched
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeAzureDevOpsID(tc.in); got != tc.want {
				t.Errorf("normalizeAzureDevOpsID(%q) = %q", tc.in, got)
			}
		})
	}
}

func TestAzureDevOpsProvider_Name(t *testing.T) {
	p := NewAzureDevOpsProvider("https://dev.azure.com", "org", "proj", "tok", "")
	if got := p.Name(); got != NameAzureDevOps {
		t.Errorf("Name = %q", got)
	}
}

func TestAzureDevOpsProvider_DefaultRepoName(t *testing.T) {
	p := NewAzureDevOpsProvider("https://dev.azure.com", "org", "proj", "tok", "")
	if p.repoName != "proj" {
		t.Errorf("repoName = %q, want proj (default to project)", p.repoName)
	}
}

func TestAzureDevOpsProvider_NoToken(t *testing.T) {
	p := NewAzureDevOpsProvider("https://dev.azure.com", "org", "proj", "", "myrepo")
	ctx := context.Background()

	if _, err := p.FetchTask(ctx, "1"); err == nil || !strings.Contains(err.Error(), "AZURE_DEVOPS_TOKEN") {
		t.Errorf("FetchTask should require token, got %v", err)
	}
	if err := p.UpdateStatus(ctx, "1", "Active"); err == nil {
		t.Error("UpdateStatus should require token")
	}
	if err := p.AddComment(ctx, "1", "comment"); err == nil {
		t.Error("AddComment should require token")
	}
	if _, err := p.CreatePR(ctx, PROptions{}); err == nil {
		t.Error("CreatePR should require token")
	}
}

func TestAzureDevOpsProvider_FetchTask_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/_apis/wit/workitems/42") {
			http.Error(w, "unexpected path", http.StatusBadRequest)

			return
		}

		wi := azureWorkItem{
			ID:  42,
			URL: "https://dev.azure.com/org/proj/_workitems/edit/42",
			Fields: map[string]any{
				"System.Title":        "Test task",
				"System.Description":  "<p>Some <strong>html</strong></p>",
				"System.WorkItemType": "Bug",
				"System.State":        "Active",
				"System.Tags":         "frontend; urgent",
			},
		}
		_ = json.NewEncoder(w).Encode(wi)
	}))
	defer srv.Close()

	p := NewAzureDevOpsProvider(srv.URL, "org", "proj", "secret", "myrepo")

	task, err := p.FetchTask(context.Background(), "42")
	if err != nil {
		t.Fatalf("FetchTask: %v", err)
	}

	if task.ID != "42" {
		t.Errorf("ID = %q", task.ID)
	}
	if task.Title != "Test task" {
		t.Errorf("Title = %q", task.Title)
	}
	if !strings.Contains(task.Description, "html") {
		t.Errorf("Description should contain unwrapped text, got %q", task.Description)
	}
	if strings.Contains(task.Description, "<p>") {
		t.Errorf("Description should not contain HTML tags, got %q", task.Description)
	}
	if task.Type != "bug" {
		t.Errorf("Type = %q (should be lowercase work item type)", task.Type)
	}
	wantLabels := []string{"frontend", "urgent", "Active"}
	if len(task.Labels) != len(wantLabels) {
		t.Errorf("Labels = %v, want %v", task.Labels, wantLabels)
	}
}

func TestAzureDevOpsProvider_FetchTask_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	p := NewAzureDevOpsProvider(srv.URL, "org", "proj", "secret", "myrepo")
	if _, err := p.FetchTask(context.Background(), "99"); err == nil {
		t.Error("expected error for 404")
	}
}

func TestWorkItemWebURL(t *testing.T) {
	c := newAzureDevOpsClient("https://dev.azure.com", "myorg", "myproj", "tok")
	got := c.workItemWebURL(123)
	want := "https://dev.azure.com/myorg/myproj/_workitems/edit/123"
	if got != want {
		t.Errorf("workItemWebURL = %q, want %q", got, want)
	}
}

func TestAzureDevOpsProvider_UpdateStatus(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPatch {
			http.Error(w, "want PATCH", http.StatusMethodNotAllowed)

			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	p := NewAzureDevOpsProvider(srv.URL, "org", "proj", "secret", "myrepo")
	if err := p.UpdateStatus(context.Background(), "1", "Active"); err != nil {
		t.Errorf("UpdateStatus: %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestAzureDevOpsProvider_AddComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	p := NewAzureDevOpsProvider(srv.URL, "org", "proj", "secret", "myrepo")
	if err := p.AddComment(context.Background(), "1", "hello"); err != nil {
		t.Errorf("AddComment: %v", err)
	}
}
