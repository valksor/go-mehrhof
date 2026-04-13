package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// JiraClient is an HTTP client wrapper for the Jira REST API v3.
type JiraClient struct {
	baseURL    string
	email      string
	token      string
	httpClient *http.Client
}

// NewJiraClient creates a new Jira API client.
// baseURL should be the Jira instance URL (e.g., "https://yoursite.atlassian.net").
// Auth uses basic auth with email:token.
func NewJiraClient(baseURL, email, token string) *JiraClient {
	return &JiraClient{
		baseURL:    baseURL,
		email:      email,
		token:      token,
		httpClient: httpClient,
	}
}

// --- internal types for Jira REST API responses ---

type jiraIssue struct {
	ID     string          `json:"id"`
	Key    string          `json:"key"`
	Fields jiraIssueFields `json:"fields"`
}

type jiraIssueFields struct {
	Summary     string           `json:"summary"`
	Description any              `json:"description"` // Can be ADF (map) or plain string
	Status      *jiraStatus      `json:"status"`
	Priority    *jiraPriority    `json:"priority"`
	IssueType   *jiraIssueType   `json:"issuetype"`
	Labels      []string         `json:"labels"`
	Parent      *jiraParentField `json:"parent"`
	Subtasks    []jiraIssue      `json:"subtasks"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraPriority struct {
	Name string `json:"name"`
}

type jiraIssueType struct {
	Name string `json:"name"`
}

type jiraParentField struct {
	ID     string          `json:"id"`
	Key    string          `json:"key"`
	Fields jiraIssueFields `json:"fields"`
}

type jiraTransition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetIssue fetches a Jira issue by key (e.g., "PROJ-123").
func (c *JiraClient) GetIssue(ctx context.Context, key string) (*jiraIssue, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: create request: %w", err)
	}

	c.setAuth(req)

	slog.Debug("jira: fetching issue", "key", key)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("jira: get issue: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("jira: get issue %s: status %d - %s", key, resp.StatusCode, string(body))
	}

	var issue jiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("jira: decode issue: %w", err)
	}

	return &issue, nil
}

// AddComment posts a comment on a Jira issue.
func (c *JiraClient) AddComment(ctx context.Context, key, body string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", c.baseURL, key)

	// Jira REST API v3 expects ADF format for comment body
	commentPayload := map[string]any{
		"body": map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []map[string]any{
				{
					"type": "paragraph",
					"content": []map[string]any{
						{
							"type": "text",
							"text": body,
						},
					},
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(commentPayload)
	if err != nil {
		return fmt.Errorf("jira: marshal comment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("jira: create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("jira: adding comment", "key", key)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return fmt.Errorf("jira: add comment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("jira: add comment on %s: status %d - %s", key, resp.StatusCode, string(respBody))
	}

	return nil
}

// TransitionIssue executes a status transition on a Jira issue.
func (c *JiraClient) TransitionIssue(ctx context.Context, key string, transitionID string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, key)

	payload := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jira: marshal transition: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("jira: create request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("jira: transitioning issue", "key", key, "transition_id", transitionID)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return fmt.Errorf("jira: transition issue: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Jira returns 204 No Content on successful transition
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("jira: transition %s with id %s: status %d - %s", key, transitionID, resp.StatusCode, string(respBody))
	}

	return nil
}

// GetIssueTransitions fetches available transitions for a Jira issue.
func (c *JiraClient) GetIssueTransitions(ctx context.Context, key string) ([]jiraTransition, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: create request: %w", err)
	}

	c.setAuth(req)

	slog.Debug("jira: fetching transitions", "key", key)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("jira: get transitions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("jira: get transitions for %s: status %d - %s", key, resp.StatusCode, string(body))
	}

	var result struct {
		Transitions []jiraTransition `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: decode transitions: %w", err)
	}

	return result.Transitions, nil
}

// setAuth sets Basic auth headers on the request.
func (c *JiraClient) setAuth(req *http.Request) {
	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")
}
