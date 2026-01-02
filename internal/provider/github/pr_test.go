package github

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// ──────────────────────────────────────────────────────────────────────────────
// GeneratePRTitle tests
// ──────────────────────────────────────────────────────────────────────────────

func TestGeneratePRTitle(t *testing.T) {
	tests := []struct {
		name     string
		taskWork *storage.TaskWork
		want     string
	}{
		{
			name:     "nil task work",
			taskWork: nil,
			want:     "Implementation",
		},
		{
			name: "task with external key and title",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "123",
					Title:       "Fix authentication bug",
				},
			},
			want: "[#123] Fix authentication bug",
		},
		{
			name: "task with external key but no title",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					ExternalKey: "456",
				},
			},
			want: "[#456] Implementation",
		},
		{
			name: "task with title but no external key",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Add new feature",
				},
			},
			want: "Add new feature",
		},
		{
			name: "task with empty metadata",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{},
			},
			want: "Implementation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePRTitle(tt.taskWork)
			if got != tt.want {
				t.Errorf("GeneratePRTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GeneratePRBody tests
// ──────────────────────────────────────────────────────────────────────────────

func TestGeneratePRBody(t *testing.T) {
	tests := []struct {
		name         string
		taskWork     *storage.TaskWork
		specs        []*storage.Specification
		diffStat     string
		wantContains []string
	}{
		{
			name:     "nil inputs",
			taskWork: nil,
			specs:    nil,
			diffStat: "",
			wantContains: []string{
				"## Summary",
				"## Test Plan",
				"- [ ] Manual testing",
			},
		},
		{
			name: "task with title and GitHub source",
			taskWork: &storage.TaskWork{
				Source: storage.SourceInfo{
					Type: "github",
				},
				Metadata: storage.WorkMetadata{
					ExternalKey: "42",
					Title:       "Implement user login",
				},
			},
			wantContains: []string{
				"## Summary",
				"Implementation for: Implement user login",
				"Closes #42",
				"## Test Plan",
			},
		},
		{
			name: "with specifications",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Add API endpoint",
				},
			},
			specs: []*storage.Specification{
				{
					Title:   "Authentication",
					Content: "Implement JWT token validation",
				},
				{
					Title:   "Rate Limiting",
					Content: "Add rate limiting middleware",
				},
			},
			wantContains: []string{
				"## Implementation Details",
				"### Authentication",
				"Implement JWT token validation",
				"### Rate Limiting",
				"Add rate limiting middleware",
			},
		},
		{
			name: "with long spec content",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Feature",
				},
			},
			specs: []*storage.Specification{
				{
					Title:   "Long Spec",
					Content: string(make([]byte, 600)), // Content longer than 500 chars
				},
			},
			wantContains: []string{
				"...", // Should be truncated
			},
		},
		{
			name: "with diff stat",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Bug fix",
				},
			},
			diffStat: " file1.go | 10 +++++-----\n file2.go | 5 +++--",
			wantContains: []string{
				"## Changes",
				"```",
				"file1.go | 10 +++++-----",
				"file2.go | 5 +++--",
				"```",
			},
		},
		{
			name: "non-github source",
			taskWork: &storage.TaskWork{
				Source: storage.SourceInfo{
					Type: "file",
				},
				Metadata: storage.WorkMetadata{
					ExternalKey: "123",
					Title:       "Task from file",
				},
			},
			wantContains: []string{
				"Implementation for: Task from file",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePRBody(tt.taskWork, tt.specs, tt.diffStat)

			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("GeneratePRBody() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CreatePRFromTask tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCreatePRFromTask(t *testing.T) {
	tests := []struct {
		name          string
		taskWork      *storage.TaskWork
		specs         []*storage.Specification
		sourceBranch  string
		diffStat      string
		draftInConfig bool
		wantDraft     bool
	}{
		{
			name: "basic PR",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title:       "Feature ABC",
					ExternalKey: "123",
				},
			},
			sourceBranch: "feature/abc",
			diffStat:     "main.go | 5 +++--",
			wantDraft:    false,
		},
		{
			name: "draft from config",
			taskWork: &storage.TaskWork{
				Metadata: storage.WorkMetadata{
					Title: "Draft PR",
				},
			},
			sourceBranch:  "feature/draft",
			draftInConfig: true,
			wantDraft:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This tests the title generation logic which doesn't require
			// a full Provider setup
			title := GeneratePRTitle(tt.taskWork)
			if tt.taskWork != nil && tt.taskWork.Metadata.ExternalKey != "" {
				if !contains(title, tt.taskWork.Metadata.ExternalKey) {
					t.Errorf("Expected title to contain external key %q, got %q", tt.taskWork.Metadata.ExternalKey, title)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CreatePullRequest error cases
// ──────────────────────────────────────────────────────────────────────────────

func TestCreatePullRequest_ProviderErrors(t *testing.T) {
	tests := []struct {
		name    string
		owner   string
		repo    string
		wantErr bool
	}{
		{
			name:    "error when owner not configured",
			owner:   "",
			repo:    "repo",
			wantErr: true,
		},
		{
			name:    "error when repo not configured",
			owner:   "owner",
			repo:    "",
			wantErr: true,
		},
		{
			name:    "error when both owner and repo not configured",
			owner:   "",
			repo:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				owner: tt.owner,
				repo:  tt.repo,
			}

			_, err := p.CreatePullRequest(context.Background(), provider.PullRequestOptions{
				Title:        "Test",
				Body:         "Body",
				SourceBranch: "feature",
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePullRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && !errors.Is(err, ErrRepoNotConfigured) {
				t.Errorf("CreatePullRequest() error = %v, want ErrRepoNotConfigured", err)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CreatePullRequest success tests with mock server
// ──────────────────────────────────────────────────────────────────────────────

func TestCreatePullRequest_Success(t *testing.T) {
	t.Run("creates PR successfully", func(t *testing.T) {
		prResponse := `{
			"id": 1,
			"number": 123,
			"title": "Test PR",
			"state": "open",
			"html_url": "https://github.com/owner/repo/pull/123",
			"head": { "ref": "feature" },
			"base": { "ref": "main" }
		}`

		// Repository response for default branch detection
		repoResponse := `{
			"id": 1,
			"name": "repo",
			"full_name": "owner/repo",
			"default_branch": "main"
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "pulls") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(prResponse))
			} else if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/repos/owner/repo") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(repoResponse))
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

		pr, err := p.CreatePullRequest(context.Background(), provider.PullRequestOptions{
			Title:        "Test PR",
			Body:         "Test body",
			SourceBranch: "feature",
		})
		if err != nil {
			t.Fatalf("CreatePullRequest() error = %v", err)
		}

		if pr.Number != 123 {
			t.Errorf("PR number = %d, want 123", pr.Number)
		}
		if pr.Title != "Test PR" {
			t.Errorf("PR title = %q, want %q", pr.Title, "Test PR")
		}
		if pr.State != "open" {
			t.Errorf("PR state = %q, want %q", pr.State, "open")
		}
	})

	t.Run("creates draft PR when configured", func(t *testing.T) {
		prResponse := `{
			"id": 2,
			"number": 456,
			"title": "Draft PR",
			"state": "open",
			"draft": true,
			"html_url": "https://github.com/owner/repo/pull/456"
		}`

		repoResponse := `{
			"id": 1,
			"name": "repo",
			"full_name": "owner/repo",
			"default_branch": "main"
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "pulls") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(prResponse))
			} else if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(repoResponse))
			}
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{DraftPR: true},
		}

		pr, err := p.CreatePullRequest(context.Background(), provider.PullRequestOptions{
			Title:        "Draft PR",
			SourceBranch: "draft-branch",
		})
		if err != nil {
			t.Fatalf("CreatePullRequest() error = %v", err)
		}

		if pr.Number != 456 {
			t.Errorf("PR number = %d, want 456", pr.Number)
		}
	})

	t.Run("uses explicit target branch when provided", func(t *testing.T) {
		prResponse := `{
			"id": 3,
			"number": 789,
			"title": "PR to develop",
			"state": "open",
			"html_url": "https://github.com/owner/repo/pull/789"
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				// Should not call GetDefaultBranch when target is specified
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(prResponse))
			} else {
				t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
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

		pr, err := p.CreatePullRequest(context.Background(), provider.PullRequestOptions{
			Title:        "PR to develop",
			SourceBranch: "feature",
			TargetBranch: "develop",
		})
		if err != nil {
			t.Fatalf("CreatePullRequest() error = %v", err)
		}

		if pr.Number != 789 {
			t.Errorf("PR number = %d, want 789", pr.Number)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GetDefaultBranch tests
// ──────────────────────────────────────────────────────────────────────────────

func TestProvider_GetDefaultBranch(t *testing.T) {
	t.Run("returns config target branch when set", func(t *testing.T) {
		p := &Provider{
			config: &Config{TargetBranch: "develop"},
			client: NewClient("", "owner", "repo"),
			owner:  "owner",
			repo:   "repo",
		}

		branch, err := p.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v", err)
		}
		if branch != "develop" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "develop")
		}
	})

	t.Run("fetches default branch from API when not configured", func(t *testing.T) {
		repoResponse := `{
			"id": 1,
			"name": "repo",
			"full_name": "owner/repo",
			"default_branch": "main"
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(repoResponse))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			config: &Config{}, // No TargetBranch set
			owner:  "owner",
			repo:   "repo",
		}

		branch, err := p.GetDefaultBranch(context.Background())
		if err != nil {
			t.Fatalf("GetDefaultBranch() error = %v", err)
		}
		if branch != "main" {
			t.Errorf("GetDefaultBranch() = %q, want %q", branch, "main")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// CreatePRFromTask tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCreatePRFromTask_Success(t *testing.T) {
	t.Run("creates PR from task context", func(t *testing.T) {
		prResponse := `{
			"id": 1,
			"number": 100,
			"title": "[#42] Feature Implementation",
			"state": "open",
			"html_url": "https://github.com/owner/repo/pull/100"
		}`

		repoResponse := `{
			"id": 1,
			"name": "repo",
			"full_name": "owner/repo",
			"default_branch": "main"
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(prResponse))
			case http.MethodGet:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(repoResponse))
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

		taskWork := &storage.TaskWork{
			Metadata: storage.WorkMetadata{
				ExternalKey: "42",
				Title:       "Feature Implementation",
			},
			Source: storage.SourceInfo{
				Type: "github",
			},
		}

		pr, err := p.CreatePRFromTask(context.Background(), taskWork, nil, "feature/42", "file1.go | 5 +++--")
		if err != nil {
			t.Fatalf("CreatePRFromTask() error = %v", err)
		}

		if pr.Title != "[#42] Feature Implementation" {
			t.Errorf("PR title = %q, want %q", pr.Title, "[#42] Feature Implementation")
		}
	})

	t.Run("creates draft PR from task when configured", func(t *testing.T) {
		prResponse := `{
			"id": 2,
			"number": 200,
			"title": "[#43] Another Feature",
			"state": "open",
			"draft": true,
			"html_url": "https://github.com/owner/repo/pull/200"
		}`

		repoResponse := `{
			"id": 1,
			"name": "repo",
			"full_name": "owner/repo",
			"default_branch": "main"
		}`

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(prResponse))
			case http.MethodGet:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(repoResponse))
			}
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
			config: &Config{DraftPR: true},
		}

		taskWork := &storage.TaskWork{
			Metadata: storage.WorkMetadata{
				ExternalKey: "43",
				Title:       "Another Feature",
			},
		}

		pr, err := p.CreatePRFromTask(context.Background(), taskWork, nil, "feature/43", "")
		if err != nil {
			t.Fatalf("CreatePRFromTask() error = %v", err)
		}

		if pr.Number != 200 {
			t.Errorf("PR number = %d, want 200", pr.Number)
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		p := &Provider{
			client: NewClient("", "", ""),
			config: &Config{},
			owner:  "",
			repo:   "",
		}

		taskWork := &storage.TaskWork{
			Metadata: storage.WorkMetadata{
				Title: "Test",
			},
		}

		_, err := p.CreatePRFromTask(context.Background(), taskWork, nil, "feature", "")
		if err == nil {
			t.Error("CreatePRFromTask() expected error for unconfigured repo, got nil")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Register test
// ──────────────────────────────────────────────────────────────────────────────

func TestRegister(t *testing.T) {
	t.Run("registers provider successfully", func(t *testing.T) {
		registry := provider.NewRegistry()

		// This should not panic
		Register(registry)

		// Verify the provider was registered by looking it up
		providers := registry.List()
		found := false
		for _, p := range providers {
			if p.Name == ProviderName {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Register() did not register %s provider", ProviderName)
		}
	})
}
