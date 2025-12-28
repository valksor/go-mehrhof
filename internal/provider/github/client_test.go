package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	gh "github.com/google/go-github/v67/github"
)

// ──────────────────────────────────────────────────────────────────────────────
// ptr helper tests
// ──────────────────────────────────────────────────────────────────────────────

func TestPtr(t *testing.T) {
	t.Run("string pointer", func(t *testing.T) {
		s := "hello"
		p := ptr(s)
		if p == nil {
			t.Fatal("ptr returned nil")
		}
		if *p != s {
			t.Errorf("*ptr(%q) = %q, want %q", s, *p, s)
		}
	})

	t.Run("int pointer", func(t *testing.T) {
		i := 42
		p := ptr(i)
		if p == nil {
			t.Fatal("ptr returned nil")
		}
		if *p != i {
			t.Errorf("*ptr(%d) = %d, want %d", i, *p, i)
		}
	})

	t.Run("bool pointer", func(t *testing.T) {
		b := true
		p := ptr(b)
		if p == nil {
			t.Fatal("ptr returned nil")
		}
		if *p != b {
			t.Errorf("*ptr(%v) = %v, want %v", b, *p, b)
		}
	})

	t.Run("int64 pointer", func(t *testing.T) {
		i := int64(123456789)
		p := ptr(i)
		if p == nil {
			t.Fatal("ptr returned nil")
		}
		if *p != i {
			t.Errorf("*ptr(%d) = %d, want %d", i, *p, i)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Client accessor tests
// ──────────────────────────────────────────────────────────────────────────────

func TestClientSetOwnerRepo(t *testing.T) {
	c := &Client{owner: "original-owner", repo: "original-repo"}

	c.SetOwnerRepo("new-owner", "new-repo")

	if c.Owner() != "new-owner" {
		t.Errorf("Owner() = %q, want %q", c.Owner(), "new-owner")
	}
	if c.Repo() != "new-repo" {
		t.Errorf("Repo() = %q, want %q", c.Repo(), "new-repo")
	}
}

func TestClientOwnerRepo(t *testing.T) {
	c := &Client{owner: "test-owner", repo: "test-repo"}

	if c.Owner() != "test-owner" {
		t.Errorf("Owner() = %q, want %q", c.Owner(), "test-owner")
	}
	if c.Repo() != "test-repo" {
		t.Errorf("Repo() = %q, want %q", c.Repo(), "test-repo")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ResolveToken tests
// ──────────────────────────────────────────────────────────────────────────────

func TestResolveToken(t *testing.T) {
	// Clear any existing env vars for clean test
	originalMehr := os.Getenv("MEHR_GITHUB_TOKEN")
	originalGithub := os.Getenv("GITHUB_TOKEN")
	defer func() {
		_ = os.Setenv("MEHR_GITHUB_TOKEN", originalMehr)
		_ = os.Setenv("GITHUB_TOKEN", originalGithub)
	}()

	t.Run("MEHR_GITHUB_TOKEN priority", func(t *testing.T) {
		_ = os.Setenv("MEHR_GITHUB_TOKEN", "mehr-token")
		_ = os.Setenv("GITHUB_TOKEN", "github-token")

		token, err := ResolveToken("config-token")
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if token != "mehr-token" {
			t.Errorf("token = %q, want %q", token, "mehr-token")
		}

		_ = os.Unsetenv("MEHR_GITHUB_TOKEN")
	})

	t.Run("GITHUB_TOKEN fallback", func(t *testing.T) {
		_ = os.Unsetenv("MEHR_GITHUB_TOKEN")
		_ = os.Setenv("GITHUB_TOKEN", "github-token")

		token, err := ResolveToken("config-token")
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if token != "github-token" {
			t.Errorf("token = %q, want %q", token, "github-token")
		}

		_ = os.Unsetenv("GITHUB_TOKEN")
	})

	t.Run("config token fallback", func(t *testing.T) {
		_ = os.Unsetenv("MEHR_GITHUB_TOKEN")
		_ = os.Unsetenv("GITHUB_TOKEN")

		token, err := ResolveToken("config-token")
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if token != "config-token" {
			t.Errorf("token = %q, want %q", token, "config-token")
		}
	})

	t.Run("empty config no env - tries gh CLI", func(t *testing.T) {
		_ = os.Unsetenv("MEHR_GITHUB_TOKEN")
		_ = os.Unsetenv("GITHUB_TOKEN")

		// This will try gh CLI and likely fail (returns ErrNoToken)
		// unless gh is installed and authenticated
		_, err := ResolveToken("")
		// We can't predict if gh CLI is installed, so just check it doesn't panic
		if err != nil && err != ErrNoToken {
			// gh might return a token, which is fine
			t.Logf("ResolveToken returned error (expected if gh not installed): %v", err)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// NewClient tests
// ──────────────────────────────────────────────────────────────────────────────

func TestNewClient(t *testing.T) {
	c := NewClient("test-token", "owner", "repo")

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.gh == nil {
		t.Error("gh client is nil")
	}
	if c.owner != "owner" {
		t.Errorf("owner = %q, want %q", c.owner, "owner")
	}
	if c.repo != "repo" {
		t.Errorf("repo = %q, want %q", c.repo, "repo")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Mock tests for API calls
// ──────────────────────────────────────────────────────────────────────────────

// setupMockClient creates a test server and client pointing to it
func setupMockClient(t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)

	// Create a real GitHub client and point it to our test server
	client := gh.NewClient(nil)
	serverURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = serverURL

	c := &Client{
		gh:    client,
		owner: "test-owner",
		repo:  "test-repo",
	}

	return c, func() { server.Close() }
}

func TestGetIssue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/test-owner/test-repo/issues/123" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			issue := gh.Issue{
				Number: ptr(123),
				Title:  ptr("Test Issue"),
				Body:   ptr("Issue body"),
				State:  ptr("open"),
			}
			_ = json.NewEncoder(w).Encode(issue)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		issue, err := client.GetIssue(context.Background(), 123)
		if err != nil {
			t.Fatalf("GetIssue error = %v", err)
		}
		if issue.GetNumber() != 123 {
			t.Errorf("issue.Number = %d, want 123", issue.GetNumber())
		}
		if issue.GetTitle() != "Test Issue" {
			t.Errorf("issue.Title = %q, want %q", issue.GetTitle(), "Test Issue")
		}
	})

	t.Run("not found", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.GetIssue(context.Background(), 999)
		if err == nil {
			t.Error("expected error for not found issue")
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.GetIssue(context.Background(), 1)
		if err == nil {
			t.Error("expected error for unauthorized")
		}
	})
}

func TestGetIssueComments(t *testing.T) {
	t.Run("success single page", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			comments := []*gh.IssueComment{
				{ID: ptr(int64(1)), Body: ptr("Comment 1")},
				{ID: ptr(int64(2)), Body: ptr("Comment 2")},
			}
			_ = json.NewEncoder(w).Encode(comments)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		comments, err := client.GetIssueComments(context.Background(), 123)
		if err != nil {
			t.Fatalf("GetIssueComments error = %v", err)
		}
		if len(comments) != 2 {
			t.Errorf("len(comments) = %d, want 2", len(comments))
		}
	})

	t.Run("empty comments", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]*gh.IssueComment{})
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		comments, err := client.GetIssueComments(context.Background(), 123)
		if err != nil {
			t.Fatalf("GetIssueComments error = %v", err)
		}
		if len(comments) != 0 {
			t.Errorf("len(comments) = %d, want 0", len(comments))
		}
	})

	t.Run("API error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.GetIssueComments(context.Background(), 123)
		if err == nil {
			t.Error("expected error for API failure")
		}
	})
}

func TestAddComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}

			comment := gh.IssueComment{
				ID:   ptr(int64(999)),
				Body: ptr("New comment"),
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(comment)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		comment, err := client.AddComment(context.Background(), 123, "New comment")
		if err != nil {
			t.Fatalf("AddComment error = %v", err)
		}
		if comment.GetID() != 999 {
			t.Errorf("comment.ID = %d, want 999", comment.GetID())
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.AddComment(context.Background(), 123, "Comment")
		if err == nil {
			t.Error("expected error for unauthorized")
		}
	})
}

func TestCreatePullRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}

			pr := gh.PullRequest{
				Number:  ptr(42),
				Title:   ptr("Test PR"),
				HTMLURL: ptr("https://github.com/owner/repo/pull/42"),
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(pr)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		pr, err := client.CreatePullRequest(context.Background(), "Test PR", "PR body", "feature", "main", false)
		if err != nil {
			t.Fatalf("CreatePullRequest error = %v", err)
		}
		if pr.GetNumber() != 42 {
			t.Errorf("pr.Number = %d, want 42", pr.GetNumber())
		}
	})

	t.Run("draft PR", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req gh.NewPullRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			if !req.GetDraft() {
				t.Error("expected draft to be true")
			}

			pr := gh.PullRequest{Number: ptr(1)}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(pr)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.CreatePullRequest(context.Background(), "Draft PR", "Body", "feature", "main", true)
		if err != nil {
			t.Fatalf("CreatePullRequest error = %v", err)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Validation Failed"})
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.CreatePullRequest(context.Background(), "", "", "feature", "main", false)
		if err == nil {
			t.Error("expected error for validation failure")
		}
	})
}

func TestGetDefaultBranch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			repo := gh.Repository{
				DefaultBranch: ptr("main"),
			}
			_ = json.NewEncoder(w).Encode(repo)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		branch, err := client.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch error = %v", err)
		}
		if branch != "main" {
			t.Errorf("branch = %q, want %q", branch, "main")
		}
	})

	t.Run("master branch", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			repo := gh.Repository{
				DefaultBranch: ptr("master"),
			}
			_ = json.NewEncoder(w).Encode(repo)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		branch, err := client.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch error = %v", err)
		}
		if branch != "master" {
			t.Errorf("branch = %q, want %q", branch, "master")
		}
	})

	t.Run("API error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.GetDefaultBranch(context.Background())
		if err == nil {
			t.Error("expected error for not found repo")
		}
	})
}

func TestDownloadFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GitHub returns base64-encoded content
			content := gh.RepositoryContent{
				Content:  ptr("SGVsbG8gV29ybGQ="), // "Hello World" base64
				Encoding: ptr("base64"),
			}
			_ = json.NewEncoder(w).Encode(content)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		data, err := client.DownloadFile(context.Background(), "README.md", "main")
		if err != nil {
			t.Fatalf("DownloadFile error = %v", err)
		}
		if string(data) != "Hello World" {
			t.Errorf("content = %q, want %q", string(data), "Hello World")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		_, err := client.DownloadFile(context.Background(), "nonexistent.md", "main")
		if err == nil {
			t.Error("expected error for not found file")
		}
	})
}
