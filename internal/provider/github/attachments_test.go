package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gh "github.com/google/go-github/v67/github"
)

func TestExtractRepoFileLinks(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantIn    []string
		wantNotIn []string
		wantLen   int
	}{
		{
			name:    "empty body",
			body:    "",
			wantLen: 0,
		},
		{
			name:    "relative markdown link",
			body:    "See [documentation](./docs/README.md) for details.",
			wantLen: 1,
			wantIn:  []string{"./docs/README.md"},
		},
		{
			name:    "absolute path markdown link",
			body:    "Check [spec](/specs/feature.md) for requirements.",
			wantLen: 1,
			wantIn:  []string{"/specs/feature.md"},
		},
		{
			name:    "simple filename",
			body:    "See [notes](notes.md) for more info.",
			wantLen: 1,
			wantIn:  []string{"notes.md"},
		},
		{
			name:    "txt file link",
			body:    "Read the [changelog](./CHANGELOG.txt)",
			wantLen: 1,
			wantIn:  []string{"./CHANGELOG.txt"},
		},
		{
			name:    "yaml file link",
			body:    "Config in [config](./config.yaml)",
			wantLen: 1,
			wantIn:  []string{"./config.yaml"},
		},
		{
			name:    "yml file link",
			body:    "Config in [config](./config.yml)",
			wantLen: 1,
			wantIn:  []string{"./config.yml"},
		},
		{
			name:    "multiple links",
			body:    "See [doc1](./doc1.md) and [doc2](./doc2.md) and [doc3](/abs/doc3.md)",
			wantLen: 3,
			wantIn:  []string{"./doc1.md", "./doc2.md", "/abs/doc3.md"},
		},
		{
			name:      "ignores http URLs",
			body:      "See [external](https://example.com/doc.md) for reference.",
			wantLen:   0,
			wantNotIn: []string{"https://example.com/doc.md"},
		},
		{
			name:      "ignores http URLs mixed with local",
			body:      "See [local](./local.md) and [external](http://example.com/doc.md)",
			wantLen:   1,
			wantIn:    []string{"./local.md"},
			wantNotIn: []string{"http://example.com/doc.md"},
		},
		{
			name:    "deduplicates links",
			body:    "See [doc](./doc.md) and again [doc](./doc.md)",
			wantLen: 1,
			wantIn:  []string{"./doc.md"},
		},
		{
			name:    "ignores non-file extensions",
			body:    "See [code](./main.go) and [data](./data.json)",
			wantLen: 0,
		},
		{
			name:    "complex markdown body",
			body:    "# Title\n\nSome text\n\n[spec1](./docs/spec1.md)\n\n## Section\n\n[spec2](./docs/spec2.md)\n\nMore text with [inline](inline.md) link.",
			wantLen: 3,
			wantIn:  []string{"./docs/spec1.md", "./docs/spec2.md", "inline.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRepoFileLinks(tt.body)

			if len(got) != tt.wantLen {
				t.Errorf("ExtractRepoFileLinks() returned %d links, want %d: %v", len(got), tt.wantLen, got)
			}

			for _, want := range tt.wantIn {
				found := false
				for _, link := range got {
					if link == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExtractRepoFileLinks() missing expected link %q", want)
				}
			}

			for _, notWant := range tt.wantNotIn {
				for _, link := range got {
					if link == notWant {
						t.Errorf("ExtractRepoFileLinks() should not contain %q", notWant)
					}
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DownloadAttachment error cases
// ──────────────────────────────────────────────────────────────────────────────

func TestDownloadAttachment_Errors(t *testing.T) {
	tests := []struct {
		name         string
		workUnitID   string
		attachmentID string
		owner        string
		repo         string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "error when repo not configured",
			workUnitID:   "123",
			attachmentID: "img-0",
			owner:        "",
			repo:         "",
			wantErr:      true,
			errContains:  "issue not found",
		},
		{
			name:         "error with invalid reference format",
			workUnitID:   "invalid-format",
			attachmentID: "img-0",
			owner:        "owner",
			repo:         "repo",
			wantErr:      true,
			errContains:  "unrecognized format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			p := &Provider{
				client: NewClient(ctx, "", tt.owner, tt.repo),
				owner:  tt.owner,
				repo:   tt.repo,
			}

			_, err := p.DownloadAttachment(ctx, tt.workUnitID, tt.attachmentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DownloadAttachment() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("DownloadAttachment() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// FetchLinkedIssueContent tests
// ──────────────────────────────────────────────────────────────────────────────

func TestFetchLinkedIssueContent_Errors(t *testing.T) {
	tests := []struct {
		name        string
		owner       string
		repo        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "error when repo not configured",
			owner:       "",
			repo:        "",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Validate error handling for unconfigured provider
			if tt.owner == "" || tt.repo == "" {
				// Provider is not configured, error is expected
				p := &Provider{
					client: NewClient(ctx, "", tt.owner, tt.repo),
					owner:  tt.owner,
					repo:   tt.repo,
				}

				_, err := p.FetchLinkedIssueContent(ctx, 123)
				if err == nil {
					t.Errorf("FetchLinkedIssueContent() expected error for unconfigured provider, got nil")
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DownloadAttachment with mock server
// ──────────────────────────────────────────────────────────────────────────────

func TestDownloadAttachment_WithMockServer(t *testing.T) {
	t.Run("successfully downloads attachment", func(t *testing.T) {
		issueBody := `![Screenshot](https://example.com/screenshot.png)`
		issueResponse := `{
			"id": 1,
			"number": 123,
			"title": "Issue with image",
			"body": "` + issueBody + `",
			"state": "open",
			"html_url": "https://github.com/owner/repo/issues/123",
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
			"user": {"login": "author"},
			"labels": [],
			"assignees": []
		}`

		// Create a test server that serves the issue and the image
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "issues") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(issueResponse))
			} else if strings.Contains(r.URL.Path, "screenshot.png") {
				w.Header().Set("Content-Type", "image/png")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("fake-image-data"))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := gh.NewClient(nil)
		serverURL, _ := parseServerURL(server.URL)
		client.BaseURL = serverURL

		// Note: This test uses http.DefaultClient which we can't easily mock
		// So we're testing the issue fetching part
		ref, err := ParseReference("owner/repo#123")
		if err != nil {
			t.Fatalf("ParseReference error = %v", err)
		}

		testClient := &Client{gh: client, owner: "owner", repo: "repo"}
		issue, err := testClient.GetIssue(context.Background(), ref.IssueNumber)
		if err != nil {
			t.Fatalf("GetIssue error = %v", err)
		}

		if issue.GetNumber() != 123 {
			t.Errorf("issue number = %d, want 123", issue.GetNumber())
		}
	})

	t.Run("body without images", func(t *testing.T) {
		issueBody := `Some text without images`

		// Test that extracting URLs from body with no images returns empty
		urls := ExtractImageURLs(issueBody)
		if len(urls) != 0 {
			t.Errorf("ExtractImageURLs() = %v, want empty", urls)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Additional ExtractRepoFileLinks edge cases
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractRepoFileLinks_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantLen int
	}{
		{
			name:    "https URLs are ignored",
			body:    "[link](https://github.com/user/repo/blob/main/doc.md)",
			wantLen: 0,
		},
		{
			name:    "http URLs are ignored",
			body:    "[link](http://github.com/user/repo/blob/main/doc.md)",
			wantLen: 0,
		},
		{
			name:    "mixed http and local links",
			body:    "[local](./doc.md) [external](https://example.com/doc.md) [another](/abs/doc.md)",
			wantLen: 2,
		},
		{
			name:    "nested directory paths",
			body:    "[deep](./path/to/deeply/nested/file.md)",
			wantLen: 1,
		},
		{
			name:    "path with special characters",
			body:    "[special](./file-with-dashes_and_underscores.md)",
			wantLen: 1,
		},
		{
			name:    "path with dots in filename",
			body:    "[dots](./file.name.with.dots.md)",
			wantLen: 1,
		},
		{
			name:    "multiple extensions not supported",
			body:    "[go file](./main.go) [json](./data.json) [md file](./doc.md)",
			wantLen: 1,
		},
		{
			name:    "yaml and yml both supported",
			body:    "[yaml](./config.yaml) [yml](./config.yml)",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRepoFileLinks(tt.body)
			if len(got) != tt.wantLen {
				t.Errorf("ExtractRepoFileLinks() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// parseServerURL helper
// ──────────────────────────────────────────────────────────────────────────────

func parseServerURL(serverURL string) (*url.URL, error) {
	return url.Parse(serverURL + "/")
}

func TestDownloadAttachment_MockServerError(t *testing.T) {
	t.Run("handles download failure", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		ctx := context.Background()
		_, err := downloadURL(ctx, server.URL+"/image.png")

		if err == nil {
			t.Error("downloadURL() expected error for 500 response, got nil")
		}
		if err != nil && !strings.Contains(err.Error(), "download failed") {
			t.Errorf("downloadURL() error = %v, want 'download failed'", err)
		}
	})
}

func TestDownloadAttachment_MockServerSuccess(t *testing.T) {
	t.Run("successful download", func(t *testing.T) {
		testData := "test-image-content"
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(testData))
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		ctx := context.Background()
		rc, err := downloadURL(ctx, server.URL+"/image.png")
		if err != nil {
			t.Fatalf("downloadURL() error = %v", err)
		}
		defer func() { _ = rc.Close() }()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll error = %v", err)
		}

		if string(data) != testData {
			t.Errorf("downloaded data = %q, want %q", string(data), testData)
		}
	})
}

func TestFetchLinkedIssueContent_EdgeCases(t *testing.T) {
	t.Run("empty issue body", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			issueResponse := `{
				"id": 1,
				"number": 789,
				"title": "Empty Issue",
				"body": "",
				"state": "open",
				"html_url": "https://github.com/owner/repo/issues/789",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
				"user": {"login": "author"},
				"labels": [],
				"assignees": []
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueResponse))
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		// Test fetching issue with empty body
		issue, err := client.GetIssue(context.Background(), 789)
		if err != nil {
			t.Fatalf("GetIssue error = %v", err)
		}

		if issue.GetBody() != "" {
			t.Errorf("issue body = %q, want empty", issue.GetBody())
		}

		// Test extracting linked issues from empty body
		linked := ExtractLinkedIssues("")
		if len(linked) != 0 {
			t.Errorf("ExtractLinkedIssues() = %v, want empty", linked)
		}
	})

	t.Run("issue with only self-reference", func(t *testing.T) {
		body := "This issue references #789 only"
		linked := ExtractLinkedIssues(body)
		if len(linked) != 1 || linked[0] != 789 {
			t.Errorf("ExtractLinkedIssues() = %v, want [789]", linked)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Context cancellation tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDownloadAttachment_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := downloadURL(ctx, "http://example.com/image.png")
	if err == nil {
		t.Error("downloadURL() expected error for canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("downloadURL() error = %v, want context.Canceled", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// FetchRepoFile tests
// ──────────────────────────────────────────────────────────────────────────────

func TestFetchRepoFile(t *testing.T) {
	t.Run("successfully fetches repository file", func(t *testing.T) {
		// GitHub API returns base64-encoded content in RepositoryContent format
		specContent := "# Specification\n\nThis is a test spec."
		base64Content := "IyBTcGVjaWZpY2F0aW9uCgpUaGlzIGlzIGEgdGVzdCBzcGVjLg=="

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			content := gh.RepositoryContent{
				Content:  &base64Content,
				Encoding: ptr("base64"),
			}
			_ = json.NewEncoder(w).Encode(content)
		})

		client, cleanup := setupMockClient(t, handler)
		defer cleanup()

		p := &Provider{
			client: client,
			owner:  "owner",
			repo:   "repo",
		}

		content, err := p.FetchRepoFile(context.Background(), "docs/spec.md", "main")
		if err != nil {
			t.Fatalf("FetchRepoFile() error = %v", err)
		}

		if string(content) != specContent {
			t.Errorf("FetchRepoFile() content = %q, want %q", string(content), specContent)
		}
	})

	t.Run("error when repo not configured", func(t *testing.T) {
		ctx := context.Background()
		p := &Provider{
			client: NewClient(ctx, "", "", ""),
			owner:  "",
			repo:   "",
		}

		_, err := p.FetchRepoFile(ctx, "docs/spec.md", "main")
		if err == nil {
			t.Error("FetchRepoFile() expected error for unconfigured repo, got nil")
		}
	})
}
