package wrike

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider/httpclient"
	"github.com/valksor/go-mehrhof/internal/provider/token"
)

const (
	defaultBaseURL = "https://www.wrike.com/api/v4"
)

// Config holds client configuration
type Config struct {
	Token string
	Host  string // Optional: override default API base URL
}

// Client wraps the Wrike API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a new Wrike API client
func NewClient(token, host string) *Client {
	baseURL := defaultBaseURL
	if host != "" {
		baseURL = strings.TrimSuffix(host, "/")
	}

	return &Client{
		httpClient: httpclient.NewHTTPClient(),
		baseURL:    baseURL,
		token:      token,
	}
}

// ResolveToken finds the Wrike token from multiple sources.
// Priority order:
//  1. MEHR_WRIKE_TOKEN env var
//  2. WRIKE_TOKEN env var
//  3. configToken (from config.yaml)
func ResolveToken(configToken string) (string, error) {
	return token.ResolveToken(token.Config("WRIKE", configToken).
		WithEnvVars("WRIKE_TOKEN"))
}

// doRequest performs an HTTP request to the Wrike API
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, result any) error {
	reqURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wrapAPIError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return wrapAPIError(httpclient.NewHTTPError(resp.StatusCode, string(respBody)))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// doRequestWithRetry performs an HTTP request with exponential backoff retry.
// Retries on rate limit errors (429) and service unavailable (503).
func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body io.Reader, result any) error {
	return httpclient.WithRetry(ctx, httpclient.DefaultRetryConfig(), func() error {
		return c.doRequest(ctx, method, path, body, result)
	})
}

// GetTask fetches a task by ID
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	var response taskResponse
	if err := c.doRequestWithRetry(ctx, http.MethodGet, "/tasks/"+url.PathEscape(taskID), nil, &response); err != nil {
		return nil, err
	}

	if len(response.Data) == 0 {
		return nil, ErrTaskNotFound
	}

	return &response.Data[0], nil
}

// GetTaskByPermalink fetches a task by permalink URL
// Extracts the numeric ID from the permalink and uses the standard task endpoint
func (c *Client) GetTaskByPermalink(ctx context.Context, permalink string) (*Task, error) {
	numericID := ExtractNumericID(permalink)
	if numericID == "" {
		return nil, fmt.Errorf("%w: invalid permalink format", ErrInvalidReference)
	}

	// Use the extracted numeric ID directly with standard endpoint
	// The Wrike API v4 doesn't document a ?permalink= query parameter,
	// so we extract the numeric ID and use the standard /tasks/{id} endpoint
	return c.GetTask(ctx, numericID)
}

// GetTasks fetches multiple tasks by IDs
func (c *Client) GetTasks(ctx context.Context, taskIDs []string) ([]Task, error) {
	var response taskResponse
	if err := c.doRequestWithRetry(ctx, http.MethodGet, "/tasks/"+url.PathEscape(strings.Join(taskIDs, ",")), nil, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetTasksInFolder fetches all tasks in a folder
func (c *Client) GetTasksInFolder(ctx context.Context, folderID string) ([]Task, error) {
	var response taskResponse
	if err := c.doRequestWithRetry(ctx, http.MethodGet, "/folders/"+url.PathEscape(folderID)+"/tasks", nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetTasksInSpace fetches all tasks in a space
func (c *Client) GetTasksInSpace(ctx context.Context, spaceID string) ([]Task, error) {
	var response taskResponse
	if err := c.doRequestWithRetry(ctx, http.MethodGet, "/spaces/"+url.PathEscape(spaceID)+"/tasks", nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetComments fetches comments for a task with pagination support
func (c *Client) GetComments(ctx context.Context, taskID string) ([]Comment, error) {
	var allComments []Comment
	path := "/tasks/" + url.PathEscape(taskID) + "/comments"

	for {
		var response commentsResponse
		if err := c.doRequestWithRetry(ctx, http.MethodGet, path, nil, &response); err != nil {
			if len(allComments) == 0 {
				return nil, err
			}
			break // Return what we have on error after first page
		}

		allComments = append(allComments, response.Data...)

		// Check if there's a next page
		if response.NextPage == "" {
			break
		}
		path = response.NextPage
	}

	return allComments, nil
}

// GetAttachments fetches attachments for a task
func (c *Client) GetAttachments(ctx context.Context, taskID string) ([]Attachment, error) {
	var response attachmentsResponse
	if err := c.doRequestWithRetry(ctx, http.MethodGet, "/tasks/"+url.PathEscape(taskID)+"/attachments", nil, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// DownloadAttachment downloads an attachment by ID
func (c *Client) DownloadAttachment(ctx context.Context, attachmentID string) (io.ReadCloser, string, error) {
	reqURL := c.baseURL + "/attachments/" + url.PathEscape(attachmentID) + "/download"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", wrapAPIError(err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		return nil, "", wrapAPIError(httpclient.NewHTTPError(resp.StatusCode, "download failed"))
	}

	return resp.Body, resp.Header.Get("Content-Disposition"), nil
}

// PostComment adds a comment to a task
func (c *Client) PostComment(ctx context.Context, taskID, text string) (*Comment, error) {
	requestBody := map[string]string{
		"text": text,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	var response commentResponse
	if err := c.doRequestWithRetry(ctx, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/comments",
		strings.NewReader(string(bodyBytes)), &response); err != nil {
		return nil, err
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no comment returned")
	}

	return &response.Data[0], nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Wrike API Types
// ──────────────────────────────────────────────────────────────────────────────

// Task represents a Wrike task
type Task struct {
	CreatedDate time.Time `json:"createdDate"`
	UpdatedDate time.Time `json:"updatedDate"`
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Permalink   string    `json:"permalink"`
	SubTaskIDs  []string  `json:"subTaskIds"`
}

// Comment represents a Wrike comment
type Comment struct {
	CreatedDate time.Time `json:"createdDate"`
	UpdatedDate time.Time `json:"updatedDate"`
	ID          string    `json:"id"`
	Text        string    `json:"text"`
	AuthorID    string    `json:"authorId"`
	AuthorName  string    `json:"authorName,omitempty"`
}

// Attachment represents a Wrike attachment
type Attachment struct {
	CreatedDate time.Time `json:"createdDate"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size,omitempty"`
}

// Response wrappers for Wrike API
type taskResponse struct {
	Data []Task `json:"data"`
}

type commentsResponse struct {
	NextPage string    `json:"nextPage,omitempty"`
	Data     []Comment `json:"data"`
}

type commentResponse struct {
	Data []Comment `json:"data"`
}

type attachmentsResponse struct {
	Data []Attachment `json:"data"`
}
