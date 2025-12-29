package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider/token"
)

const (
	defaultBaseURL = "https://api.linear.app"
	defaultTimeout = 30 * time.Second
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// Client wraps the Linear API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a new Linear API client
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		token:      token,
	}
}

// ResolveToken finds the Linear token from multiple sources.
// Priority order:
//  1. MEHR_LINEAR_API_KEY env var
//  2. LINEAR_API_KEY env var
//  3. configToken (from config.yaml)
func ResolveToken(configToken string) (string, error) {
	return token.ResolveToken(token.Config("LINEAR", configToken).
		WithEnvVars("LINEAR_API_KEY"))
}

// graphqlRequest represents a GraphQL request
type graphqlRequest struct {
	Variables map[string]any `json:"variables,omitempty"`
	Query     string         `json:"query"`
}

// graphqlResponse represents a GraphQL response
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
	Path    []any  `json:"path,omitempty"`
}

// doGraphQLRequest performs a GraphQL request to the Linear API
func (c *Client) doGraphQLRequest(ctx context.Context, query string, variables map[string]any, result any) error {
	reqBody := graphqlRequest{
		Query:     query,
		Variables: variables,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	reqURL := c.baseURL + "/graphql"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

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
		return wrapAPIError(&httpError{code: resp.StatusCode, message: string(respBody)})
	}

	var graphqlResp graphqlResponse
	if err := json.Unmarshal(respBody, &graphqlResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(graphqlResp.Errors) > 0 {
		return fmt.Errorf("graphql errors: %s", graphqlResp.Errors[0].Message)
	}

	if result != nil {
		if err := json.Unmarshal(graphqlResp.Data, result); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
	}

	return nil
}

// GetIssue fetches an issue by ID
func (c *Client) GetIssue(ctx context.Context, issueID string) (*Issue, error) {
	query := `
		query GetIssue($issueId: String!) {
			issue(id: $issueId) {
				id
				identifier
				title
				description
				state {
					id
					name
					type
				}
				priority
				labels {
					nodes {
						id
						name
						color
					}
				}
				assignee {
					id
					name
					email
				}
				createdAt
				updatedAt
				url
				team {
					key
					name
				}
			}
		}
	`

	var response struct {
		Issue *Issue `json:"issue"`
	}

	variables := map[string]any{
		"issueId": issueID,
	}

	if err := c.doGraphQLRequest(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	if response.Issue == nil {
		return nil, ErrIssueNotFound
	}

	return response.Issue, nil
}

// ListIssues fetches issues from a team with optional filters
func (c *Client) ListIssues(ctx context.Context, teamKey string, filters ListFilters) ([]*Issue, error) {
	query := `
		query ListIssues($teamKey: String!, $first: Int, $after: String, $state: String) {
			team(key: $teamKey) {
				issues(first: $first, after: $after, filter: {state: {name: $state}}) {
					nodes {
						id
						identifier
						title
						description
						state {
							id
							name
							type
						}
						priority
						labels {
							nodes {
								id
								name
								color
							}
						}
						assignee {
							id
							name
							email
						}
						createdAt
						updatedAt
						url
						team {
							key
							name
						}
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
		}
	`

	var allIssues []*Issue
	var after string

	for {
		var response struct {
			Team struct {
				Issues struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []*Issue `json:"nodes"`
				} `json:"issues"`
			} `json:"team"`
		}

		variables := map[string]any{
			"teamKey": teamKey,
			"first":   50,
			"after":   after,
		}

		if filters.State != "" {
			variables["state"] = filters.State
		}

		if err := c.doGraphQLRequest(ctx, query, variables, &response); err != nil {
			if len(allIssues) > 0 {
				break // Return what we have
			}
			return nil, err
		}

		allIssues = append(allIssues, response.Team.Issues.Nodes...)

		if !response.Team.Issues.PageInfo.HasNextPage {
			break
		}

		after = response.Team.Issues.PageInfo.EndCursor
	}

	return allIssues, nil
}

// CreateIssue creates a new issue
func (c *Client) CreateIssue(ctx context.Context, input CreateIssueInput) (*Issue, error) {
	query := `
		mutation CreateIssue($input: IssueCreateInput!) {
			issueCreate(input: $input) {
				issue {
					id
					identifier
					title
					description
					state {
						id
						name
						type
					}
					priority
					labels {
						nodes {
							id
							name
							color
						}
					}
					assignee {
						id
						name
						email
					}
					createdAt
					updatedAt
					url
					team {
						key
						name
					}
				}
				success
			}
		}
	`

	var response struct {
		IssueCreate struct {
			Issue   *Issue `json:"issue"`
			Success bool   `json:"success"`
		} `json:"issueCreate"`
	}

	variables := map[string]any{
		"input": input,
	}

	if err := c.doGraphQLRequest(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	if !response.IssueCreate.Success || response.IssueCreate.Issue == nil {
		return nil, errors.New("failed to create issue")
	}

	return response.IssueCreate.Issue, nil
}

// UpdateIssue updates an existing issue
func (c *Client) UpdateIssue(ctx context.Context, issueID string, input UpdateIssueInput) (*Issue, error) {
	query := `
		mutation UpdateIssue($id: String!, $input: IssueUpdateInput!) {
			issueUpdate(id: $id, input: $input) {
				issue {
					id
					identifier
					title
					description
					state {
						id
						name
						type
					}
					priority
					labels {
						nodes {
							id
							name
							color
						}
					}
					assignee {
						id
						name
						email
					}
					createdAt
					updatedAt
					url
					team {
						key
						name
					}
				}
				success
			}
		}
	`

	var response struct {
		IssueUpdate struct {
			Issue   *Issue `json:"issue"`
			Success bool   `json:"success"`
		} `json:"issueUpdate"`
	}

	variables := map[string]any{
		"id":    issueID,
		"input": input,
	}

	if err := c.doGraphQLRequest(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	if !response.IssueUpdate.Success || response.IssueUpdate.Issue == nil {
		return nil, errors.New("failed to update issue")
	}

	return response.IssueUpdate.Issue, nil
}

// AddComment adds a comment to an issue
func (c *Client) AddComment(ctx context.Context, issueID, body string) (*Comment, error) {
	query := `
		mutation CreateComment($input: CommentCreateInput!) {
			commentCreate(input: $input) {
				success
				comment {
					id
					body
					user {
						id
						name
					}
					createdAt
					updatedAt
				}
			}
		}
	`

	var response struct {
		CommentCreate struct {
			Comment *Comment `json:"comment"`
			Success bool     `json:"success"`
		} `json:"commentCreate"`
	}

	variables := map[string]any{
		"input": map[string]any{
			"issueId": issueID,
			"body":    body,
		},
	}

	if err := c.doGraphQLRequest(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	if !response.CommentCreate.Success || response.CommentCreate.Comment == nil {
		return nil, errors.New("failed to create comment")
	}

	return response.CommentCreate.Comment, nil
}

// GetComments fetches comments for an issue
func (c *Client) GetComments(ctx context.Context, issueID string) ([]*Comment, error) {
	query := `
		query GetComments($issueId: String!) {
			issue(id: $issueId) {
				comments {
					nodes {
						id
						body
						user {
							id
							name
						}
						createdAt
						updatedAt
					}
				}
			}
		}
	`

	var response struct {
		Issue struct {
			Comments struct {
				Nodes []*Comment `json:"nodes"`
			} `json:"comments"`
		} `json:"issue"`
	}

	variables := map[string]any{
		"issueId": issueID,
	}

	if err := c.doGraphQLRequest(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	return response.Issue.Comments.Nodes, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Linear API Types
// ──────────────────────────────────────────────────────────────────────────────

// Issue represents a Linear issue
type Issue struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	State       *State    `json:"state"`
	Assignee    *User     `json:"assignee"`
	Team        *Team     `json:"team"`
	ID          string    `json:"id"`
	Identifier  string    `json:"identifier"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Labels      []*Label  `json:"labels"`
	Priority    int       `json:"priority"`
}

// State represents the state of an issue
type State struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Label represents a label
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// User represents a user
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Team represents a team
type Team struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Comment represents a comment
type Comment struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	User      *User     `json:"user"`
	ID        string    `json:"id"`
	Body      string    `json:"body"`
}

// CreateIssueInput represents the input for creating an issue
type CreateIssueInput struct {
	Priority    *int     `json:"priority,omitempty"`
	TeamID      string   `json:"teamId,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	AssigneeID  string   `json:"assigneeId,omitempty"`
	StateID     string   `json:"stateId,omitempty"`
	LabelIDs    []string `json:"labelIds,omitempty"`
}

// UpdateIssueInput represents the input for updating an issue
type UpdateIssueInput struct {
	Priority *int     `json:"priority,omitempty"`
	StateID  string   `json:"stateId,omitempty"`
	LabelIDs []string `json:"labelIds,omitempty"`
}

// ListFilters represents filters for listing issues
type ListFilters struct {
	State string
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Error wrapper
// ──────────────────────────────────────────────────────────────────────────────

// httpError wraps an HTTP error for proper error handling
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
