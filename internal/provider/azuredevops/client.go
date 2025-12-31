package azuredevops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	apiVersion     = "7.1"
)

// Client wraps the Azure DevOps API
type Client struct {
	httpClient   *http.Client
	organization string
	project      string
	token        string
}

// NewClient creates a new Azure DevOps API client
func NewClient(organization, project, token string) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: defaultTimeout},
		organization: organization,
		project:      project,
		token:        token,
	}
}

// ResolveToken finds Azure DevOps token from multiple sources
// Priority:
//  1. MEHR_AZURE_DEVOPS_TOKEN
//  2. AZURE_DEVOPS_TOKEN
//  3. SYSTEM_ACCESSTOKEN (for Azure Pipelines)
//  4. Config value
func ResolveToken(configToken string) (string, error) {
	if t := os.Getenv("MEHR_AZURE_DEVOPS_TOKEN"); t != "" {
		return t, nil
	}
	if t := os.Getenv("AZURE_DEVOPS_TOKEN"); t != "" {
		return t, nil
	}
	if t := os.Getenv("SYSTEM_ACCESSTOKEN"); t != "" {
		return t, nil
	}
	if configToken != "" {
		return configToken, nil
	}
	return "", ErrNoToken
}

// SetOrganization updates the organization
func (c *Client) SetOrganization(org string) {
	c.organization = org
}

// SetProject updates the project
func (c *Client) SetProject(project string) {
	c.project = project
}

// --- API Types ---

// WorkItem represents an Azure DevOps work item
type WorkItem struct {
	ID        int                `json:"id"`
	Rev       int                `json:"rev"`
	Fields    WorkItemFields     `json:"fields"`
	URL       string             `json:"url"`
	Relations []WorkItemRelation `json:"relations,omitempty"`
}

// WorkItemFields contains work item field values
type WorkItemFields struct {
	Title              string    `json:"System.Title"`
	Description        string    `json:"System.Description"`
	State              string    `json:"System.State"`
	Reason             string    `json:"System.Reason"`
	WorkItemType       string    `json:"System.WorkItemType"`
	AreaPath           string    `json:"System.AreaPath"`
	IterationPath      string    `json:"System.IterationPath"`
	AssignedTo         *Identity `json:"System.AssignedTo"`
	CreatedDate        string    `json:"System.CreatedDate"`
	ChangedDate        string    `json:"System.ChangedDate"`
	CreatedBy          *Identity `json:"System.CreatedBy"`
	ChangedBy          *Identity `json:"System.ChangedBy"`
	Priority           int       `json:"Microsoft.VSTS.Common.Priority"`
	Severity           string    `json:"Microsoft.VSTS.Common.Severity"`
	Tags               string    `json:"System.Tags"`
	CommentCount       int       `json:"System.CommentCount"`
	ReproSteps         string    `json:"Microsoft.VSTS.TCM.ReproSteps"`
	AcceptanceCriteria string    `json:"Microsoft.VSTS.Common.AcceptanceCriteria"`
}

// Identity represents an Azure DevOps user identity
type Identity struct {
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	ID          string `json:"id"`
	UniqueName  string `json:"uniqueName"`
	ImageURL    string `json:"imageUrl"`
}

// WorkItemRelation represents a work item relation
type WorkItemRelation struct {
	Rel        string                 `json:"rel"`
	URL        string                 `json:"url"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Comment represents a work item comment
type Comment struct {
	ID           int       `json:"id"`
	WorkItemID   int       `json:"workItemId"`
	Text         string    `json:"text"`
	CreatedBy    *Identity `json:"createdBy"`
	CreatedDate  string    `json:"createdDate"`
	ModifiedBy   *Identity `json:"modifiedBy"`
	ModifiedDate string    `json:"modifiedDate"`
	Version      int       `json:"version"`
}

// CommentsResponse represents the response from listing comments
type CommentsResponse struct {
	TotalCount int       `json:"totalCount"`
	Count      int       `json:"count"`
	Comments   []Comment `json:"comments"`
}

// WorkItemQueryResult represents a WIQL query result
type WorkItemQueryResult struct {
	QueryType         string              `json:"queryType"`
	QueryResultType   string              `json:"queryResultType"`
	AsOf              string              `json:"asOf"`
	WorkItems         []WorkItemReference `json:"workItems,omitempty"`
	WorkItemRelations []WorkItemRelation  `json:"workItemRelations,omitempty"`
}

// WorkItemReference is a minimal work item reference
type WorkItemReference struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

// WorkItemBatch represents a batch of work items
type WorkItemBatch struct {
	Count int        `json:"count"`
	Value []WorkItem `json:"value"`
}

// AzurePullRequest represents an Azure DevOps pull request
type AzurePullRequest struct {
	PullRequestID int         `json:"pullRequestId"`
	Repository    *Repository `json:"repository"`
	Status        string      `json:"status"`
	CreationDate  string      `json:"creationDate"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	SourceRefName string      `json:"sourceRefName"`
	TargetRefName string      `json:"targetRefName"`
	MergeStatus   string      `json:"mergeStatus"`
	URL           string      `json:"url"`
}

// Repository represents an Azure DevOps repository
type Repository struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Project *Project `json:"project"`
}

// Project represents an Azure DevOps project
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	State       string `json:"state"`
}

// --- HTTP Methods ---

func (c *Client) buildURL(path string) string {
	return fmt.Sprintf("https://dev.azure.com/%s/%s/_apis%s?api-version=%s",
		c.organization, c.project, path, apiVersion)
}

func (c *Client) doRequest(ctx context.Context, method, url string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Azure DevOps uses Basic auth with PAT (empty username, token as password)
	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

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

// --- Work Item API ---

// GetWorkItem fetches a work item by ID
func (c *Client) GetWorkItem(ctx context.Context, id int) (*WorkItem, error) {
	url := c.buildURL(fmt.Sprintf("/wit/workitems/%d", id)) + "&$expand=relations"

	body, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var workItem WorkItem
	if err := json.Unmarshal(body, &workItem); err != nil {
		return nil, fmt.Errorf("unmarshal work item: %w", err)
	}

	return &workItem, nil
}

// QueryWorkItems executes a WIQL query and returns work item IDs
func (c *Client) QueryWorkItems(ctx context.Context, wiql string) ([]int, error) {
	url := c.buildURL("/wit/wiql")

	reqBody := map[string]string{
		"query": wiql,
	}

	body, err := c.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	var result WorkItemQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal query result: %w", err)
	}

	var ids []int
	for _, wi := range result.WorkItems {
		ids = append(ids, wi.ID)
	}

	return ids, nil
}

// GetWorkItems fetches multiple work items by IDs
func (c *Client) GetWorkItems(ctx context.Context, ids []int) ([]WorkItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build comma-separated ID list
	var idStrs []string
	for _, id := range ids {
		idStrs = append(idStrs, fmt.Sprintf("%d", id))
	}

	url := c.buildURL("/wit/workitems")
	// Add ids to query string
	url = url + "&ids=" + joinStrings(idStrs, ",")

	body, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var batch WorkItemBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		return nil, fmt.Errorf("unmarshal work items: %w", err)
	}

	return batch.Value, nil
}

// GetWorkItemComments fetches comments for a work item
func (c *Client) GetWorkItemComments(ctx context.Context, id int) ([]Comment, error) {
	url := c.buildURL(fmt.Sprintf("/wit/workitems/%d/comments", id))

	body, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var resp CommentsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal comments: %w", err)
	}

	return resp.Comments, nil
}

// AddWorkItemComment adds a comment to a work item
func (c *Client) AddWorkItemComment(ctx context.Context, id int, text string) (*Comment, error) {
	url := c.buildURL(fmt.Sprintf("/wit/workitems/%d/comments", id))

	reqBody := map[string]string{
		"text": text,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := json.Unmarshal(respBody, &comment); err != nil {
		return nil, fmt.Errorf("unmarshal comment: %w", err)
	}

	return &comment, nil
}

// UpdateWorkItem updates work item fields using JSON Patch
func (c *Client) UpdateWorkItem(ctx context.Context, id int, updates []PatchOperation) (*WorkItem, error) {
	url := c.buildURL(fmt.Sprintf("/wit/workitems/%d", id))

	// Create the request manually to set the correct content type
	jsonBody, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json-patch+json")

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

	var workItem WorkItem
	if err := json.Unmarshal(respBody, &workItem); err != nil {
		return nil, fmt.Errorf("unmarshal work item: %w", err)
	}

	return &workItem, nil
}

// PatchOperation represents a JSON Patch operation
type PatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
	From  string `json:"from,omitempty"`
}

// UpdateWorkItemState updates the state of a work item
func (c *Client) UpdateWorkItemState(ctx context.Context, id int, state string) (*WorkItem, error) {
	updates := []PatchOperation{
		{
			Op:    "add",
			Path:  "/fields/System.State",
			Value: state,
		},
	}
	return c.UpdateWorkItem(ctx, id, updates)
}

// --- Pull Request API ---

// CreatePullRequest creates a new pull request
func (c *Client) CreatePullRequest(ctx context.Context, repoID, sourceBranch, targetBranch, title, description string, workItemIDs []int) (*AzurePullRequest, error) {
	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories/%s/pullrequests?api-version=%s",
		c.organization, c.project, repoID, apiVersion)

	reqBody := map[string]any{
		"sourceRefName": "refs/heads/" + sourceBranch,
		"targetRefName": "refs/heads/" + targetBranch,
		"title":         title,
		"description":   description,
	}

	// Link work items if provided
	if len(workItemIDs) > 0 {
		var workItemRefs []map[string]any
		for _, id := range workItemIDs {
			workItemRefs = append(workItemRefs, map[string]any{
				"id":  fmt.Sprintf("%d", id),
				"url": fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/wit/workitems/%d", c.organization, c.project, id),
			})
		}
		reqBody["workItemRefs"] = workItemRefs
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	var pr AzurePullRequest
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return nil, fmt.Errorf("unmarshal pull request: %w", err)
	}

	return &pr, nil
}

// GetRepositories lists repositories in the project
func (c *Client) GetRepositories(ctx context.Context) ([]Repository, error) {
	url := c.buildURL("/git/repositories")

	body, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Value []Repository `json:"value"`
		Count int          `json:"count"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal repositories: %w", err)
	}

	return resp.Value, nil
}

// CreateWorkItem creates a new work item of the specified type
func (c *Client) CreateWorkItem(ctx context.Context, workItemType string, updates []PatchOperation) (*WorkItem, error) {
	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/wit/workitems/$%s?api-version=%s",
		c.organization, c.project, workItemType, apiVersion)

	// Create the request manually to set the correct content type
	jsonBody, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json-patch+json")

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

	var workItem WorkItem
	if err := json.Unmarshal(respBody, &workItem); err != nil {
		return nil, fmt.Errorf("unmarshal work item: %w", err)
	}

	return &workItem, nil
}

// QueryWorkItemLinks executes a WIQL link query and returns target work item IDs
// This is used for tree/hierarchy queries (e.g., fetching child work items)
func (c *Client) QueryWorkItemLinks(ctx context.Context, wiql string) ([]int, error) {
	url := c.buildURL("/wit/wiql")

	reqBody := map[string]string{
		"query": wiql,
	}

	body, err := c.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	var result WorkItemLinkQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal query result: %w", err)
	}

	// Extract target IDs (children) from relations
	var ids []int
	for _, rel := range result.WorkItemRelations {
		// Skip the source item (target is the child)
		if rel.Target != nil && rel.Target.ID > 0 {
			ids = append(ids, rel.Target.ID)
		}
	}

	return ids, nil
}

// WorkItemLinkQueryResult represents a WIQL link query result
type WorkItemLinkQueryResult struct {
	QueryType         string                 `json:"queryType"`
	QueryResultType   string                 `json:"queryResultType"`
	AsOf              string                 `json:"asOf"`
	WorkItemRelations []WorkItemLinkRelation `json:"workItemRelations,omitempty"`
}

// WorkItemLinkRelation represents a link relation in query results
type WorkItemLinkRelation struct {
	Source *WorkItemReference `json:"source"`
	Target *WorkItemReference `json:"target"`
	Rel    string             `json:"rel"`
}

// Helper function
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
