package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// linearRouter routes Linear GraphQL POSTs to a response based on a substring
// match against the query text. The first matching key (in iteration order is
// non-deterministic, so keys must be mutually exclusive) wins.
func linearRouter(routes map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		for needle, resp := range routes {
			if strings.Contains(req.Query, needle) {
				_ = json.NewEncoder(w).Encode(resp)

				return
			}
		}
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func TestLinearProvider_ListTasks_HTTPTest(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"issues": map[string]any{
				"nodes": []map[string]any{
					{
						"id": "i1", "identifier": "ENG-1", "title": "First",
						"state": map[string]any{"type": "started", "name": "In Progress"},
						"team":  map[string]any{"id": "t1", "key": "ENG"},
					},
					{
						"id": "i2", "identifier": "ENG-2", "title": "Second",
						"state": map[string]any{"type": "completed", "name": "Done"},
						"team":  map[string]any{"id": "t1", "key": "ENG"},
					},
				},
				"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "cursor-xyz"},
			},
		},
	}
	cleanup := linearTestServer(linearGraphQLHandler(resp))
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	res, err := lp.ListTasks(context.Background(), ListOptions{Team: "ENG", Status: "started", Limit: 50, Cursor: "prev"})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(res.Tasks) != 2 {
		t.Fatalf("ListTasks() returned %d tasks, want 2", len(res.Tasks))
	}
	if !res.HasMore {
		t.Error("HasMore should be true")
	}
	if res.NextCursor != "cursor-xyz" {
		t.Errorf("NextCursor = %q, want cursor-xyz", res.NextCursor)
	}
}

func TestLinearProvider_CreateTask_HTTPTest(t *testing.T) {
	routes := map[string]any{
		"query Team(": map[string]any{
			"data": map[string]any{"team": map[string]any{"id": "team-uuid"}},
		},
		"TeamLabels": map[string]any{
			"data": map[string]any{"team": map[string]any{"labels": map[string]any{
				"nodes": []map[string]any{{"id": "label-1", "name": "bug"}},
			}}},
		},
		"IssueCreate": map[string]any{
			"data": map[string]any{"issueCreate": map[string]any{
				"success": true,
				"issue": map[string]any{
					"id": "new-1", "identifier": "ENG-99", "title": "Created issue",
					"state": map[string]any{"type": "unstarted", "name": "Todo"},
					"team":  map[string]any{"id": "team-uuid", "key": "ENG"},
				},
			}},
		},
	}
	cleanup := linearTestServer(linearRouter(routes))
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	task, err := lp.CreateTask(context.Background(), CreateTaskOptions{
		Title:       "Created issue",
		Description: "desc",
		Priority:    "high",
		Labels:      []string{"bug"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.ID != "ENG-99" {
		t.Errorf("task.ID = %q, want ENG-99", task.ID)
	}
	if task.Title != "Created issue" {
		t.Errorf("task.Title = %q, want 'Created issue'", task.Title)
	}
}

func TestLinearProvider_CreateTask_Validation(t *testing.T) {
	// No token.
	if _, err := NewLinearProvider("", "").CreateTask(context.Background(), CreateTaskOptions{}); err == nil {
		t.Error("CreateTask() without token should error")
	}
	// Token but no team available.
	if _, err := NewLinearProvider("tok", "").CreateTask(context.Background(), CreateTaskOptions{Title: "x"}); err == nil {
		t.Error("CreateTask() without a team should error")
	}
}

func TestLinearProvider_AddLabels_HTTPTest(t *testing.T) {
	routes := map[string]any{
		"IssueByIdentifier": map[string]any{
			"data": map[string]any{"issues": map[string]any{"nodes": []map[string]any{
				{
					"id": "issue-1", "identifier": "ENG-5", "title": "Has labels",
					"team":   map[string]any{"id": "team-1", "key": "ENG"},
					"labels": map[string]any{"nodes": []map[string]any{{"id": "existing-1", "name": "old"}}},
				},
			}}},
		},
		"TeamLabels": map[string]any{
			"data": map[string]any{"team": map[string]any{"labels": map[string]any{
				"nodes": []map[string]any{{"id": "new-label", "name": "feature"}},
			}}},
		},
		"mutation IssueUpdate": map[string]any{
			"data": map[string]any{"issueUpdate": map[string]any{"success": true}},
		},
	}
	cleanup := linearTestServer(linearRouter(routes))
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	if err := lp.AddLabels(context.Background(), "ENG-5", []string{"feature"}); err != nil {
		t.Fatalf("AddLabels() error = %v", err)
	}
}

func TestLinearProvider_RemoveLabels_HTTPTest(t *testing.T) {
	routes := map[string]any{
		"IssueByIdentifier": map[string]any{
			"data": map[string]any{"issues": map[string]any{"nodes": []map[string]any{
				{
					"id": "issue-1", "identifier": "ENG-6", "title": "Remove labels",
					"team": map[string]any{"id": "team-1", "key": "ENG"},
					"labels": map[string]any{"nodes": []map[string]any{
						{"id": "keep-1", "name": "keep"},
						{"id": "drop-1", "name": "drop"},
					}},
				},
			}}},
		},
		"TeamLabels": map[string]any{
			"data": map[string]any{"team": map[string]any{"labels": map[string]any{
				"nodes": []map[string]any{{"id": "drop-1", "name": "drop"}},
			}}},
		},
		"mutation IssueUpdate": map[string]any{
			"data": map[string]any{"issueUpdate": map[string]any{"success": true}},
		},
	}
	cleanup := linearTestServer(linearRouter(routes))
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	if err := lp.RemoveLabels(context.Background(), "ENG-6", []string{"drop"}); err != nil {
		t.Fatalf("RemoveLabels() error = %v", err)
	}
}

func TestLinearProvider_AddRemoveLabels_NoToken(t *testing.T) {
	lp := NewLinearProvider("", "")
	if err := lp.AddLabels(context.Background(), "ENG-1", []string{"x"}); err == nil {
		t.Error("AddLabels() without token should error")
	}
	if err := lp.RemoveLabels(context.Background(), "ENG-1", []string{"x"}); err == nil {
		t.Error("RemoveLabels() without token should error")
	}
}

func TestLinearProvider_UpdateStatus_ByName(t *testing.T) {
	// "ready for review" is not a known status keyword, so UpdateStatus falls
	// through to findWorkflowStateByName.
	routes := map[string]any{
		"IssueByIdentifier": map[string]any{
			"data": map[string]any{"issues": map[string]any{"nodes": []map[string]any{
				{
					"id": "issue-1", "identifier": "ENG-7", "title": "By name",
					"team": map[string]any{"id": "team-1", "key": "ENG"},
				},
			}}},
		},
		"WorkflowStates": map[string]any{
			"data": map[string]any{"team": map[string]any{"states": map[string]any{
				"nodes": []map[string]any{
					{"id": "review-state", "name": "Ready For Review", "type": "started"},
				},
			}}},
		},
		"mutation IssueUpdate": map[string]any{
			"data": map[string]any{"issueUpdate": map[string]any{"success": true}},
		},
	}
	cleanup := linearTestServer(linearRouter(routes))
	defer cleanup()

	lp := NewLinearProvider("test-token", "ENG")
	if err := lp.UpdateStatus(context.Background(), "ENG-7", "ready for review"); err != nil {
		t.Fatalf("UpdateStatus() by name error = %v", err)
	}
}

func TestLinearProvider_DownloadAttachment(t *testing.T) {
	t.Run("rejects disallowed host", func(t *testing.T) {
		lp := NewLinearProvider("tok", "")
		if _, err := lp.DownloadAttachment(context.Background(), "https://evil.example.com/file.png"); err == nil {
			t.Error("DownloadAttachment() should reject a disallowed host")
		}
	})

	t.Run("no token", func(t *testing.T) {
		lp := NewLinearProvider("", "")
		if _, err := lp.DownloadAttachment(context.Background(), "https://uploads.linear.app/x"); err == nil {
			t.Error("DownloadAttachment() should error without token")
		}
	})

	t.Run("downloads allowed host", func(t *testing.T) {
		cleanup := linearTestServer(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("file-bytes"))
		})
		defer cleanup()

		lp := NewLinearProvider("test-token", "")
		data, err := lp.DownloadAttachment(context.Background(), "https://uploads.linear.app/path/to/file")
		if err != nil {
			t.Fatalf("DownloadAttachment() error = %v", err)
		}
		if string(data) != "file-bytes" {
			t.Errorf("data = %q, want file-bytes", string(data))
		}
	})
}
