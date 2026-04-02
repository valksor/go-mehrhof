package provider

import (
	"context"
	"errors"
	"fmt"
)

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
