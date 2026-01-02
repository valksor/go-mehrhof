package github

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ──────────────────────────────────────────────────────────────────────────────
// List tests
// ──────────────────────────────────────────────────────────────────────────────

func TestList(t *testing.T) {
	t.Run("success with open issues", func(t *testing.T) {
		issues := `[
			{
				"id": 1,
				"number": 123,
				"title": "First issue",
				"state": "open",
				"body": "Issue description",
				"html_url": "https://github.com/owner/repo/issues/123",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
				"labels": [{"name": "bug"}],
				"assignees": [],
				"pull_request": null
			},
			{
				"id": 2,
				"number": 456,
				"title": "Second issue",
				"state": "open",
				"body": "Another issue",
				"html_url": "https://github.com/owner/repo/issues/456",
				"created_at": "2024-01-02T00:00:00Z",
				"updated_at": "2024-01-02T00:00:00Z",
				"labels": [{"name": "feature"}],
				"assignees": [],
				"pull_request": null
			}
		]`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "issues") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(issues))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		result, err := p.List(context.Background(), provider.ListOptions{})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}

		if len(result) != 2 {
			t.Errorf("len(result) = %d, want 2", len(result))
		}

		// Check first issue
		if result[0].Title != "First issue" {
			t.Errorf("result[0].Title = %q, want %q", result[0].Title, "First issue")
		}
		if result[0].ID != "123" {
			t.Errorf("result[0].ID = %q, want %q", result[0].ID, "123")
		}
		if result[0].Status != provider.StatusOpen {
			t.Errorf("result[0].Status = %q, want %q", result[0].Status, provider.StatusOpen)
		}
	})

	t.Run("filters out pull requests", func(t *testing.T) {
		issues := `[
			{
				"id": 1,
				"number": 100,
				"title": "Issue",
				"state": "open",
				"body": "Just an issue",
				"html_url": "https://github.com/owner/repo/issues/100",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
				"labels": [],
				"assignees": []
			},
			{
				"id": 2,
				"number": 200,
				"title": "PR",
				"state": "open",
				"body": "A pull request",
				"html_url": "https://github.com/owner/repo/pull/200",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
				"labels": [],
				"assignees": [],
				"pull_request": {"url": "http://example.com"}
			}
		]`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issues))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		result, err := p.List(context.Background(), provider.ListOptions{})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}

		if len(result) != 1 {
			t.Errorf("len(result) = %d, want 1 (PR should be filtered)", len(result))
		}
		if result[0].Title != "Issue" {
			t.Errorf("result[0].Title = %q, want %q", result[0].Title, "Issue")
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		p := &Provider{
			client: &Client{},
			owner:  "",
			repo:   "",
			config: &Config{},
		}

		_, err := p.List(context.Background(), provider.ListOptions{})
		if !errors.Is(err, ErrRepoNotConfigured) {
			t.Errorf("error = %v, want %v", err, ErrRepoNotConfigured)
		}
	})

	t.Run("respects limit option", func(t *testing.T) {
		// Create 5 issues
		issues := `[
			{"id": 1, "number": 1, "title": "Issue 1", "state": "open", "body": "desc", "html_url": "https://github.com/o/r/issues/1", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "labels": [], "assignees": [], "pull_request": null},
			{"id": 2, "number": 2, "title": "Issue 2", "state": "open", "body": "desc", "html_url": "https://github.com/o/r/issues/2", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "labels": [], "assignees": [], "pull_request": null},
			{"id": 3, "number": 3, "title": "Issue 3", "state": "open", "body": "desc", "html_url": "https://github.com/o/r/issues/3", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "labels": [], "assignees": [], "pull_request": null},
			{"id": 4, "number": 4, "title": "Issue 4", "state": "open", "body": "desc", "html_url": "https://github.com/o/r/issues/4", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "labels": [], "assignees": [], "pull_request": null},
			{"id": 5, "number": 5, "title": "Issue 5", "state": "open", "body": "desc", "html_url": "https://github.com/o/r/issues/5", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "labels": [], "assignees": [], "pull_request": null}
		]`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issues))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		result, err := p.List(context.Background(), provider.ListOptions{Limit: 3})
		if err != nil {
			t.Fatalf("List error = %v", err)
		}

		if len(result) != 3 {
			t.Errorf("len(result) = %d, want 3", len(result))
		}
	})
}
