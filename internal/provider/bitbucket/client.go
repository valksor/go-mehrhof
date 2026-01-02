package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	defaultBaseURL = "https://api.bitbucket.org/2.0"
	defaultTimeout = 30 * time.Second
)

// Client wraps the Bitbucket API.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	username    string
	appPassword string
	workspace   string
	repoSlug    string
}

// NewClient creates a new Bitbucket API client.
func NewClient(username, appPassword, workspace, repoSlug string) *Client {
	return &Client{
		httpClient:  &http.Client{Timeout: defaultTimeout},
		baseURL:     defaultBaseURL,
		username:    username,
		appPassword: appPassword,
		workspace:   workspace,
		repoSlug:    repoSlug,
	}
}

// ResolveCredentials finds Bitbucket credentials from multiple sources
// Priority:
//  1. MEHR_BITBUCKET_USERNAME / MEHR_BITBUCKET_APP_PASSWORD
//  2. BITBUCKET_USERNAME / BITBUCKET_APP_PASSWORD
//  3. Config values
func ResolveCredentials(configUsername, configAppPassword string) (username, appPassword string, err error) {
	// Username resolution
	if u := os.Getenv("MEHR_BITBUCKET_USERNAME"); u != "" {
		username = u
	} else if u := os.Getenv("BITBUCKET_USERNAME"); u != "" {
		username = u
	} else if configUsername != "" {
		username = configUsername
	}

	// App password resolution
	if p := os.Getenv("MEHR_BITBUCKET_APP_PASSWORD"); p != "" {
		appPassword = p
	} else if p := os.Getenv("BITBUCKET_APP_PASSWORD"); p != "" {
		appPassword = p
	} else if configAppPassword != "" {
		appPassword = configAppPassword
	}

	if username == "" {
		return "", "", ErrNoUsername
	}
	if appPassword == "" {
		return "", "", ErrNoToken
	}

	return username, appPassword, nil
}

// SetWorkspaceRepo updates the workspace and repo for the client.
func (c *Client) SetWorkspaceRepo(workspace, repoSlug string) {
	c.workspace = workspace
	c.repoSlug = repoSlug
}

// Workspace returns the current workspace.
func (c *Client) Workspace() string {
	return c.workspace
}

// RepoSlug returns the current repository slug.
func (c *Client) RepoSlug() string {
	return c.repoSlug
}

// --- API Types ---

// Issue represents a Bitbucket issue.
type Issue struct {
	ID       int      `json:"id"`
	Title    string   `json:"title"`
	Content  *Content `json:"content"`
	State    string   `json:"state"`    // new, open, resolved, on hold, invalid, duplicate, wontfix, closed
	Priority string   `json:"priority"` // trivial, minor, major, critical, blocker
	//nolint:godox
	Kind      string     `json:"kind"` // bug, enhancement, proposal, task
	Assignee  *User      `json:"assignee"`
	Reporter  *User      `json:"reporter"`
	CreatedOn time.Time  `json:"created_on"`
	UpdatedOn time.Time  `json:"updated_on"`
	Links     Links      `json:"links"`
	Component *Component `json:"component"`
	Milestone *Milestone `json:"milestone"`
	Version   *Version   `json:"version"`
}

// Content represents issue content.
type Content struct {
	Raw    string `json:"raw"`
	Markup string `json:"markup"`
	HTML   string `json:"html"`
}

// User represents a Bitbucket user.
type User struct {
	UUID        string `json:"uuid"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AccountID   string `json:"account_id"`
	Links       Links  `json:"links"`
}

// Links contains API links.
type Links struct {
	Self     *Link `json:"self"`
	HTML     *Link `json:"html"`
	Avatar   *Link `json:"avatar"`
	Comments *Link `json:"comments"`
}

// Link represents a single API link.
type Link struct {
	Href string `json:"href"`
}

// Component represents a component.
type Component struct {
	Name string `json:"name"`
}

// Milestone represents a milestone.
type Milestone struct {
	Name string `json:"name"`
}

// Version represents a version.
type Version struct {
	Name string `json:"name"`
}

// Comment represents a comment on an issue.
type Comment struct {
	ID        int       `json:"id"`
	Content   *Content  `json:"content"`
	User      *User     `json:"user"`
	CreatedOn time.Time `json:"created_on"`
	UpdatedOn time.Time `json:"updated_on"`
	Links     Links     `json:"links"`
}

// PullRequest represents a Bitbucket pull request.
type PullRequest struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       string    `json:"state"` // OPEN, MERGED, DECLINED, SUPERSEDED
	Source      PRBranch  `json:"source"`
	Destination PRBranch  `json:"destination"`
	Author      *User     `json:"author"`
	CreatedOn   time.Time `json:"created_on"`
	UpdatedOn   time.Time `json:"updated_on"`
	Links       Links     `json:"links"`
}

// PRBranch represents a branch in a PR.
type PRBranch struct {
	Branch     Branch     `json:"branch"`
	Repository Repository `json:"repository"`
}

// Branch represents a git branch.
type Branch struct {
	Name string `json:"name"`
}

// Repository represents a repository reference.
type Repository struct {
	FullName string `json:"full_name"`
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
}

// RepoInfo represents repository information.
type RepoInfo struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	FullName    string  `json:"full_name"`
	Description string  `json:"description"`
	MainBranch  *Branch `json:"mainbranch"`
	HasIssues   bool    `json:"has_issues"`
	Owner       *User   `json:"owner"`
	Links       Links   `json:"links"`
}

// PaginatedResponse wraps paginated API responses.
type PaginatedResponse[T any] struct {
	Size     int    `json:"size"`
	Page     int    `json:"page"`
	PageLen  int    `json:"pagelen"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Values   []T    `json:"values"`
}

// --- HTTP Methods ---

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	u := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.appPassword)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, wrapAPIError(fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody)))
	}

	return respBody, nil
}

// --- Issue API ---

// GetIssue fetches an issue by ID.
func (c *Client) GetIssue(ctx context.Context, issueID int) (*Issue, error) {
	path := fmt.Sprintf("/repositories/%s/%s/issues/%d", c.workspace, c.repoSlug, issueID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal issue: %w", err)
	}

	return &issue, nil
}

// ListIssues lists issues with optional filters.
func (c *Client) ListIssues(ctx context.Context, state string, limit int) ([]Issue, error) {
	path := fmt.Sprintf("/repositories/%s/%s/issues", c.workspace, c.repoSlug)

	params := url.Values{}
	if state != "" {
		params.Set("q", fmt.Sprintf(`state="%s"`, state))
	}
	if limit > 0 {
		params.Set("pagelen", fmt.Sprintf("%d", limit))
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp PaginatedResponse[Issue]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal issues: %w", err)
	}

	return resp.Values, nil
}

// GetIssueComments fetches comments on an issue.
func (c *Client) GetIssueComments(ctx context.Context, issueID int) ([]Comment, error) {
	path := fmt.Sprintf("/repositories/%s/%s/issues/%d/comments", c.workspace, c.repoSlug, issueID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp PaginatedResponse[Comment]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal comments: %w", err)
	}

	return resp.Values, nil
}

// AddIssueComment adds a comment to an issue.
func (c *Client) AddIssueComment(ctx context.Context, issueID int, body string) (*Comment, error) {
	path := fmt.Sprintf("/repositories/%s/%s/issues/%d/comments", c.workspace, c.repoSlug, issueID)

	reqBody := map[string]any{
		"content": map[string]string{
			"raw": body,
		},
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := json.Unmarshal(respBody, &comment); err != nil {
		return nil, fmt.Errorf("unmarshal comment: %w", err)
	}

	return &comment, nil
}

// UpdateIssueState updates the state of an issue.
func (c *Client) UpdateIssueState(ctx context.Context, issueID int, state string) (*Issue, error) {
	path := fmt.Sprintf("/repositories/%s/%s/issues/%d", c.workspace, c.repoSlug, issueID)

	reqBody := map[string]any{
		"state": state,
	}

	respBody, err := c.doRequest(ctx, http.MethodPut, path, reqBody)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal issue: %w", err)
	}

	return &issue, nil
}

// --- Repository API ---

// GetRepository fetches repository information.
func (c *Client) GetRepository(ctx context.Context) (*RepoInfo, error) {
	path := fmt.Sprintf("/repositories/%s/%s", c.workspace, c.repoSlug)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var repo RepoInfo
	if err := json.Unmarshal(body, &repo); err != nil {
		return nil, fmt.Errorf("unmarshal repository: %w", err)
	}

	return &repo, nil
}

// GetDefaultBranch returns the repository's default branch.
func (c *Client) GetDefaultBranch(ctx context.Context) (string, error) {
	repo, err := c.GetRepository(ctx)
	if err != nil {
		return "", err
	}

	if repo.MainBranch != nil && repo.MainBranch.Name != "" {
		return repo.MainBranch.Name, nil
	}

	return "main", nil // Default fallback
}

// --- Pull Request API ---

// CreatePullRequest creates a new pull request.
func (c *Client) CreatePullRequest(ctx context.Context, title, description, sourceBranch, targetBranch string, closeSourceBranch bool) (*PullRequest, error) {
	path := fmt.Sprintf("/repositories/%s/%s/pullrequests", c.workspace, c.repoSlug)

	reqBody := map[string]any{
		"title":       title,
		"description": description,
		"source": map[string]any{
			"branch": map[string]string{
				"name": sourceBranch,
			},
		},
		"destination": map[string]any{
			"branch": map[string]string{
				"name": targetBranch,
			},
		},
		"close_source_branch": closeSourceBranch,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var pr PullRequest
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return nil, fmt.Errorf("unmarshal pull request: %w", err)
	}

	return &pr, nil
}

// --- Issue Creation API ---

// CreateIssue creates a new issue in the repository.
func (c *Client) CreateIssue(ctx context.Context, title, content, priority, kind string) (*Issue, error) {
	path := fmt.Sprintf("/repositories/%s/%s/issues", c.workspace, c.repoSlug)

	reqBody := map[string]any{
		"title": title,
		"content": map[string]string{
			"raw": content,
		},
		"priority": priority,
		"kind":     kind,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal issue: %w", err)
	}

	return &issue, nil
}
