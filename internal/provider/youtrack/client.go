package youtrack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider/token"
)

const (
	defaultBaseURL = "https://youtrack.cloud/api"
	defaultTimeout = 30 * time.Second
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// Config holds client configuration
type Config struct {
	Token string
	Host  string // Optional: override default API base URL
}

// Client wraps the YouTrack API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a new YouTrack API client
func NewClient(token, host string) *Client {
	baseURL := defaultBaseURL
	if host != "" {
		// Ensure proper API path structure
		host = strings.TrimSuffix(host, "/")
		if !strings.HasSuffix(host, "/api") {
			// If host includes /youtrack, replace it with /api
			if strings.Contains(host, "/youtrack") {
				baseURL = strings.Replace(host, "/youtrack", "/api", 1)
			} else {
				baseURL = host + "/api"
			}
		} else {
			baseURL = host
		}
	}

	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    baseURL,
		token:      token,
	}
}

// ResolveToken finds the YouTrack token from multiple sources.
// Priority order:
//  1. MEHR_YOUTRACK_TOKEN env var
//  2. YOUTRACK_TOKEN env var
//  3. configToken (from config.yaml)
func ResolveToken(configToken string) (string, error) {
	return token.ResolveToken(token.Config("YOUTRACK", configToken).
		WithEnvVars("YOUTRACK_TOKEN"))
}

// GetIssue fetches an issue by readable ID (e.g., "ABC-123")
func (c *Client) GetIssue(ctx context.Context, issueID string) (*Issue, error) {
	fields := url.QueryEscape("id,idReadable,summary,description,created,updated,resolved," +
		"project(id,name,shortName),reporter(id,login,name,email)," +
		"updater(id,login,name),customFields(name,value,$type)," +
		tagsField + ",commentsCount,subtasks(id,idReadable),parent(id,idReadable)")

	var response issueResponse
	err := c.doRequestWithRetry(ctx, http.MethodGet,
		"/issues/"+url.PathEscape(issueID)+"?fields="+fields, nil, &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetIssuesByQuery fetches issues matching a query
func (c *Client) GetIssuesByQuery(ctx context.Context, query string, top, skip int) ([]Issue, error) {
	path := "/issues"
	params := url.Values{}
	params.Add("fields", "id,idReadable,summary,description,created,updated,project(id,name,shortName),"+
		"reporter(id,login,name),customFields(name,value),"+tagsField+",commentsCount")
	if query != "" {
		params.Add("query", query)
	}
	if top > 0 {
		params.Add("$top", fmt.Sprintf("%d", top))
	}
	if skip > 0 {
		params.Add("$skip", fmt.Sprintf("%d", skip))
	}

	var response issuesResponse
	err := c.doRequestWithRetry(ctx, http.MethodGet,
		path+"?"+params.Encode(), nil, &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetComments fetches comments for an issue
func (c *Client) GetComments(ctx context.Context, issueID string) ([]Comment, error) {
	fields := url.QueryEscape("id,text,author(id,login,name),created,updated,deleted")
	var response commentsResponse
	err := c.doRequestWithRetry(ctx, http.MethodGet,
		"/issues/"+url.PathEscape(issueID)+"/comments?fields="+fields, nil, &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// AddComment adds a comment to an issue
func (c *Client) AddComment(ctx context.Context, issueID, text string) (*Comment, error) {
	requestBody := map[string]string{"text": text}
	bodyBytes, _ := json.Marshal(requestBody)

	fields := url.QueryEscape("id,text,author(id,login,name),created")
	var response commentsResponse
	err := c.doRequestWithRetry(ctx, http.MethodPost,
		"/issues/"+url.PathEscape(issueID)+"/comments?fields="+fields,
		bytesReader(bodyBytes), &response)
	if err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no comment returned")
	}
	return &response.Data[0], nil
}

// GetTags fetches tags for an issue
func (c *Client) GetTags(ctx context.Context, issueID string) ([]Tag, error) {
	fields := url.QueryEscape("id,name")
	var response tagsResponse
	err := c.doRequestWithRetry(ctx, http.MethodGet,
		"/issues/"+url.PathEscape(issueID)+"/tags?fields="+fields, nil, &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// AddTag adds a tag to an issue by tag name (YouTrack creates tag if it doesn't exist)
func (c *Client) AddTag(ctx context.Context, issueID, tagName string) (*Tag, error) {
	requestBody := map[string]string{"name": tagName}
	bodyBytes, _ := json.Marshal(requestBody)

	fields := url.QueryEscape("id,name")
	var response tagsResponse
	err := c.doRequestWithRetry(ctx, http.MethodPost,
		"/issues/"+url.PathEscape(issueID)+"/tags?fields="+fields,
		bytesReader(bodyBytes), &response)
	if err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no tag returned")
	}
	return &response.Data[0], nil
}

// RemoveTag removes a tag from an issue by tag ID
func (c *Client) RemoveTag(ctx context.Context, issueID, tagID string) error {
	return c.doRequestWithRetry(ctx, http.MethodDelete,
		"/issues/"+url.PathEscape(issueID)+"/tags/"+url.PathEscape(tagID), nil, nil)
}

// UpdateIssue updates an issue
func (c *Client) UpdateIssue(ctx context.Context, issueID string, updates map[string]interface{}) (*Issue, error) {
	bodyBytes, _ := json.Marshal(updates)
	fields := url.QueryEscape("id,idReadable,summary,customFields(name,value)")

	var response issueResponse
	err := c.doRequestWithRetry(ctx, http.MethodPost,
		"/issues/"+url.PathEscape(issueID)+"?fields="+fields,
		bytesReader(bodyBytes), &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// CreateIssue creates a new issue
func (c *Client) CreateIssue(ctx context.Context, projectID, summary, description string, customFields []map[string]interface{}) (*Issue, error) {
	requestBody := map[string]interface{}{
		"project": map[string]string{"id": projectID},
		"summary": summary,
	}
	if description != "" {
		requestBody["description"] = description
	}
	if len(customFields) > 0 {
		requestBody["customFields"] = customFields
	}

	bodyBytes, _ := json.Marshal(requestBody)
	fields := url.QueryEscape("id,idReadable,summary,description,created,project(id,name)")

	var response issueResponse
	err := c.doRequestWithRetry(ctx, http.MethodPost,
		"/issues?fields="+fields,
		bytesReader(bodyBytes), &response)
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetAttachments fetches attachments for an issue
func (c *Client) GetAttachments(ctx context.Context, issueID string) ([]Attachment, error) {
	fields := url.QueryEscape("attachments(id,name,created,size,mimeType,url)")
	var response issueResponse
	err := c.doRequestWithRetry(ctx, http.MethodGet,
		"/issues/"+url.PathEscape(issueID)+"?fields="+fields, nil, &response)
	if err != nil {
		return nil, err
	}
	return response.Data.Attachments, nil
}

// DownloadAttachment downloads an attachment
func (c *Client) DownloadAttachment(ctx context.Context, attachmentID string) (io.ReadCloser, string, error) {
	reqURL := c.baseURL + "/attachments/" + url.PathEscape(attachmentID) + "/content"

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
		return nil, "", wrapAPIError(newHTTPError(resp.StatusCode, ""))
	}

	return resp.Body, resp.Header.Get("Content-Disposition"), nil
}

// doRequest performs an HTTP request
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return wrapAPIError(newHTTPError(resp.StatusCode, string(respBody)))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// doRequestWithRetry performs request with exponential backoff
func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body io.Reader, result any) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := c.doRequest(ctx, method, path, body, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if retryable
		var httpErr *httpError
		if !errors.As(err, &httpErr) {
			return err
		}

		// Retry on rate limit (429), service unavailable (503), or network errors
		isRetryable := httpErr.code == http.StatusTooManyRequests ||
			httpErr.code == http.StatusServiceUnavailable ||
			errors.Is(err, ErrNetworkError)

		if !isRetryable || attempt == maxRetries {
			return err
		}

		// Exponential backoff
		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// bytesReader creates an io.Reader from a byte slice
func bytesReader(b []byte) io.Reader {
	return strings.NewReader(string(b))
}

// tagsField is the field specification for tags in API requests
const tagsField = "tags(id,name)"
