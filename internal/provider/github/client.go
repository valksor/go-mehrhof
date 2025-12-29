package github

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"

	"github.com/valksor/go-mehrhof/internal/cache"
	"github.com/valksor/go-mehrhof/internal/provider/token"
)

// ptr is a helper to create a pointer to a value
func ptr[T any](v T) *T {
	return &v
}

// Client wraps the GitHub API client
type Client struct {
	gh    *github.Client
	cache *cache.Cache
	owner string
	repo  string
}

// NewClient creates a new GitHub API client
func NewClient(token, owner, repo string) *Client {
	return NewClientWithCache(token, owner, repo, nil)
}

// NewClientWithCache creates a new GitHub API client with an optional cache
func NewClientWithCache(token, owner, repo string, c *cache.Cache) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		gh:    github.NewClient(tc),
		owner: owner,
		repo:  repo,
		cache: c,
	}
}

// SetCache sets or updates the cache for this client
func (c *Client) SetCache(cache *cache.Cache) {
	c.cache = cache
}

// CacheKey generates a namespaced cache key for this client
func (c *Client) CacheKey(resourceType, id string) string {
	return fmt.Sprintf("github:%s/%s:%s:%s", c.owner, c.repo, resourceType, id)
}

// ResolveToken finds the GitHub token from multiple sources.
// Priority order:
//  1. MEHR_GITHUB_TOKEN env var
//  2. GITHUB_TOKEN env var
//  3. configToken (from config.yaml)
//  4. gh CLI auth token (via `gh auth token`)
func ResolveToken(configToken string) (string, error) {
	return token.ResolveToken(token.Config("GITHUB", configToken).
		WithEnvVars("GITHUB_TOKEN").
		WithCLIFallback(getGHCLIToken))
}

// getGHCLIToken attempts to get the token from the gh CLI
func getGHCLIToken() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetIssue fetches an issue by number
func (c *Client) GetIssue(ctx context.Context, number int) (*github.Issue, error) {
	key := c.CacheKey("issue", fmt.Sprintf("%d", number))

	// Try cache first
	if c.cache != nil && c.cache.Enabled() {
		if val, ok := c.cache.Get(key); ok && val != nil {
			if issue, ok := val.(*github.Issue); ok {
				return issue, nil
			}
		}
	}

	// Cache miss or disabled, fetch from API
	issue, _, err := c.gh.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, wrapAPIError(err)
	}

	// Store in cache
	if c.cache != nil && c.cache.Enabled() {
		c.cache.Set(key, issue, cache.DefaultIssueTTL)
	}

	return issue, nil
}

// GetIssueComments fetches all comments on an issue
func (c *Client) GetIssueComments(ctx context.Context, number int) ([]*github.IssueComment, error) {
	key := c.CacheKey("comments", fmt.Sprintf("%d", number))

	// Try cache first
	if c.cache != nil && c.cache.Enabled() {
		if val, ok := c.cache.Get(key); ok && val != nil {
			if comments, ok := val.([]*github.IssueComment); ok {
				return comments, nil
			}
		}
	}

	// Cache miss or disabled, fetch from API
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

	// Store in cache
	if c.cache != nil && c.cache.Enabled() {
		c.cache.Set(key, allComments, cache.DefaultCommentsTTL)
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

	// Invalidate comments cache for this issue
	if c.cache != nil {
		key := c.CacheKey("comments", fmt.Sprintf("%d", number))
		c.cache.Delete(key)
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
	key := c.CacheKey("metadata", "default-branch")

	// Try cache first
	if c.cache != nil && c.cache.Enabled() {
		if val, ok := c.cache.Get(key); ok && val != nil {
			if branch, ok := val.(string); ok {
				return branch, nil
			}
		}
	}

	// Cache miss or disabled, fetch from API
	repo, _, err := c.gh.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		return "", wrapAPIError(err)
	}
	branch := repo.GetDefaultBranch()

	// Store in cache
	if c.cache != nil && c.cache.Enabled() {
		c.cache.Set(key, branch, cache.DefaultMetadataTTL)
	}

	return branch, nil
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
