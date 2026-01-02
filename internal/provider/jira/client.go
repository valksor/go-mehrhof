package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
	"github.com/valksor/go-mehrhof/internal/provider/token"
)

const (
	defaultTimeout = 30 * time.Second
	maxRetries     = 3
	initialBackoff = 1 * time.Second

	// Jira API versions.
	cloudAPIVersion   = "3"
	serverAPIVersion  = "2"
	defaultAPIVersion = "3" // Default to v3 (Cloud)
)

// Client wraps the Jira API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	email      string
	apiVersion string
}

// NewClient creates a new Jira API client.
func NewClient(token, email, baseURL string) *Client {
	apiVersion := defaultAPIVersion

	// Detect API version from base URL
	if baseURL != "" {
		if strings.Contains(baseURL, "atlassian.net") {
			apiVersion = cloudAPIVersion
		} else {
			// Non-atlassian.net likely means Server/Data Center
			apiVersion = serverAPIVersion
		}
	}

	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    baseURL,
		token:      token,
		email:      email,
		apiVersion: apiVersion,
	}
}

// SetBaseURL updates the base URL.
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
	if strings.Contains(baseURL, "atlassian.net") {
		c.apiVersion = cloudAPIVersion
	} else {
		c.apiVersion = serverAPIVersion
	}
}

// ResolveToken finds the Jira token from multiple sources.
// Priority order:
//  1. MEHR_JIRA_TOKEN env var
//  2. JIRA_TOKEN env var
//  3. configToken (from config.yaml)
func ResolveToken(configToken string) (string, error) {
	return token.ResolveToken(token.Config("JIRA", configToken).
		WithEnvVars("JIRA_TOKEN"))
}

// buildAPIURL constructs the full API URL for a given endpoint.
func (c *Client) buildAPIURL(endpoint string) (string, error) {
	if c.baseURL == "" {
		return "", errors.New("base URL not set")
	}

	// Clean base URL
	baseURL := strings.TrimSuffix(c.baseURL, "/")

	// Add /rest/api if not present
	if !strings.Contains(baseURL, "/rest/api") {
		baseURL += "/rest/api/" + c.apiVersion
	}

	return baseURL + endpoint, nil
}

// getAuthHeader returns the authorization header value.
func (c *Client) getAuthHeader() string {
	if c.email != "" {
		// Jira Cloud uses email + token as Basic Auth
		auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))

		return "Basic " + auth
	}
	// Jira Server/Data Center may use PAT as Bearer
	if strings.HasPrefix(c.token, "Bearer ") {
		return c.token
	}
	// Try Basic Auth with token as password
	auth := base64.StdEncoding.EncodeToString([]byte(": " + c.token))

	return "Basic " + auth
}

// doRequest performs an HTTP request to the Jira API.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body any, result any) error {
	apiURL, err := c.buildAPIURL(endpoint)
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiURL, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.getAuthHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wrapAPIError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return wrapAPIError(&httpError{code: resp.StatusCode, message: string(respBody)})
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// GetIssue fetches an issue by key.
func (c *Client) GetIssue(ctx context.Context, issueKey string) (*Issue, error) {
	endpoint := "/issue/" + issueKey

	var response Issue
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListIssues fetches issues with JQL filtering.
func (c *Client) ListIssues(ctx context.Context, jql string, startAt, maxResults int) ([]*Issue, int, error) {
	endpoint := fmt.Sprintf("/search?jql=%s&startAt=%d&maxResults=%d",
		url.QueryEscape(jql), startAt, maxResults)

	var response SearchResponse
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, 0, err
	}

	return response.Issues, response.Total, nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(ctx context.Context, input CreateIssueInput) (*Issue, error) {
	var response CreateIssueResponse
	if err := c.doRequest(ctx, http.MethodPost, "/issue", input, &response); err != nil {
		return nil, err
	}

	// Fetch the created issue to get all details
	return c.GetIssue(ctx, response.Key)
}

// UpdateIssue updates an existing issue.
func (c *Client) UpdateIssue(ctx context.Context, issueKey string, input UpdateIssueInput) error {
	endpoint := "/issue/" + issueKey

	return c.doRequest(ctx, http.MethodPut, endpoint, input, nil)
}

// GetTransitions fetches available transitions for an issue.
func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]*Transition, error) {
	endpoint := fmt.Sprintf("/issue/%s/transitions", issueKey)

	var response TransitionsResponse
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response.Transitions, nil
}

// DoTransition performs a workflow transition.
func (c *Client) DoTransition(ctx context.Context, issueKey, transitionID string) error {
	endpoint := fmt.Sprintf("/issue/%s/transitions", issueKey)
	input := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}

	return c.doRequest(ctx, http.MethodPost, endpoint, input, nil)
}

// AddComment adds a comment to an issue.
func (c *Client) AddComment(ctx context.Context, issueKey, body string) (*Comment, error) {
	endpoint := fmt.Sprintf("/issue/%s/comment", issueKey)
	input := map[string]string{"body": body}

	var response Comment
	if err := c.doRequest(ctx, http.MethodPost, endpoint, input, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetComments fetches comments for an issue.
func (c *Client) GetComments(ctx context.Context, issueKey string) ([]*Comment, error) {
	endpoint := fmt.Sprintf("/issue/%s/comment", issueKey)

	var response CommentsResponse
	if err := c.doRequest(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response.Comments, nil
}

// GetAttachments lists attachments for an issue.
func (c *Client) GetAttachments(ctx context.Context, issueKey string) ([]*Attachment, error) {
	// Attachments are included in the issue data
	issue, err := c.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, err
	}

	return issue.Fields.Attachments, nil
}

// DownloadAttachment downloads an attachment file.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentURL string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attachmentURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.getAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", wrapAPIError(err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()

		return nil, "", wrapAPIError(&httpError{code: resp.StatusCode, message: "download failed"})
	}

	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// GetSubtasks fetches subtasks for an issue.
func (c *Client) GetSubtasks(ctx context.Context, issueKey string) ([]*Issue, error) {
	// Get the issue to extract subtasks from the fields
	issue, err := c.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, err
	}

	return issue.Fields.Subtasks, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Jira API Types
// ──────────────────────────────────────────────────────────────────────────────

// Issue represents a Jira issue.
type Issue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields Fields `json:"fields"`
}

// Fields contains issue fields.
type Fields struct {
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Status      *Status       `json:"status"`
	Priority    *Priority     `json:"priority"`
	Labels      []string      `json:"labels"`
	Assignee    *User         `json:"assignee"`
	Reporter    *User         `json:"reporter"`
	Created     time.Time     `json:"created"`
	Updated     time.Time     `json:"updated"`
	Project     *Project      `json:"project"`
	Issuetype   *IssueType    `json:"issuetype"`
	Sprint      *Sprint       `json:"sprint"`
	Attachments []*Attachment `json:"attachment"`
	Subtasks    []*Issue      `json:"subtasks"`
	Parent      *Issue        `json:"parent"`
}

// Status represents issue status.
type Status struct {
	Self string `json:"self"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// Priority represents issue priority.
type Priority struct {
	Self string `json:"self"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

// User represents a Jira user.
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// Project represents a Jira project.
type Project struct {
	Self string `json:"self"`
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// IssueType represents the type of issue.
type IssueType struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Name        string `json:"name"`
}

// Sprint represents an agile sprint.
type Sprint struct {
	Name  string `json:"name"`
	State string `json:"state"`
	ID    int64  `json:"id"`
}

// Comment represents a Jira comment.
type Comment struct {
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Author  *User     `json:"author"`
	ID      string    `json:"id"`
	Self    string    `json:"self"`
	Body    string    `json:"body"`
}

// Attachment represents a file attachment.
type Attachment struct {
	Created  time.Time `json:"created"`
	ID       string    `json:"id"`
	Self     string    `json:"self"`
	Filename string    `json:"filename"`
	Content  string    `json:"content"`
	MimeType string    `json:"mimeType"`
	Size     int64     `json:"size"`
}

// Transition represents a workflow transition.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateIssueInput represents the input for creating an issue.
type CreateIssueInput struct {
	Fields struct {
		Project     *Project   `json:"project"`
		IssueType   *IssueType `json:"issuetype"`
		Priority    *Priority  `json:"priority,omitempty"`
		Assignee    *User      `json:"assignee,omitempty"`
		Summary     string     `json:"summary"`
		Description string     `json:"description,omitempty"`
		Labels      []string   `json:"labels,omitempty"`
	} `json:"fields"`
}

// UpdateIssueInput represents the input for updating an issue.
type UpdateIssueInput struct {
	Fields struct {
		Priority *Priority `json:"priority,omitempty"`
		Summary  string    `json:"summary,omitempty"`
		Labels   []string  `json:"labels,omitempty"`
	} `json:"fields,omitempty"`
}

// SearchResponse represents Jira search results.
type SearchResponse struct {
	Issues     []*Issue `json:"issues"`
	StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Total      int      `json:"total"`
}

// CreateIssueResponse represents the response from creating an issue.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// TransitionsResponse represents the response from getting transitions.
type TransitionsResponse struct {
	Transitions []*Transition `json:"transitions"`
}

// CommentsResponse represents the response from getting comments.
type CommentsResponse struct {
	Comments   []*Comment `json:"comments"`
	StartAt    int        `json:"startAt"`
	MaxResults int        `json:"maxResults"`
	Total      int        `json:"total"`
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Error wrapper
// ──────────────────────────────────────────────────────────────────────────────

// httpError wraps an HTTP error for proper error handling.
type httpError struct {
	message string
	code    int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.code, e.message)
}

func (e *httpError) HTTPStatusCode() int {
	return e.code
}

// wrapAPIError converts errors to typed errors using shared error package.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	var httpErr interface{ HTTPStatusCode() int }
	if errors.As(err, &httpErr) {
		switch httpErr.HTTPStatusCode() {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %w", providererrors.ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %w", providererrors.ErrRateLimited, err)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %w", providererrors.ErrNotFound, err)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %w", providererrors.ErrRateLimited, err)
		}
	}

	return err
}
