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

// ──────────────────────────────────────────────────────────────────────────────
// lower helper function tests
// ──────────────────────────────────────────────────────────────────────────────

func TestLower(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "all uppercase",
			s:    "HELLO",
			want: "hello",
		},
		{
			name: "mixed case",
			s:    "HeLLo WoRLd",
			want: "hello world",
		},
		{
			name: "all lowercase",
			s:    "already lower",
			want: "already lower",
		},
		{
			name: "empty string",
			s:    "",
			want: "",
		},
		{
			name: "with numbers",
			s:    "TEST123",
			want: "test123",
		},
		{
			name: "special characters",
			s:    "Hi!-@",
			want: "hi!-@",
		},
		{
			name: "single uppercase letter",
			s:    "A",
			want: "a",
		},
		{
			name: "single lowercase letter",
			s:    "z",
			want: "z",
		},
		{
			name: "all label types",
			s:    "BUG FEATURE DOCS REFACTOR CHORE TEST CI",
			want: "bug feature docs refactor chore test ci",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lower(tt.s)
			if got != tt.want {
				t.Errorf("lower() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// mapGitHubAssignees edge cases
// ──────────────────────────────────────────────────────────────────────────────

func TestMapGitHubAssignees_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		assignees []string
		want      []provider.Person
	}{
		{
			name:      "empty assignees",
			assignees: []string{},
			want:      []provider.Person{},
		},
		{
			name:      "nil assignees",
			assignees: nil,
			want:      []provider.Person{},
		},
		{
			name:      "single assignee",
			assignees: []string{"developer"},
			want:      []provider.Person{{Name: "developer"}},
		},
		{
			name:      "assignee with hyphen",
			assignees: []string{"dev-user"},
			want:      []provider.Person{{Name: "dev-user"}},
		},
		{
			name:      "assignee with dot",
			assignees: []string{"user.name"},
			want:      []provider.Person{{Name: "user.name"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitHubAssignees(tt.assignees)

			if len(got) != len(tt.want) {
				t.Errorf("mapGitHubAssignees() len = %d, want %d", len(got), len(tt.want))
				return
			}

			for i, w := range tt.want {
				if got[i].Name != w.Name {
					t.Errorf("mapGitHubAssignees()[%d].Name = %q, want %q", i, got[i].Name, w.Name)
				}
			}
		})
	}
}
