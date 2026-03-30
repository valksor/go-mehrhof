package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	linearAPIURL      = "https://api.linear.app/graphql"
	maxLinearSiblings = 5
)

// linearAllowedAttachmentHosts defines hosts from which attachments can be downloaded.
// Linear stores attachments on these CDN domains.
var linearAllowedAttachmentHosts = map[string]bool{
	"uploads.linear.app": true,
	"cdn.linear.app":     true,
}

// linearAllowedGCSPrefixes defines allowed GCS bucket path prefixes for Linear attachments.
var linearAllowedGCSPrefixes = []string{
	"/uploads.linear.app",
	"/public.linear.app",
	"/imports.linear.app",
	"/linear-uploads-europe-west1",
	"/linear-imports-europe-west1",
}

// isAllowedLinearAttachmentURL validates that a URL is from an allowed Linear attachment host.
func isAllowedLinearAttachmentURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())

	// Direct Linear CDN hosts
	if linearAllowedAttachmentHosts[host] {
		return nil
	}

	// Google Cloud Storage with allowed prefixes
	if host == "storage.googleapis.com" {
		for _, prefix := range linearAllowedGCSPrefixes {
			if strings.HasPrefix(parsed.Path, prefix) {
				return nil
			}
		}
	}

	return fmt.Errorf("attachment host not allowed: %s", host)
}

// LinearProvider implements Provider, HierarchyProvider, CommentProvider,
// LabelProvider, ListProvider, CreateProvider, and AttachmentProvider
// for Linear.app issues.
type LinearProvider struct {
	token string
	team  string // default team key (optional)
}

// NewLinearProvider creates a new Linear provider.
// Token should come from Settings (settings.Providers.Linear.Token).
func NewLinearProvider(token, team string) *LinearProvider {
	return &LinearProvider{
		token: token,
		team:  team,
	}
}

func (p *LinearProvider) Name() string {
	return NameLinear
}

// --- internal types for GraphQL responses ---

type linearIssue struct {
	ID          string          `json:"id"`
	Identifier  string          `json:"identifier"` // "ENG-123"
	Title       string          `json:"title"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	Priority    int             `json:"priority"`
	State       *linearState    `json:"state"`
	Team        *linearTeam     `json:"team"`
	Parent      *linearParent   `json:"parent"`
	Labels      *linearLabels   `json:"labels"`
	Assignee    *linearUser     `json:"assignee"`
	Children    *linearChildren `json:"children"`
}

type linearState struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // backlog, unstarted, started, completed, canceled
}

type linearTeam struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type linearParent struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
}

type linearLabels struct {
	Nodes []linearLabel `json:"nodes"`
}

type linearLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearChildren struct {
	Nodes []linearIssue `json:"nodes"`
}

type linearComment struct {
	ID        string      `json:"id"`
	Body      string      `json:"body"`
	User      *linearUser `json:"user"`
	CreatedAt string      `json:"createdAt"`
}

type linearComments struct {
	Nodes []linearComment `json:"nodes"`
}

// --- Provider interface ---

// FetchTask fetches an issue from Linear by identifier (e.g., "ENG-123").
func (p *LinearProvider) FetchTask(ctx context.Context, id string) (*Task, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	issue, err := p.fetchIssueByIdentifier(ctx, id)
	if err != nil {
		return nil, err
	}

	return p.issueToTask(issue), nil
}

// UpdateStatus updates the status of a Linear issue.
// Maps generic statuses to Linear workflow states.
func (p *LinearProvider) UpdateStatus(ctx context.Context, id string, status string) error {
	if p.token == "" {
		return errors.New("LINEAR_TOKEN not set")
	}

	// First fetch the issue to get its team
	issue, err := p.fetchIssueByIdentifier(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch issue: %w", err)
	}

	if issue.Team == nil {
		return errors.New("issue has no team")
	}

	// Find a matching workflow state
	stateID, err := p.findWorkflowState(ctx, issue.Team.ID, status)
	if err != nil {
		return fmt.Errorf("find workflow state: %w", err)
	}

	// Update the issue
	mutation := `
		mutation IssueUpdate($issueId: String!, $stateId: String!) {
			issueUpdate(id: $issueId, input: { stateId: $stateId }) {
				success
			}
		}
	`

	var result struct {
		Data struct {
			IssueUpdate struct {
				Success bool `json:"success"`
			} `json:"issueUpdate"`
		} `json:"data"`
	}

	err = p.graphql(ctx, mutation, map[string]any{
		"issueId": issue.ID,
		"stateId": stateID,
	}, &result)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	if !result.Data.IssueUpdate.Success {
		return errors.New("linear api: update failed")
	}

	return nil
}

// --- HierarchyProvider interface ---

// FetchParent returns the parent issue if this is a sub-issue.
func (p *LinearProvider) FetchParent(ctx context.Context, task *Task) (*Task, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	parentID := task.Metadata("linear_parent_id")
	if parentID == "" {
		return nil, nil //nolint:nilnil // nil, nil signals "no parent" (not an error)
	}

	// Fetch by internal ID, not identifier
	issue, err := p.fetchIssueByID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("fetch parent: %w", err)
	}

	return p.issueToTask(issue), nil
}

// FetchSiblings returns sibling issues (children of the same parent).
func (p *LinearProvider) FetchSiblings(ctx context.Context, task *Task) ([]*Task, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	parentID := task.Metadata("linear_parent_id")
	if parentID == "" {
		return nil, nil
	}

	// Fetch parent's children
	query := `
		query IssueChildren($id: String!) {
			issue(id: $id) {
				children(first: 10) {
					nodes {
						id
						identifier
						title
						description
						url
						priority
						state { id name type }
						team { id key }
						labels { nodes { id name } }
						assignee { id name }
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Issue struct {
				Children linearChildren `json:"children"`
			} `json:"issue"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"id": parentID}, &result)
	if err != nil {
		return nil, fmt.Errorf("fetch siblings: %w", err)
	}

	siblings := make([]*Task, 0, maxLinearSiblings)
	for _, child := range result.Data.Issue.Children.Nodes {
		if child.ID == task.Metadata("linear_id") {
			continue // Skip self
		}
		siblings = append(siblings, p.issueToTask(&child))
		if len(siblings) >= maxLinearSiblings {
			break
		}
	}

	return siblings, nil
}

// --- CommentProvider interface ---

// FetchComments returns comments on the issue.
func (p *LinearProvider) FetchComments(ctx context.Context, id string) ([]Comment, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	issue, err := p.fetchIssueByIdentifier(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch issue: %w", err)
	}

	query := `
		query IssueComments($id: String!) {
			issue(id: $id) {
				comments(first: 50) {
					nodes {
						id
						body
						user { id name }
						createdAt
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Issue struct {
				Comments linearComments `json:"comments"`
			} `json:"issue"`
		} `json:"data"`
	}

	err = p.graphql(ctx, query, map[string]any{"id": issue.ID}, &result)
	if err != nil {
		return nil, fmt.Errorf("fetch comments: %w", err)
	}

	comments := make([]Comment, 0, len(result.Data.Issue.Comments.Nodes))
	for _, c := range result.Data.Issue.Comments.Nodes {
		author := ""
		if c.User != nil {
			author = c.User.Name
		}
		comments = append(comments, Comment{
			ID:        c.ID,
			Body:      c.Body,
			Author:    author,
			CreatedAt: c.CreatedAt,
		})
	}

	return comments, nil
}

// AddComment adds a comment to an issue.
func (p *LinearProvider) AddComment(ctx context.Context, id string, body string) error {
	if p.token == "" {
		return errors.New("LINEAR_TOKEN not set")
	}

	issue, err := p.fetchIssueByIdentifier(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch issue: %w", err)
	}

	mutation := `
		mutation CreateComment($issueId: String!, $body: String!) {
			commentCreate(input: { issueId: $issueId, body: $body }) {
				success
			}
		}
	`

	var result struct {
		Data struct {
			CommentCreate struct {
				Success bool `json:"success"`
			} `json:"commentCreate"`
		} `json:"data"`
	}

	err = p.graphql(ctx, mutation, map[string]any{
		"issueId": issue.ID,
		"body":    body,
	}, &result)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	if !result.Data.CommentCreate.Success {
		return errors.New("linear api: create comment failed")
	}

	return nil
}

// --- LabelProvider interface ---

// AddLabels adds labels to an issue.
func (p *LinearProvider) AddLabels(ctx context.Context, id string, labels []string) error {
	if p.token == "" {
		return errors.New("LINEAR_TOKEN not set")
	}

	issue, err := p.fetchIssueByIdentifier(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch issue: %w", err)
	}
	if issue.Team == nil {
		return errors.New("issue has no team assigned")
	}

	// Get label IDs by name
	labelIDs, err := p.getLabelIDs(ctx, issue.Team.ID, labels)
	if err != nil {
		return fmt.Errorf("get label ids: %w", err)
	}

	// Get existing label IDs into a set for deduplication
	labelSet := make(map[string]struct{})
	if issue.Labels != nil {
		for _, l := range issue.Labels.Nodes {
			labelSet[l.ID] = struct{}{}
		}
	}
	// Add new labels (deduplicates automatically)
	for _, id := range labelIDs {
		labelSet[id] = struct{}{}
	}
	// Convert set to slice
	allIDs := make([]string, 0, len(labelSet))
	for id := range labelSet {
		allIDs = append(allIDs, id)
	}

	mutation := `
		mutation IssueUpdate($issueId: String!, $labelIds: [String!]!) {
			issueUpdate(id: $issueId, input: { labelIds: $labelIds }) {
				success
			}
		}
	`

	var result struct {
		Data struct {
			IssueUpdate struct {
				Success bool `json:"success"`
			} `json:"issueUpdate"`
		} `json:"data"`
	}

	err = p.graphql(ctx, mutation, map[string]any{
		"issueId":  issue.ID,
		"labelIds": allIDs,
	}, &result)
	if err != nil {
		return fmt.Errorf("add labels: %w", err)
	}
	if !result.Data.IssueUpdate.Success {
		return errors.New("add labels: linear api update failed")
	}

	return nil
}

// RemoveLabels removes labels from an issue.
func (p *LinearProvider) RemoveLabels(ctx context.Context, id string, labels []string) error {
	if p.token == "" {
		return errors.New("LINEAR_TOKEN not set")
	}

	issue, err := p.fetchIssueByIdentifier(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch issue: %w", err)
	}
	if issue.Team == nil {
		return errors.New("issue has no team assigned")
	}

	// Get label IDs to remove
	removeIDs, err := p.getLabelIDs(ctx, issue.Team.ID, labels)
	if err != nil {
		return fmt.Errorf("get label ids: %w", err)
	}

	// Filter out labels to remove
	removeSet := make(map[string]bool)
	for _, id := range removeIDs {
		removeSet[id] = true
	}

	remainingIDs := make([]string, 0)
	if issue.Labels != nil {
		for _, l := range issue.Labels.Nodes {
			if !removeSet[l.ID] {
				remainingIDs = append(remainingIDs, l.ID)
			}
		}
	}

	mutation := `
		mutation IssueUpdate($issueId: String!, $labelIds: [String!]!) {
			issueUpdate(id: $issueId, input: { labelIds: $labelIds }) {
				success
			}
		}
	`

	var result struct {
		Data struct {
			IssueUpdate struct {
				Success bool `json:"success"`
			} `json:"issueUpdate"`
		} `json:"data"`
	}

	err = p.graphql(ctx, mutation, map[string]any{
		"issueId":  issue.ID,
		"labelIds": remainingIDs,
	}, &result)
	if err != nil {
		return fmt.Errorf("remove labels: %w", err)
	}
	if !result.Data.IssueUpdate.Success {
		return errors.New("remove labels: linear api update failed")
	}

	return nil
}

// --- ListProvider interface ---

// ListTasks lists issues from Linear.
func (p *LinearProvider) ListTasks(ctx context.Context, opts ListOptions) (*ListResult, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	limit := opts.Limit
	if limit == 0 {
		limit = 50
	}

	// Build filter
	filter := make(map[string]any)
	if opts.Team != "" {
		filter["team"] = map[string]any{"key": map[string]any{"eq": opts.Team}}
	} else if p.team != "" {
		filter["team"] = map[string]any{"key": map[string]any{"eq": p.team}}
	}
	if opts.Status != "" {
		filter["state"] = map[string]any{"type": map[string]any{"eq": opts.Status}}
	}

	query := `
		query Issues($first: Int!, $after: String, $filter: IssueFilter) {
			issues(first: $first, after: $after, filter: $filter) {
				nodes {
					id
					identifier
					title
					description
					url
					priority
					state { id name type }
					team { id key }
					parent { id identifier }
					labels { nodes { id name } }
					assignee { id name }
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}
	`

	variables := map[string]any{
		"first": limit,
	}
	if opts.Cursor != "" {
		variables["after"] = opts.Cursor
	}
	if len(filter) > 0 {
		variables["filter"] = filter
	}

	var result struct {
		Data struct {
			Issues struct {
				Nodes    []linearIssue `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"issues"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, variables, &result)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	tasks := make([]*Task, 0, len(result.Data.Issues.Nodes))
	for i := range result.Data.Issues.Nodes {
		tasks = append(tasks, p.issueToTask(&result.Data.Issues.Nodes[i]))
	}

	return &ListResult{
		Tasks:      tasks,
		NextCursor: result.Data.Issues.PageInfo.EndCursor,
		HasMore:    result.Data.Issues.PageInfo.HasNextPage,
	}, nil
}

// --- CreateProvider interface ---

// CreateTask creates a new issue in Linear.
func (p *LinearProvider) CreateTask(ctx context.Context, opts CreateTaskOptions) (*Task, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	team := opts.Team
	if team == "" {
		team = p.team
	}
	if team == "" {
		return nil, errors.New("team is required for creating Linear issues")
	}

	// Get team ID from key
	teamID, err := p.getTeamID(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("get team id: %w", err)
	}

	input := map[string]any{
		"teamId":      teamID,
		"title":       opts.Title,
		"description": opts.Description,
	}

	// Map priority
	if opts.Priority != "" {
		input["priority"] = linearPriorityFromString(opts.Priority)
	}

	// Get label IDs
	if len(opts.Labels) > 0 {
		labelIDs, err := p.getLabelIDs(ctx, teamID, opts.Labels)
		if err != nil {
			return nil, fmt.Errorf("get label ids: %w", err)
		}
		input["labelIds"] = labelIDs
	}

	mutation := `
		mutation IssueCreate($input: IssueCreateInput!) {
			issueCreate(input: $input) {
				success
				issue {
					id
					identifier
					title
					description
					url
					priority
					state { id name type }
					team { id key }
					labels { nodes { id name } }
					assignee { id name }
				}
			}
		}
	`

	var result struct {
		Data struct {
			IssueCreate struct {
				Success bool        `json:"success"`
				Issue   linearIssue `json:"issue"`
			} `json:"issueCreate"`
		} `json:"data"`
	}

	err = p.graphql(ctx, mutation, map[string]any{"input": input}, &result)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	if !result.Data.IssueCreate.Success {
		return nil, errors.New("linear api: create issue failed")
	}

	return p.issueToTask(&result.Data.IssueCreate.Issue), nil
}

// --- AttachmentProvider interface ---

// DownloadAttachment downloads an attachment from Linear.
// Linear stores attachments on approved CDN hosts; this validates the URL and adds auth.
func (p *LinearProvider) DownloadAttachment(ctx context.Context, attachmentURL string) ([]byte, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	// Validate URL is from an allowed Linear attachment host
	if err := isAllowedLinearAttachmentURL(attachmentURL); err != nil {
		return nil, fmt.Errorf("validate attachment URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attachmentURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Linear API: Personal API keys (lin_api_*) are used directly without prefix.
	// OAuth tokens should include "Bearer " prefix in the settings configuration.
	req.Header.Set("Authorization", p.token)

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download attachment: status %d", resp.StatusCode)
	}

	// Limit attachment size to prevent OOM on very large files (100MB)
	const maxAttachmentSize = 100 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxAttachmentSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if len(data) > maxAttachmentSize {
		return nil, fmt.Errorf("attachment exceeds max size (%d bytes)", maxAttachmentSize)
	}

	return data, nil
}

// --- internal helpers ---

// graphql executes a GraphQL query against Linear API.
func (p *LinearProvider) graphql(ctx context.Context, query string, variables map[string]any, result any) error {
	payload := map[string]any{
		"query": query,
	}
	if variables != nil {
		payload["variables"] = variables
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPIURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payloadBytes)), nil
	}

	req.Header.Set("Authorization", p.token)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("linear: graphql request")

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		slog.Error("linear: graphql request failed", "error", err)

		return fmt.Errorf("linear api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		slog.Error("linear: graphql error response", "status_code", resp.StatusCode)

		return fmt.Errorf("linear api error: %d - %s", resp.StatusCode, string(body))
	}

	// Check for GraphQL errors
	var graphqlResp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	// Read body into buffer so we can decode twice
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(bodyBytes, &graphqlResp); err == nil && len(graphqlResp.Errors) > 0 {
		slog.Error("linear: graphql api error", "error", graphqlResp.Errors[0].Message)

		return fmt.Errorf("linear api: %s", graphqlResp.Errors[0].Message)
	}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	slog.Debug("linear: graphql request completed")

	return nil
}

// fetchIssueByIdentifier fetches an issue by its identifier (e.g., "ENG-123").
func (p *LinearProvider) fetchIssueByIdentifier(ctx context.Context, identifier string) (*linearIssue, error) {
	// Clean up identifier (remove any leading "linear:" or "ln:")
	identifier = strings.TrimPrefix(identifier, "linear:")
	identifier = strings.TrimPrefix(identifier, "ln:")
	identifier = strings.ToUpper(identifier)

	query := `
		query IssueByIdentifier($filter: IssueFilter!) {
			issues(filter: $filter, first: 1) {
				nodes {
					id
					identifier
					title
					description
					url
					priority
					state { id name type }
					team { id key }
					parent { id identifier }
					labels { nodes { id name } }
					assignee { id name }
					children(first: 10) {
						nodes {
							id
							identifier
							title
							state { id name type }
						}
					}
				}
			}
		}
	`

	// Parse identifier to extract team key and number
	parts := strings.SplitN(identifier, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid linear identifier: %s", identifier)
	}
	issueNumber, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid issue number in identifier %q: %w", identifier, err)
	}

	filter := map[string]any{
		"and": []map[string]any{
			{"team": map[string]any{"key": map[string]any{"eq": parts[0]}}},
			{"number": map[string]any{"eq": issueNumber}},
		},
	}

	var result struct {
		Data struct {
			Issues struct {
				Nodes []linearIssue `json:"nodes"`
			} `json:"issues"`
		} `json:"data"`
	}

	err = p.graphql(ctx, query, map[string]any{"filter": filter}, &result)
	if err != nil {
		return nil, err
	}

	if len(result.Data.Issues.Nodes) == 0 {
		return nil, fmt.Errorf("issue not found: %s", identifier)
	}

	return &result.Data.Issues.Nodes[0], nil
}

// fetchIssueByID fetches an issue by its internal ID.
func (p *LinearProvider) fetchIssueByID(ctx context.Context, id string) (*linearIssue, error) {
	query := `
		query Issue($id: String!) {
			issue(id: $id) {
				id
				identifier
				title
				description
				url
				priority
				state { id name type }
				team { id key }
				parent { id identifier }
				labels { nodes { id name } }
				assignee { id name }
			}
		}
	`

	var result struct {
		Data struct {
			Issue linearIssue `json:"issue"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"id": id}, &result)
	if err != nil {
		return nil, err
	}

	if result.Data.Issue.ID == "" {
		return nil, fmt.Errorf("issue not found: %s", id)
	}

	return &result.Data.Issue, nil
}

// issueToTask converts a Linear issue to a Task.
func (p *LinearProvider) issueToTask(issue *linearIssue) *Task {
	labels := make([]string, 0)
	if issue.Labels != nil {
		for _, l := range issue.Labels.Nodes {
			labels = append(labels, l.Name)
		}
	}

	// Add state as a label
	if issue.State != nil {
		labels = append(labels, issue.State.Name)
	}

	task := &Task{
		ID:          issue.Identifier,
		Title:       issue.Title,
		Description: issue.Description,
		URL:         issue.URL,
		Labels:      labels,
		Source:      NameLinear,
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Override priority from Linear if set
	if issue.Priority > 0 {
		task.Priority = linearPriorityToString(issue.Priority)
	}

	// Subtasks from children
	if issue.Children != nil {
		for i, child := range issue.Children.Nodes {
			completed := false
			if child.State != nil {
				completed = child.State.Type == "completed" || child.State.Type == "canceled"
			}
			task.Subtasks = append(task.Subtasks, &Subtask{
				ID:        child.Identifier,
				Text:      child.Title,
				Completed: completed,
				Index:     i,
			})
		}
	}

	// Dependencies (parsed from description)
	task.Dependencies = p.resolveDependencies(task)

	// Metadata
	task.SetMetadata("linear_id", issue.ID)
	task.SetMetadata("linear_identifier", issue.Identifier)
	if issue.State != nil {
		task.SetMetadata("linear_state_id", issue.State.ID)
		task.SetMetadata("linear_state_type", issue.State.Type)
	}
	if issue.Team != nil {
		task.SetMetadata("linear_team_key", issue.Team.Key)
		task.SetMetadata("linear_team_id", issue.Team.ID)
	}
	if issue.Parent != nil {
		task.SetMetadata("linear_parent_id", issue.Parent.ID)
		task.SetMetadata("linear_parent_identifier", issue.Parent.Identifier)
	}
	if issue.Assignee != nil {
		task.SetMetadata("linear_assignee", issue.Assignee.Name)
	}

	return task
}

// resolveDependencies parses dependency references from description.
func (p *LinearProvider) resolveDependencies(task *Task) []*Task {
	refs := ParseDependencies(task.Description)
	if len(refs) == 0 {
		return nil
	}

	deps := make([]*Task, 0, len(refs))
	for _, ref := range refs {
		depID := ref
		// Handle shorthand refs (e.g., "ENG-123" without prefix)
		if !strings.Contains(ref, "-") {
			continue // Not a valid Linear reference
		}
		deps = append(deps, &Task{
			ID:     depID,
			Source: NameLinear,
		})
	}

	return deps
}

// findWorkflowState finds a workflow state matching the status.
func (p *LinearProvider) findWorkflowState(ctx context.Context, teamID, status string) (string, error) {
	// Map status to Linear state types
	var stateTypes []string
	switch strings.ToLower(status) {
	case stateOpen, "pending", "todo", "backlog":
		stateTypes = []string{"backlog", "unstarted"}
	case "in_progress", "started", "doing":
		stateTypes = []string{"started"}
	case "done", "completed", "closed":
		stateTypes = []string{"completed"}
	case "canceled", "cancelled":
		stateTypes = []string{"canceled"}
	default:
		// Try to find by name
		return p.findWorkflowStateByName(ctx, teamID, status)
	}

	query := `
		query WorkflowStates($teamId: String!) {
			team(id: $teamId) {
				states {
					nodes {
						id
						name
						type
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				States struct {
					Nodes []linearState `json:"nodes"`
				} `json:"states"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"teamId": teamID}, &result)
	if err != nil {
		return "", err
	}

	// Find first matching state type
	for _, stateType := range stateTypes {
		for _, state := range result.Data.Team.States.Nodes {
			if state.Type == stateType {
				return state.ID, nil
			}
		}
	}

	return "", fmt.Errorf("no matching workflow state for: %s", status)
}

// findWorkflowStateByName finds a workflow state by name.
func (p *LinearProvider) findWorkflowStateByName(ctx context.Context, teamID, name string) (string, error) {
	query := `
		query WorkflowStates($teamId: String!) {
			team(id: $teamId) {
				states {
					nodes {
						id
						name
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				States struct {
					Nodes []linearState `json:"nodes"`
				} `json:"states"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"teamId": teamID}, &result)
	if err != nil {
		return "", err
	}

	nameLower := strings.ToLower(name)
	for _, state := range result.Data.Team.States.Nodes {
		if strings.ToLower(state.Name) == nameLower {
			return state.ID, nil
		}
	}

	return "", fmt.Errorf("workflow state not found: %s", name)
}

// getTeamID gets the team ID from a team key.
func (p *LinearProvider) getTeamID(ctx context.Context, key string) (string, error) {
	query := `
		query Team($key: String!) {
			team(key: $key) {
				id
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"key": key}, &result)
	if err != nil {
		return "", err
	}

	if result.Data.Team.ID == "" {
		return "", fmt.Errorf("team not found: %s", key)
	}

	return result.Data.Team.ID, nil
}

// getLabelIDs gets label IDs by name for a team.
func (p *LinearProvider) getLabelIDs(ctx context.Context, teamID string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}

	query := `
		query TeamLabels($teamId: String!) {
			team(id: $teamId) {
				labels {
					nodes {
						id
						name
					}
				}
			}
		}
	`

	var result struct {
		Data struct {
			Team struct {
				Labels struct {
					Nodes []linearLabel `json:"nodes"`
				} `json:"labels"`
			} `json:"team"`
		} `json:"data"`
	}

	err := p.graphql(ctx, query, map[string]any{"teamId": teamID}, &result)
	if err != nil {
		return nil, err
	}

	// Build name -> ID map
	labelMap := make(map[string]string)
	for _, l := range result.Data.Team.Labels.Nodes {
		labelMap[strings.ToLower(l.Name)] = l.ID
	}

	ids := make([]string, 0, len(names))
	for _, name := range names {
		if id, ok := labelMap[strings.ToLower(name)]; ok {
			ids = append(ids, id)
		}
		// Skip unknown labels silently
	}

	return ids, nil
}

// Priority conversion helpers

func linearPriorityToString(p int) string {
	switch p {
	case 1:
		return priorityCritical
	case 2:
		return priorityHigh
	case 3:
		return priorityNormal
	case 4:
		return priorityLow
	default:
		return priorityNormal
	}
}

func linearPriorityFromString(s string) int {
	switch strings.ToLower(s) {
	case "critical", "urgent":
		return 1
	case "high":
		return 2
	case "normal", "medium":
		return 3
	case "low":
		return 4
	default:
		return 0 // No priority
	}
}
