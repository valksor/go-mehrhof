package github

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-github/v67/github"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Ref
		wantErr   bool
		errString string
	}{
		{
			name:  "bare number",
			input: "5",
			want: &Ref{
				IssueNumber: 5,
				IsExplicit:  false,
			},
		},
		{
			name:  "hash number",
			input: "#123",
			want: &Ref{
				IssueNumber: 123,
				IsExplicit:  false,
			},
		},
		{
			name:  "github scheme bare number",
			input: "github:42",
			want: &Ref{
				IssueNumber: 42,
				IsExplicit:  false,
			},
		},
		{
			name:  "gh scheme bare number",
			input: "gh:99",
			want: &Ref{
				IssueNumber: 99,
				IsExplicit:  false,
			},
		},
		{
			name:  "explicit owner/repo#number",
			input: "owner/repo#5",
			want: &Ref{
				Owner:       "owner",
				Repo:        "repo",
				IssueNumber: 5,
				IsExplicit:  true,
			},
		},
		{
			name:  "github scheme with explicit repo",
			input: "github:myorg/myproject#100",
			want: &Ref{
				Owner:       "myorg",
				Repo:        "myproject",
				IssueNumber: 100,
				IsExplicit:  true,
			},
		},
		{
			name:  "gh scheme with explicit repo",
			input: "gh:acme/widgets#1",
			want: &Ref{
				Owner:       "acme",
				Repo:        "widgets",
				IssueNumber: 1,
				IsExplicit:  true,
			},
		},
		{
			name:  "hyphenated owner and repo",
			input: "github:my-org/my-project#42",
			want: &Ref{
				Owner:       "my-org",
				Repo:        "my-project",
				IssueNumber: 42,
				IsExplicit:  true,
			},
		},
		{
			name:  "underscored names",
			input: "github:my_org/my_project#7",
			want: &Ref{
				Owner:       "my_org",
				Repo:        "my_project",
				IssueNumber: 7,
				IsExplicit:  true,
			},
		},
		{
			name:      "empty string",
			input:     "",
			wantErr:   true,
			errString: "empty reference",
		},
		{
			name:      "invalid format",
			input:     "not-a-reference",
			wantErr:   true,
			errString: "unrecognized format",
		},
		{
			name:  "zero issue number",
			input: "0",
			// Note: 0 is syntactically valid, API will reject it
			want: &Ref{
				IssueNumber: 0,
				IsExplicit:  false,
			},
		},
		{
			name:      "negative number",
			input:     "-5",
			wantErr:   true,
			errString: "unrecognized format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseReference(%q) expected error containing %q, got nil", tt.input, tt.errString)
					return
				}
				if tt.errString != "" && !contains(err.Error(), tt.errString) {
					t.Errorf("ParseReference(%q) error = %v, want error containing %q", tt.input, err, tt.errString)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseReference(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got.Owner != tt.want.Owner {
				t.Errorf("ParseReference(%q).Owner = %q, want %q", tt.input, got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("ParseReference(%q).Repo = %q, want %q", tt.input, got.Repo, tt.want.Repo)
			}
			if got.IssueNumber != tt.want.IssueNumber {
				t.Errorf("ParseReference(%q).IssueNumber = %d, want %d", tt.input, got.IssueNumber, tt.want.IssueNumber)
			}
			if got.IsExplicit != tt.want.IsExplicit {
				t.Errorf("ParseReference(%q).IsExplicit = %v, want %v", tt.input, got.IsExplicit, tt.want.IsExplicit)
			}
		})
	}
}

func TestDetectRepository(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "ssh url",
			remoteURL: "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "https url",
			remoteURL: "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "https url without .git",
			remoteURL: "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "ssh url without .git",
			remoteURL: "git@github.com:owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "hyphenated names",
			remoteURL: "git@github.com:my-org/my-repo.git",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:      "empty url",
			remoteURL: "",
			wantErr:   true,
		},
		{
			name:      "non-github url",
			remoteURL: "git@gitlab.com:owner/repo.git",
			wantErr:   true,
		},
		{
			name:      "invalid format",
			remoteURL: "not-a-url",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := DetectRepository(tt.remoteURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DetectRepository(%q) expected error, got nil", tt.remoteURL)
				}
				return
			}

			if err != nil {
				t.Errorf("DetectRepository(%q) unexpected error: %v", tt.remoteURL, err)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("DetectRepository(%q) owner = %q, want %q", tt.remoteURL, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("DetectRepository(%q) repo = %q, want %q", tt.remoteURL, repo, tt.wantRepo)
			}
		})
	}
}

func TestExtractLinkedIssues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []int
	}{
		{
			name: "single reference",
			body: "This fixes #123",
			want: []int{123},
		},
		{
			name: "multiple references",
			body: "Related to #1, #2, and #3",
			want: []int{1, 2, 3},
		},
		{
			name: "no references",
			body: "No issue references here",
			want: []int{},
		},
		{
			name: "mixed content",
			body: "Fixes #42 and closes #100. Also see issue #7.",
			want: []int{42, 100, 7},
		},
		{
			name: "duplicate references",
			body: "See #5 and also #5 again",
			want: []int{5},
		},
		{
			name: "references at line start",
			body: "#10 is the main issue\n#20 is related",
			want: []int{10, 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractLinkedIssues(tt.body)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractLinkedIssues() = %v, want %v", got, tt.want)
				return
			}

			// Check that all expected values are present
			gotMap := make(map[int]bool)
			for _, n := range got {
				gotMap[n] = true
			}
			for _, w := range tt.want {
				if !gotMap[w] {
					t.Errorf("ExtractLinkedIssues() missing %d, got %v", w, got)
				}
			}
		})
	}
}

func TestExtractImageURLs(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single image",
			body: "![alt](https://example.com/image.png)",
			want: []string{"https://example.com/image.png"},
		},
		{
			name: "multiple images",
			body: "![a](http://a.com/1.jpg) ![b](http://b.com/2.png)",
			want: []string{"http://a.com/1.jpg", "http://b.com/2.png"},
		},
		{
			name: "no images",
			body: "No images here",
			want: []string{},
		},
		{
			name: "github user attachment",
			body: "![Screenshot](https://user-images.githubusercontent.com/123/456.png)",
			want: []string{"https://user-images.githubusercontent.com/123/456.png"},
		},
		{
			name: "empty alt text",
			body: "![](https://example.com/img.gif)",
			want: []string{"https://example.com/img.gif"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImageURLs(tt.body)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractImageURLs() = %v, want %v", got, tt.want)
				return
			}

			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("ExtractImageURLs()[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

// Note: inferTypeFromLabels is tested indirectly via integration tests
// as it takes []*github.Label which requires the external library type.

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  *Ref
		want string
	}{
		{
			name: "explicit owner/repo",
			ref: &Ref{
				Owner:       "owner",
				Repo:        "repo",
				IssueNumber: 123,
				IsExplicit:  true,
			},
			want: "owner/repo#123",
		},
		{
			name: "simple reference",
			ref: &Ref{
				IssueNumber: 42,
				IsExplicit:  false,
			},
			want: "#42",
		},
		{
			name: "empty owner with repo",
			ref: &Ref{
				Owner:       "",
				Repo:        "repo",
				IssueNumber: 5,
			},
			want: "#5",
		},
		{
			name: "owner with empty repo",
			ref: &Ref{
				Owner:       "owner",
				Repo:        "",
				IssueNumber: 10,
			},
			want: "#10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("Ref.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWrapAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantErr     error
		wantNil     bool
		wantWrapped bool
	}{
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
		{
			name: "401 unauthorized",
			err: &github.ErrorResponse{
				Response: &http.Response{StatusCode: 401},
				Message:  "Bad credentials",
			},
			wantErr:     ErrUnauthorized,
			wantWrapped: true,
		},
		{
			name: "403 rate limit",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 403,
					Header:     http.Header{"X-RateLimit-Reset": []string{"1234567890"}},
				},
				Message: "API rate limit exceeded",
			},
			wantErr:     ErrRateLimited,
			wantWrapped: true,
		},
		{
			name: "403 insufficient scope",
			err: &github.ErrorResponse{
				Response: &http.Response{StatusCode: 403},
				Message:  "Resource not accessible",
			},
			wantErr:     ErrInsufficientScope,
			wantWrapped: true,
		},
		{
			name: "404 not found",
			err: &github.ErrorResponse{
				Response: &http.Response{StatusCode: 404},
				Message:  "Not Found",
			},
			wantErr:     ErrIssueNotFound,
			wantWrapped: true,
		},
		{
			name:        "generic error passthrough",
			err:         errors.New("some other error"),
			wantWrapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapAPIError(tt.err)

			if tt.wantNil {
				if got != nil {
					t.Errorf("wrapAPIError() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Error("wrapAPIError() = nil, want non-nil error")
				return
			}

			if tt.wantWrapped {
				if !errors.Is(got, tt.wantErr) {
					t.Errorf("wrapAPIError() error = %v, want wrapped %v", got, tt.wantErr)
				}
			}
		})
	}
}
