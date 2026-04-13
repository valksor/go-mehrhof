package provider

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// azureDevOpsClient wraps HTTP calls to the Azure DevOps REST API v7.0.
type azureDevOpsClient struct {
	httpClient *http.Client
	baseURL    string
	org        string
	project    string
	token      string
}

func newAzureDevOpsClient(baseURL, org, project, token string) *azureDevOpsClient {
	baseURL = strings.TrimRight(cmp.Or(baseURL, "https://dev.azure.com"), "/")

	return &azureDevOpsClient{
		httpClient: &http.Client{},
		baseURL:    baseURL,
		org:        org,
		project:    project,
		token:      token,
	}
}

// azureWorkItem represents an Azure DevOps work item from the REST API.
type azureWorkItem struct {
	ID     int            `json:"id"`
	Rev    int            `json:"rev"`
	Fields map[string]any `json:"fields"`
	URL    string         `json:"url"`
	Links  map[string]any `json:"_links"`
}

// azurePR represents an Azure DevOps pull request.
type azurePR struct {
	PullRequestID int    `json:"pullRequestId"`
	Status        string `json:"status"`
	URL           string `json:"url"`
	Title         string `json:"title"`
}

// GetWorkItem fetches a work item by ID.
func (c *azureDevOpsClient) GetWorkItem(ctx context.Context, id string) (*azureWorkItem, error) {
	url := fmt.Sprintf("%s/%s/%s/_apis/wit/workitems/%s?api-version=7.0&$expand=relations",
		c.baseURL, c.org, c.project, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("azuredevops: create request: %w", err)
	}
	c.setAuth(req)

	slog.Debug("azuredevops: fetching work item", "id", id)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("azuredevops: get work item: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("azuredevops: get work item %s: status %d - %s", id, resp.StatusCode, string(body))
	}

	var wi azureWorkItem
	if err := json.NewDecoder(resp.Body).Decode(&wi); err != nil {
		return nil, fmt.Errorf("azuredevops: decode work item: %w", err)
	}

	return &wi, nil
}

// UpdateWorkItemState updates the state field of a work item using a JSON Patch operation.
func (c *azureDevOpsClient) UpdateWorkItemState(ctx context.Context, id string, state string) error {
	url := fmt.Sprintf("%s/%s/%s/_apis/wit/workitems/%s?api-version=7.0",
		c.baseURL, c.org, c.project, id)

	patch := []map[string]any{
		{
			"op":    "replace",
			"path":  "/fields/System.State",
			"value": state,
		},
	}
	payloadBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("azuredevops: marshal patch: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("azuredevops: create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json-patch+json")

	slog.Debug("azuredevops: updating work item state", "id", id, "state", state)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return fmt.Errorf("azuredevops: update state: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("azuredevops: update work item %s state: status %d - %s", id, resp.StatusCode, string(body))
	}

	return nil
}

// AddWorkItemComment adds a comment to a work item.
func (c *azureDevOpsClient) AddWorkItemComment(ctx context.Context, id string, text string) error {
	url := fmt.Sprintf("%s/%s/%s/_apis/wit/workitems/%s/comments?api-version=7.0-preview.4",
		c.baseURL, c.org, c.project, id)

	payload := map[string]string{"text": text}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("azuredevops: marshal comment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("azuredevops: create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("azuredevops: adding comment", "id", id)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return fmt.Errorf("azuredevops: add comment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("azuredevops: add comment on %s: status %d - %s", id, resp.StatusCode, string(body))
	}

	return nil
}

// CreatePullRequest creates a pull request in the default repository of the project.
func (c *azureDevOpsClient) CreatePullRequest(ctx context.Context, repoName string, opts PROptions) (*azurePR, error) {
	url := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/pullrequests?api-version=7.0",
		c.baseURL, c.org, c.project, repoName)

	payload := map[string]any{
		"sourceRefName": "refs/heads/" + opts.Head,
		"targetRefName": "refs/heads/" + opts.Base,
		"title":         opts.Title,
		"description":   opts.Body,
		"isDraft":       opts.Draft,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("azuredevops: marshal PR: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("azuredevops: create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("azuredevops: creating pull request", "repo", repoName, "branch", opts.Head)

	resp, err := DoWithRetry(c.httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("azuredevops: create PR: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("azuredevops: create PR: status %d - %s", resp.StatusCode, string(body))
	}

	var pr azurePR
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("azuredevops: decode PR response: %w", err)
	}

	return &pr, nil
}

func (c *azureDevOpsClient) setAuth(req *http.Request) {
	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")
}

// workItemWebURL constructs the browser URL for a work item.
func (c *azureDevOpsClient) workItemWebURL(id int) string {
	return fmt.Sprintf("%s/%s/%s/_workitems/edit/%d", c.baseURL, c.org, c.project, id)
}
