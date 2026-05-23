package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// jiraTestServer starts an httptest server, redirects the package httpClient to
// it, and returns a JiraProvider plus a cleanup func.
func jiraTestServer(t *testing.T, handler http.HandlerFunc) (*JiraProvider, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{base: http.DefaultTransport, targetURL: srv.URL}

	cleanup := func() {
		httpClient.Transport = origTransport
		srv.Close()
	}

	return NewJiraProvider("https://test.atlassian.net", "user@test.com", "test-token"), cleanup
}

func TestJiraProvider_UpdateStatus(t *testing.T) {
	var transitioned bool
	p, cleanup := jiraTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/transitions"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"transitions": []map[string]any{
					{"id": "11", "name": "To Do"},
					{"id": "21", "name": "In Progress"},
					{"id": "31", "name": "Done"},
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/transitions"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			transitioned = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer cleanup()

	if err := p.UpdateStatus(context.Background(), "PROJ-1", "Done"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if !transitioned {
		t.Error("expected a transition POST to be made")
	}
}

func TestJiraProvider_UpdateStatus_NoMatch(t *testing.T) {
	p, cleanup := jiraTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transitions": []map[string]any{{"id": "11", "name": "To Do"}},
		})
	})
	defer cleanup()

	if err := p.UpdateStatus(context.Background(), "PROJ-1", "Nonexistent"); err == nil {
		t.Error("UpdateStatus() should error when no transition matches")
	}
}

func TestJiraProvider_UpdateStatus_NoToken(t *testing.T) {
	p := NewJiraProvider("https://x", "e", "")
	if err := p.UpdateStatus(context.Background(), "PROJ-1", "Done"); err == nil {
		t.Error("UpdateStatus() should error without token")
	}
}

func TestJiraProvider_FetchParent(t *testing.T) {
	p, cleanup := jiraTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jiraIssue{
			ID:     "100",
			Key:    "PROJ-100",
			Fields: jiraIssueFields{Summary: "Parent epic", Status: &jiraStatus{Name: "Open"}},
		})
	})
	defer cleanup()

	t.Run("with parent metadata", func(t *testing.T) {
		task := &Task{ID: "PROJ-101"}
		task.SetMetadata("jira_parent_key", "PROJ-100")
		parent, err := p.FetchParent(context.Background(), task)
		if err != nil {
			t.Fatalf("FetchParent() error = %v", err)
		}
		if parent == nil || parent.ID != "PROJ-100" {
			t.Fatalf("FetchParent() = %+v, want PROJ-100", parent)
		}
	})

	t.Run("no parent metadata returns nil", func(t *testing.T) {
		parent, err := p.FetchParent(context.Background(), &Task{ID: "PROJ-101"})
		if err != nil {
			t.Fatalf("FetchParent() error = %v", err)
		}
		if parent != nil {
			t.Errorf("FetchParent() = %+v, want nil for task with no parent", parent)
		}
	})
}

func TestJiraProvider_FetchSiblings(t *testing.T) {
	p, cleanup := jiraTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jiraIssue{
			ID:  "100",
			Key: "PROJ-100",
			Fields: jiraIssueFields{
				Summary: "Parent",
				Subtasks: []jiraIssue{
					{Key: "PROJ-101", Fields: jiraIssueFields{Summary: "Self"}},
					{Key: "PROJ-102", Fields: jiraIssueFields{Summary: "Sibling A", Status: &jiraStatus{Name: "Done"}}},
					{Key: "PROJ-103", Fields: jiraIssueFields{Summary: "Sibling B"}},
				},
			},
		})
	})
	defer cleanup()

	task := &Task{ID: "PROJ-101"}
	task.SetMetadata("jira_parent_key", "PROJ-100")

	siblings, err := p.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	// Self (PROJ-101) is excluded, leaving two siblings.
	if len(siblings) != 2 {
		t.Fatalf("FetchSiblings() returned %d, want 2", len(siblings))
	}
	for _, s := range siblings {
		if s.ID == "PROJ-101" {
			t.Error("FetchSiblings() should not include the task itself")
		}
	}
}

func TestJiraProvider_FetchSiblings_NoParent(t *testing.T) {
	p := NewJiraProvider("https://x", "e", "tok")
	siblings, err := p.FetchSiblings(context.Background(), &Task{ID: "PROJ-1"})
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	if siblings != nil {
		t.Errorf("FetchSiblings() = %v, want nil with no parent", siblings)
	}
}

func TestJiraProvider_CreatePR_Unsupported(t *testing.T) {
	p := NewJiraProvider("https://x", "e", "tok")
	if _, err := p.CreatePR(context.Background(), PROptions{}); err == nil {
		t.Error("CreatePR() should be unsupported for Jira")
	}
}

func TestJiraClient_GetIssueTransitions_Error(t *testing.T) {
	p, cleanup := jiraTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer cleanup()

	if _, err := p.client.GetIssueTransitions(context.Background(), "PROJ-1"); err == nil {
		t.Error("GetIssueTransitions() should error on non-200")
	}
}

func TestJiraClient_TransitionIssue_Error(t *testing.T) {
	p, cleanup := jiraTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	defer cleanup()

	if err := p.client.TransitionIssue(context.Background(), "PROJ-1", "31"); err == nil {
		t.Error("TransitionIssue() should error on non-204/200")
	}
}

func TestJiraPriorityToString(t *testing.T) {
	cases := map[string]string{
		"Highest": priorityCritical,
		"Blocker": priorityCritical,
		"High":    priorityHigh,
		"Medium":  priorityNormal,
		"Low":     priorityLow,
		"Lowest":  priorityLow,
		"Unknown": priorityNormal,
	}
	for in, want := range cases {
		if got := jiraPriorityToString(in); got != want {
			t.Errorf("jiraPriorityToString(%q) = %q, want %q", in, got, want)
		}
	}
}
