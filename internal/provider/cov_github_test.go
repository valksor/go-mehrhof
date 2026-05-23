package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGitHubProvider_ListTasks(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues" {
			w.Header().Set("Content-Type", "application/json")
			// Include a PR (has pull_request) which must be filtered out.
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 1, "title": "Real issue", "state": "open", "html_url": "u1"},
				{"number": 2, "title": "A PR", "state": "open", "pull_request": map[string]any{"url": "x"}},
			})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	res, err := p.ListTasks(context.Background(), ListOptions{Team: "owner/repo", Status: "open", Limit: 30})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(res.Tasks) != 1 {
		t.Fatalf("ListTasks() returned %d tasks, want 1 (PR filtered out)", len(res.Tasks))
	}
	if res.Tasks[0].Title != "Real issue" {
		t.Errorf("Title = %q, want 'Real issue'", res.Tasks[0].Title)
	}
}

func TestGitHubProvider_ListTasks_Errors(t *testing.T) {
	p := newTestGitHubProvider(t, newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if _, err := p.ListTasks(context.Background(), ListOptions{}); err == nil {
		t.Error("ListTasks() with empty Team should error")
	}
	if _, err := p.ListTasks(context.Background(), ListOptions{Team: "noslash"}); err == nil {
		t.Error("ListTasks() with malformed Team should error")
	}
	if _, err := p.ListTasks(context.Background(), ListOptions{Team: "o/r", Cursor: "notanumber"}); err == nil {
		t.Error("ListTasks() with non-numeric cursor should error")
	}
}

func TestGitHubProvider_FetchComments(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/7/comments" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 100, "body": "first comment", "user": map[string]any{"login": "alice"}},
				{"id": 101, "body": "second comment", "user": map[string]any{"login": "bob"}},
			})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)

	comments, err := p.FetchComments(context.Background(), "owner/repo#7")
	if err != nil {
		t.Fatalf("FetchComments() error = %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("FetchComments() returned %d, want 2", len(comments))
	}
	if comments[0].Body != "first comment" || comments[0].Author != "alice" {
		t.Errorf("comment[0] = %+v, want body 'first comment' author 'alice'", comments[0])
	}
}

func TestGitHubProvider_AddLabels(t *testing.T) {
	var gotLabels []string
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues/3/labels" && r.Method == http.MethodPost {
			var labels []string
			_ = json.NewDecoder(r.Body).Decode(&labels)
			gotLabels = labels
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "bug"}})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	if err := p.AddLabels(context.Background(), "owner/repo#3", []string{"bug", "urgent"}); err != nil {
		t.Fatalf("AddLabels() error = %v", err)
	}
	if len(gotLabels) != 2 {
		t.Errorf("server received %d labels, want 2", len(gotLabels))
	}

	// Malformed ID should error before any HTTP call.
	if err := p.AddLabels(context.Background(), "bad-id", []string{"x"}); err == nil {
		t.Error("AddLabels() with malformed ID should error")
	}
}

func TestGitHubProvider_RemoveLabels(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		// First label exists; second returns 404 (already absent) — both tolerated.
		if r.Method == http.MethodDelete && r.URL.Path == "/repos/owner/repo/issues/4/labels/gone" {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		if r.Method == http.MethodDelete {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	if err := p.RemoveLabels(context.Background(), "owner/repo#4", []string{"present", "gone"}); err != nil {
		t.Fatalf("RemoveLabels() error = %v", err)
	}

	if err := p.RemoveLabels(context.Background(), "bad", []string{"x"}); err == nil {
		t.Error("RemoveLabels() with malformed ID should error")
	}
}

func TestGitHubProvider_CreateTask(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/issues" && r.Method == http.MethodPost {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   55,
				"title":    req["title"],
				"body":     req["body"],
				"state":    "open",
				"html_url": "https://github.com/owner/repo/issues/55",
			})

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	task, err := p.CreateTask(context.Background(), CreateTaskOptions{
		Team:        "owner/repo",
		Title:       "New issue",
		Description: "the body",
		Labels:      []string{"enhancement"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.Title != "New issue" {
		t.Errorf("Title = %q, want 'New issue'", task.Title)
	}
	if task.ID != "owner/repo#55" {
		t.Errorf("ID = %q, want owner/repo#55", task.ID)
	}

	// Validation errors.
	if _, err := p.CreateTask(context.Background(), CreateTaskOptions{}); err == nil {
		t.Error("CreateTask() with empty Team should error")
	}
	if _, err := p.CreateTask(context.Background(), CreateTaskOptions{Team: "noslash"}); err == nil {
		t.Error("CreateTask() with malformed Team should error")
	}
}

func TestGitHubProvider_DeleteBranch(t *testing.T) {
	srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/repos/owner/repo/git/refs/heads/feature" {
			w.WriteHeader(http.StatusNoContent)

			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := newTestGitHubProvider(t, srv)
	if err := p.DeleteBranch(context.Background(), "owner/repo", "feature"); err != nil {
		t.Fatalf("DeleteBranch() error = %v", err)
	}

	if err := p.DeleteBranch(context.Background(), "noslash", "feature"); err == nil {
		t.Error("DeleteBranch() with malformed repo should error")
	}
}

func TestGitHubProvider_GetBranchProtection(t *testing.T) {
	t.Run("with rules", func(t *testing.T) {
		srv := newTestGitHubServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/owner/repo/branches/main/protection" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"required_status_checks": map[string]any{
						"checks": []map[string]any{
							{"context": "ci/test"},
							{"context": "ci/lint"},
						},
					},
					"required_pull_request_reviews": map[string]any{
						"required_approving_review_count": 2,
						"dismiss_stale_reviews":           true,
					},
					"enforce_admins": map[string]any{"enabled": true},
				})

				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer srv.Close()

		p := newTestGitHubProvider(t, srv)
		bp, err := p.GetBranchProtection(context.Background(), "owner", "repo", "main")
		if err != nil {
			t.Fatalf("GetBranchProtection() error = %v", err)
		}
		if len(bp.RequiredChecks) != 2 {
			t.Errorf("RequiredChecks = %v, want 2 entries", bp.RequiredChecks)
		}
		if !bp.RequireReviews || bp.MinReviewers != 2 || !bp.DismissStaleReviews {
			t.Errorf("review settings wrong: %+v", bp)
		}
		if !bp.EnforceAdmins {
			t.Error("EnforceAdmins should be true")
		}
	})

	t.Run("no protection returns empty", func(t *testing.T) {
		srv := newTestGitHubServer(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		defer srv.Close()

		p := newTestGitHubProvider(t, srv)
		bp, err := p.GetBranchProtection(context.Background(), "owner", "repo", "main")
		if err != nil {
			t.Fatalf("GetBranchProtection() error = %v", err)
		}
		if bp == nil || len(bp.RequiredChecks) != 0 {
			t.Errorf("expected empty BranchProtection for unprotected branch, got %+v", bp)
		}
	})
}

func TestGitHubProvider_ResolveDependencies(t *testing.T) {
	p := &GitHubProvider{}

	t.Run("no dependencies", func(t *testing.T) {
		task := &Task{Description: "Just a plain description"}
		if deps := p.resolveDependencies(task); deps != nil {
			t.Errorf("resolveDependencies() = %v, want nil", deps)
		}
	})

	t.Run("full and shorthand refs", func(t *testing.T) {
		task := &Task{Description: "Some context.\nDepends on: other/repo#5, #9\n"}
		task.SetMetadata("github_owner", "owner")
		task.SetMetadata("github_repo", "repo")

		deps := p.resolveDependencies(task)
		if len(deps) != 2 {
			t.Fatalf("resolveDependencies() returned %d deps, want 2", len(deps))
		}
		// Shorthand "#9" should be expanded using owner/repo metadata.
		ids := map[string]bool{}
		for _, d := range deps {
			ids[d.ID] = true
			if d.Source != NameGitHub {
				t.Errorf("dep Source = %q, want github", d.Source)
			}
		}
		if !ids["other/repo#5"] {
			t.Errorf("expected full ref other/repo#5 preserved, got %v", ids)
		}
		if !ids["owner/repo#9"] {
			t.Errorf("expected shorthand #9 expanded to owner/repo#9, got %v", ids)
		}
	})
}

func TestIsTransientMergeError(t *testing.T) {
	if isTransientMergeError(nil) {
		t.Error("isTransientMergeError(nil) = true, want false")
	}
	// A non-GitHub error is not transient.
	if isTransientMergeError(context.Canceled) {
		t.Error("isTransientMergeError(context.Canceled) = true, want false")
	}
}
