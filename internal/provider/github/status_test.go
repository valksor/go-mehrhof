package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v67/github"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ──────────────────────────────────────────────────────────────────────────────
// UpdateStatus tests
// ──────────────────────────────────────────────────────────────────────────────

func TestUpdateStatus(t *testing.T) {
	t.Run("closes an issue", func(t *testing.T) {
		updatedIssue := `{
			"id": 1,
			"number": 123,
			"title": "Test Issue",
			"state": "closed",
			"body": "Description",
			"html_url": "https://github.com/owner/repo/issues/123",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"labels": [],
			"assignees": []
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(updatedIssue))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockStatusClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		err := p.UpdateStatus(context.Background(), "owner/repo#123", provider.StatusClosed)
		if err != nil {
			t.Fatalf("UpdateStatus error = %v", err)
		}
	})

	t.Run("opens an issue", func(t *testing.T) {
		updatedIssue := `{
			"id": 1,
			"number": 123,
			"title": "Test Issue",
			"state": "open",
			"body": "Description",
			"html_url": "https://github.com/owner/repo/issues/123",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"labels": [],
			"assignees": []
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(updatedIssue))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockStatusClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		err := p.UpdateStatus(context.Background(), "123", provider.StatusOpen)
		if err != nil {
			t.Fatalf("UpdateStatus error = %v", err)
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		p := &Provider{
			client: &Client{},
			owner:  "",
			repo:   "",
			config: &Config{},
		}

		err := p.UpdateStatus(context.Background(), "123", provider.StatusClosed)
		if err != ErrRepoNotConfigured {
			t.Errorf("error = %v, want %v", err, ErrRepoNotConfigured)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// mapStatusToGitHubState tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapStatusToGitHubState(t *testing.T) {
	tests := []struct {
		name   string
		status provider.Status
		want   string
	}{
		{"open maps to open", provider.StatusOpen, "open"},
		{"in_progress maps to open", provider.StatusInProgress, "open"},
		{"review maps to open", provider.StatusReview, "open"},
		{"closed maps to closed", provider.StatusClosed, "closed"},
		{"done maps to closed", provider.StatusDone, "closed"},
		{"unknown maps to open", provider.Status("unknown"), "open"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStatusToGitHubState(tt.status)
			if got != tt.want {
				t.Errorf("mapStatusToGitHubState(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func setupMockStatusClient(t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)

	client := gh.NewClient(nil)
	serverURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = serverURL

	return &Client{gh: client, owner: "owner", repo: "repo"}, func() {
		server.Close()
	}
}
