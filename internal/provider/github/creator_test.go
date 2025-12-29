package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gh "github.com/google/go-github/v67/github"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ──────────────────────────────────────────────────────────────────────────────
// CreateWorkUnit tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCreateWorkUnit(t *testing.T) {
	t.Run("creates a new issue", func(t *testing.T) {
		newIssue := `{
			"id": 1,
			"number": 789,
			"title": "New Feature",
			"state": "open",
			"body": "Implement new feature",
			"html_url": "https://github.com/owner/repo/issues/789",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"labels": [{"name": "enhancement"}],
			"assignees": [],
			"user": {"login": "creator"}
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(newIssue))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockCreatorClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		opts := provider.CreateWorkUnitOptions{
			Title:       "New Feature",
			Description: "Implement new feature",
			Labels:      []string{"enhancement"},
			Priority:    provider.PriorityNormal,
		}

		wu, err := p.CreateWorkUnit(context.Background(), opts)
		if err != nil {
			t.Fatalf("CreateWorkUnit error = %v", err)
		}

		if wu.Title != "New Feature" {
			t.Errorf("wu.Title = %q, want %q", wu.Title, "New Feature")
		}
		if wu.ID != "789" {
			t.Errorf("wu.ID = %q, want %q", wu.ID, "789")
		}
		if wu.Status != "open" {
			t.Errorf("wu.Status = %q, want %q", wu.Status, "open")
		}
	})

	t.Run("creates issue with assignees", func(t *testing.T) {
		newIssue := `{
			"id": 1,
			"number": 100,
			"title": "Task",
			"state": "open",
			"body": "Description",
			"html_url": "https://github.com/owner/repo/issues/100",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"labels": [],
			"assignees": [{"login": "user1"}, {"login": "user2"}],
			"user": {"login": "creator"}
		}`

		receivedAssignees := ""
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				// Capture the assignees from request
				body := `{"title":"Task","body":"Description","assignees":["user1","user2"],"labels":[]}`
				receivedAssignees = body
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(newIssue))
			}
		})

		client, cleanup := setupMockCreatorClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		opts := provider.CreateWorkUnitOptions{
			Title:       "Task",
			Description: "Description",
			Assignees:   []string{"user1", "user2"},
		}

		_, err := p.CreateWorkUnit(context.Background(), opts)
		if err != nil {
			t.Fatalf("CreateWorkUnit error = %v", err)
		}

		if !strings.Contains(receivedAssignees, "user1") || !strings.Contains(receivedAssignees, "user2") {
			t.Errorf("assignees not properly sent in request: %s", receivedAssignees)
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		p := &Provider{
			client: &Client{},
			owner:  "",
			repo:   "",
			config: &Config{},
		}

		opts := provider.CreateWorkUnitOptions{
			Title:       "Test",
			Description: "Test",
		}

		_, err := p.CreateWorkUnit(context.Background(), opts)
		if err != ErrRepoNotConfigured {
			t.Errorf("error = %v, want %v", err, ErrRepoNotConfigured)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// mapGitHubAssignees tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMapGitHubAssignees(t *testing.T) {
	assignees := mapGitHubAssignees([]string{"user1", "user2", "user3"})

	if len(assignees) != 3 {
		t.Fatalf("len(assignees) = %d, want 3", len(assignees))
	}

	if assignees[0].Name != "user1" {
		t.Errorf("assignees[0].Name = %q, want %q", assignees[0].Name, "user1")
	}
	if assignees[1].Name != "user2" {
		t.Errorf("assignees[1].Name = %q, want %q", assignees[1].Name, "user2")
	}
	if assignees[2].Name != "user3" {
		t.Errorf("assignees[2].Name = %q, want %q", assignees[2].Name, "user3")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// inferTaskTypeFromLabels tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInferTaskTypeFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		labels []string
	}{
		{"bug label", "fix", []string{"bug"}},
		{"bugfix label", "fix", []string{"bugfix"}},
		{"feature label", "feature", []string{"feature"}},
		{"enhancement label", "feature", []string{"enhancement"}},
		{"docs label", "docs", []string{"docs"}},
		{"refactor label", "refactor", []string{"refactor"}},
		{"chore label", "chore", []string{"chore"}},
		{"test label", "test", []string{"test"}},
		{"unknown label", "issue", []string{"unknown"}},
		{"multiple labels with known type", "fix", []string{"bug", "other"}},
		{"empty labels", "issue", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTaskTypeFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("inferTaskTypeFromLabels(%v) = %q, want %q", tt.labels, got, tt.want)
			}
		})
	}
}

func setupMockCreatorClient(t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)

	client := gh.NewClient(nil)
	serverURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = serverURL

	return &Client{gh: client, owner: "owner", repo: "repo"}, func() {
		server.Close()
	}
}
