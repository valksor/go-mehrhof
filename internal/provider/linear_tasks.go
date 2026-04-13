package provider

import (
	"context"
	"errors"
	"fmt"
)

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
