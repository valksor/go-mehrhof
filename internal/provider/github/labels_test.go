package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gh "github.com/google/go-github/v67/github"
)

// ──────────────────────────────────────────────────────────────────────────────
// AddLabels tests
// ──────────────────────────────────────────────────────────────────────────────

func TestAddLabels(t *testing.T) {
	t.Run("adds labels to issue", func(t *testing.T) {
		labels := `[
			{"name": "bug"},
			{"name": "priority:high"}
		]`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/labels") && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(labels))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockLabelsClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		err := p.AddLabels(context.Background(), "123", []string{"bug", "priority:high"})
		if err != nil {
			t.Fatalf("AddLabels error = %v", err)
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		p := &Provider{
			client: &Client{},
			owner:  "",
			repo:   "",
			config: &Config{},
		}

		err := p.AddLabels(context.Background(), "123", []string{"bug"})
		if err != ErrRepoNotConfigured {
			t.Errorf("error = %v, want %v", err, ErrRepoNotConfigured)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// RemoveLabels tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRemoveLabels(t *testing.T) {
	t.Run("removes labels from issue", func(t *testing.T) {
		calls := 0
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/labels/bug") && r.Method == http.MethodDelete {
				calls++
				w.WriteHeader(http.StatusOK)
			} else if strings.HasSuffix(r.URL.Path, "/labels/priority") && r.Method == http.MethodDelete {
				calls++
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockLabelsClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		err := p.RemoveLabels(context.Background(), "123", []string{"bug", "priority"})
		if err != nil {
			t.Fatalf("RemoveLabels error = %v", err)
		}

		if calls != 2 {
			t.Errorf("expected 2 API calls, got %d", calls)
		}
	})

	t.Run("continues on error removing one label", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/labels/bug") {
				w.WriteHeader(http.StatusNotFound) // Label doesn't exist
			} else if strings.HasSuffix(r.URL.Path, "/labels/feature") {
				w.WriteHeader(http.StatusOK) // This one succeeds
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		client, cleanup := setupMockLabelsClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{},
		}

		// Should not return error even if one label fails to remove
		err := p.RemoveLabels(context.Background(), "123", []string{"bug", "feature"})
		if err != nil {
			t.Fatalf("RemoveLabels error = %v", err)
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		p := &Provider{
			client: &Client{},
			owner:  "",
			repo:   "",
			config: &Config{},
		}

		err := p.RemoveLabels(context.Background(), "123", []string{"bug"})
		if err != ErrRepoNotConfigured {
			t.Errorf("error = %v, want %v", err, ErrRepoNotConfigured)
		}
	})
}

func setupMockLabelsClient(t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)

	client := gh.NewClient(nil)
	serverURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = serverURL

	return &Client{gh: client, owner: "owner", repo: "repo"}, func() {
		server.Close()
	}
}
