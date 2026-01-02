package github

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ──────────────────────────────────────────────────────────────────────────────
// FetchSubtests tests
// ──────────────────────────────────────────────────────────────────────────────

func TestFetchSubtasks(t *testing.T) {
	t.Run("returns empty when issue has no task list", func(t *testing.T) {
		issueBody := `This issue has no task list.`
		issueResponse := `{
			"id": 1,
			"number": 123,
			"title": "Issue without tasks",
			"body": "` + issueBody + `",
			"state": "open",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"user": {"login": "author"}
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueResponse))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
		}

		subtasks, err := p.FetchSubtasks(context.Background(), "owner/repo#123")
		if err != nil {
			t.Fatalf("FetchSubtasks() error = %v", err)
		}
		if subtasks != nil {
			t.Errorf("FetchSubtasks() = %v, want nil", subtasks)
		}
	})

	t.Run("parses task list from issue body", func(t *testing.T) {
		issueBody := `# Task

- [ ] First task
- [x] Completed task
- [ ] Third task
`
		issueResponse := `{
			"id": 1,
			"number": 456,
			"title": "Issue with tasks",
			"body": "` + strings.ReplaceAll(issueBody, "\n", "\\n") + `",
			"state": "open",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"user": {"login": "author"}
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueResponse))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
		}

		subtasks, err := p.FetchSubtasks(context.Background(), "owner/repo#456")
		if err != nil {
			t.Fatalf("FetchSubtasks() error = %v", err)
		}

		if len(subtasks) != 3 {
			t.Fatalf("FetchSubtasks() returned %d subtasks, want 3", len(subtasks))
		}

		// Check first task (open)
		if subtasks[0].Title != "First task" {
			t.Errorf("subtasks[0].Title = %q, want %q", subtasks[0].Title, "First task")
		}
		if subtasks[0].Status != provider.StatusOpen {
			t.Errorf("subtasks[0].Status = %q, want %q", subtasks[0].Status, provider.StatusOpen)
		}

		// Check second task (completed)
		if subtasks[1].Title != "Completed task" {
			t.Errorf("subtasks[1].Title = %q, want %q", subtasks[1].Title, "Completed task")
		}
		if subtasks[1].Status != provider.StatusDone {
			t.Errorf("subtasks[1].Status = %q, want %q", subtasks[1].Status, provider.StatusDone)
		}

		// Check metadata
		if subtasks[0].Metadata["parent_id"] != "owner/repo#456" {
			t.Errorf("subtasks[0].Metadata[parent_id] = %q, want %q", subtasks[0].Metadata["parent_id"], "owner/repo#456")
		}
		if subtasks[0].Metadata["is_subtask"] != true {
			t.Errorf("subtasks[0].Metadata[is_subtask] = %v, want true", subtasks[0].Metadata["is_subtask"])
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		ctx := context.Background()
		p := &Provider{
			client: NewClient(ctx, "", "", ""),
			owner:  "",
			repo:   "",
		}

		_, err := p.FetchSubtasks(ctx, "123")
		if err == nil {
			t.Error("FetchSubtasks() expected error for unconfigured repo, got nil")
		}
	})

	t.Run("error with invalid reference format", func(t *testing.T) {
		ctx := context.Background()
		p := &Provider{
			client: NewClient(ctx, "", "owner", "repo"),
			owner:  "owner",
			repo:   "repo",
		}

		_, err := p.FetchSubtasks(ctx, "invalid-format")
		if err == nil {
			t.Error("FetchSubtasks() expected error for invalid format, got nil")
		}
	})

	t.Run("uses provider owner/repo when not specified", func(t *testing.T) {
		issueResponse := `{
			"id": 1,
			"number": 789,
			"title": "Issue",
			"body": "- [ ] Task",
			"state": "open",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"user": {"login": "author"}
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the URL contains our owner/repo
			if !strings.Contains(r.URL.Path, "repos/testowner/testrepo") {
				t.Errorf("URL does not contain expected owner/repo: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueResponse))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "testowner",
			repo:   "testrepo",
		}

		subtasks, err := p.FetchSubtasks(context.Background(), "789")
		if err != nil {
			t.Fatalf("FetchSubtasks() error = %v", err)
		}

		if len(subtasks) != 1 {
			t.Fatalf("FetchSubtasks() returned %d subtasks, want 1", len(subtasks))
		}
	})
}
