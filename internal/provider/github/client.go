package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"
)

// ptr is a helper to create a pointer to a value
func ptr[T any](v T) *T {
	return &v
}

// Client wraps the GitHub API client
type Client struct {
	gh    *github.Client
	owner string
	repo  string
}

// NewClient creates a new GitHub API client
func NewClient(token, owner, repo string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		gh:    github.NewClient(tc),
		owner: owner,
		repo:  repo,
	}
}

// ResolveToken finds the GitHub token from multiple sources
// Priority order:
//  1. MEHR_GITHUB_TOKEN env var
//  2. GITHUB_TOKEN env var
//  3. configToken (from config.yaml)
//  4. gh CLI auth token (via `gh auth token`)
func ResolveToken(configToken string) (string, error) {
	// 1. Check MEHR_GITHUB_TOKEN
	if token := os.Getenv("MEHR_GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Check GITHUB_TOKEN
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// 3. Check config token
	if configToken != "" {
		return configToken, nil
	}

	// 4. Try gh CLI
	if token := getGHCLIToken(); token != "" {
		return token, nil
	}

	return "", ErrNoToken
}

// getGHCLIToken attempts to get the token from the gh CLI
func getGHCLIToken() string {
	cmd := exec.Command("gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetIssue fetches an issue by number
func (c *Client) GetIssue(ctx context.Context, number int) (*github.Issue, error) {
	issue, _, err := c.gh.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return issue, nil
}

// GetIssueComments fetches all comments on an issue
func (c *Client) GetIssueComments(ctx context.Context, number int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allComments []*github.IssueComment
	for {
		comments, resp, err := c.gh.Issues.ListComments(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, wrapAPIError(err)
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allComments, nil
}

// AddComment adds a comment to an issue
func (c *Client) AddComment(ctx context.Context, number int, body string) (*github.IssueComment, error) {
	comment, _, err := c.gh.Issues.CreateComment(ctx, c.owner, c.repo, number, &github.IssueComment{
		Body: ptr(body),
	})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return comment, nil
}

// CreatePullRequest creates a new pull request
func (c *Client) CreatePullRequest(ctx context.Context, title, body, head, base string, draft bool) (*github.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, c.owner, c.repo, &github.NewPullRequest{
		Title: ptr(title),
		Body:  ptr(body),
		Head:  ptr(head),
		Base:  ptr(base),
		Draft: ptr(draft),
	})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return pr, nil
}

// GetDefaultBranch returns the repository's default branch
func (c *Client) GetDefaultBranch(ctx context.Context) (string, error) {
	repo, _, err := c.gh.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		return "", wrapAPIError(err)
	}
	return repo.GetDefaultBranch(), nil
}

// DownloadFile downloads a file from the repository
func (c *Client) DownloadFile(ctx context.Context, path, ref string) ([]byte, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	content, _, _, err := c.gh.Repositories.GetContents(ctx, c.owner, c.repo, path, opts)
	if err != nil {
		return nil, wrapAPIError(err)
	}

	decoded, err := content.GetContent()
	if err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}
	return []byte(decoded), nil
}

// SetOwnerRepo updates the owner and repo for the client
func (c *Client) SetOwnerRepo(owner, repo string) {
	c.owner = owner
	c.repo = repo
}

// Owner returns the current owner
func (c *Client) Owner() string {
	return c.owner
}

// Repo returns the current repo
func (c *Client) Repo() string {
	return c.repo
}
