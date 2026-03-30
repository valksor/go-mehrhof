package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// linearTestServer starts an httptest server and swaps the package-level
// httpClient transport so all Linear GraphQL requests are redirected there.
// Returns a cleanup func that must be deferred.
func linearTestServer(handler http.HandlerFunc) func() {
	srv := httptest.NewServer(handler)
	origTransport := httpClient.Transport
	httpClient.Transport = &rewriteTransport{
		base:      http.DefaultTransport,
		targetURL: srv.URL,
	}

	return func() {
		httpClient.Transport = origTransport
		srv.Close()
	}
}

// linearGraphQLHandler returns an http.HandlerFunc that serves a fixed JSON body
// for every POST request.
func linearGraphQLHandler(responseBody any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseBody)
	}
}

// ============================================================
// FetchTask via httptest
// ============================================================

func TestLinearProvider_FetchTask_HTTPTest(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"issues": map[string]any{
				"nodes": []map[string]any{
					{
						"id":          "issue-uuid-1",
						"identifier":  "ENG-42",
						"title":       "Fix the bug",
						"description": "Detailed description",
						"url":         "https://linear.app/team/issue/ENG-42",
						"priority":    2,
						"state": map[string]any{
							"id":   "state-1",
							"name": "In Progress",
							"type": "started",
						},
						"team": map[string]any{
							"id":  "team-id-1",
							"key": "ENG",
						},
					},
				},
			},
		},
	}

	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	task, err := lp.FetchTask(context.Background(), "ENG-42")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.ID != "ENG-42" {
		t.Errorf("task.ID = %q, want ENG-42", task.ID)
	}
	if task.Title != "Fix the bug" {
		t.Errorf("task.Title = %q, want 'Fix the bug'", task.Title)
	}
	if task.Description != "Detailed description" {
		t.Errorf("task.Description = %q, want 'Detailed description'", task.Description)
	}
	if task.Priority != "high" {
		t.Errorf("task.Priority = %q, want high (priority=2)", task.Priority)
	}
	if task.Source != NameLinear {
		t.Errorf("task.Source = %q, want linear", task.Source)
	}
	if task.Metadata("linear_id") != "issue-uuid-1" {
		t.Errorf("linear_id = %q, want issue-uuid-1", task.Metadata("linear_id"))
	}
	if task.Metadata("linear_team_key") != "ENG" {
		t.Errorf("linear_team_key = %q, want ENG", task.Metadata("linear_team_key"))
	}
}

func TestLinearProvider_FetchTask_NotFound(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"issues": map[string]any{
				"nodes": []any{},
			},
		},
	}

	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	_, err := lp.FetchTask(context.Background(), "ENG-999")
	if err == nil {
		t.Error("FetchTask() should return error when issue not found")
	}
}

func TestLinearProvider_FetchTask_GraphQLError(t *testing.T) {
	resp := map[string]any{
		"errors": []map[string]any{
			{"message": "Unauthorized"},
		},
	}

	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	_, err := lp.FetchTask(context.Background(), "ENG-1")
	if err == nil {
		t.Error("FetchTask() should return error on GraphQL error response")
	}
}

// ============================================================
// UpdateStatus via httptest
// ============================================================

func TestLinearProvider_UpdateStatus_HTTPTest(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		// Decode request to find which query is being made
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch callCount {
		case 1:
			// fetchIssueByIdentifier
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issues": map[string]any{
						"nodes": []map[string]any{
							{
								"id":         "issue-uuid-2",
								"identifier": "ENG-10",
								"title":      "Some issue",
								"team":       map[string]any{"id": "team-id-2", "key": "ENG"},
								"state":      map[string]any{"id": "s1", "name": "Todo", "type": "unstarted"},
							},
						},
					},
				},
			})
		case 2:
			// findWorkflowState — returns matching state
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"team": map[string]any{
						"states": map[string]any{
							"nodes": []map[string]any{
								{"id": "done-state-id", "name": "Done", "type": "completed"},
							},
						},
					},
				},
			})
		case 3:
			// issueUpdate mutation
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issueUpdate": map[string]any{
						"success": true,
					},
				},
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	cleanup := linearTestServer(handler)
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	err := lp.UpdateStatus(context.Background(), "ENG-10", "done")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
}

func TestLinearProvider_UpdateStatus_NoToken_HTTP(t *testing.T) {
	lp := NewLinearProvider("", "")
	err := lp.UpdateStatus(context.Background(), "ENG-1", "done")
	if err == nil {
		t.Error("UpdateStatus() should return error when token is empty")
	}
}

// ============================================================
// FetchParent via httptest
// ============================================================

func TestLinearProvider_FetchParent_HTTPTest(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"issue": map[string]any{
				"id":          "parent-uuid",
				"identifier":  "ENG-1",
				"title":       "Parent epic",
				"description": "",
				"url":         "https://linear.app/team/issue/ENG-1",
				"priority":    1,
				"state":       map[string]any{"id": "s2", "name": "Backlog", "type": "backlog"},
				"team":        map[string]any{"id": "team-id", "key": "ENG"},
			},
		},
	}

	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	task := &Task{ID: "ENG-5"}
	task.SetMetadata("linear_parent_id", "parent-uuid")

	parent, err := lp.FetchParent(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchParent() error = %v", err)
	}
	if parent == nil {
		t.Fatal("FetchParent() returned nil, want a task")
	}
	if parent.ID != "ENG-1" {
		t.Errorf("parent.ID = %q, want ENG-1", parent.ID)
	}
	if parent.Title != "Parent epic" {
		t.Errorf("parent.Title = %q, want 'Parent epic'", parent.Title)
	}
}

func TestLinearProvider_FetchParent_NoParentID(t *testing.T) {
	lp := NewLinearProvider("test-token", "")

	// Task with no linear_parent_id should return nil, nil
	task := &Task{ID: "ENG-5"}

	parent, err := lp.FetchParent(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchParent() error = %v", err)
	}
	if parent != nil {
		t.Errorf("FetchParent() = %v, want nil when no parent_id", parent)
	}
}

// ============================================================
// FetchSiblings via httptest
// ============================================================

func TestLinearProvider_FetchSiblings_HTTPTest(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"issue": map[string]any{
				"children": map[string]any{
					"nodes": []map[string]any{
						{
							"id":         "sibling-uuid-1",
							"identifier": "ENG-6",
							"title":      "Sibling task 1",
							"state":      map[string]any{"id": "s3", "name": "Todo", "type": "unstarted"},
						},
						{
							"id":         "sibling-uuid-2",
							"identifier": "ENG-7",
							"title":      "Sibling task 2",
							"state":      map[string]any{"id": "s4", "name": "Done", "type": "completed"},
						},
					},
				},
			},
		},
	}

	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	task := &Task{ID: "ENG-5"}
	task.SetMetadata("linear_parent_id", "parent-uuid")
	task.SetMetadata("linear_id", "self-uuid")

	siblings, err := lp.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}

	// Both siblings returned (neither is self)
	if len(siblings) != 2 {
		t.Errorf("FetchSiblings() len = %d, want 2", len(siblings))
	}
	if len(siblings) > 0 && siblings[0].ID != "ENG-6" {
		t.Errorf("siblings[0].ID = %q, want ENG-6", siblings[0].ID)
	}
}

func TestLinearProvider_FetchSiblings_SelfExcluded(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"issue": map[string]any{
				"children": map[string]any{
					"nodes": []map[string]any{
						{
							"id":         "self-uuid",
							"identifier": "ENG-5",
							"title":      "The task itself",
							"state":      map[string]any{"id": "s5", "name": "Todo", "type": "unstarted"},
						},
						{
							"id":         "sibling-uuid-3",
							"identifier": "ENG-8",
							"title":      "Another sibling",
							"state":      map[string]any{"id": "s6", "name": "Todo", "type": "unstarted"},
						},
					},
				},
			},
		},
	}

	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	task := &Task{ID: "ENG-5"}
	task.SetMetadata("linear_parent_id", "parent-uuid")
	task.SetMetadata("linear_id", "self-uuid") // matches first node — should be skipped

	siblings, err := lp.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}

	// Only one sibling (self filtered out)
	if len(siblings) != 1 {
		t.Errorf("FetchSiblings() len = %d, want 1 (self should be excluded)", len(siblings))
	}
	if len(siblings) == 1 && siblings[0].ID != "ENG-8" {
		t.Errorf("siblings[0].ID = %q, want ENG-8", siblings[0].ID)
	}
}

func TestLinearProvider_FetchSiblings_NoParentID(t *testing.T) {
	lp := NewLinearProvider("test-token", "")
	task := &Task{ID: "ENG-5"}
	// No linear_parent_id

	siblings, err := lp.FetchSiblings(context.Background(), task)
	if err != nil {
		t.Fatalf("FetchSiblings() error = %v", err)
	}
	if siblings != nil {
		t.Errorf("FetchSiblings() = %v, want nil when no parent", siblings)
	}
}

// ============================================================
// AddComment via httptest
// ============================================================

func TestLinearProvider_AddComment_HTTPTest(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch callCount {
		case 1:
			// fetchIssueByIdentifier
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issues": map[string]any{
						"nodes": []map[string]any{
							{"id": "issue-uuid-3", "identifier": "ENG-20", "title": "Issue 20"},
						},
					},
				},
			})
		case 2:
			// commentCreate mutation
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"commentCreate": map[string]any{
						"success": true,
					},
				},
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	cleanup := linearTestServer(handler)
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	err := lp.AddComment(context.Background(), "ENG-20", "Great work!")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
}

func TestLinearProvider_AddComment_NoToken_HTTP(t *testing.T) {
	lp := NewLinearProvider("", "")
	err := lp.AddComment(context.Background(), "ENG-1", "comment")
	if err == nil {
		t.Error("AddComment() should return error when token is empty")
	}
}

// ============================================================
// FetchComments via httptest
// ============================================================

func TestLinearProvider_FetchComments_HTTPTest(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch callCount {
		case 1:
			// fetchIssueByIdentifier
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issues": map[string]any{
						"nodes": []map[string]any{
							{"id": "issue-uuid-4", "identifier": "ENG-30", "title": "Issue 30"},
						},
					},
				},
			})
		case 2:
			// IssueComments query
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issue": map[string]any{
						"comments": map[string]any{
							"nodes": []map[string]any{
								{
									"id":        "comment-1",
									"body":      "First comment",
									"user":      map[string]any{"id": "u1", "name": "Alice"},
									"createdAt": "2026-01-01T10:00:00Z",
								},
								{
									"id":        "comment-2",
									"body":      "Second comment",
									"user":      map[string]any{"id": "u2", "name": "Bob"},
									"createdAt": "2026-01-02T10:00:00Z",
								},
							},
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	cleanup := linearTestServer(handler)
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	comments, err := lp.FetchComments(context.Background(), "ENG-30")
	if err != nil {
		t.Fatalf("FetchComments() error = %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("FetchComments() len = %d, want 2", len(comments))
	}
	if comments[0].Body != "First comment" {
		t.Errorf("comments[0].Body = %q, want 'First comment'", comments[0].Body)
	}
	if comments[0].Author != "Alice" {
		t.Errorf("comments[0].Author = %q, want Alice", comments[0].Author)
	}
	if comments[1].ID != "comment-2" {
		t.Errorf("comments[1].ID = %q, want comment-2", comments[1].ID)
	}
}

func TestLinearProvider_FetchComments_NoToken_HTTP(t *testing.T) {
	lp := NewLinearProvider("", "")
	_, err := lp.FetchComments(context.Background(), "ENG-1")
	if err == nil {
		t.Error("FetchComments() should return error when token is empty")
	}
}

func TestLinearProvider_FetchComments_NilUser(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch callCount {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issues": map[string]any{
						"nodes": []map[string]any{
							{"id": "issue-uuid-5", "identifier": "ENG-31", "title": "Issue 31"},
						},
					},
				},
			})
		case 2:
			// Comment with null user
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test helper
				"data": map[string]any{
					"issue": map[string]any{
						"comments": map[string]any{
							"nodes": []map[string]any{
								{
									"id":        "comment-3",
									"body":      "Anonymous comment",
									"user":      nil,
									"createdAt": "2026-01-03T10:00:00Z",
								},
							},
						},
					},
				},
			})
		}
	})

	cleanup := linearTestServer(handler)
	defer cleanup()

	lp := NewLinearProvider("test-token", "")
	comments, err := lp.FetchComments(context.Background(), "ENG-31")
	if err != nil {
		t.Fatalf("FetchComments() error = %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("FetchComments() len = %d, want 1", len(comments))
	}
	if comments[0].Author != "" {
		t.Errorf("comments[0].Author = %q, want empty string for nil user", comments[0].Author)
	}
}
